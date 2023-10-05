// Copyright 2016-2023, Pulumi Corporation.
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

package tfbridge

import (
	"github.com/Masterminds/semver"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type tokenHistory[T ~string] struct {
	Current T          `json:"current"`        // the current Pulumi token for the resource
	Past    []alias[T] `json:"past,omitempty"` // Previous tokens

	MajorVersion int                      `json:"majorVersion,omitempty"`
	Fields       map[string]*fieldHistory `json:"fields,omitempty"`
}

type alias[T ~string] struct {
	Name         T    `json:"name"`         // The previous token.
	InCodegen    bool `json:"inCodegen"`    // If the alias is a fully generated resource, or just a schema alias.
	MajorVersion int  `json:"majorVersion"` // The provider's major version when Name was introduced.
}

type aliasHistory struct {
	Resources   map[string]*tokenHistory[tokens.Type]         `json:"resources,omitempty"`
	DataSources map[string]*tokenHistory[tokens.ModuleMember] `json:"datasources,omitempty"`
}

type fieldHistory struct {
	MaxItemsOne *bool `json:"maxItemsOne,omitempty"`

	Fields map[string]*fieldHistory `json:"fields,omitempty"`
	Elem   *fieldHistory            `json:"elem,omitempty"`
}

// Automatically applies backwards compatibility best practices.
//
// Specifically, [ApplyAutoAliases] may perform the following actions:
//
// - Call [ProviderInfo.RenameResourceWithAlias] or [ProviderInfo.RenameDataSource]
// - Edit [ResourceInfo.Aliases]
// - Edit [SchemaInfo.MaxItemsOne]
//
// The goal is to always maximize backwards compatibility and reduce breaking changes for
// the users of the Pulumi providers.
//
// Resource aliases help mask TF provider resource renames or changes in mapped tokens so
// older programs continue to work.  See [ResourceInfo.RenameResourceWithAlias] and
// [ResourceInfo.Aliases] for more details.
//
// [SchemaInfo.MaxItemsOne] changes are also important because they involve flattening and
// pluralizing names. Collections (lists or sets) marked with MaxItems=1 are projected as
// scalar types in Pulumi SDKs. Therefore changes to the MaxItems property may be breaking
// the compilation of programs as the type changes from `T to List[T]` or vice versa. To
// avoid these breaking changes, this method undoes any upstream changes to MaxItems using
// [SchemaInfo.MaxItemsOne] overrides. This happens until a major version change is
// detected, and then overrides are cleared. Effectively this makes sure that upstream
// MaxItems changes are deferred until the next major version.
//
// Implementation note: to operate correctly this method needs to keep a persistent track
// of a database of past decision history. This is currently done by doing reads and
// writes to `providerInfo.GetMetadata()`, which is assumed to be persisted across
// provider releases. The bridge framework keeps this information written out to an opaque
// `bridge-metadata.json` blob which is expected to be stored in source control to persist
// across releases.
//
// Panics if [ProviderInfo.ApplyAutoAliases] would return an error.
func (info *ProviderInfo) MustApplyAutoAliases() {
	err := info.ApplyAutoAliases()
	contract.AssertNoErrorf(err, "Failed to apply aliases")
}

// Automatically applies backwards compatibility best practices.
//
// The goal is to prevent breaking changes from Pulumi maintainers or from the upstream
// provider from causing breaking changes in minor version bumps. We do this by deferring
// certain types of breaking changes to major versions.
//
// ApplyAutoAliases attempts to mitigate 3 types of unwanted breaking changes:
//
// - The token mapping for a resource has changed. For example:
//
//	The maintainer is correcting a typo in a manual resource mapping.
//
// - The token mapping for a resource has changed, and a major update caused us to remove
// the old name from the schema.
//
// - The upstream provider has added or removed `MaxItems: 1` from a field.
//
// [ApplyAutoAliases] applies three mitigation strategies: one for each breaking change it
// is attempting to mitigate.
//
// - Call [ProviderInfo.RenameResourceWithAlias] or [ProviderInfo.RenameDataSource]: This
// creates a "hard alias", preventing the user from experiencing a breaking change between
// major versions.
//
// - Edit [ResourceInfo.Aliases]: This creates a "soft alias", making it easier for users
// to move to the new resource when the old resource is removed.
//
// - Edit [SchemaInfo.MaxItemsOne]: This allows us to defer MaxItemsOne changes to the
// next major release.
//
// All mitigations act on [ProviderInfo.Resources] / [ProviderInfo.DataSources]. These
// mitigations are then propagated to the schema (if during tfgen) or used at runtime (at
// runtime). Conceptually, [ApplyAutoAliases] performs the same kind of mitigations that a
// careful provider author would perform manually: invoking
// [ProviderInfo.RenameResourceWithAlias], adding token aliases for resources that have
// been moved, and fixing MaxItemsOne to avoid backwards compatibility breaks.
//
// The goal is to always maximize backwards compatibility and reduce breaking changes for
// the users of the Pulumi providers. The basic functionality behind each action is
// identical; ApplyAutoAliases keeps a record of which TF token maps to which Pulumi
// token, which fields have MaxItemsOne (true or false), and what version the record is
// from.
//
// This records is stored using the
// github.com/pulumi/pulumi-terraform-bridge/unstable/metadata interface. It is written to
// at tfgen time and read from when starting up a provider (tfgen time and normal runtime).
//
// For example, this is the (abbreviated & modified) history for GCP's compute autoscalar:
//
//	"google_compute_autoscaler": {
//	    "current": "gcp:compute/autoscalar:Autoscalar",
//	    "past": [
//	        {
//	            "name": "gcp:auto/autoscalar:Autoscalar",
//	            "inCodegen": true,
//	            "majorVersion": 6
//	        },
//	        {
//	            "name": "gcp:auto/scaler:Scaler",
//	            "inCodegen": false,
//	            "majorVersion": 5
//	        }
//	    ],
//	    "majorVersion": 6,
//	    "fields": {
//	        "autoscaling_policy": {
//	            "maxItemsOne": true,
//	            "elem": {
//	                "fields": {
//	                    "cpu_utilization": {
//	                        "maxItemsOne": false
//	                    }
//	                }
//	            }
//	        }
//	    }
//	}
//
// I will address each action as it applies to `"google_compute_autoscaler" in turn:
//
// # Call [ProviderInfo.RenameResourceWithAlias] or [ProviderInfo.RenameDataSource]
//
// "google_compute_autoscaler.majorVersion" tells us that this record was last updated at
// major version 6. One of the previous names is also at major version 6, so we want to
// keep full backwards compatibility.
//
// ApplyAutoAliases will call [ProviderInfo.RenameResourceWithAlias] to create a SDK entry
// for the old Pulumi token ("gcp:auto/autoscalar:Autoscalar"). This token will no longer
// be created when `make tfgen` is run on version 7.
//
// # Edit [ResourceInfo.Aliases]
//
// In this history, we have recorded two prior names for "google_compute_autoscaler":
// "gcp:auto/autoscalar:Autoscalar" and "gcp:auto/scaler:Scaler". Since
// "gcp:auto/scaler:Scaler" was from a previous major version, we don't need to maintain
// full backwards compatibility. Instead, we will apply a type alias to
// `.Resources["gcp:compute/autoscalar:Autoscalar"].Aliases`: `AliasInfo{Type:
// ""gcp:auto/scaler:Scaler"}`. This makes it easy for consumers to upgrade from the old
// name to the new.
//
// # Edit [SchemaInfo.MaxItemsOne]
//
// The provider has been shipped with fields that could have `MaxItemsOne` applied. Any
// change here is breaking to our users, so we prevent it. As long as the provider's major
// version is 6, ApplyAutoAliases will override the MaxItemsOne status of
// "autoscaling_policy" (to true) and "autoscaling_policy.elem.cpu_utilization" (to
// false), regardless of what upstream does. We will read new MaxItemsOne values from the
// provider when the next major version (v7 in this example) is released. Effectively this
// makes sure that upstream MaxItems changes are deferred until the next major version.
//
// ---
//
// Implementation note: to operate correctly this method needs to keep a persistent track
// of a database of past decision history. This is currently done by doing reads and
// writes to `providerInfo.GetMetadata()`, which is assumed to be persisted across
// provider releases. The bridge framework keeps this information written out to an opaque
// `bridge-metadata.json` blob which is expected to be stored in source control to persist
// across releas
func (info *ProviderInfo) ApplyAutoAliases() error {
	artifact := info.GetMetadata()
	hist, err := getHistory(artifact)
	if err != nil {
		return err
	}

	var currentVersion int
	// If version is missing, we assume the current version is the most recent major
	// version in mentioned in history.
	if info.Version != "" {
		v, err := semver.NewVersion(info.Version)
		if err != nil {
			return err
		}
		currentVersion = int(v.Major())
	} else {
		for _, r := range hist.Resources {
			for _, p := range r.Past {
				if p.MajorVersion > currentVersion {
					currentVersion = p.MajorVersion
				}
			}
		}
		for _, d := range hist.DataSources {
			for _, p := range d.Past {
				if p.MajorVersion > currentVersion {
					currentVersion = p.MajorVersion
				}
			}
		}
	}

	rMap := info.P.ResourcesMap()
	dMap := info.P.DataSourcesMap()

	// Applying resource aliases adds new resources to providerInfo.Resources. To keep
	// this process deterministic, we don't apply resource aliases until all resources
	// have been examined.
	//
	// The same logic applies to datasources.
	applyAliases := []func(){}

	for tfToken, computed := range info.Resources {
		r, _ := rMap.GetOk(tfToken)
		aliasResource(info, r, &applyAliases, hist.Resources,
			computed, tfToken, currentVersion)
	}

	for tfToken, computed := range info.DataSources {
		ds, _ := dMap.GetOk(tfToken)
		aliasDataSource(info, ds, &applyAliases, hist.DataSources,
			computed, tfToken, currentVersion)
	}

	for _, f := range applyAliases {
		f()
	}

	if err := md.Set(artifact, aliasMetadataKey, hist); err != nil {
		// Set fails only when `hist` is not serializable. Because `hist` is
		// composed of marshallable, non-cyclic types, this is impossible.
		contract.AssertNoErrorf(err, "History failed to serialize")
	}

	return nil
}

const aliasMetadataKey = "auto-aliasing"

func getHistory(artifact ProviderMetadata) (aliasHistory, error) {
	hist, ok, err := md.Get[aliasHistory](artifact, aliasMetadataKey)
	if err != nil {
		return aliasHistory{}, err
	}
	if !ok {
		hist = aliasHistory{
			Resources:   map[string]*tokenHistory[tokens.Type]{},
			DataSources: map[string]*tokenHistory[tokens.ModuleMember]{},
		}
	}
	return hist, nil
}

func aliasResource(
	p *ProviderInfo, res shim.Resource,
	applyResourceAliases *[]func(),
	hist map[string]*tokenHistory[tokens.Type], computed *ResourceInfo,
	tfToken string, version int,
) {
	prev, hasPrev := hist[tfToken]
	if !hasPrev {
		// It's not in the history, so it must be new. Stick it in the history for
		// next time.
		hist[tfToken] = &tokenHistory[tokens.Type]{
			Current: computed.Tok,
		}
	} else {
		// We don't do this eagerly because aliasResource is called while
		// iterating over p.Resources which aliasOrRenameResource mutates.
		*applyResourceAliases = append(*applyResourceAliases,
			func() { aliasOrRenameResource(p, computed, tfToken, prev, version) })
	}

	// Apply Aliasing to MaxItemOne by traversing the field tree and applying the
	// stored value.
	//
	// Note: If the user explicitly sets a MaxItemOne value, that value is respected
	// and overwrites the current history.'

	if res == nil {
		return
	}

	// If we are behind the major version, reset the fields and the major version.
	if hist[tfToken].MajorVersion < version {
		hist[tfToken].MajorVersion = version
		hist[tfToken].Fields = nil
	}

	applyResourceMaxItemsOneAliasing(res, &hist[tfToken].Fields, &computed.Fields)
}

// applyResourceMaxItemsOneAliasing traverses a shim.Resource, applying walk to each field in the resource.
func applyResourceMaxItemsOneAliasing(
	r shim.Resource, hist *map[string]*fieldHistory, info *map[string]*SchemaInfo,
) (bool, bool) {
	if r == nil {
		return hist != nil, info != nil
	}
	m := r.Schema()
	if m == nil {
		return hist != nil, info != nil
	}

	var rHasH, rHasI bool

	m.Range(func(k string, v shim.Schema) bool {
		h, hasH := getNonNil(hist, k)
		i, hasI := getNonNil(info, k)
		fieldHasHist, fieldHasInfo := applyMaxItemsOneAliasing(v, h, i)

		hasH = hasH || fieldHasHist
		hasI = hasI || fieldHasInfo

		if !hasH {
			delete(*hist, k)
		}
		if !hasI {
			delete(*info, k)
		}

		rHasH = rHasH || hasH
		rHasI = rHasI || hasI

		return true
	})

	return rHasH, rHasI
}

// When walking the schema tree for a resource, we create mirroring trees in
// *fieldHistory and *SchemaInfo. To avoid polluting either tree (and
// interfering with other actions such as SetAutonaming), we clean up the paths
// that we created but did not store any information into.
//
// For example, consider the schema for a field of type `Object{ Key1:
// List[String] }`.  The schema tree for this field looks like this:
//
//	Object:
//	  Fields:
//	    Key1:
//	      List:
//	        Elem:
//	          String
//
// When we walk the tree, we create an almost identical history tree:
//
//	Object:
//	  Fields:
//	    Key1:
//	      List:
//	        MaxItemsOne: false
//	        Elem:
//	          String
//
// We stored the additional piece of information `MaxItemsOne: false`. We need to
// keep enough of the tree to maintain that information, but no more. We can
// discard the unnecessary `Elem: String`.
//
// This keeps the tree as clean as possible for other processes which expect a
// `nil` element when making changes. Since other processes (like SetAutonaming)
// act on edge nodes (like our String), this allows us to inter-operate with them
// without interference.
//
// applyMaxItemsOneAliasing traverses a generic shim.Schema recursively, applying fieldHistory to
// SchemaInfo and vise versa as necessary to avoid breaking changes in the
// resulting sdk.
func applyMaxItemsOneAliasing(schema shim.Schema, h *fieldHistory, info *SchemaInfo) (hasH bool, hasI bool) {
	//revive:disable-next-line:empty-block
	if schema == nil || (schema.Type() != shim.TypeList && schema.Type() != shim.TypeSet) {
		// MaxItemsOne does not apply, so do nothing
	} else if info.MaxItemsOne != nil {
		// The user has overwritten the value, so we will just record that.
		h.MaxItemsOne = info.MaxItemsOne
		hasH = true
	} else if h.MaxItemsOne != nil {
		// If we have a previous value in the history, we keep it as is.
		info.MaxItemsOne = h.MaxItemsOne
		hasI = true
	} else {
		// There is no history for this value, so we bake it into the
		// alias history.
		h.MaxItemsOne = BoolRef(IsMaxItemsOne(schema, info))
		hasH = true
	}

	// Ensure that the h.Elem and info.Elem fields are non-nil so they can be
	// safely recursed on.
	//
	// If the .Elem existed before this function, we mark it as unsafe to cleanup.
	var hasElemH, hasElemI bool
	populateElem := func() {
		if h.Elem == nil {
			h.Elem = &fieldHistory{}
		} else {
			hasElemH = true
		}
		if info.Elem == nil {
			info.Elem = &SchemaInfo{}
		} else {
			hasElemI = true
		}
	}
	// Cleanup after we have walked a .Elem value.
	//
	// If the .Elem field was created in populateElem and the field was not
	// changed, we then delete the field.
	cleanupElem := func(elemHist, elemInfo bool) {
		hasElemH = hasElemH || elemHist
		hasElemI = hasElemI || elemInfo
		if !hasElemH {
			h.Elem = nil
		}
		if !hasElemI {
			info.Elem = nil
		}
	}

	e := schema.Elem()
	switch e := e.(type) {
	case shim.Resource:
		populateElem()
		eHasH, eHasI := applyResourceMaxItemsOneAliasing(e, &h.Elem.Fields, &info.Elem.Fields)
		cleanupElem(eHasH, eHasI)
	case shim.Schema:
		populateElem()
		eHasH, eHasI := applyMaxItemsOneAliasing(e, h.Elem, info.Elem)
		cleanupElem(eHasH, eHasI)
	}

	return hasH || hasElemH, hasI || hasElemI
}

func getNonNil[K comparable, V any](m *map[K]*V, key K) (_ *V, alreadyThere bool) {
	contract.Assertf(m != nil, "Cannot restore map if ptr is nil")
	if *m == nil {
		*m = map[K]*V{}
	}
	v := (*m)[key]

	if v == nil {
		var new V
		v = &new
		(*m)[key] = v
	} else {
		alreadyThere = true
	}
	return v, alreadyThere
}

func aliasOrRenameResource(
	p *ProviderInfo,
	res *ResourceInfo, tfToken string,
	hist *tokenHistory[tokens.Type], currentVersion int,
) {
	var alreadyPresent bool
	for _, a := range hist.Past {
		if a.Name == hist.Current {
			alreadyPresent = true
			break
		}
	}
	if !alreadyPresent && res.Tok != hist.Current {
		// The resource is in history, but the name has changed. Update the new current name
		// and add the old name to the history.
		hist.Past = append(hist.Past, alias[tokens.Type]{
			Name:         hist.Current,
			InCodegen:    true,
			MajorVersion: currentVersion,
		})
		hist.Current = res.Tok
	}
	for _, a := range hist.Past {
		legacy := a.Name
		// Only respect hard aliases introduced in the same major version
		if a.InCodegen && a.MajorVersion == currentVersion {
			p.RenameResourceWithAlias(tfToken, legacy,
				res.Tok, legacy.Module().Name().String(),
				res.Tok.Module().Name().String(), res)
		} else {
			res.Aliases = append(res.Aliases,
				AliasInfo{Type: (*string)(&legacy)})
		}
	}

}

func aliasDataSource(
	p *ProviderInfo,
	ds shim.Resource,
	queue *[]func(),
	hist map[string]*tokenHistory[tokens.ModuleMember],
	computed *DataSourceInfo,
	tfToken string,
	version int,
) {
	prev, hasPrev := hist[tfToken]
	if !hasPrev {
		// It's not in the history, so it must be new. Stick it in the history for
		// next time.
		hist[tfToken] = &tokenHistory[tokens.ModuleMember]{
			Current:      computed.Tok,
			MajorVersion: version,
		}
	} else {
		*queue = append(*queue,
			func() { aliasOrRenameDataSource(p, computed, tfToken, prev, version) })
	}

	if ds == nil {
		return
	}

	// If we are behind the major version, reset the fields and the major version.
	if hist[tfToken].MajorVersion < version {
		hist[tfToken].MajorVersion = version
		hist[tfToken].Fields = nil
	}

	applyResourceMaxItemsOneAliasing(ds, &hist[tfToken].Fields, &computed.Fields)

}

func aliasOrRenameDataSource(
	p *ProviderInfo,
	ds *DataSourceInfo, tfToken string,
	prev *tokenHistory[tokens.ModuleMember],
	currentVersion int,
) {
	// re-fetch the resource, to make sure we have the right pointer.
	computed, ok := p.DataSources[tfToken]
	if !ok {
		// The DataSource to alias has been removed. There
		// is nothing to alias anymore.
		return
	}

	var alreadyPresent bool
	for _, a := range prev.Past {
		if a.Name == prev.Current {
			alreadyPresent = true
			break
		}
	}
	if !alreadyPresent && ds.Tok != prev.Current {
		prev.Past = append(prev.Past, alias[tokens.ModuleMember]{
			Name:         prev.Current,
			MajorVersion: currentVersion,
		})
	}
	for _, a := range prev.Past {
		if a.MajorVersion != currentVersion {
			continue
		}
		legacy := a.Name
		p.RenameDataSource(tfToken, legacy,
			computed.Tok, legacy.Module().Name().String(),
			computed.Tok.Module().Name().String(), computed)
	}

}
