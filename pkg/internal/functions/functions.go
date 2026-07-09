// Copyright 2016-2026, Pulumi Corporation.
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

// Package functions holds logic shared between build-time schema generation and the
// runtime bridge for Terraform provider-defined functions: the naming of positional
// arguments and the synthesis of shim schemas from the tftypes type constraints that
// describe function signatures.
//
// Function signatures carry no Terraform schema, but the bridge's naming
// ([tfbridge.TerraformToPulumiNameV2]) and value conversion ([pkg/convert]) machinery is
// schema-driven. Synthesizing a shim schema from the type constraints lets functions
// reuse both, so property naming and value conversion behave exactly like the rest of
// the bridge.
package functions

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

// ArgumentNames assigns a distinct Pulumi property name to each positional parameter of
// fn, including a trailing variadic parameter. Terraform treats parameter names as
// documentation-only, so empty and duplicate names are resolved deterministically.
//
// The returned names key the Pulumi schema's inputs object and multiArgumentInputs list,
// and the argument values a provider receives at runtime.
func ArgumentNames(fn shim.Function) []string {
	names := make([]string, 0, len(fn.Parameters)+1)
	seen := make(map[string]bool, len(fn.Parameters)+1)
	assign := func(raw string, position int) {
		base := ""
		if raw != "" {
			base = tfbridge.TerraformToPulumiNameV2(raw, nil, nil)
		}
		if base == "" {
			base = fmt.Sprintf("arg%d", position+1)
		}
		name := base
		for n := 2; seen[name]; n++ {
			name = fmt.Sprintf("%s%d", base, n)
		}
		seen[name] = true
		names = append(names, name)
	}
	for i, p := range fn.Parameters {
		assign(p.Name, i)
	}
	if v := fn.VariadicParameter; v != nil {
		assign(v.Name, len(fn.Parameters))
	}
	return names
}

// SchemaFromType synthesizes a shim schema mirroring the given type constraint, so that
// schema-driven naming and value conversion apply to provider function signatures.
func SchemaFromType(t tftypes.Type) (shim.Schema, error) {
	s := &schema.Schema{Optional: true}
	switch tt := t.(type) {
	case tftypes.List:
		element, err := SchemaFromType(tt.ElementType)
		if err != nil {
			return nil, err
		}
		s.Type, s.Elem = shim.TypeList, element
	case tftypes.Set:
		element, err := SchemaFromType(tt.ElementType)
		if err != nil {
			return nil, err
		}
		s.Type, s.Elem = shim.TypeSet, element
	case tftypes.Map:
		element, err := SchemaFromType(tt.ElementType)
		if err != nil {
			return nil, err
		}
		s.Type, s.Elem = shim.TypeMap, element
	case tftypes.Object:
		// A single-nested object is a TypeMap with a Resource element; see the
		// documentation on [shim.Schema.Elem].
		attrs, err := SchemaMapFromObject(tt)
		if err != nil {
			return nil, err
		}
		s.Type, s.Elem = shim.TypeMap, (&schema.Resource{Schema: attrs}).Shim()
	case tftypes.Tuple:
		// No provider function in the wild uses tuples, and Pulumi schema has no
		// equivalent type.
		return nil, fmt.Errorf("tuple types are not supported")
	default:
		switch {
		case t.Is(tftypes.String):
			s.Type = shim.TypeString
		case t.Is(tftypes.Bool):
			s.Type = shim.TypeBool
		case t.Is(tftypes.Number):
			s.Type = shim.TypeFloat
		case t.Is(tftypes.DynamicPseudoType):
			s.Type = shim.TypeDynamic
		default:
			return nil, fmt.Errorf("unsupported type %v", t)
		}
	}
	return s.Shim(), nil
}

// SchemaMapFromObject synthesizes a shim schema map for the attributes of an object type
// constraint. Passing the result to [tfbridge.TerraformToPulumiNameV2] yields the Pulumi
// property name of each attribute, with the same inflection rules as the rest of the
// bridge.
func SchemaMapFromObject(t tftypes.Object) (shim.SchemaMap, error) {
	m := schema.SchemaMap{}
	for attr, attrType := range t.AttributeTypes {
		s, err := SchemaFromType(attrType)
		if err != nil {
			return nil, fmt.Errorf("attribute %q: %w", attr, err)
		}
		m[attr] = s
	}
	return m, nil
}

// ObjectSchema describes a synthetic object over parts of a function signature, in the
// form the pkg/convert encoders and decoders consume.
type ObjectSchema struct {
	Type        tftypes.Object
	SchemaMap   shim.SchemaMap
	SchemaInfos map[string]*info.Schema
}

// ArgumentsSchema synthesizes an object over the positional arguments of fn, keyed and
// named by argNames (from [ArgumentNames]). A trailing variadic parameter appears as a
// list over its element type. Names are pinned exactly, since the schema's
// multiArgumentInputs list refers to them positionally.
func ArgumentsSchema(fn shim.Function, argNames []string) (ObjectSchema, error) {
	attrTypes := map[string]tftypes.Type{}
	schemaMap := schema.SchemaMap{}
	schemaInfos := map[string]*info.Schema{}
	add := func(name string, t tftypes.Type) error {
		s, err := SchemaFromType(t)
		if err != nil {
			return fmt.Errorf("argument %q: %w", name, err)
		}
		attrTypes[name] = t
		schemaMap[name] = s
		schemaInfos[name] = &info.Schema{Name: name}
		return nil
	}
	for i, p := range fn.Parameters {
		if err := add(argNames[i], p.Type); err != nil {
			return ObjectSchema{}, err
		}
	}
	if v := fn.VariadicParameter; v != nil {
		if err := add(argNames[len(fn.Parameters)], tftypes.List{ElementType: v.Type}); err != nil {
			return ObjectSchema{}, err
		}
	}
	return ObjectSchema{
		Type:        tftypes.Object{AttributeTypes: attrTypes},
		SchemaMap:   schemaMap,
		SchemaInfos: schemaInfos,
	}, nil
}

// ResultSchema synthesizes an object over the return value of fn.
//
// An object return decodes directly, with standard attribute naming. Any other return
// type wraps into a single property named by wrapProperty, matching the single-property
// map SDKs expect from invokes with a direct (non-object) return type.
func ResultSchema(fn shim.Function, wrapProperty string) (ObjectSchema, bool, error) {
	if obj, ok := fn.Return.(tftypes.Object); ok {
		m, err := SchemaMapFromObject(obj)
		if err != nil {
			return ObjectSchema{}, false, err
		}
		return ObjectSchema{Type: obj, SchemaMap: m}, true, nil
	}
	s, err := SchemaFromType(fn.Return)
	if err != nil {
		return ObjectSchema{}, false, err
	}
	return ObjectSchema{
		Type:        tftypes.Object{AttributeTypes: map[string]tftypes.Type{wrapProperty: fn.Return}},
		SchemaMap:   schema.SchemaMap{wrapProperty: s},
		SchemaInfos: map[string]*info.Schema{wrapProperty: {Name: wrapProperty}},
	}, false, nil
}
