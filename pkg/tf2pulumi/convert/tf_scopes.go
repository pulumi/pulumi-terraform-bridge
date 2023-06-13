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

package convert

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/terraform/pkg/addrs"
	"github.com/pulumi/terraform/pkg/lang"
	"github.com/pulumi/terraform/pkg/tfdiags"
	"github.com/zclconf/go-cty/cty"
)

// Used to return info about a path in the schema.
type PathInfo struct {
	// The final part of the path (e.g. a_field)
	Name string

	// The Resource that contains the path (e.g. data.simple_data_source)
	Resource shim.Resource
	// The DataSourceInfo for the path (e.g. data.simple_data_source)
	DataSourceInfo *tfbridge.DataSourceInfo
	// The ResourceInfo for the path (e.g. simple_resource)
	ResourceInfo *tfbridge.ResourceInfo

	// The Schema for the path (e.g. data.simple_data_source.a_field)
	Schema shim.Schema
	// The SchemaInfo for the path (e.g. data.simple_data_source.a_field)
	SchemaInfo *tfbridge.SchemaInfo

	// The expression for a local variable
	Expression *hcl.Expression
}

type scopes struct {
	info il.ProviderInfoSource

	// All known roots, keyed by fully qualified path e.g. data.some_data_source
	roots map[string]PathInfo

	// Local variables that are in scope from for expressions
	locals []map[string]string

	// Set non-nil if "count.index" can be mapped
	countIndex hcl.Traversal
	eachKey    hcl.Traversal
	eachValue  hcl.Traversal

	scope *lang.Scope
}

func newScopes(info il.ProviderInfoSource) *scopes {
	s := &scopes{
		info:   info,
		roots:  make(map[string]PathInfo),
		locals: make([]map[string]string, 0),
	}
	scope := &lang.Scope{
		Data:     s,
		PureOnly: true,
		BaseDir:  ".",
	}
	s.scope = scope
	return s
}

// lookup the given name in roots and locals
func (s *scopes) lookup(name string) string {
	for i := len(s.locals) - 1; i >= 0; i-- {
		if s.locals[i][name] != "" {
			return s.locals[i][name]
		}
	}
	if root, has := s.roots[name]; has {
		return root.Name
	}
	return ""
}

func (s *scopes) push(locals map[string]string) {
	s.locals = append(s.locals, locals)
}

func (s *scopes) pop() {
	s.locals = s.locals[0 : len(s.locals)-1]
}

// isUsed returns if _any_ root scope currently uses the name "name"
func (s *scopes) isUsed(name string) bool {
	// We don't have many, but there's a few _keywords_ in pcl that are easier if we just never emit them
	if name == "range" {
		return true
	}

	for _, usedName := range s.roots {
		if usedName.Name == name {
			return true
		}
	}
	return false
}

// generateUniqueName takes "name" and ensures it's unique.
// First by appending `suffix` to it, and then appending an incrementing count
func (s *scopes) generateUniqueName(name, prefix, suffix string) string {
	// Not used, just return it
	if !s.isUsed(name) {
		return name
	}
	// It's used, so add the prefix and suffix
	name = prefix + name + suffix
	if !s.isUsed(name) {
		return name
	}
	// Still used add a counter
	baseName := name
	counter := 2
	for {
		name = fmt.Sprintf("%s%d", baseName, counter)
		if !s.isUsed(name) {
			return name
		}
		counter = counter + 1
	}
}

func (s *scopes) getOrAddOutput(name string) string {
	root, has := s.roots[name]
	if has {
		return root.Name
	}
	parts := strings.Split(name, ".")
	tfName := parts[len(parts)-1]
	pulumiName := camelCaseName(tfName)
	s.roots[name] = PathInfo{Name: pulumiName}
	return pulumiName
}

// getPulumiName takes "name" and ensures it's unique.
// First by appending `suffix` to it, and then appending an incrementing count
func (s *scopes) getOrAddPulumiName(name, prefix, suffix string) string {
	root, has := s.roots[name]
	if has {
		return root.Name
	}
	parts := strings.Split(name, ".")
	tfName := parts[len(parts)-1]
	pulumiName := camelCaseName(tfName)
	pulumiName = s.generateUniqueName(pulumiName, prefix, suffix)
	s.roots[name] = PathInfo{Name: pulumiName}
	return pulumiName
}

// Given a fully typed path (e.g. data.simple_data_source.a_field) returns the final part of that path
// (a_field) and the either the Resource or Schema, and SchemaInfo for that path (if any).
func (s *scopes) getInfo(fullyQualifiedPath string) PathInfo {
	parts := strings.Split(fullyQualifiedPath, ".")
	contract.Assertf(len(parts) >= 2, "empty path passed into getInfo: %s", fullyQualifiedPath)
	contract.Assertf(parts[0] != "", "empty path part passed into getInfo: %s", fullyQualifiedPath)
	contract.Assertf(parts[1] != "", "empty path part passed into getInfo: %s", fullyQualifiedPath)

	var getInner func(sch shim.SchemaMap, info map[string]*tfbridge.SchemaInfo, parts []string) PathInfo
	getInner = func(sch shim.SchemaMap, info map[string]*tfbridge.SchemaInfo, parts []string) PathInfo {
		contract.Assertf(parts[0] != "", "empty path part passed into getInfo")

		// At this point parts[0] may be an property + indexer or just a property. Work that out first.
		part, rest, indexer := strings.Cut(parts[0], "[]")

		// Lookup the info for this part
		var curSch shim.Schema
		if sch != nil {
			curSch = sch.Get(part)
		}
		curInfo := info[part]

		// We want this part
		if len(parts) == 1 && !indexer {
			return PathInfo{
				Name:       part,
				Schema:     curSch,
				SchemaInfo: curInfo,
			}
		}

		// Else recurse into the next part of the type, how we do this depends on if this was indexed or not
		if !indexer {
			// No indexers, simple recurse on fields
			var nextSchema shim.SchemaMap
			var nextInfo map[string]*tfbridge.SchemaInfo
			if curSch != nil {
				if sch, ok := curSch.Elem().(shim.Resource); ok {
					nextSchema = sch.Schema()
				}
			}
			if curInfo != nil {
				nextInfo = curInfo.Fields
			}
			return getInner(nextSchema, nextInfo, parts[1:])
		}

		// part was indexed (i.e. something like "part[]" or part[]foo[][]bar), so rather than looking at the
		// fields we need to look at the elements.
		for {
			var resourceOrSchema interface{}
			if curSch != nil {
				resourceOrSchema = curSch.Elem()
			}
			if curInfo != nil {
				curInfo = curInfo.Elem
			}

			if rest == "" && len(parts) == 1 {
				// This element is what we we're looking for
				res, _ := resourceOrSchema.(shim.Resource)
				sch, _ := resourceOrSchema.(shim.Schema)
				return PathInfo{
					Name:       part,
					Resource:   res,
					Schema:     sch,
					SchemaInfo: curInfo,
				}
			} else if rest == "" {
				// Can recurse into the next set of fields
				var nextSchema shim.SchemaMap
				var nextInfo map[string]*tfbridge.SchemaInfo
				if sch, ok := resourceOrSchema.(shim.Resource); ok {
					nextSchema = sch.Schema()
				}
				if curInfo != nil {
					nextInfo = curInfo.Fields
				}
				return getInner(nextSchema, nextInfo, parts[1:])
			}

			panic(fmt.Sprintf("complex indexerParts not implemented: %v", rest))
		}
	}

	if parts[0] == "data" {
		contract.Assertf(len(parts) >= 3, "empty path passed into getInfo: %s", fullyQualifiedPath)
		contract.Assertf(parts[2] != "", "empty path part passed into getInfo: %s", fullyQualifiedPath)

		root, has := s.roots[parts[0]+"."+parts[1]+"."+parts[2]]
		if len(parts) == 3 {
			if has {
				return root
			}
			// If we don't have a root, just return the name
			return PathInfo{Name: parts[2]}
		}

		var currentSchema shim.SchemaMap
		var currentInfo map[string]*tfbridge.SchemaInfo
		if root.Resource != nil {
			currentSchema = root.Resource.Schema()
		}
		if root.DataSourceInfo != nil {
			currentInfo = root.DataSourceInfo.Fields
		}

		return getInner(currentSchema, currentInfo, parts[3:])

	}

	root, has := s.roots[parts[0]+"."+parts[1]]

	if len(parts) == 2 {
		if has {
			return root
		}
		// If we don't have a root, just return the name
		return PathInfo{Name: parts[1]}
	}

	var currentSchema shim.SchemaMap
	var currentInfo map[string]*tfbridge.SchemaInfo
	if root.Resource != nil {
		currentSchema = root.Resource.Schema()
	}
	if root.ResourceInfo != nil {
		currentInfo = root.ResourceInfo.Fields
	}

	return getInner(currentSchema, currentInfo, parts[2:])
}

// Given a fully typed path (e.g. data.simple_data_source.my_data.a_field) returns the pulumi name for that path.
func (s *scopes) pulumiName(fullyQualifiedPath string) string {
	info := s.getInfo(fullyQualifiedPath)

	// This should only be called for attribute paths, so panic if this returned a resource
	contract.Assertf(info.ResourceInfo == nil, "pulumiName called on a resource or data source")
	contract.Assertf(info.DataSourceInfo == nil, "pulumiName called on a resource or data source")

	// If we have a SchemaInfo and name use it
	schemaInfo := info.SchemaInfo
	if schemaInfo != nil && schemaInfo.Name != "" {
		return schemaInfo.Name
	}

	// If we have a shim schema use it to translate
	sch := info.Schema
	if sch != nil {
		return tfbridge.TerraformToPulumiNameV2(info.Name,
			schema.SchemaMap(map[string]shim.Schema{info.Name: sch}),
			map[string]*tfbridge.SchemaInfo{info.Name: schemaInfo})
	}

	// Else just return the name camel cased
	return camelCaseName(info.Name)
}

// Given a fully typed path (e.g. data.simple_data_source.my_data.a_field) returns if the schema says it's a map.
func (s *scopes) isMap(fullyQualifiedPath string) *bool {
	info := s.getInfo(fullyQualifiedPath)

	// This should only be called for attribute paths, so panic if this returned a resource
	contract.Assertf(info.ResourceInfo == nil, "isMap called on a resource or data source")
	contract.Assertf(info.DataSourceInfo == nil, "isMap called on a resource or data source")

	// If we have a shim schema use the type from that
	sch := info.Schema
	if sch != nil {
		isMap := sch.Type() == shim.TypeMap
		return &isMap
	}

	// If we have a Resource schema this must be an object
	if info.Resource != nil {
		isMap := false
		return &isMap
	}

	return nil
}

// Given a fully typed path (e.g. data.simple_data_source.a_field) returns whether a_field has maxItemsOne set
func (s *scopes) maxItemsOne(fullyQualifiedPath string) bool {
	info := s.getInfo(fullyQualifiedPath)

	// This should only be called for attribute paths, so panic if this returned a resource
	contract.Assertf(info.ResourceInfo == nil, "maxItemsOne called on a resource or data source")
	contract.Assertf(info.DataSourceInfo == nil, "maxItemsOne called on a resource or data source")

	// If we have a SchemaInfo and a MaxItems override use it
	schemaInfo := info.SchemaInfo
	if schemaInfo != nil && schemaInfo.MaxItemsOne != nil {
		return *schemaInfo.MaxItemsOne
	}

	// If we have a shim schema use it's MaxItems
	sch := info.Schema
	if sch != nil {
		return sch.MaxItems() == 1
	}

	// Else assume false
	return false
}

// Given a fully typed path (e.g. data.simple_data_source.a_field) returns whether a_field has Asset information set
func (s *scopes) isAsset(fullyQualifiedPath string) *tfbridge.AssetTranslation {
	info := s.getInfo(fullyQualifiedPath)

	// This should only be called for attribute paths, so panic if this returned a resource
	contract.Assertf(info.ResourceInfo == nil, "isAsset called on a resource or data source")
	contract.Assertf(info.DataSourceInfo == nil, "isAsset called on a resource or data source")

	// If we have a SchemaInfo and a asset info return that
	schemaInfo := info.SchemaInfo
	if schemaInfo != nil {
		return schemaInfo.Asset
	}

	return nil
}

// Helper function to call into the terraform evaluator
func (s *scopes) EvalExpr(expr hcl.Expression) (cty.Value, tfdiags.Diagnostics) {
	return s.scope.EvalExpr(expr, cty.DynamicPseudoType)
}

type diagnostic struct {
	severity tfdiags.Severity
	summary  string
	subject  *tfdiags.SourceRange
}

func (d diagnostic) Severity() tfdiags.Severity {
	return d.severity
}

func (d diagnostic) Description() tfdiags.Description {
	return tfdiags.Description{
		Summary: d.summary,
	}
}

func (d diagnostic) Source() tfdiags.Source {
	return tfdiags.Source{
		Subject: d.subject,
	}
}

func (d diagnostic) FromExpr() *tfdiags.FromExpr {
	return nil
}

func (d diagnostic) ExtraInfo() interface{} {
	return nil
}

func makeErrorDiagnostic(summary string, subject tfdiags.SourceRange) tfdiags.Diagnostics {
	return tfdiags.Diagnostics{diagnostic{
		severity: tfdiags.Error,
		summary:  summary,
		subject:  &subject,
	}}
}

// We implement a minimal subset of terraform/lang.Data so we can evaluate some fixed expressions.

func (s *scopes) StaticValidateReferences(refs []*addrs.Reference, self addrs.Referenceable) tfdiags.Diagnostics {
	return nil
}

func (s *scopes) GetCountAttr(_ addrs.CountAttr, src tfdiags.SourceRange) (cty.Value, tfdiags.Diagnostics) {
	return cty.NilVal, makeErrorDiagnostic("GetCountAttr is not supported", src)
}

func (s *scopes) GetForEachAttr(_ addrs.ForEachAttr, src tfdiags.SourceRange) (cty.Value, tfdiags.Diagnostics) {
	return cty.NilVal, makeErrorDiagnostic("GetForEachAttr is not supported", src)
}

func (s *scopes) GetResource(_ addrs.Resource, src tfdiags.SourceRange) (cty.Value, tfdiags.Diagnostics) {
	return cty.NilVal, makeErrorDiagnostic("GetResource is not supported", src)
}

func (s *scopes) GetLocalValue(addr addrs.LocalValue, src tfdiags.SourceRange) (cty.Value, tfdiags.Diagnostics) {
	// try and find the local and evaluate it
	var found *PathInfo
	for name, root := range s.roots {
		r := root
		if name == addr.String() {
			found = &r
			break
		}
	}
	if found == nil {
		return cty.NilVal, makeErrorDiagnostic("local not found", src)
	}

	val, diags := s.scope.EvalExpr(*found.Expression, cty.DynamicPseudoType)

	return val, diags
}

func (s *scopes) GetModule(_ addrs.ModuleCall, src tfdiags.SourceRange) (cty.Value, tfdiags.Diagnostics) {
	return cty.NilVal, makeErrorDiagnostic("GetCountAttr is not supported", src)
}

func (s *scopes) GetPathAttr(_ addrs.PathAttr, src tfdiags.SourceRange) (cty.Value, tfdiags.Diagnostics) {
	return cty.NilVal, makeErrorDiagnostic("GetPathAttr is not supported", src)
}

func (s *scopes) GetTerraformAttr(_ addrs.TerraformAttr, src tfdiags.SourceRange) (cty.Value, tfdiags.Diagnostics) {
	return cty.NilVal, makeErrorDiagnostic("GetTerraformAttr is not supported", src)
}

func (s *scopes) GetInputVariable(_ addrs.InputVariable, src tfdiags.SourceRange) (cty.Value, tfdiags.Diagnostics) {
	return cty.NilVal, makeErrorDiagnostic("GetInputVariable is not supported", src)
}
