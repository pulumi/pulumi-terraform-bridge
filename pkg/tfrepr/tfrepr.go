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

// A package for converting between tf's value type: tftypes.Value and tfshim's types.
package tfrepr

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func NewNull(t shim.Schema) tftypes.Value {
	return tftypes.NewValue(typeOf(t), nil)
}

func NewUnknown(t shim.Schema) tftypes.Value {
	return tftypes.NewValue(typeOf(t), tftypes.UnknownValue)
}

func ObjectType(t shim.SchemaMap) tftypes.Object {
	return typeOfObject(t)
}

func typeOf(t shim.Schema) tftypes.Type {
	if t == nil {
		return nil
	}

	// Primitive types
	switch t.Type() {
	case shim.TypeBool:
		return tftypes.Bool
	case shim.TypeInt, shim.TypeFloat:
		return tftypes.Number
	case shim.TypeString:
		return tftypes.String
	case shim.TypeList:
		return tftypes.List{ElementType: typeOfElem(t.Elem())}
	case shim.TypeMap:
		return tftypes.Map{ElementType: typeOfElem(t.Elem())}
	case shim.TypeSet:
		return tftypes.Set{ElementType: typeOfElem(t.Elem())}
	default:
		return tftypes.DynamicPseudoType
	}
}

func typeOfElem(elem any) tftypes.Type {
	switch elem := elem.(type) {
	case shim.Schema:
		return typeOf(elem)
	case shim.Resource:
		return typeOfObject(elem.Schema())
	default:
		return tftypes.DynamicPseudoType
	}
}

func typeOfObject(s shim.SchemaMap) tftypes.Object {
	attrs := make(map[string]tftypes.Type, s.Len())
	optionals := make(map[string]struct{})

	s.Range(func(key string, v shim.Schema) bool {
		attrs[key] = typeOf(v)
		if v.Optional() {
			optionals[key] = struct{}{}
		}
		return true
	})

	return tftypes.Object{
		AttributeTypes:     attrs,
		OptionalAttributes: optionals,
	}
}
