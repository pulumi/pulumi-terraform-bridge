// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tokens

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptokens "github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/internal/metadatakeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/internal/naming"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/log"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

// applyDefaultFixups installs the bridge's historical default ResourceInfo fixups.
//
// Build-time callers still run the schema-inspecting path and record the
// decisions that are safe to replay. Runtime callers may replay those recorded
// decisions for PF-owned resources so provider startup can avoid calling
// Schema() for every PF resource.
func applyDefaultFixups(p *info.Provider) error {
	fixCtx, err := newFixCtx(p)
	if err != nil {
		return err
	}
	err = walkResources(fixCtx, p, func(r resourceInfo) error {
		// A true result means runtime metadata had a resource entry and replayed
		// everything build time learned for this resource. The entry may be empty:
		// that still proves build time inspected the schema and found no default
		// fixup to apply.
		applied, err := fixCtx.applyPrecomputedResourceFixups(p, r.TFName, r.Schema)
		if err != nil {
			return err
		}
		if applied {
			return nil
		}

		// If no precomputed entry handled the resource, run the eager path. The
		// recorder captures the small set of eager decisions that can be serialized
		// into provider metadata for future runtime replay.
		recorder := &resourceFixupRecorder{}
		r.recorder = recorder
		if p.Name == "" {
			err = fixMissingID(r)
		} else {
			err = errors.Join(
				fixPropertyConflictForResource(p, r),
				fixMissingID(r),
			)
		}
		if err != nil {
			return err
		}

		fixCtx.recordResourceFixups(p, r)
		return nil
	})
	if err != nil {
		return err
	}
	return fixCtx.saveResourceFixups()
}

// newFixCtx builds the phase-aware state used by the resource walk.
//
// Runtime mode is detected from the metadata payload, not MetadataInfo.Path,
// because real providers commonly pass embedded runtime metadata through
// NewProviderMetadata. Build-time mode rewrites the fixup metadata from the
// current schema walk; runtime mode only consumes existing metadata.
func newFixCtx(p *info.Provider) (fixCtx, error) {
	ignoredMappings := make(map[string]struct{}, len(p.IgnoreMappings))
	for _, v := range p.IgnoreMappings {
		ignoredMappings[v] = struct{}{}
	}
	ctx := fixCtx{ignoredMappings: ignoredMappings}
	if p.MetadataInfo == nil || p.MetadataInfo.Data == nil {
		return ctx, nil
	}
	ctx.metadata = p.MetadataInfo.Data
	// The runtime marker is authoritative. Runtime metadata may still be loaded
	// through NewProviderMetadata, so MetadataInfo.Path is not a reliable phase
	// signal.
	runtimeMode, foundRuntimeMarker, err := metadata.Get[bool](
		p.MetadataInfo.Data, metadatakeys.RuntimeMetadata)
	if err != nil {
		return ctx, err
	}
	ctx.runtimeMode = foundRuntimeMarker && runtimeMode
	if !ctx.runtimeMode {
		// Build time starts from an empty set so stale resource entries disappear
		// naturally when schemas or ownership change. If the key existed already,
		// mark the context dirty so saveResourceFixups can also delete the key when
		// this build records no eligible resources.
		ctx.defaultFixups = defaultResourceSchemaFixups{
			Resources: map[string]defaultResourceSchemaFixup{},
		}
		previousFixups, found, err := metadata.Get[defaultResourceSchemaFixups](
			p.MetadataInfo.Data, metadatakeys.DefaultResourceSchemaFixups)
		if err != nil {
			return ctx, err
		}
		if found {
			ctx.previousDefaultFixups = previousFixups
		}
		ctx.defaultChanged = found
		return ctx, nil
	}
	// Runtime never creates or refreshes these records. If the runtime marker or
	// the fixup key is absent, callers fall back to the eager schema-inspecting
	// path for compatibility.
	fixups, found, err := metadata.Get[defaultResourceSchemaFixups](
		p.MetadataInfo.Data, metadatakeys.DefaultResourceSchemaFixups)
	if err != nil {
		return ctx, err
	}
	if fixups.Resources == nil {
		fixups.Resources = map[string]defaultResourceSchemaFixup{}
	}
	ctx.defaultFixups = fixups
	ctx.hasDefaultFixups = found
	return ctx, nil
}

// fixCtx is the shared state for one default-fixup pass. It keeps runtime replay
// and build-time recording behind the same resource walk without letting runtime
// mutate generated metadata.
type fixCtx struct {
	ignoredMappings map[string]struct{}

	// defaultFixups is either the build-time output being assembled or the
	// runtime input being replayed.
	defaultFixups    defaultResourceSchemaFixups
	hasDefaultFixups bool
	// previousDefaultFixups is build-time only. It lets repeated fixup passes
	// preserve serialized ComputeID decisions that were already installed on
	// ResourceInfo before the recorder was attached.
	previousDefaultFixups defaultResourceSchemaFixups
	// defaultChanged is build-time only. It is set when the metadata key needs to
	// be written or removed after the walk.
	defaultChanged bool
	// runtimeMode is true only when the loaded metadata explicitly says it is
	// runtime metadata.
	runtimeMode bool
	// metadata is retained so build-time callers can persist rewritten fixups.
	metadata info.ProviderMetadata
}

// resourceInfo bundles the Pulumi ResourceInfo with the Terraform shim resource
// currently being fixed. recorder is non-nil only while the eager path is
// running and is used to capture replayable ComputeID decisions.
type resourceInfo struct {
	Schema   *info.Resource
	TF       shim.Resource
	TFName   string
	recorder *resourceFixupRecorder
}

// defaultResourceSchemaFixups is generated bridge metadata that records the
// schema-derived default fixup decisions made at build time. A resource entry
// may be empty; that still means build time inspected the schema and found no
// default field rename or ComputeID fixup to replay at runtime.
type defaultResourceSchemaFixups struct {
	Resources map[string]defaultResourceSchemaFixup `json:"resources,omitempty"`
}

// defaultResourceSchemaFixup is the runtime replay payload for one Terraform
// resource token. An all-zero value is still meaningful when it is stored in the
// Resources map: it means build time inspected the resource and had nothing to
// replay, so runtime does not need to inspect the schema again.
type defaultResourceSchemaFixup struct {
	Fields    map[string]defaultSchemaFieldFixup `json:"fields,omitempty"`
	ComputeID *defaultComputeIDFixup             `json:"computeID,omitempty"`
}

// defaultSchemaFieldFixup stores only the ResourceInfo.Fields attributes that
// default property-conflict fixups synthesize. Those fields are consumed by
// mapping, encoding, decoding, and schema metadata before any runtime schema
// load would happen.
type defaultSchemaFieldFixup struct {
	Name string       `json:"name,omitempty"`
	Type ptokens.Type `json:"type,omitempty"`
}

// defaultComputeIDFixup describes a ComputeID closure that can be rebuilt at
// runtime. Function values are not serializable, so the metadata stores only the
// default-fixup kind and, for delegated IDs, the Pulumi field to read.
type defaultComputeIDFixup struct {
	Kind  string `json:"kind"`
	Field string `json:"field,omitempty"`
}

const (
	// These kinds intentionally cover only ComputeID functions synthesized by
	// this file's default fixups.
	defaultComputeIDMissing  = "missing"
	defaultComputeIDDelegate = "delegate"

	pulumiTerraformProviderRepoURL = "https://github.com/pulumi/pulumi-terraform-provider"
)

const (
	defaultFixupPropertyID     = "id"
	defaultFixupPropertyPulumi = "pulumi"
	defaultFixupPropertyURN    = "urn"
)

// precomputedResourceSchemaFixupCandidate identifies resources that may use
// build-time fixup metadata at runtime. This is only an eligibility signal:
// runtime skips schema inspection only when the runtime metadata marker exists
// and the metadata contains an entry for the resource, even if that entry is
// empty because no fixup was needed.
type precomputedResourceSchemaFixupCandidate interface {
	ResourceSchemaFixupsMayBePrecomputed(tfToken string) bool
}

// resourceFixupRecorder is attached while eager fixups run so they can record
// the serializable form of any default ComputeID they install. It avoids
// re-deriving the same decision from a resource schema during runtime startup.
type resourceFixupRecorder struct {
	computeID *defaultComputeIDFixup
}

// recordComputeIDFixup is a no-op outside the eager path used for build-time
// recording.
func (r resourceInfo) recordComputeIDFixup(kind, field string) {
	if r.recorder == nil {
		return
	}
	r.recorder.computeID = &defaultComputeIDFixup{Kind: kind, Field: field}
}

// applyPrecomputedResourceFixups replays build-time fixup metadata for one
// runtime resource. It returns true once a metadata entry handles the resource,
// even when that entry is empty, because the entry itself is the proof that
// build time already inspected the schema.
func (ctx fixCtx) applyPrecomputedResourceFixups(
	p *info.Provider, tfToken string, res *info.Resource,
) (bool, error) {
	// Runtime replay requires all three gates: a runtime metadata payload, a
	// provider/resource that is eligible for precomputed fixups, and a generated
	// fixup section to read from.
	if !ctx.runtimeMode || !ctx.resourceSchemaFixupsMayBePrecomputed(p, tfToken) || !ctx.hasDefaultFixups {
		return false, nil
	}
	fixup, ok := ctx.defaultFixups.Resources[tfToken]
	if !ok {
		return false, nil
	}

	// Fill only fields that the provider has not set explicitly. The replay path
	// should reproduce build-time defaults, not override provider-authored
	// ResourceInfo.
	for key, field := range fixup.Fields {
		schema := getField(&res.Fields, key)
		if schema.Name == "" {
			schema.Name = field.Name
		}
		if schema.Type == "" {
			schema.Type = field.Type
		}
	}

	// Rebuild only the ComputeID functions produced by default fixups. Any other
	// provider-authored ComputeID must come from normal ResourceInfo.
	if fixup.ComputeID != nil {
		switch fixup.ComputeID.Kind {
		case defaultComputeIDMissing:
			if res.ComputeID == nil {
				res.ComputeID = missingIDComputeID()
			}
		case defaultComputeIDDelegate:
			if fixup.ComputeID.Field == "" {
				return false, fmt.Errorf("precomputed default fixup for %s has delegate ComputeID without a field", tfToken)
			}
			if res.ComputeID == nil {
				res.ComputeID = delegateIDField(resource.PropertyKey(fixup.ComputeID.Field), p.Name,
					pulumiTerraformProviderRepoURL)
			}
		default:
			return false, fmt.Errorf("precomputed default fixup for %s has unknown ComputeID kind %q",
				tfToken, fixup.ComputeID.Kind)
		}
	}
	return true, nil
}

// resourceSchemaFixupsMayBePrecomputed asks the provider whether this Terraform
// token is a valid candidate for metadata replay. It is only an eligibility
// signal; runtime still requires a concrete metadata entry before skipping
// schema inspection.
func (ctx fixCtx) resourceSchemaFixupsMayBePrecomputed(p *info.Provider, tfToken string) bool {
	candidate, ok := p.P.(precomputedResourceSchemaFixupCandidate)
	return ok && candidate.ResourceSchemaFixupsMayBePrecomputed(tfToken)
}

// recordResourceFixups writes one build-time resource entry after eager fixups
// have run. Empty entries are kept deliberately so runtime can distinguish
// "schema inspected, no fixup needed" from "no build-time proof, use eager
// fallback."
func (ctx *fixCtx) recordResourceFixups(p *info.Provider, r resourceInfo) {
	if ctx.runtimeMode || ctx.metadata == nil || !ctx.resourceSchemaFixupsMayBePrecomputed(p, r.TFName) {
		return
	}

	if ctx.defaultFixups.Resources == nil {
		ctx.defaultFixups.Resources = map[string]defaultResourceSchemaFixup{}
	}
	fixup := captureResourceFixup(r)
	if fixup.ComputeID == nil && r.Schema.ComputeID != nil {
		// A build may call ComputeTokens more than once on the same ProviderInfo.
		// The first pass installs the default ComputeID; later passes see it
		// already present and therefore do not call recordComputeIDFixup again.
		// Reuse the previous metadata for this current resource so a second pass
		// does not rewrite a replayable ComputeID into an empty entry.
		previous := ctx.previousDefaultFixups.Resources[r.TFName]
		if previous.ComputeID != nil {
			computeID := *previous.ComputeID
			fixup.ComputeID = &computeID
		}
	}
	ctx.defaultFixups.Resources[r.TFName] = fixup
	ctx.defaultChanged = true
}

// saveResourceFixups persists the build-time records assembled by the walk.
// Passing nil clears a stale metadata key when the current build has no
// precomputable resources left.
func (ctx fixCtx) saveResourceFixups() error {
	if !ctx.defaultChanged {
		return nil
	}
	if len(ctx.defaultFixups.Resources) == 0 {
		return metadata.Set(ctx.metadata, metadatakeys.DefaultResourceSchemaFixups, nil)
	}
	return metadata.Set(ctx.metadata, metadatakeys.DefaultResourceSchemaFixups, ctx.defaultFixups)
}

// captureResourceFixup extracts the narrow set of eager default-fixup decisions
// that runtime needs before any schema load: synthesized field metadata for
// reserved Terraform property names, plus the default ComputeID decision.
func captureResourceFixup(r resourceInfo) defaultResourceSchemaFixup {
	var fixup defaultResourceSchemaFixup
	for _, propertyFixup := range defaultPropertyFixups {
		key := propertyFixup.key
		field := captureSchemaFieldFixup(r.Schema.Fields[key])
		if field == (defaultSchemaFieldFixup{}) {
			continue
		}
		if fixup.Fields == nil {
			fixup.Fields = map[string]defaultSchemaFieldFixup{}
		}
		fixup.Fields[key] = field
	}

	if r.recorder != nil && r.recorder.computeID != nil {
		computeID := *r.recorder.computeID
		fixup.ComputeID = &computeID
	}
	return fixup
}

// captureSchemaFieldFixup copies only field metadata that the default fixups may
// synthesize and that runtime replay can safely restore.
func captureSchemaFieldFixup(field *info.Schema) defaultSchemaFieldFixup {
	if field == nil {
		return defaultSchemaFieldFixup{}
	}
	return defaultSchemaFieldFixup{Name: field.Name, Type: field.Type}
}

// fixMissingID preserves the historical bridge default for Terraform resources
// without a usable string ID property. When it installs a default ComputeID, the
// recorder notes the serializable kind for build-time metadata.
func fixMissingID(r resourceInfo) error {
	getIDType := func(r *info.Resource) ptokens.Type {
		s := r.Fields["id"]
		if s == nil {
			return ""
		}
		return s.Type
	}
	id, hasID := r.TF.Schema().GetOk("id")
	if !hasID {
		if r.Schema.ComputeID == nil {
			r.Schema.ComputeID = missingIDComputeID()
			r.recordComputeIDFixup(defaultComputeIDMissing, "")
		}
		return nil
	}

	ok := (id.Type() == shim.TypeString || getIDType(r.Schema) == "string") && id.Computed()
	if !ok && r.Schema.ComputeID == nil {
		r.Schema.ComputeID = missingIDComputeID()
		r.recordComputeIDFixup(defaultComputeIDMissing, "")
	}
	return nil
}

func fixPropertyConflictForResource(p *info.Provider, r resourceInfo) error {
	var retError []error
	r.TF.Schema().Range(func(key string, _ shim.Schema) bool {
		if fix := badPropertyName(p.Name, p.GetResourcePrefix(), key); fix != nil {
			err := fix(r)
			if err != nil {
				retError = append(retError, fmt.Errorf("%s: %w", key, err))
			}
		}
		return true
	})
	return errors.Join(retError...)
}

func getField(i *map[string]*info.Schema, name string) *info.Schema {
	contract.Assertf(i != nil, "i cannot be nil")
	if *i == nil {
		*i = map[string]*info.Schema{}
	}
	v, ok := (*i)[name]
	if !ok {
		v = new(info.Schema)
		(*i)[name] = v
	}
	return v
}

type fixupProperty func(resourceInfo) error

type defaultPropertyFixupFactory func(providerName, tokenPrefix string) fixupProperty

var defaultPropertyFixups = [...]struct {
	key     string
	factory defaultPropertyFixupFactory
}{
	{
		key: defaultFixupPropertyID,
		factory: func(providerName, tokenPrefix string) fixupProperty {
			return fixID(providerName, tokenPrefix)
		},
	},
	{
		key: defaultFixupPropertyPulumi,
		factory: func(_, _ string) fixupProperty {
			return fixPulumi()
		},
	},
	{
		key: defaultFixupPropertyURN,
		factory: func(providerName, _ string) fixupProperty {
			return fixURN(providerName)
		},
	},
}

func badPropertyName(providerName, tokenPrefix, key string) fixupProperty {
	for _, propertyFixup := range defaultPropertyFixups {
		if propertyFixup.key == key {
			return propertyFixup.factory(providerName, tokenPrefix)
		}
	}
	return nil
}

func fixID(providerName, tokenPrefix string) fixupProperty {
	getResourceName := func(tk string) string {
		name := strings.TrimPrefix(tk, tokenPrefix)
		if name == "" {
			name = tk
		}
		return strings.TrimLeft(name, "_")
	}
	return func(r resourceInfo) error {
		tfSchema := r.TF.Schema()
		tfIDProperty, ok := tfSchema.GetOk("id")
		if !ok {
			return nil
		}

		if f := r.Schema.Fields["id"]; f != nil && f.Name != "" {
			return nil
		}

		if tfIDProperty.Computed() && !tfIDProperty.Optional() && !tfIDProperty.Required() {
			// A computed-only string "id" already occupies Pulumi's resource ID slot.
			if tfIDProperty.Type() == shim.TypeString {
				return nil
			}

			// Preserve explicit provider decisions for computed-only IDs.
			// Providers that already overrode the Pulumi view of "id" should not
			// get a second renamed output property synthesized on top.
			if f := r.Schema.Fields["id"]; f != nil && f.Type == "string" {
				return nil
			}
			if r.Schema.ComputeID != nil && computedIDTypeIsCoercibleToString(tfIDProperty.Type()) {
				getField(&r.Schema.Fields, "id").Type = "string"
				return nil
			}
		}

		candidateNames := []string{
			getResourceName(r.TFName) + "_id",
			strings.ReplaceAll(providerName, "-", "_") + "_" + getResourceName(r.TFName) + "_id",
			"resource_id",
			strings.ReplaceAll(providerName, "-", "_") + "_id",
		}

		for _, proposedIDFieldName := range candidateNames {
			if _, ok := tfSchema.GetOk(proposedIDFieldName); ok {
				continue
			}

			newIDField := naming.TerraformToPulumiNameV2(proposedIDFieldName, tfSchema, r.Schema.Fields)
			getField(&r.Schema.Fields, "id").Name = newIDField

			if (tfIDProperty.Required() || (tfIDProperty.Optional() && tfIDProperty.Computed())) &&
				r.Schema.ComputeID == nil {
				r.Schema.ComputeID = delegateIDField(resource.PropertyKey(newIDField), providerName,
					pulumiTerraformProviderRepoURL)
				// The delegate closure cannot be serialized, so record the field
				// name needed to rebuild it during runtime replay.
				r.recordComputeIDFixup(defaultComputeIDDelegate, newIDField)
			}

			return nil
		}

		return fmt.Errorf("no available new name, tried %#v", candidateNames)
	}
}

func computedIDTypeIsCoercibleToString(t shim.ValueType) bool {
	switch t {
	case shim.TypeBool, shim.TypeInt, shim.TypeFloat:
		return true
	default:
		return false
	}
}

func fixURN(providerName string) fixupProperty {
	return func(r resourceInfo) error {
		s := r.TF.Schema()
		v := providerName + "_urn"
		if _, ok := s.GetOk(v); ok {
			return fmt.Errorf("no available new name, tried %q", v)
		}
		if f := getField(&r.Schema.Fields, "urn"); f.Name == "" {
			f.Name = naming.TerraformToPulumiNameV2(v, s, r.Schema.Fields)
		}
		return nil
	}
}

func fixPulumi() fixupProperty {
	return func(resource resourceInfo) error {
		schemaMap := resource.TF.Schema()
		if _, ok := schemaMap.GetOk("pulumi_info"); ok {
			return fmt.Errorf("no available new name for the 'pulumi' string, tried %q", "pulumi_info")
		}
		if schemaInfo := getField(&resource.Schema.Fields, "pulumi"); schemaInfo.Name == "" {
			schemaInfo.Name = naming.TerraformToPulumiNameV2("pulumi_info", schemaMap, resource.Schema.Fields)
		}
		return nil
	}
}

// fixProviderResource renames any resource that would otherwise be called
// `Provider`, since that conflicts with the package's actual `Provider`
// resource.
func fixProviderResource(p *info.Provider, skipExisting bool) error {
	if p.Name == "" {
		return nil
	}

	tfToken := p.GetResourcePrefix() + "_provider"
	if skipExisting {
		return nil
	}

	_, ok := p.P.ResourcesMap().GetOk(tfToken)
	if !ok {
		return nil
	}

	if p.Resources == nil {
		return nil
	}
	res := p.Resources[tfToken]
	if res == nil || res.Tok == "" {
		return nil
	}

	conflicting := &info.Resource{}
	if err := SingleModule(
		p.GetResourcePrefix(), "index", MakeStandard(p.Name),
	).Resource(tfToken, conflicting); err != nil {
		return err
	}
	if conflicting.Tok == "" || res.Tok != conflicting.Tok {
		return nil
	}

	candidateTokens := []string{
		p.GetResourcePrefix() + "_" + p.Name + "_provider",
		p.GetResourcePrefix() + "_provider_resource",
		p.GetResourcePrefix() + "_" + p.Name + "_provider_resource",
	}
	for _, candidate := range candidateTokens {
		if _, exists := p.P.ResourcesMap().GetOk(candidate); exists {
			continue
		}

		rewritten := &info.Resource{}
		if err := SingleModule(
			p.GetResourcePrefix(), "index", MakeStandard(p.Name),
		).Resource(candidate, rewritten); err != nil {
			return err
		}
		res.Tok = rewritten.Tok
		return nil
	}

	return fmt.Errorf("no available provider resource rename, tried %#v", candidateTokens)
}

func walkResources(fixCtx fixCtx, p *info.Provider, f func(resourceInfo) error) error {
	var errs []error

	for key, tf := range p.P.ResourcesMap().Range {
		if _, ok := fixCtx.ignoredMappings[key]; ok {
			continue
		}
		if tf == nil {
			continue
		}

		res, isPresent := p.Resources[key]
		shouldStore := !isPresent || res == nil
		if shouldStore {
			res = &info.Resource{}
		}
		if err := f(resourceInfo{
			Schema: res,
			TF:     tf,
			TFName: key,
		}); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}

		if shouldStore && !reflect.ValueOf(*res).IsZero() {
			if p.Resources == nil {
				p.Resources = map[string]*info.Resource{}
			}
			p.Resources[key] = res
		}
	}

	return errors.Join(errs...)
}

func missingIDComputeID() info.ComputeID {
	return func(ctx context.Context, state resource.PropertyMap) (resource.ID, error) {
		return resource.ID("missing ID"), nil
	}
}

func delegateIDField(field resource.PropertyKey, providerName, repoURL string) info.ComputeID {
	return delegateIDProperty(resource.PropertyPath{string(field)}, providerName, repoURL)
}

func delegateIDProperty(prop resource.PropertyPath, providerName, repoURL string) info.ComputeID {
	return func(ctx context.Context, state resource.PropertyMap) (resource.ID, error) {
		err := func(msg string, a ...any) error {
			return delegateIDPropertyError{
				msg:          fmt.Sprintf(msg, a...),
				providerName: providerName,
				repoURL:      repoURL,
			}
		}
		fieldValue, ok := prop.Get(resource.NewProperty(state))
		if !ok {
			return "", err("Could not find required property '%s' in state", prop)
		}

		contract.Assertf(
			!fieldValue.IsComputed() && (!fieldValue.IsOutput() || fieldValue.OutputValue().Known),
			"ComputeID is only called during when preview=false, so we should never need to "+
				"deal with computed properties",
		)

		if fieldValue.IsSecret() || (fieldValue.IsOutput() && fieldValue.OutputValue().Secret) {
			msg := fmt.Sprintf("Setting non-secret resource ID as '%s' (which is secret)", prop)
			logger := log.TryGetLogger(ctx)
			if logger != nil {
				logger.Warn(msg)
			}
			if fieldValue.IsSecret() {
				fieldValue = fieldValue.SecretValue().Element
			} else {
				fieldValue = fieldValue.OutputValue().Element
			}
		}

		if !fieldValue.IsString() {
			return "", err("Expected '%s' property to be a string, found %s",
				prop, fieldValue.TypeString())
		}

		return resource.ID(fieldValue.StringValue()), nil
	}
}

type delegateIDPropertyError struct {
	msg                   string
	providerName, repoURL string
}

func (err delegateIDPropertyError) Error() string {
	return fmt.Sprintf("%s. This is an error in %s resource provider, please report at %s",
		err.msg, err.providerName, err.repoURL)
}
