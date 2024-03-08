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
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Working with this package requires knowing the tftypes.Object type approximation, and while it is normally available
// from the underlying Plugin Framework provider, in some test scenarios it is helpful to infer and compute it.
func InferObjectType(sm shim.SchemaMap) tftypes.Object {
	o := tftypes.Object{
		AttributeTypes:     map[string]tftypes.Type{},
		OptionalAttributes: map[string]struct{}{},
	}
	sm.Range(func(key string, value shim.Schema) bool {
		o.AttributeTypes[key] = InferType(value)
		if value.Optional() {
			o.OptionalAttributes[key] = struct{}{}
		}
		return true
	})
	return o
}

// Similar to [InferObjectType] but generalizes to all types.
func InferType(s shim.Schema) tftypes.Type {
	switch s.Type() {
	case shim.TypeInvalid:
		return nil // invalid type, how do we represent it?
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
			return tftypes.List{ElementType: nil} // unknown element type, how do we represent it?
		case shim.Schema:
			return tftypes.List{ElementType: InferType(elem)}
		case shim.Resource:
			return InferObjectType(elem.Schema())
		default:
			contract.Failf("unexpected Elem(): %#T", elem)
			return nil
		}
	case shim.TypeSet:
		switch elem := s.Elem().(type) {
		case nil:
			return tftypes.Set{ElementType: nil} // unknown element type, how do we represent it?
		case shim.Schema:
			return tftypes.Set{ElementType: InferType(elem)}
		case shim.Resource:
			return tftypes.Set{ElementType: InferObjectType(elem.Schema())}
		default:
			contract.Failf("unexpected Elem(): %#T", elem)
			return nil
		}
	case shim.TypeMap:
		switch elem := s.Elem().(type) {
		case nil:
			return tftypes.Map{ElementType: nil} // unknown element type, how do we represent it?
		case shim.Schema:
			return tftypes.Map{ElementType: InferType(elem)}
		case shim.Resource:
			return InferObjectType(elem.Schema()) // quirk: see docs on Elem(), single-nested block
		default:
			contract.Failf("unexpected Elem(): %#T", elem)
			return nil
		}
	default:
		contract.Failf("unexpected schema type %v", s.Type())
		return nil
	}
}
