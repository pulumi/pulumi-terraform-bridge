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
// arguments and object attributes, and the conversion between Pulumi property values and
// Terraform values for the tftypes type constraints that describe function signatures.
package functions

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
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

// PropertyName translates a Terraform object attribute name into its Pulumi property
// name. The translation applies no schema-based inflection (such as pluralization), so
// it is invertible via [AttributeName].
func PropertyName(attr string) string {
	return tfbridge.TerraformToPulumiNameV2(attr, nil, nil)
}

// AttributeName is the inverse of [PropertyName].
func AttributeName(property string) string {
	return tfbridge.PulumiToTerraformName(property, nil, nil)
}

// SortedAttributeNames returns the attribute names of t in a stable order.
func SortedAttributeNames(t tftypes.Object) []string {
	attrs := make([]string, 0, len(t.AttributeTypes))
	for attr := range t.AttributeTypes {
		attrs = append(attrs, attr)
	}
	sort.Strings(attrs)
	return attrs
}

// EncodeValue converts a Pulumi property value into a Terraform value of the given type
// constraint. Secret and output wrappers must be removed before calling; unknown values
// are rejected, since function arguments are always known.
func EncodeValue(t tftypes.Type, pv resource.PropertyValue) (tftypes.Value, error) {
	switch {
	case pv.IsComputed() || pv.IsOutput() && !pv.OutputValue().Known:
		return tftypes.Value{}, fmt.Errorf("unexpected unknown value")
	case pv.IsSecret():
		return EncodeValue(t, pv.SecretValue().Element)
	case pv.IsOutput():
		return EncodeValue(t, pv.OutputValue().Element)
	case pv.IsNull():
		return tftypes.NewValue(t, nil), nil
	}

	if t.Is(tftypes.DynamicPseudoType) {
		return encodeDynamic(pv)
	}

	switch t := t.(type) {
	case tftypes.List:
		vals, err := encodeElements(t.ElementType, pv)
		if err != nil {
			return tftypes.Value{}, err
		}
		return tftypes.NewValue(t, vals), nil
	case tftypes.Set:
		vals, err := encodeElements(t.ElementType, pv)
		if err != nil {
			return tftypes.Value{}, err
		}
		return tftypes.NewValue(t, vals), nil
	case tftypes.Map:
		if !pv.IsObject() {
			return tftypes.Value{}, fmt.Errorf("expected a map, got %v", pv.TypeString())
		}
		vals := map[string]tftypes.Value{}
		for k, v := range pv.ObjectValue() {
			// Map keys are data, not property names; they are not renamed.
			ev, err := EncodeValue(t.ElementType, v)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf("[%q]: %w", string(k), err)
			}
			vals[string(k)] = ev
		}
		return tftypes.NewValue(t, vals), nil
	case tftypes.Object:
		if !pv.IsObject() {
			return tftypes.Value{}, fmt.Errorf("expected an object, got %v", pv.TypeString())
		}
		obj := pv.ObjectValue()
		known := map[resource.PropertyKey]bool{}
		vals := map[string]tftypes.Value{}
		for attr, attrType := range t.AttributeTypes {
			key := resource.PropertyKey(PropertyName(attr))
			known[key] = true
			v, has := obj[key]
			if !has {
				v = resource.NewNullProperty()
			}
			ev, err := EncodeValue(attrType, v)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf(".%s: %w", key, err)
			}
			vals[attr] = ev
		}
		for k := range obj {
			if !known[k] {
				return tftypes.Value{}, fmt.Errorf("unexpected property %q", string(k))
			}
		}
		return tftypes.NewValue(t, vals), nil
	case tftypes.Tuple:
		return tftypes.Value{}, fmt.Errorf("tuple types are not supported")
	}

	switch {
	case t.Is(tftypes.String):
		if !pv.IsString() {
			return tftypes.Value{}, fmt.Errorf("expected a string, got %v", pv.TypeString())
		}
		return tftypes.NewValue(t, pv.StringValue()), nil
	case t.Is(tftypes.Bool):
		if !pv.IsBool() {
			return tftypes.Value{}, fmt.Errorf("expected a boolean, got %v", pv.TypeString())
		}
		return tftypes.NewValue(t, pv.BoolValue()), nil
	case t.Is(tftypes.Number):
		if !pv.IsNumber() {
			return tftypes.Value{}, fmt.Errorf("expected a number, got %v", pv.TypeString())
		}
		return tftypes.NewValue(t, pv.NumberValue()), nil
	}
	return tftypes.Value{}, fmt.Errorf("unsupported type %v", t)
}

func encodeElements(elementType tftypes.Type, pv resource.PropertyValue) ([]tftypes.Value, error) {
	if !pv.IsArray() {
		return nil, fmt.Errorf("expected a list, got %v", pv.TypeString())
	}
	arr := pv.ArrayValue()
	vals := make([]tftypes.Value, len(arr))
	for i, e := range arr {
		ev, err := EncodeValue(elementType, e)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		vals[i] = ev
	}
	return vals, nil
}

// encodeDynamic performs a best-effort conversion for DynamicPseudoType constraints at
// the data level, inferring a concrete Terraform type from the value. Object property
// names are passed through unmodified, mirroring the dynamic encoding used elsewhere in
// the bridge.
func encodeDynamic(pv resource.PropertyValue) (tftypes.Value, error) {
	switch {
	case pv.IsNull():
		return tftypes.NewValue(tftypes.DynamicPseudoType, nil), nil
	case pv.IsBool():
		return tftypes.NewValue(tftypes.Bool, pv.BoolValue()), nil
	case pv.IsNumber():
		return tftypes.NewValue(tftypes.Number, pv.NumberValue()), nil
	case pv.IsString():
		return tftypes.NewValue(tftypes.String, pv.StringValue()), nil
	case pv.IsArray():
		vals := make([]tftypes.Value, 0, len(pv.ArrayValue()))
		for i, e := range pv.ArrayValue() {
			ev, err := encodeDynamic(e)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf("[%d]: %w", i, err)
			}
			vals = append(vals, ev)
		}
		// Elements of a Terraform list must share one type; fall back to a tuple for
		// heterogeneous arrays.
		if commonType, err := tftypes.TypeFromElements(vals); err == nil {
			return tftypes.NewValue(tftypes.List{ElementType: commonType}, vals), nil
		}
		types := make([]tftypes.Type, len(vals))
		for i, v := range vals {
			types[i] = v.Type()
		}
		return tftypes.NewValue(tftypes.Tuple{ElementTypes: types}, vals), nil
	case pv.IsObject():
		objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
		vals := map[string]tftypes.Value{}
		for k, v := range pv.ObjectValue() {
			ev, err := encodeDynamic(v)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf(".%s: %w", string(k), err)
			}
			vals[string(k)] = ev
			objType.AttributeTypes[string(k)] = ev.Type()
		}
		return tftypes.NewValue(objType, vals), nil
	case pv.IsSecret():
		return encodeDynamic(pv.SecretValue().Element)
	case pv.IsOutput() && pv.OutputValue().Known:
		return encodeDynamic(pv.OutputValue().Element)
	default:
		return tftypes.Value{}, fmt.Errorf("cannot encode %v as a dynamic value", pv.TypeString())
	}
}

// DecodeValue converts a Terraform value of the given type constraint back into a Pulumi
// property value. When the constraint is DynamicPseudoType the value decodes by its own
// concrete type, with object property names passed through unmodified.
func DecodeValue(t tftypes.Type, v tftypes.Value) (resource.PropertyValue, error) {
	if !v.IsKnown() {
		return resource.PropertyValue{}, fmt.Errorf("unexpected unknown value")
	}
	if v.IsNull() {
		return resource.NewPropertyValue(nil), nil
	}

	if t.Is(tftypes.DynamicPseudoType) {
		return decodeDynamic(v)
	}

	switch t := t.(type) {
	case tftypes.List:
		return decodeElements(t.ElementType, v)
	case tftypes.Set:
		return decodeElements(t.ElementType, v)
	case tftypes.Map:
		var elements map[string]tftypes.Value
		if err := v.As(&elements); err != nil {
			return resource.PropertyValue{}, err
		}
		result := make(resource.PropertyMap, len(elements))
		for k, e := range elements {
			ev, err := DecodeValue(t.ElementType, e)
			if err != nil {
				return resource.PropertyValue{}, fmt.Errorf("[%q]: %w", k, err)
			}
			result[resource.PropertyKey(k)] = ev
		}
		return resource.NewObjectProperty(result), nil
	case tftypes.Object:
		var attrs map[string]tftypes.Value
		if err := v.As(&attrs); err != nil {
			return resource.PropertyValue{}, err
		}
		result := make(resource.PropertyMap, len(attrs))
		for attr, e := range attrs {
			attrType, ok := t.AttributeTypes[attr]
			if !ok {
				return resource.PropertyValue{}, fmt.Errorf("unexpected attribute %q", attr)
			}
			ev, err := DecodeValue(attrType, e)
			if err != nil {
				return resource.PropertyValue{}, fmt.Errorf(".%s: %w", attr, err)
			}
			result[resource.PropertyKey(PropertyName(attr))] = ev
		}
		return resource.NewObjectProperty(result), nil
	case tftypes.Tuple:
		return resource.PropertyValue{}, fmt.Errorf("tuple types are not supported")
	}

	switch {
	case t.Is(tftypes.String):
		var s string
		if err := v.As(&s); err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewStringProperty(s), nil
	case t.Is(tftypes.Bool):
		var b bool
		if err := v.As(&b); err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewBoolProperty(b), nil
	case t.Is(tftypes.Number):
		var n big.Float
		if err := v.As(&n); err != nil {
			return resource.PropertyValue{}, err
		}
		f, _ := n.Float64()
		return resource.NewNumberProperty(f), nil
	}
	return resource.PropertyValue{}, fmt.Errorf("unsupported type %v", t)
}

func decodeElements(elementType tftypes.Type, v tftypes.Value) (resource.PropertyValue, error) {
	var elements []tftypes.Value
	if err := v.As(&elements); err != nil {
		return resource.PropertyValue{}, err
	}
	result := make([]resource.PropertyValue, len(elements))
	for i, e := range elements {
		ev, err := DecodeValue(elementType, e)
		if err != nil {
			return resource.PropertyValue{}, fmt.Errorf("[%d]: %w", i, err)
		}
		result[i] = ev
	}
	return resource.NewArrayProperty(result), nil
}

// decodeDynamic decodes a value by its own concrete type, without renaming object
// property names.
func decodeDynamic(v tftypes.Value) (resource.PropertyValue, error) {
	switch {
	case !v.IsKnown():
		return resource.PropertyValue{}, fmt.Errorf("unexpected unknown value")
	case v.IsNull():
		return resource.NewPropertyValue(nil), nil
	case v.Type().Is(tftypes.Bool):
		var b bool
		if err := v.As(&b); err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewBoolProperty(b), nil
	case v.Type().Is(tftypes.Number):
		var n big.Float
		if err := v.As(&n); err != nil {
			return resource.PropertyValue{}, err
		}
		f, _ := n.Float64()
		return resource.NewNumberProperty(f), nil
	case v.Type().Is(tftypes.String):
		var s string
		if err := v.As(&s); err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewStringProperty(s), nil
	case v.Type().Is(tftypes.List{}), v.Type().Is(tftypes.Set{}), v.Type().Is(tftypes.Tuple{}):
		var elements []tftypes.Value
		if err := v.As(&elements); err != nil {
			return resource.PropertyValue{}, err
		}
		result := make([]resource.PropertyValue, len(elements))
		for i, e := range elements {
			ev, err := decodeDynamic(e)
			if err != nil {
				return resource.PropertyValue{}, fmt.Errorf("[%d]: %w", i, err)
			}
			result[i] = ev
		}
		return resource.NewArrayProperty(result), nil
	case v.Type().Is(tftypes.Map{}), v.Type().Is(tftypes.Object{}):
		var elements map[string]tftypes.Value
		if err := v.As(&elements); err != nil {
			return resource.PropertyValue{}, err
		}
		result := make(resource.PropertyMap, len(elements))
		for k, e := range elements {
			ev, err := decodeDynamic(e)
			if err != nil {
				return resource.PropertyValue{}, fmt.Errorf(".%s: %w", k, err)
			}
			result[resource.PropertyKey(k)] = ev
		}
		return resource.NewObjectProperty(result), nil
	default:
		return resource.PropertyValue{}, fmt.Errorf("unsupported type %v", v.Type())
	}
}
