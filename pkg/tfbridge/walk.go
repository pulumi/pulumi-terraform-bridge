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
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/util"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

type SchemaPath = walk.SchemaPath

// Translate a Pulumi property path to a bridged provider schema path.
//
// The function hides the complexity of mapping Pulumi property names to Terraform names, joining Schema with user
// overrides in SchemaInfos, and accounting for MaxItems=1 situations where Pulumi flattens collections to plain values.
// and therefore SchemaPath values are longer than the PropertyPath values.
//
// PropertyPathToSchemaPath may return nil if there is no matching schema found. This may happen when drilling down to
// values of unknown type, attributes not tracked in schema, or when there is a type mismatch between the path and the
// schema.
func PropertyPathToSchemaPath(
	propertyPath resource.PropertyPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo) SchemaPath {
	return propertyPathToSchemaPath(walk.NewSchemaPath(), propertyPath, schemaMap, schemaInfos)
}

func propertyPathToSchemaPath(
	basePath SchemaPath,
	propertyPath resource.PropertyPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) SchemaPath {

	if len(propertyPath) == 0 {
		return basePath
	}

	if schemaInfos == nil {
		schemaInfos = make(map[string]*SchemaInfo)
	}

	firstStep, ok := propertyPath[0].(string)
	if !ok {
		return nil
	}

	firstStepTF := PulumiToTerraformName(firstStep, schemaMap, schemaInfos)

	fieldSchema, found := schemaMap.GetOk(firstStepTF)
	if !found {
		return nil
	}
	fieldInfo := schemaInfos[firstStepTF]
	return propertyPathToSchemaPathInner(basePath.GetAttr(firstStepTF), propertyPath[1:], fieldSchema, fieldInfo)
}

func propertyPathToSchemaPathInner(
	basePath SchemaPath,
	propertyPath resource.PropertyPath,
	schema shim.Schema,
	schemaInfo *SchemaInfo,
) SchemaPath {

	if len(propertyPath) == 0 {
		return basePath
	}

	if schemaInfo == nil {
		schemaInfo = &SchemaInfo{}
	}

	// Detect single-nested blocks (object types).
	if res, isRes := schema.Elem().(shim.Resource); schema.Type() == shim.TypeMap && isRes {
		return propertyPathToSchemaPath(basePath, propertyPath, res.Schema(), schemaInfo.Fields)
	}

	// Detect collections.
	switch schema.Type() {
	case shim.TypeList, shim.TypeMap, shim.TypeSet:
		var elemPP resource.PropertyPath
		if IsMaxItemsOne(schema, schemaInfo) {
			// Pulumi flattens MaxItemsOne values, so the element path is the same as the current path.
			elemPP = propertyPath
		} else {
			// For normal collections the first element drills down into the collection, skip it.
			elemPP = propertyPath[1:]
		}
		switch e := schema.Elem().(type) {
		case shim.Resource: // object element type
			return propertyPathToSchemaPath(basePath.Element(), elemPP, e.Schema(), schemaInfo.Fields)
		case shim.Schema: // non-object element type
			return propertyPathToSchemaPathInner(basePath.Element(), elemPP, e, schemaInfo.Elem)
		case nil: // unknown element type
			// Cannot drill down further, but len(propertyPath)>0.
			return nil
		}
	}

	// Cannot drill down further, but len(propertyPath)>0.
	return nil
}

// Translate a a bridged provider schema path into a Pulumi property path.
//
// The function hides the complexity of mapping Terraform names to Pulumi property names,
// joining Schema with user overrides in [SchemaInfo]s, and accounting for MaxItems=1
// situations where Pulumi flattens collections to plain values.
//
// [SchemaPathToPropertyPath] may return nil if there is no matching schema found. This
// may happen when drilling down to values of unknown type, attributes not tracked in
// schema, or when there is a type mismatch between the path and the schema.
//
// ## Element handling
//
// [SchemaPath]s can be either attributes or existential elements. .Element() segments are
// existential because they represent some element access, but not a specific element
// access. For example:
//
//	NewSchemaPath().GetAttr("x").Element().GetAttr("y")
//
// [resource.PropertyPath]s have attributes or instantiated elements. Elements are
// instantiated because they represent a specific element access. For example:
//
//	x[3].y
//
// [SchemaPathToPropertyPath] translates all existential elements into the "*"
// (_universal_) element. For example:
//
//	NewSchemaPath().GetAttr("x").Element().GetAttr("y") => x["*"].y
//
// This is information preserving, but "*"s are not usable in all functions that
// accept [resource.PropertyPath].
func SchemaPathToPropertyPath(
	schemaPath SchemaPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo) resource.PropertyPath {
	return schemaPathToPropertyPath(resource.PropertyPath{}, schemaPath, schemaMap, schemaInfos)
}

func schemaPathToPropertyPath(
	basePath resource.PropertyPath,
	schemaPath SchemaPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo) resource.PropertyPath {

	if len(schemaPath) == 0 {
		return basePath
	}

	if schemaInfos == nil {
		schemaInfos = make(map[string]*SchemaInfo)
	}

	firstStep, ok := schemaPath[0].(walk.GetAttrStep)
	if !ok {
		return nil
	}

	firstStepPu := TerraformToPulumiNameV2(firstStep.Name, schemaMap, schemaInfos)

	fieldSchema, found := schemaMap.GetOk(firstStep.Name)
	if !found {
		return nil
	}
	fieldInfo := schemaInfos[firstStep.Name]
	return schemaPathToPropertyPathInner(append(basePath, firstStepPu), schemaPath[1:], fieldSchema, fieldInfo)
}

func schemaPathToPropertyPathInner(
	basePath resource.PropertyPath,
	schemaPath SchemaPath,
	schema shim.Schema,
	schemaInfo *SchemaInfo,
) resource.PropertyPath {

	if len(schemaPath) == 0 {
		return basePath
	}

	if schemaInfo == nil {
		schemaInfo = &SchemaInfo{}
	}

	// Detect single-nested blocks (object types).
	if obj, isObject := util.CastToTypeObject(schema); isObject {
		return schemaPathToPropertyPath(basePath, schemaPath, obj, schemaInfo.Fields)
	}

	// Detect collections.
	switch schema.Type() {
	case shim.TypeList, shim.TypeMap, shim.TypeSet:
		// If a element is MaxItemsOne, it doesn't appear in the resource.PropertyPath at all.
		//
		// Otherwise we represent Elem relationships with a "*", since the schema
		// change applies to all nested items.
		if !IsMaxItemsOne(schema, schemaInfo) {
			basePath = append(basePath, "*")
		}
		switch e := schema.Elem().(type) {
		case shim.Resource: // object element type
			return schemaPathToPropertyPath(basePath, schemaPath[1:], e.Schema(), schemaInfo.Fields)
		case shim.Schema: // non-object element type
			return schemaPathToPropertyPathInner(basePath, schemaPath[1:], e, schemaInfo.Elem)
		case nil: // unknown element type
			// Cannot drill down further, but len(propertyPath)>0.
			return nil
		}
	}

	// Cannot drill down further, but len(propertyPath)>0.
	return nil
}

// Convenience method to lookup both a Schema and a SchemaInfo by path.
func LookupSchemas(schemaPath SchemaPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo) (shim.Schema, *SchemaInfo, error) {

	s, err := walk.LookupSchemaMapPath(schemaPath, schemaMap)
	if err != nil {
		return nil, nil, err
	}

	return s, LookupSchemaInfoMapPath(schemaPath, schemaInfos), nil
}

// Drill down a path from a map of SchemaInfo objects and find a matching SchemaInfo if any.
func LookupSchemaInfoMapPath(
	schemaPath SchemaPath,
	schemaInfos map[string]*SchemaInfo,
) *SchemaInfo {

	if len(schemaPath) == 0 {
		return nil
	}

	if schemaInfos == nil {
		return nil
	}

	switch step := schemaPath[0].(type) {
	case walk.ElementStep:
		return nil
	case walk.GetAttrStep:
		return LookupSchemaInfoPath(schemaPath[1:], schemaInfos[step.Name])
	}

	return nil
}

// Drill down a path from a  SchemaInfo object and find a matching SchemaInfo if any.
func LookupSchemaInfoPath(schemaPath SchemaPath, schemaInfo *SchemaInfo) *SchemaInfo {
	if len(schemaPath) == 0 {
		return schemaInfo
	}
	if schemaInfo == nil {
		return nil
	}
	switch schemaPath[0].(type) {
	case walk.ElementStep:
		return LookupSchemaInfoPath(schemaPath[1:], schemaInfo.Elem)
	case walk.GetAttrStep:
		return LookupSchemaInfoMapPath(schemaPath, schemaInfo.Fields)
	default:
		return nil
	}
}

// Drill down a path from a SchemaInfo object and find a matching SchemaInfo. If no
// SchemaInfo exists, it will be created.
func getOrCreateSchemaInfoPath(schemaPath SchemaPath, schemaInfo *SchemaInfo) *SchemaInfo {
	if len(schemaPath) == 0 {
		return schemaInfo
	}

	switch first := schemaPath[0].(type) {
	case walk.ElementStep:
		return getOrCreateSchemaInfoPath(schemaPath[1:], ensureField(&schemaInfo.Elem))
	case walk.GetAttrStep:
		return getOrCreateSchemaInfoPath(schemaPath[1:], ensureMapKey(&schemaInfo.Fields, first.Name))
	default:
		contract.Failf("Unable to walk SchemaPath segment of type %T", schemaPath[0])
		return nil
	}
}

type PropertyVisitInfo struct {
	Root VisitRoot

	schemaPath SchemaPath

	shimSchema    shim.Schema
	getSchemaInfo func() *SchemaInfo

	getShimSchemaMap func() shim.SchemaMap
	getSchemaInfoMap func() map[string]*SchemaInfo
}

type PropertyVisitResult struct {
	// If the visitor has done something.
	//
	// Visits will only be replayed during runtime startup if `HasEffect: true`.
	HasEffect bool
}

func (v *PropertyVisitInfo) SchemaPath() SchemaPath  { return v.schemaPath }
func (v *PropertyVisitInfo) ShimSchema() shim.Schema { return v.shimSchema }
func (v *PropertyVisitInfo) SchemaInfo() *SchemaInfo { return v.getSchemaInfo() }
func (v *PropertyVisitInfo) EnclosingSchemaInfoMap() map[string]*SchemaInfo {
	return v.getSchemaInfoMap()
}
func (v *PropertyVisitInfo) EnclosingSchemaMap() shim.SchemaMap { return v.getShimSchemaMap() }

type VisitRoot interface {
	PulumiToken() tokens.Type

	isVisitRoot()
}

// This ensures that we can peform casts from VisitRoot to T for each valid root. Changing
// this from T to *T is a breaking change.
var (
	_ VisitRoot = VisitResourceRoot{}
	_ VisitRoot = VisitDataSourceRoot{}
	_ VisitRoot = VisitConfigRoot{}
)

type VisitResourceRoot struct {
	token tokens.Type

	Info *ResourceInfo

	TfToken string
}

func (r VisitResourceRoot) PulumiToken() tokens.Type { return r.token }
func (r VisitResourceRoot) isVisitRoot()             {}

type VisitDataSourceRoot struct {
	token tokens.Type

	Info *DataSourceInfo

	TfToken string
}

func (r VisitDataSourceRoot) PulumiToken() tokens.Type { return r.token }
func (r VisitDataSourceRoot) isVisitRoot()             {}

type VisitConfigRoot struct{ token tokens.Type }

func (r VisitConfigRoot) PulumiToken() tokens.Type { return r.token }
func (r VisitConfigRoot) isVisitRoot()             {}

type PropertyVisitor = func(PropertyVisitInfo) (PropertyVisitResult, error)

// A wrapper around [TraverseProperties] that panics on error. See [TraverseProperties] for
// documentation.
func MustTraverseProperties(prov *ProviderInfo, traversalID string, visitor PropertyVisitor) {
	err := TraverseProperties(prov, traversalID, visitor)
	contract.AssertNoErrorf(err, "failed to traverse provider schemas")
}

// _Efficiently_ traverse all property fields in a provider.
//
// Each property is rooted in a resource, data source or provider config. Roots themselves
// are not visited, only properties.
//
// TraverseProperties operates in two stages:
//
//   - During `make tfgen` time, all fields in the provider are visited. TraverseProperties
//     builds a list of fields where `VisitResult{ HasEffect: true }` is returned.
//
//   - During the provider's normal runtime, only "effectful" fields are visited.
//
// traversalId must be unique for each traversal within a `make tfgen`, but the same
// between `make tfgen` and the provider runtime. A simple descriptive string like `"apply
// labels"` works great here.
func TraverseProperties(
	prov *ProviderInfo, traversalID string, visitor PropertyVisitor,
	opts ...TraversalOption,
) error {
	prov.MetadataInfo.assertValid() // We must have metadata info for an efficient (sparse) traversal.

	var errs []error

	var traverseOptions traversalOptions
	for _, opt := range opts {
		opt(&traverseOptions)
	}

	// forEffect indicates that the traversal is being run for side effects only, so
	// we can visit only "effectful" nodes.
	forEffect := currentRuntimeStage == runningProviderStage
	if b := traverseOptions.forEffect; b != nil {
		forEffect = *b
	}

	var traversalEffects traversalRecord

	// We are in forEffect mode, so we need to load in the list of paths where we need
	// to replay effects.
	if forEffect {
		var ok bool
		var err error
		traversalEffects, ok, err = md.Get[traversalRecord](prov.GetMetadata(), traversalID)
		contract.Assertf(ok, "could not find traversalID %q in metadata, are you missing a `make tfgen`?", traversalID)
		if err != nil {
			return fmt.Errorf("unable to find effect list: %w", err)
		}
	}

	ignoredTokens := ignoredTokens(prov)

	prov.P.ResourcesMap().Range(func(tfName string, rShim shim.Resource) bool {
		if ignoredTokens[tfName] {
			return true
		}

		var err error
		effectPaths := ensureMap(&traversalEffects.Resources)[tfName]
		effectPaths, err = traverseResourceOrDataSource(tfName, rShim, ensureMapKey(&prov.Resources, tfName),
			visitor, forEffect, effectPaths)
		if err != nil {
			errs = append(errs, err)
		}
		traversalEffects.Resources[tfName] = effectPaths

		return true
	})

	prov.P.DataSourcesMap().Range(func(tfName string, dShim shim.Resource) bool {
		if ignoredTokens[tfName] {
			return true
		}

		var err error
		effectPaths := ensureMap(&traversalEffects.DataSources)[tfName]
		effectPaths, err = traverseResourceOrDataSource(tfName, dShim, ensureMapKey(&prov.DataSources, tfName),
			visitor, forEffect, effectPaths)
		if err != nil {
			errs = append(errs, err)
		}
		traversalEffects.DataSources[tfName] = effectPaths
		return true
	})

	{
		var err error
		traversalEffects.Config, err = traverseSchemaMap(prov.P.Schema(), ensureMap(&prov.Config),
			VisitConfigRoot{tokens.Type("pulumi:providers" + prov.Name)}, visitor, forEffect, traversalEffects.Config)
		if err != nil {
			errs = append(errs, err)
		}
	}

	// We are in !forEffect mode, so we need to save the list of effects to generate.
	if !forEffect {
		traversalEffects.clean()
		declareRuntimeMetadata(traversalID)
		err := md.Set(prov.GetMetadata(), traversalID, traversalEffects)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

type traversalOptions struct {
	forEffect *bool
}

type TraversalOption func(*traversalOptions)

// Set [TraverseProperties] to run in a specific mode.
//
// If forEffect is `false`, then [TraverseProperties] will always touch every field,
// regardless of runtime.
//
// If forEffect is `true`, then [TraverseProperties] will only touch fields based off of the
// "effectful" list contained in [prov.MetadataInfo].
func TraverseForEffect(forEffect bool) TraversalOption {
	return func(opts *traversalOptions) {
		opts.forEffect = &forEffect
	}
}

// traversalRecord is the record of "effectful" traversals generated by [TraverseProperties]
// during `make tfgen`. It is used to replay _only_ "effectful" changes during provider
// startup.
type traversalRecord struct {
	Resources   map[string][]SchemaPath `json:"resources,omitempty"`
	DataSources map[string][]SchemaPath `json:"dataSources,omitempty"`
	Config      []SchemaPath            `json:"config,omitempty"`
}

// clean removes unnecessary records and makes the result deterministic.
//
// It should be called before marshalling.
func (r *traversalRecord) clean() {
	cleanList := func(l []SchemaPath) []SchemaPath {
		walk.SortSchemaPaths(l)
		return l
	}

	clean := func(m map[string][]SchemaPath) {
		for k, v := range m {
			if len(v) == 0 {
				delete(m, k)
				continue
			}
			m[k] = cleanList(v)
		}
	}

	clean(r.Resources)
	clean(r.DataSources)
	r.Config = cleanList(r.Config)
}

// traverseResourceOrDataSource traverses a single resource or datasource.
//
// During the provider runtime, visitor is only replayed on paths where there was an effect.
func traverseResourceOrDataSource(
	tfToken string, schema shim.Resource, info ResourceOrDataSourceInfo,
	visitor PropertyVisitor, forEffect bool, effects []SchemaPath,
) ([]SchemaPath, error) {
	// If we are gathering effects for this resource, we don't want to observe any
	// previously gathered effects.
	if !forEffect {
		effects = []SchemaPath{}
	}

	var root VisitRoot
	switch info := info.(type) {
	case *ResourceInfo:
		root = VisitResourceRoot{
			token:   info.Tok,
			Info:    info,
			TfToken: tfToken,
		}
		if info.Fields == nil {
			info.Fields = map[string]*SchemaInfo{}
		}
	case *DataSourceInfo:
		root = VisitDataSourceRoot{
			token:   tokens.Type(info.GetTok()),
			Info:    info,
			TfToken: tfToken,
		}
		if info.Fields == nil {
			info.Fields = map[string]*SchemaInfo{}
		}
	default:
		contract.Failf("info must be a *ResourceInfo or *DataSourceInfo, found %T", info)
	}
	return traverseSchemaMap(schema.Schema(), info.GetFields(), root, visitor, forEffect, effects)
}

func traverseSchemaMap(
	schema shim.SchemaMap, info map[string]*SchemaInfo, root VisitRoot,
	visitor PropertyVisitor, forEffect bool, effects []SchemaPath,
) ([]SchemaPath, error) {
	var errs []error

	f := func(p SchemaPath, s shim.Schema) {
		getSchemaInfo := func() *SchemaInfo {
			r := getSchemaInfoRoot(p, info)
			contract.Assertf(r != nil, "failed to get non-nil resource info")
			return r
		}
		getSchemaInfoMap := func() map[string]*SchemaInfo {
			return getSchemaInfoRoot(p[:len(p)-1], info).Fields
		}
		getShimSchemaMap := func() shim.SchemaMap {
			m, err := walk.LookupSchemaMapPath(p[:len(p)-1], schema)
			if err != nil {
				errs = append(errs, err)
			}
			if m == nil {
				return nil
			}
			elem, ok := m.Elem().(shim.Resource)
			if !ok {
				return nil
			}
			return elem.Schema()
		}
		result, err := visitor(PropertyVisitInfo{
			Root:             root,
			schemaPath:       p,
			shimSchema:       s,
			getShimSchemaMap: getShimSchemaMap,
			getSchemaInfo:    getSchemaInfo,
			getSchemaInfoMap: getSchemaInfoMap,
		})
		if err != nil {
			errs = append(errs, err)
			return
		}
		if result.HasEffect && !forEffect {
			effects = append(effects, p)
		}
		contract.Assertf(!(forEffect && !result.HasEffect),
			"A runtime traversal did not request an effect. This indicates a provider bug.")
	}

	if forEffect {
		for _, effect := range effects {
			s, err := walk.LookupSchemaMapPath(effect, schema)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			f(effect, s)
		}
		return effects, errors.Join(errs...)
	}

	walk.VisitSchemaMap(schema, f)

	return effects, errors.Join(errs...)
}

func getSchemaInfoRoot(p SchemaPath, info map[string]*SchemaInfo) *SchemaInfo {
	if len(p) == 0 { // This must be the root of a resource or datasource, so we mock the root.
		return &SchemaInfo{Fields: info}
	}
	contract.Assertf(info != nil, "info cannot be nil")
	contract.Assertf(len(p) > 0, "p cannot point to the resource or datasource")

	switch root := p[0].(type) {
	case walk.GetAttrStep:
		return getOrCreateSchemaInfoPath(p[1:], ensureMapKey(&info, root.Name))
	default:
		contract.Failf("The first step of a resource or datasource must be a attr")
		return nil
	}
}

func ensureField[T any](t **T) *T {
	if *t == nil {
		*t = new(T)
	}
	return *t
}

func ensureMap[K comparable, V any](m *map[K]V) map[K]V {
	if *m == nil {
		*m = map[K]V{}
	}
	return *m
}

func ensureMapKey[K comparable, V any](m *map[K]*V, key K) *V {
	ensureMap(m)
	v, ok := (*m)[key]
	if ok {
		return v
	}
	v = new(V)
	(*m)[key] = v
	return v
}
