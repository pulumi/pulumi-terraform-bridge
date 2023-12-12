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
	"sort"

	"github.com/Masterminds/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

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
// Panics if [ProviderInfo.ApplyAutoAliases] would return an error.
func (info *ProviderInfo) MustApplyAutoAliases() {
	err := info.ApplyAutoAliases()
	contract.AssertNoErrorf(err, "Failed to apply aliases")
}

// See [MustApplyAutoAliases].
func (info *ProviderInfo) ApplyAutoAliases() error {
	// Do minimal work at runtime to avoid adding to provider startup delay.
	if currentRuntimeStage == runningProviderStage {
		autoSettings, err := loadAutoSettings(info.GetMetadata())
		if err != nil {
			return err
		}
		autoSettings.apply(info)
		return nil
	}

	artifact := info.GetMetadata()

	hist, err := loadHistory(artifact)
	if err != nil {
		return err
	}

	h := &autoAliasHelper{info}

	currentVersion, err := h.computeCurrentVersion(hist)
	if err != nil {
		return err
	}

	// This needs to happen before newHist forgets the historical versions.
	h.clearFieldHistoryForMajorUpgrades(hist, currentVersion)

	newHist := h.updatedAliasHistory(hist, currentVersion)

	autoSettings, err := h.computeAutoSettings(newHist, currentVersion)
	if err != nil {
		return err
	}

	// Apply automatic settings by modifying info so that tfgen sees these settings.
	autoSettings.apply(info)

	// Now that info has updated Field values, save them to the history for later.
	h.updateFieldHistory(newHist)

	// Save updated history for later invocations of tfgen.
	if err := md.Set(artifact, aliasMetadataKey, newHist); err != nil {
		// Set fails only when `hist` is not serializable. Because `newHist` is composed of
		// marshallable, non-cyclic types, this is impossible.
		contract.AssertNoErrorf(err, "History failed to serialize")
	}

	// Save automatic settings so that the provider can load them at runtime.
	if err := md.Set(artifact, autoSettingsKey, autoSettings); err != nil {
		contract.AssertNoErrorf(err, "Auto settings failed to serialize")
	}

	return nil
}

const (
	// Key for storing aliasHistory in ProviderMetadata.
	aliasMetadataKey = "auto-aliasing"

	// Key for storing autoSettings in ProviderMetadata.
	autoSettingsKey = "auto-settings"
)

// History used by ApplyAutoAliases. This data is persisted across provider builds so that schema
// generation can consult decisions from the previous release. For ApplyAutoAliases to operate
// correctly, a persistent database of past decision history is required. This is currently done by
// doing reads and writes to `providerInfo.GetMetadata()`, which is assumed to be persisted across
// provider releases. The bridge framework keeps this information written out to an opaque
// `bridge-metadata.json` blob which is expected to be stored in source control to persist across
// releases. This data should not be used by the provider at runtime, and remain a tfgen-only
// concern.
type aliasHistory struct {
	Resources   map[string]*tokenHistory[tokens.Type]         `json:"resources,omitempty"`
	DataSources map[string]*tokenHistory[tokens.ModuleMember] `json:"datasources,omitempty"`
}

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

type fieldHistory struct {
	MaxItemsOne *bool `json:"maxItemsOne,omitempty"`

	Fields map[string]*fieldHistory `json:"fields,omitempty"`
	Elem   *fieldHistory            `json:"elem,omitempty"`
}

func (fields *fieldHistory) lookup(path walk.SchemaPath) *fieldHistory {
	here := fields
	for len(path) > 0 {
		switch step := path[0].(type) {
		case walk.ElementStep:
			if here.Elem == nil {
				return nil
			}
			here, path = here.Elem, path[1:]
		case walk.GetAttrStep:
			if here.Fields == nil {
				return nil
			}
			if _, ok := here.Fields[step.Name]; !ok {
				return nil
			}
			here, path = here.Fields[step.Name], path[1:]
		default:
			contract.Failf("impossible")
		}
	}
	return here
}

func (fields *fieldHistory) getOrCreate(path walk.SchemaPath) *fieldHistory {
	here := fields
	for len(path) > 0 {
		switch step := path[0].(type) {
		case walk.ElementStep:
			if here.Elem == nil {
				here.Elem = &fieldHistory{}
			}
			here, path = here.Elem, path[1:]
		case walk.GetAttrStep:
			if here.Fields == nil {
				here.Fields = map[string]*fieldHistory{}
			}
			if _, ok := here.Fields[step.Name]; !ok {
				here.Fields[step.Name] = &fieldHistory{}
			}
			here, path = here.Fields[step.Name], path[1:]
		default:
			contract.Failf("impossible")
		}
	}
	return here
}

type autoAliasHelper struct {
	info *ProviderInfo
}

func (h autoAliasHelper) computeAutoSettings(
	history aliasHistory,
	currentVersion int,
) (autoSettings, error) {
	info := h.info
	settings := autoSettings{}

	for tfToken := range info.Resources {
		r, ok := info.P.ResourcesMap().GetOk(tfToken)
		if !ok {
			continue
		}
		fields := info.Resources[tfToken].GetFields()
		var fieldHist map[string]*fieldHistory
		if hr, ok := history.Resources[tfToken]; ok {
			fieldHist = hr.Fields
		}
		if r != nil {
			over := h.computeMaxItemsOneOverrides(r.Schema(), fields, fieldHist)
			settings.setResourceOverrides(tfToken, over)
		}
		h.computeResourceAliasesAndRenames(tfToken, history, currentVersion, &settings)
	}

	for tfToken := range info.DataSources {
		d, ok := info.P.DataSourcesMap().GetOk(tfToken)
		if !ok {
			continue
		}
		fields := info.DataSources[tfToken].GetFields()
		var fieldHist map[string]*fieldHistory
		if hr, ok := history.DataSources[tfToken]; ok {
			fieldHist = hr.Fields
		}
		if d != nil {
			over := h.computeMaxItemsOneOverrides(d.Schema(), fields, fieldHist)
			settings.setDataSourceOverrides(tfToken, over)
		}
		h.computeDataSourceRenames(tfToken, history, currentVersion, &settings)
	}

	return settings, nil
}

func (h autoAliasHelper) computeCurrentVersion(hist aliasHistory) (int, error) {
	info := h.info
	var currentVersion int
	// If version is missing, we assume the current version is the most recent major
	// version in mentioned in history.
	if info.Version != "" {
		v, err := semver.NewVersion(info.Version)
		if err != nil {
			return 0, err
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
	return currentVersion, nil
}

func (autoAliasHelper) computeMaxItemsOneOverrides(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
	history map[string]*fieldHistory,
) maxItemsOneOverrides {
	if schemaMap == nil {
		return nil
	}
	over := make(maxItemsOneOverrides)
	fh := &fieldHistory{Fields: history}
	walk.VisitSchemaMap(schemaMap, func(sp walk.SchemaPath, s shim.Schema) {
		info := LookupSchemaInfoMapPath(sp, schemaInfos)

		// If the user already set a MaxItemsOne override, respect it.
		if info != nil && info.MaxItemsOne != nil {
			return
		}

		actual := IsMaxItemsOne(s, info)
		prev := fh.lookup(sp)

		// Preserve historical MaxItemsOne if it is available and disagrees with current.
		if prev != nil && prev.MaxItemsOne != nil && *prev.MaxItemsOne != actual {
			over.set(sp, *prev.MaxItemsOne)
		}
	})
	return over
}

func (h autoAliasHelper) computeResourceAliasesAndRenames(
	tfToken string,
	history aliasHistory,
	currentVersion int,
	settings *autoSettings,
) {
	hist := history.Resources[tfToken]
	if hist == nil {
		return
	}
	for _, a := range hist.Past {
		// Only respect hard aliases introduced in the same major version
		if a.InCodegen && a.MajorVersion == currentVersion {
			settings.addResourceAlias(tfToken, a.Name)
		} else {
			settings.addResourceRename(tfToken, a.Name)
		}
	}
}

func (h autoAliasHelper) computeDataSourceRenames(
	tfToken string,
	history aliasHistory,
	currentVersion int,
	settings *autoSettings,
) {
	hist := history.DataSources[tfToken]
	if hist == nil {
		return
	}
	for _, a := range hist.Past {
		if a.MajorVersion != currentVersion {
			continue
		}
		settings.addDataSourceRename(tfToken, a.Name)
	}
}

// This modifies hist in-place to drop Fields overrides that are not on the current version.
func (h autoAliasHelper) clearFieldHistoryForMajorUpgrades(hist aliasHistory, currentVersion int) {
	for _, v := range hist.DataSources {
		if v.MajorVersion < currentVersion {
			v.Fields = nil
		}
	}
	for _, v := range hist.Resources {
		if v.MajorVersion < currentVersion {
			v.Fields = nil
		}
	}
}

// Non-destructively computes updated alias history.
func (h autoAliasHelper) updatedAliasHistory(hist aliasHistory, currentVersion int) aliasHistory {
	info := h.info
	newHist := aliasHistory{
		DataSources: map[string]*tokenHistory[tokens.ModuleMember]{},
		Resources:   map[string]*tokenHistory[tokens.Type]{},
	}

	for tfToken, res := range info.Resources {
		_, ok := info.P.ResourcesMap().GetOk(tfToken)
		if !ok {
			continue
		}
		old := hist.Resources[tfToken]
		upd := updatedTokenHistory(old, currentVersion, res.Tok)
		newHist.Resources[tfToken] = upd
	}

	for tfToken, ds := range info.DataSources {
		_, ok := info.P.DataSourcesMap().GetOk(tfToken)
		if !ok {
			continue
		}
		old := hist.DataSources[tfToken]
		upd := updatedTokenHistory(old, currentVersion, ds.Tok)
		newHist.DataSources[tfToken] = upd
	}

	return newHist
}

// Returns an updated alias history with the new currentToken and currentMajorVersion. Previous
// values for "Current" are recorded in the Past list. Preserves the invariant Past and Current are
// unique, that is Past aliases are unique by Name and none of them has a Name matching Current.
func updatedTokenHistory[T ~string](
	th *tokenHistory[T],
	currentMajorVersion int,
	currentToken T,
) *tokenHistory[T] {
	if th == nil {
		return &tokenHistory[T]{
			Current:      currentToken,
			MajorVersion: currentMajorVersion,
		}
	}

	past := map[T]alias[T]{}

	for _, a := range append(th.Past, alias[T]{
		Name:         th.Current,
		InCodegen:    true,
		MajorVersion: th.MajorVersion,
	}) {
		if a.Name != currentToken {
			past[a.Name] = a
		}
	}

	u := tokenHistory[T]{
		Current:      currentToken,
		MajorVersion: currentMajorVersion,
		Fields:       th.Fields,
	}

	for _, a := range past {
		u.Past = append(u.Past, a)
	}

	sort.Slice(u.Past, func(i, j int) bool {
		return u.Past[i].Name < u.Past[j].Name
	})

	return &u
}

// Destructively updates hist Fields by recording the actual values of MaxItemsOne found in the
// modified Provider from h.info.
func (h autoAliasHelper) updateFieldHistory(hist aliasHistory) {
	for tfToken, r := range hist.Resources {
		res := h.info.P.ResourcesMap().Get(tfToken)
		if res != nil {
			r.Fields = h.captureFieldHistory(
				res.Schema(),
				h.info.Resources[tfToken].GetFields(),
			).Fields
		}
	}
	for tfToken, d := range hist.DataSources {
		ds := h.info.P.DataSourcesMap().Get(tfToken)
		if ds != nil {
			d.Fields = h.captureFieldHistory(
				ds.Schema(),
				h.info.DataSources[tfToken].GetFields(),
			).Fields
		}
	}
}

func (h autoAliasHelper) captureFieldHistory(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) *fieldHistory {
	fh := &fieldHistory{}

	if schemaMap == nil {
		return fh
	}

	walk.VisitSchemaMap(schemaMap, func(sp walk.SchemaPath, tfs shim.Schema) {
		info := LookupSchemaInfoMapPath(sp, schemaInfos)
		if tfs == nil || (tfs.Type() != shim.TypeList && tfs.Type() != shim.TypeSet) {
			return
		}
		actual := IsMaxItemsOne(tfs, info)
		fh.getOrCreate(sp).MaxItemsOne = &actual
	})

	return fh
}

func loadAutoSettings(artifact ProviderMetadata) (autoSettings, error) {
	hist, ok, err := md.Get[autoSettings](artifact, autoSettingsKey)
	if err != nil {
		return autoSettings{}, err
	}
	if !ok {
		hist = autoSettings{}
	}
	return hist, nil
}

func loadHistory(artifact ProviderMetadata) (aliasHistory, error) {
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

// Minimal data computed at tfgen time that is needed to inform the runtime provider behavior.
type autoSettings struct {
	Resources   map[string]*autoResourceSettings   `json:"resources,omitempty"`
	DataSources map[string]*autoDataSourceSettings `json:"datasources,omitempty"`
}

func (a *autoSettings) findOrCreateResource(key string) *autoResourceSettings {
	if a.Resources == nil {
		a.Resources = map[string]*autoResourceSettings{}
	}
	if _, ok := a.Resources[key]; !ok {
		a.Resources[key] = &autoResourceSettings{}
	}
	return a.Resources[key]
}

func (a *autoSettings) findOrCreateDataSource(key string) *autoDataSourceSettings {
	if a.DataSources == nil {
		a.DataSources = map[string]*autoDataSourceSettings{}
	}
	if _, ok := a.DataSources[key]; !ok {
		a.DataSources[key] = &autoDataSourceSettings{}
	}
	return a.DataSources[key]
}

func (a *autoSettings) addResourceAlias(key string, alias tokens.Type) {
	r := a.findOrCreateResource(key)
	if !contains(r.Aliases, alias) {
		r.Aliases = append(r.Aliases, alias)
	}
}

func (a *autoSettings) addResourceRename(key string, rename tokens.Type) {
	r := a.findOrCreateResource(key)
	if !contains(r.Renames, rename) {
		r.Renames = append(r.Renames, rename)
	}
}

func (a *autoSettings) addDataSourceRename(key string, rename tokens.ModuleMember) {
	d := a.findOrCreateDataSource(key)
	if !contains(d.Renames, rename) {
		d.Renames = append(d.Renames, rename)
	}
}

func (a *autoSettings) setResourceOverrides(key string, over maxItemsOneOverrides) {
	if over.isEmpty() {
		return
	}
	a.findOrCreateResource(key).MaxItemsOneOverrides = over
}

func (a *autoSettings) setDataSourceOverrides(key string, over maxItemsOneOverrides) {
	if over.isEmpty() {
		return
	}
	a.findOrCreateDataSource(key).MaxItemsOneOverrides = over
}

func (a *autoSettings) apply(p *ProviderInfo) {
	for k, v := range a.Resources {
		v.apply(p, k)
	}
	for k, v := range a.DataSources {
		v.apply(p, k)
	}
}

type autoResourceSettings struct {
	Aliases              []tokens.Type        `json:"aliases,omitempty"`
	MaxItemsOneOverrides maxItemsOneOverrides `json:"maxItemsOneOverrides,omitempty"`
	Renames              []tokens.Type        `json:"renames,omitempty"`
}

func (a autoResourceSettings) apply(p *ProviderInfo, tfToken string) {
	r, ok := p.Resources[tfToken]
	contract.Assertf(ok && r.Tok != "",
		"Unexpected resource token lacking a mapping: %q", tfToken)
	for _, old := range a.Aliases {
		oldMod := old.Module().Name().String()
		newMod := r.Tok.Module().Name().String()
		p.RenameResourceWithAlias(tfToken, old, r.Tok, oldMod, newMod, r)
	}
	for _, old := range a.Renames {
		ty := old.String()
		r.Aliases = append(r.Aliases, AliasInfo{Type: &ty})
	}
	a.MaxItemsOneOverrides.applyToResource(p, tfToken)
}

type autoDataSourceSettings struct {
	Renames              []tokens.ModuleMember `json:"renames,omitempty"`
	MaxItemsOneOverrides maxItemsOneOverrides  `json:"maxItemsOneOverrides,omitempty"`
}

func (a autoDataSourceSettings) apply(p *ProviderInfo, tfToken string) {
	d, ok := p.DataSources[tfToken]
	contract.Assertf(ok && d.Tok != "",
		"Unexpected data source token lacking a mapping: %q", tfToken)
	for _, old := range a.Renames {
		oldMod := old.Module().Name().String()
		newMod := d.Tok.Module().Name().String()
		p.RenameDataSource(tfToken, old, d.Tok, oldMod, newMod, d)
	}
	a.MaxItemsOneOverrides.applyToDataSource(p, tfToken)
}

type maxItemsOneOverrides map[string]bool

func (over maxItemsOneOverrides) isEmpty() bool {
	return len(over) == 0
}

func (over maxItemsOneOverrides) set(path walk.SchemaPath, maxItemsOne bool) {
	over[path.MustEncodeSchemaPath()] = maxItemsOne
}

func (over *maxItemsOneOverrides) applyToResource(p *ProviderInfo, tfToken string) {
	r, ok := p.Resources[tfToken]
	contract.Assertf(ok && r.Tok != "",
		"Unexpected resource token lacking a mapping: %q", tfToken)
	over.applyMaxItemsOneOverridesToFields(&r.Fields)
}

func (over *maxItemsOneOverrides) applyToDataSource(p *ProviderInfo, tfToken string) {
	d, ok := p.DataSources[tfToken]
	contract.Assertf(ok && d.Tok != "",
		"Unexpected data source token lacking a mapping: %q", tfToken)
	over.applyMaxItemsOneOverridesToFields(&d.Fields)
}

func (over maxItemsOneOverrides) applyMaxItemsOneOverridesToFields(fields *map[string]*SchemaInfo) {
	s := &SchemaInfo{Fields: *fields}
	over.applyMaxItemsOneOverrides(s)
	*fields = s.Fields
}

func (over maxItemsOneOverrides) applyMaxItemsOneOverrides(info *SchemaInfo) {
	for k, v := range over {
		v := v
		p := walk.DecodeSchemaPath(k)
		getOrCreateSchemaInfo(info, p).MaxItemsOne = &v
	}
}

func contains[T ~string](xs []T, x T) bool {
	for _, y := range xs {
		if x == y {
			return true
		}
	}
	return false
}

func getOrCreateSchemaInfo(info *SchemaInfo, path walk.SchemaPath) *SchemaInfo {
	here := info
	for len(path) > 0 {
		switch step := path[0].(type) {
		case walk.ElementStep:
			if here.Elem == nil {
				here.Elem = &SchemaInfo{}
			}
			here, path = here.Elem, path[1:]
		case walk.GetAttrStep:
			if here.Fields == nil {
				here.Fields = map[string]*SchemaInfo{}
			}
			if _, ok := here.Fields[step.Name]; !ok {
				here.Fields[step.Name] = &SchemaInfo{}
			}
			here, path = here.Fields[step.Name], path[1:]
		default:
			contract.Failf("impossible")
		}
	}
	return here
}
