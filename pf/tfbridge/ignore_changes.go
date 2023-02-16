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
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/convert"
)

// Implements support for ignoreChanges property.
//
// There is some complexity here since Plugin Framework returns diffs in terms of Terraform paths, so the implementation
// tries to translate TF paths to Pulumi paths to decide if they match.
//
// See also https://www.pulumi.com/docs/intro/concepts/resources/options/ignorechanges/
type ignoreChanges struct {
	// Delegate syntax parsing to resource.PropertyPath.
	paths []resource.PropertyPath
	ss    schemaStepper
}

func newIgnoreChanges(
	schema *schema.PackageSpec,
	token tokens.Token,
	renames convert.PropertyNames,
	rawIgnoreChanges []string,
) (*ignoreChanges, error) {
	var paths []resource.PropertyPath
	for _, p := range rawIgnoreChanges {
		pp, err := resource.ParsePropertyPath(p)
		if err != nil {
			return nil, err
		}
		paths = append(paths, pp)
	}
	return &ignoreChanges{
		paths: paths,
		ss: &namedEntitySchemaStepper{
			schema: schema,
			token:  token,
			// Another complication is that Pulumi renames the properties, this uses the Renames framework
			// to track the name tables.
			renames: renames,
		},
	}, nil
}

func (ic *ignoreChanges) IsIgnored(ap *tftypes.AttributePath) bool {
	for _, p := range ic.paths {
		if ic.isIgnoredBy(p, ap) {
			return true
		}
	}
	return false
}

func (ic *ignoreChanges) isIgnoredBy(pattern resource.PropertyPath, ap *tftypes.AttributePath) bool {
	mc := &matchingContext{
		remainingPattern: pattern,
		schemaStepper:    ic.ss,
	}
	match, remain, err := tftypes.WalkAttributePath(mc, ap)
	if err != nil {
		return false
	}
	if remain != nil && len(remain.Steps()) > 0 {
		return false
	}
	if m, ok := match.(*matchingContext); ok {
		if !m.matchFailed {
			return true
		}
	}
	return false
}

// All the helper infrastructure below including schemaStepper, typeSchemaStepper and namedEntitySchemaStepper simply
// support a recursive algorithm from ignoreChanges.isIgnoredBy. This algorithm may have a simpler expression as a set
// of recursive function calls, but using types allows the code to use tftypes.WalkAttributePath to actually recur.
//
// The types are really contexts of the traversal of an AttributePath that is being matched segment-wise against a
// PropertyPath pattern. As the algo drills down the path, it needs to keep track of where it is in the schema so that
// it can find the right Property rename metadata.
//
// See ignore_changes_test.go for test cases that define what the matching should do.
type schemaStepper interface {
	Property(resource.PropertyKey) schemaStepper
	Element() schemaStepper
	PropertyKey(convert.TerraformPropertyName) resource.PropertyKey
}

// Matching names at an intermediary type such as List[T].
type typeSchemaStepper struct {
	renames convert.PropertyNames
	schema  *schema.PackageSpec
	t       *schema.TypeSpec
}

var _ schemaStepper = (*typeSchemaStepper)(nil)

func (p *typeSchemaStepper) Element() schemaStepper {
	if p.t.AdditionalProperties != nil {
		return typeStepper(p.renames, p.schema, p.t.AdditionalProperties)
	}
	if p.t.Items != nil {
		return typeStepper(p.renames, p.schema, p.t.Items)
	}
	return nil
}

func (*typeSchemaStepper) Property(k resource.PropertyKey) schemaStepper {
	// there are no named properties here, those are supported by namedEntitySchemaStepper
	return nil
}

func (p *typeSchemaStepper) PropertyKey(n convert.TerraformPropertyName) resource.PropertyKey {
	// translate verbatim, no tables
	return resource.PropertyKey(tokens.Name(n))
}

// Matching names at a Resource or named object type.
type namedEntitySchemaStepper struct {
	schema  *schema.PackageSpec
	token   tokens.Token
	renames convert.PropertyNames
}

func (r *namedEntitySchemaStepper) Property(k resource.PropertyKey) schemaStepper {
	// If this is a Resource..
	if rr, ok := r.schema.Resources[string(r.token)]; ok {
		// Only InputProperties here, not Properties because the docs state that:
		//
		//     The ignoreChanges option only applies to resource inputs, not outputs.
		//
		// See https://www.pulumi.com/docs/intro/concepts/resources/options/ignorechanges/
		if p, ok := rr.InputProperties[string(k)]; ok {
			return typeStepper(r.renames, r.schema, &p.TypeSpec)
		}
	}
	// If this is a named type..
	if tt, ok := r.schema.Types[string(r.token)]; ok {
		if p, ok := tt.Properties[string(k)]; ok {
			return typeStepper(r.renames, r.schema, &p.TypeSpec)
		}
	}
	return nil
}

func (*namedEntitySchemaStepper) Element() schemaStepper {
	// list or map element path does not make sense for resources, data sources, named object types
	return nil
}

func (r *namedEntitySchemaStepper) PropertyKey(n convert.TerraformPropertyName) resource.PropertyKey {
	return r.renames.PropertyKey(r.token, n, nil)
}

func typeStepper(renames convert.PropertyNames, s *schema.PackageSpec, t *schema.TypeSpec) schemaStepper {
	// properties may refer to named object types via refs
	if t.Ref != "" {
		// understand local type refs
		if strings.HasPrefix(t.Ref, "#/types/") {
			tok := strings.TrimPrefix(t.Ref, "#/types/")
			// dangling refs not supported
			if _, ok := s.Types[tok]; !ok {
				return nil
			}
			return &namedEntitySchemaStepper{
				schema:  s,
				token:   tokens.Token(tok),
				renames: renames,
			}
		}

		// According to https://www.pulumi.com/docs/guides/pulumi-packages/schema/#type there may be external
		// references also like "/aws/v3.30.0/schema.json#/resources/aws:lambda%2Ffunction:Function".
		//
		// The current implementation uses Renames tables under the hood to understand Pulumi-TF property name
		// mapping, but currently does not know how to lookup the tables across providers. Because of this
		// limitation, ignoreChanges bails here and does not work for ignoring changes over cross-package
		// imported types.
		return nil
	}
	return &typeSchemaStepper{
		renames: renames,
		schema:  s,
		t:       t,
	}
}

var _ schemaStepper = (*namedEntitySchemaStepper)(nil)

type matchingContext struct {
	schemaStepper    schemaStepper
	remainingPattern resource.PropertyPath
	matchFailed      bool
}

func (*matchingContext) fail() *matchingContext {
	return &matchingContext{matchFailed: true}
}

func (*matchingContext) pulumiNameMatches(pattern interface{}, puName resource.PropertyKey) bool {
	if p, ok := pattern.(string); ok {
		if p == "*" {
			return true
		}
		if p == string(puName) {
			return true
		}
	}
	return false
}

func (*matchingContext) rawNameMatches(pattern interface{}, rawName string) bool {
	if p, ok := pattern.(string); ok {
		if p == "*" {
			return true
		}
		if p == rawName {
			return true
		}
	}
	return false
}

func (*matchingContext) intMatches(pattern interface{}, n int64) bool {
	if p, ok := pattern.(string); ok {
		if p == "*" {
			return true
		}
	}
	if p, ok := pattern.(int); ok {
		if int64(p) == n {
			return true
		}
	}
	return false
}

func (mc *matchingContext) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (interface{}, error) {
	switch s := step.(type) {
	case tftypes.AttributeName:
		if len(mc.remainingPattern) == 0 {
			return mc.fail(), nil
		}
		tfName := convert.TerraformPropertyName(s)
		puName := mc.schemaStepper.PropertyKey(tfName)
		if !mc.pulumiNameMatches(mc.remainingPattern[0], puName) {
			return mc.fail(), nil
		}
		down := mc.schemaStepper.Property(puName)
		if down == nil {
			return mc.fail(), nil
		}
		return &matchingContext{
			schemaStepper:    down,
			remainingPattern: mc.remainingPattern[1:],
		}, nil
	case tftypes.ElementKeyString:
		if len(mc.remainingPattern) == 0 {
			return mc.fail(), nil
		}
		rawName := string(s)
		if !mc.rawNameMatches(mc.remainingPattern[0], rawName) {
			return mc.fail(), nil
		}
		down := mc.schemaStepper.Element()
		if down == nil {
			return mc.fail(), nil
		}
		return &matchingContext{
			schemaStepper:    down,
			remainingPattern: mc.remainingPattern[1:],
		}, nil
	case tftypes.ElementKeyInt:
		if len(mc.remainingPattern) == 0 {
			return mc.fail(), nil
		}
		i := int64(s)
		if !mc.intMatches(mc.remainingPattern[0], i) {
			return mc.fail(), nil
		}
		down := mc.schemaStepper.Element()
		if down == nil {
			return mc.fail(), nil
		}
		return &matchingContext{
			schemaStepper:    down,
			remainingPattern: mc.remainingPattern[1:],
		}, nil
	case tftypes.ElementKeyValue:
		// ignoreChanges not supported yet for set elements
		return mc.fail(), nil
	}
	return mc, nil
}

var _ tftypes.AttributePathStepper = (*matchingContext)(nil)
