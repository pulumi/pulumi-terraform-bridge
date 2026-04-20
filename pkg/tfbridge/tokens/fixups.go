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
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/internal/naming"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/log"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func applyDefaultFixups(p *info.Provider) error {
	fixCtx := newFixCtx(p)
	if p.Name == "" {
		return fixMissingIDs(fixCtx, p)
	}
	return errors.Join(
		fixPropertyConflict(fixCtx, p),
		fixMissingIDs(fixCtx, p),
	)
}

func newFixCtx(p *info.Provider) fixCtx {
	ignoredMappings := make(map[string]struct{}, len(p.IgnoreMappings))
	for _, v := range p.IgnoreMappings {
		ignoredMappings[v] = struct{}{}
	}
	return fixCtx{ignoredMappings}
}

type fixCtx struct {
	ignoredMappings map[string]struct{}
}

type resourceInfo struct {
	Schema *info.Resource
	TF     shim.Resource
	TFName string
}

func fixMissingIDs(fixCtx fixCtx, p *info.Provider) error {
	getIDType := func(r *info.Resource) ptokens.Type {
		s := r.Fields["id"]
		if s == nil {
			return ""
		}
		return s.Type
	}
	return walkResources(fixCtx, p, func(r resourceInfo) error {
		id, hasID := r.TF.Schema().GetOk("id")
		if !hasID {
			if r.Schema.ComputeID == nil {
				r.Schema.ComputeID = missingIDComputeID()
			}
			return nil
		}

		if id.Computed() && !id.Optional() && !id.Required() {
			// A computed-only top-level ID is either valid as-is, auto-coerced by
			// fixID above, or should fail validation if its shape is unsupported.
			return nil
		}

		ok := (id.Type() == shim.TypeString || getIDType(r.Schema) == "string") && id.Computed()
		if !ok && r.Schema.ComputeID == nil {
			r.Schema.ComputeID = missingIDComputeID()
		}
		return nil
	})
}

func fixPropertyConflict(fixCtx fixCtx, p *info.Provider) error {
	return walkResources(fixCtx, p, func(r resourceInfo) error {
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
	})
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

func badPropertyName(providerName, tokenPrefix, key string) fixupProperty {
	switch key {
	case "urn":
		return fixURN(providerName)
	case "id":
		return fixID(providerName, tokenPrefix)
	case "pulumi":
		return fixPulumi()
	default:
		return nil
	}
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

		// A computed-only top-level "id" is already the Pulumi resource ID slot.
		// For these resources, fixups should not expose a second renamed output
		// property. If the upstream type is non-string, teach the schema to treat
		// it as a string instead so validation matches runtime extraction.
		if tfIDProperty.Computed() && !tfIDProperty.Optional() && !tfIDProperty.Required() {
			if computedIDTypeIsCoercibleToString(tfIDProperty.Type()) {
				getField(&r.Schema.Fields, "id").Type = "string"
			}
			return nil
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
					"https://github.com/pulumi/pulumi-terraform-provider")
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
