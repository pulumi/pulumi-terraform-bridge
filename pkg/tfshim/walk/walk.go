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

package walk

import (
	"bytes"
	"fmt"
	"strings"

	hcty "github.com/hashicorp/go-cty/cty"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

// Represents locations in a tfshim.Schema value as a sequence of steps to locate it.
//
// An empty SchemaPath represents the current location.
//
// Values of this type are immutable by convention, use Copy as necessary for local mutations.
type SchemaPath []SchemaPathStep

func (p SchemaPath) GoString() string {
	parts := []string{"walk", "NewSchemaPath()"}
	for _, step := range p {
		switch s := step.(type) {
		case ElementStep:
			parts = append(parts, "Element()")
		case GetAttrStep:
			parts = append(parts, fmt.Sprintf("GetAttr(%q)", s.Name))
		}
	}
	return strings.Join(parts, ".")
}

func (p SchemaPath) Copy() SchemaPath {
	ret := make(SchemaPath, len(p))
	copy(ret, p)
	return ret
}

func (p SchemaPath) Element() SchemaPath {
	return p.WithStep(ElementStep{})
}

func (p SchemaPath) GetAttr(name string) SchemaPath {
	return p.WithStep(GetAttrStep{name})
}

func (p SchemaPath) WithStep(suffix SchemaPathStep) SchemaPath {
	ret := make(SchemaPath, len(p)+1)
	copy(ret, p)
	ret[len(p)] = suffix
	return ret
}

func (p SchemaPath) EncodeSchemaPath() (string, error) {
	var buf bytes.Buffer
	for i, step := range p {
		if i > 0 {
			fmt.Fprintf(&buf, ".")
		}
		switch step := step.(type) {
		case ElementStep:
			fmt.Fprintf(&buf, "$")
		case GetAttrStep:
			if strings.Contains(step.Name, ".") {
				return "", fmt.Errorf("Cannot encode SchemaPath %q containing '.'", step.Name)
			}
			if step.Name == "$" {
				return "", fmt.Errorf("Cannot encode SchemaPath %q", step.Name)
			}
			fmt.Fprintf(&buf, step.Name)
		default:
			contract.Failf("impossible")
		}
	}
	return buf.String(), nil
}

func (p SchemaPath) MustEncodeSchemaPath() string {
	s, err := p.EncodeSchemaPath()
	contract.AssertNoErrorf(err, "Unexpected SchemaPath encoding error")
	return s
}

func DecodeSchemaPath(path string) SchemaPath {
	p := NewSchemaPath()
	if path == "" {
		return p
	}
	for _, frag := range strings.Split(path, ".") {
		if frag == "$" {
			p = p.Element()
		} else {
			p = p.GetAttr(frag)
		}
	}
	return p
}

// Builds a new empty SchemaPath.
func NewSchemaPath() SchemaPath {
	return make(SchemaPath, 0)
}

// Finds a nested Schema at a given path.
func LookupSchemaPath(path SchemaPath, schema shim.Schema) (shim.Schema, error) {
	p := path
	current := NewSchemaPath()
	result := schema
	for {
		if len(p) == 0 {
			return result, nil
		}
		nextResult, err := p[0].Lookup(result)
		if err != nil {
			return nil, fmt.Errorf("LookupSchemaPath failed at %s: %w", current.GoString(), err)
		}
		result, p, current = nextResult, p[1:], current.WithStep(p[0])
	}
}

// Similar to LookupSchemaPath but starts the initial step from a SchemaMap.
func LookupSchemaMapPath(path SchemaPath, schemaMap shim.SchemaMap) (shim.Schema, error) {
	return LookupSchemaPath(path, wrapSchemaMap(schemaMap))
}

// Represents elements of a SchemaPath.
//
// This interface is closed, the only the implementations given in the current package are allowed.
type SchemaPathStep interface {
	isSchemaPathStep()

	GoString() string
	Lookup(shim.Schema) (shim.Schema, error)
}

// Drill down into an attribute by the given attribute name.
type GetAttrStep struct {
	Name string
}

func (GetAttrStep) isSchemaPathStep() {}

func (step GetAttrStep) GoString() string {
	return fmt.Sprintf("walk.GetAttrStep{%q}", step.Name)
}

func (step GetAttrStep) Lookup(s shim.Schema) (shim.Schema, error) {
	if sm, ok := unwrapSchemaMap(s); ok {
		s, found := sm.GetOk(step.Name)
		if !found {
			return nil, fmt.Errorf("%s not found", step.GoString())
		}
		return s, nil
	}
	return nil, fmt.Errorf("%s is not applicable", step.GoString())
}

// Drill down into a Map, Set or List element schema.
type ElementStep struct{}

func (ElementStep) isSchemaPathStep() {}

func (step ElementStep) GoString() string {
	return "walk.ElementStep{}"
}

func (step ElementStep) Lookup(s shim.Schema) (shim.Schema, error) {
	switch elem := s.Elem().(type) {
	case shim.Resource:
		switch s.Type() {
		case shim.TypeMap:
			return nil, fmt.Errorf("%s is not applicable to object types", step.GoString())
		case shim.TypeList, shim.TypeSet:
			return wrapSchemaMap(elem.Schema()), nil
		default:
			return nil, fmt.Errorf("%s is not applicable", step.GoString())
		}
	case shim.Schema:
		return elem, nil
	default:
		return nil, fmt.Errorf("%s is not applicable", step.GoString())
	}
}

func wrapSchemaMap(sm shim.SchemaMap) shim.Schema {
	return (&schema.Schema{
		Type: shim.TypeMap,
		Elem: (&schema.Resource{Schema: sm}).Shim(),
	}).Shim()
}

// Utility function to recognize nested object field type schemas encoded in shim.Schema.
func unwrapSchemaMap(s shim.Schema) (shim.SchemaMap, bool) {
	switch elem := s.Elem().(type) {
	case shim.Resource:
		return elem.Schema(), true
	default:
		return nil, false
	}
}

type SchemaVisitor = func(SchemaPath, shim.Schema)

// Visit all nested schemas, including the current one.
func VisitSchema(schema shim.Schema, visitor SchemaVisitor) {
	visitSchemaInner(NewSchemaPath(), schema, visitor)
}

// Visit all nested schemas in a SchemaMap, keeping track of SchemaPath location.
func VisitSchemaMap(schemaMap shim.SchemaMap, visitor SchemaVisitor) {
	visitSchemaMapInner(NewSchemaPath(), schemaMap, visitor)
}

func visitSchemaInner(path SchemaPath, schema shim.Schema, visitor SchemaVisitor) {
	visitor(path, schema)
	switch elem := schema.Elem().(type) {
	case shim.Resource:
		var nestedPath SchemaPath
		if schema.Type() == shim.TypeMap {
			// Single-nested blocks are special, drilling down into the elements of the block's object type
			// can begin immediately without an Element step.
			nestedPath = path
		} else {
			nestedPath = path.Element()
		}
		visitSchemaMapInner(nestedPath, elem.Schema(), visitor)
	case shim.Schema:
		visitSchemaInner(path.Element(), elem, visitor)
	}
}

func visitSchemaMapInner(path SchemaPath, schemaMap shim.SchemaMap, visitor SchemaVisitor) {
	schemaMap.Range(func(key string, schema shim.Schema) bool {
		visitSchemaInner(path.GetAttr(key), schema, visitor)
		return true
	})
}

// Converts a value path to a Schema Path (zclconf package representation).
func FromCtyPath(path cty.Path) SchemaPath {
	p := NewSchemaPath()
	for _, subPath := range path {
		switch s := subPath.(type) {
		case cty.IndexStep:
			p = p.Element()
		case cty.GetAttrStep:
			p = p.GetAttr(s.Name)
		}
	}
	return p
}

// Converts a value path to a Schema Path (hashicorp/go-cty representation).
func FromHCtyPath(path hcty.Path) SchemaPath {
	p := NewSchemaPath()
	for _, subPath := range path {
		switch s := subPath.(type) {
		case hcty.IndexStep:
			p = p.Element()
		case hcty.GetAttrStep:
			p = p.GetAttr(s.Name)
		}
	}
	return p
}
