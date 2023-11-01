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

// TF has nominal typing, which precludes recursive types. To model recursive types, TF
// unrolls the recursion up to some nesting value.
//
// Pulumi has recursive types, and doesn't deal well with the number of types generated
// during manual unrolling. This file provides code to re-roll an unrolled recursive type.

import (
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func (prov *ProviderInfo) FixRecursiveResource(tfToken string) {
	info := prov.Resources[tfToken]
	if info == nil {
		info = &ResourceInfo{}
		prov.Resources[tfToken] = info
	}
	fixRecursiveResource(prov.P.ResourcesMap().Get(tfToken).Schema(), info)
}

func fixRecursiveResource(tfSchema shim.SchemaMap, info *ResourceInfo) {
	if info.Fields == nil {
		info.Fields = make(map[string]*SchemaInfo)
	}
	tfSchema.Range(func(k string, inner shim.Schema) bool {

	})
}

// A type is similar to another type if:
//
// 1. They match on scalar fields, and
// 2. Object fields are themselves similar, or
// 3. One type is missing an object field that the other type has (the base case).
func isRecursionOf(outer, inner shim.SchemaMap) bool {
	s := make(map[string]struct{}, outer.Len())
	similar := true
	outer.Range(func(k string, schema shim.Schema) bool {
		s[k] = struct{}{}
		v, ok := inner.GetOk(k)
		switch schema.Type() {
		case shim.TypeInvalid, shim.TypeBool, shim.TypeInt, shim.TypeFloat, shim.TypeString:
			if !ok {
				similar = false
				return false
			}
			similar = shallowEqual(schema, v)
			return similar

		case shim.TypeList, shim.TypeSet, shim.TypeMap:
			if v.Type() != schema.Type() {
				similar = false
				return similar
			}

			schemaElem, vElem := schema.Elem(), v.Elem()
			switch schemaElem := schemaElem.(type) {
			case shim.Schema:
				vElem, ok := vElem.(shim.Schema)
				if !ok {
					similar = false
					return similar
				}
				similar = shallowEqual(schemaElem, vElem)
				return similar
			case shim.Resource:
				vElem, ok := vElem.(shim.Resource)
				if !ok {
					similar = false
					return similar
				}
				similar = isRecursionOf(schemaElem.Schema(), vElem.Schema())
				return similar
			case nil:
				similar = vElem == nil
				return similar
			}
		default:
			contract.Failf("Unexpected type %s (%[1]v)", schema.Type())
		}

		return true
	})
	if !similar {
		// We have already failed so we return early
		return false
	}
	inner.Range(func(k string, schema shim.Schema) bool {
		if _, ok := s[k]; ok {
			return true
		}

		// This could be a base case, so accept it
		_, similar = schema.Elem().(shim.Resource)
		return similar
	})
	return similar
}

func shallowEqual(a, b shim.Schema) bool {
	sliceEq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i, v := range a {
			if v != b[i] {
				return false
			}
		}
		return true
	}

	return a.Type() == b.Type() &&
		a.Optional() == b.Optional() &&
		a.Required() == b.Required() &&
		// We don't assess the equality of default
		a.Computed() == b.Computed() &&
		a.ForceNew() == b.ForceNew() &&
		a.MaxItems() == b.MaxItems() &&
		a.MinItems() == b.MinItems() &&
		sliceEq(a.ConflictsWith(), b.ConflictsWith()) &&
		sliceEq(a.ExactlyOneOf(), b.ExactlyOneOf()) &&
		a.Removed() == b.Removed() &&
		a.Sensitive() == b.Sensitive()
}
