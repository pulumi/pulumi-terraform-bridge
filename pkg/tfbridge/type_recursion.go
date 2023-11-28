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
	"fmt"

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
		s, ok := inner.Elem().(shim.Resource)
		if !ok {
			return true
		}
		outer := s.Schema()
		outer.Range(func(k string, inner shim.Schema) bool {
			s, ok := inner.Elem().(shim.Resource)
			if !ok {
				return true
			}
			if isRecursionOf(outer, s.Schema()) {
				panic(fmt.Sprintf("Detected recursion on %s", k))
			}
			return true
		})
		return true
	})
}

func isCollection(s shim.Schema) bool {
	if s == nil {
		return true
	}
	t := s.Type()
	return t == shim.TypeList || t == shim.TypeSet || t == shim.TypeMap
}

func isScalar(s shim.Schema) bool { return !isCollection(s) }

func isRecursionOf(outer, inner shim.SchemaMap) bool {
	similar, _ := isRecursionOfHelper(outer, inner)
	return similar
}

// A type is similar to another type if:
//
// 1. They match on scalar fields, and
// 2. Object fields are themselves similar, or
// 3. One type is missing an object field that the other type has (the base case).
func isRecursionOfHelper(outer, inner shim.SchemaMap) (bool, bool) {
	s := make(map[string]struct{}, inner.Len())
	similar := true
	ratchet := false
	inner.Range(func(k string, schema shim.Schema) bool {
		s[k] = struct{}{}
		v, ok := outer.GetOk(k)
		if !ok && isScalar(schema) {
			similar = false
			return false
		}

		switch schema.Type() {
		case shim.TypeInvalid, shim.TypeBool, shim.TypeInt, shim.TypeFloat, shim.TypeString:
			similar = shallowEqual(schema, v)
			if !similar {
				ratchet = false
			}
			return similar

		case shim.TypeMap:
			// If the missing key is paired with a strongly typed object, then
			// this could be a base case and we should return early.
			//
			// Mark this as a ratchet so we try walking inward on "outer" to
			// see if this lines up.
			if _, ok := schema.Elem().(shim.Resource); ok {
				ratchet = true
				similar = false
				return true
			}

			fallthrough
		case shim.TypeList, shim.TypeSet:
			if v.Type() != schema.Type() {
				similar = false
				return similar
			}

			schemaElem, vElem := schema.Elem(), v.Elem()
			switch schemaElem := schemaElem.(type) {
			case shim.Resource:
				vElem, ok := vElem.(shim.Resource)
				if !ok {
					similar = false
					return similar
				}
				similar, ratchet = isRecursionOfHelper(schemaElem.Schema(), vElem.Schema())
				for ratchet && !similar {
					similar, ratchet = isRecursionOfHelper(schemaElem.Schema(), inner)
				}

				return similar || ratchet
			case shim.Schema:
				vElem, ok := vElem.(shim.Schema)
				if !ok {
					similar = false
					return similar
				}
				similar = shallowEqual(schemaElem, vElem)
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
		return false, ratchet
	}
	outer.Range(func(k string, schema shim.Schema) bool {
		if _, ok := s[k]; ok {
			return true
		}

		// This could be a base case, so accept it
		_, similar = schema.Elem().(shim.Resource)
		return similar
	})
	return similar, ratchet
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
