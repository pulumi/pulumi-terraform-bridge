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

package convert

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type InferObjectTypeOptions struct{}

// Working with this package requires knowing the tftypes.Object type approximation, and while it is normally available
// from the underlying Plugin Framework provider, in some test scenarios it is helpful to infer and compute it.
func InferObjectType(sm shim.SchemaMap, opts *InferObjectTypeOptions) tftypes.Object {
	o := tftypes.Object{
		AttributeTypes:     map[string]tftypes.Type{},
		OptionalAttributes: map[string]struct{}{},
	}
	sm.Range(func(key string, value shim.Schema) bool {
		o.AttributeTypes[key] = InferType(value, opts)
		// Looks like the use cases for this module do not accept values that also infer o.OptionalAttributes
		// from schema for the moment, so continue ignoring that.
		return true
	})
	return o
}

// Not clear how to best represent an invalid or unknown type, going for an empty object type.
func invalidType() tftypes.Type {
	return tftypes.Object{}
}

// Similar to [InferObjectType] but generalizes to all types.
func InferType(s shim.Schema, opts *InferObjectTypeOptions) tftypes.Type {
	switch s.Type() {
	case shim.TypeInvalid:
		return invalidType()
	case shim.TypeBool:
		return tftypes.Bool
	case shim.TypeInt:
		return tftypes.Number
	case shim.TypeFloat:
		return tftypes.Number
	case shim.TypeString:
		return tftypes.String
	case shim.TypeList:
		switch elem := s.Elem().(type) {
		case nil:
			return tftypes.List{ElementType: invalidType()}
		case shim.Schema:
			return tftypes.List{ElementType: InferType(elem, opts)}
		case shim.Resource:
			return tftypes.List{ElementType: InferObjectType(elem.Schema(), opts)}
		default:
			contract.Failf("unexpected Elem(): %#T", elem)
			return nil
		}
	case shim.TypeSet:
		switch elem := s.Elem().(type) {
		case nil:
			return tftypes.Set{ElementType: invalidType()}
		case shim.Schema:
			return tftypes.Set{ElementType: InferType(elem, opts)}
		case shim.Resource:
			return tftypes.Set{ElementType: InferObjectType(elem.Schema(), opts)}
		default:
			contract.Failf("unexpected Elem(): %#T", elem)
			return nil
		}
	case shim.TypeMap:
		switch elem := s.Elem().(type) {
		case nil:
			return tftypes.Map{ElementType: invalidType()}
		case shim.Schema:
			return tftypes.Map{ElementType: InferType(elem, opts)}
		case shim.Resource:
			return InferObjectType(elem.Schema(), opts) // quirk: see docs on Elem(), single-nested block
		default:
			contract.Failf("unexpected Elem(): %#T", elem)
			return nil
		}
	case shim.TypeDynamic:
		return tftypes.DynamicPseudoType
	default:
		contract.Failf("unexpected schema type %v", s.Type())
		return nil
	}
}
