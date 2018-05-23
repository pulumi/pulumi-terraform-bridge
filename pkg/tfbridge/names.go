// Copyright 2016-2018, Pulumi Corporation.
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
	"unicode"

	"github.com/gedex/inflector"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// PulumiToTerraformName performs a standard transformation on the given name string, from Pulumi's PascalCasing or
// camelCasing, to Terraform's underscore_casing.
func PulumiToTerraformName(name string, tfs map[string]*schema.Schema) string {
	var result string
	for i, c := range name {
		if c >= 'A' && c <= 'Z' {
			// if upper case, add an underscore (if it's not #1), and then the lower case version.
			if i != 0 {
				result += "_"
			}
			result += string(unicode.ToLower(c))
		} else {
			result += string(c)
		}
	}
	// Singularize names which were pluralized because they were array-shaped Pulumi values
	if tfs != nil {
		singularResult := inflector.Singularize(result)
		// Note: If the name is not found in it's singular form in the schema map, that may be because the TF name was
		// already plural, and thus pluralization was a noop.  In this case, we know we should return the raw (plural)
		// result.
		sch, ok := tfs[singularResult]
		if ok && sch.MaxItems != 1 && (sch.Type == schema.TypeList || sch.Type == schema.TypeSet) {
			result = singularResult
		}
	}
	return result
}

// TerraformToPulumiName performs a standard transformation on the given name string, from Terraform's underscore_casing
// to Pulumi's PascalCasing (if upper is true) or camelCasing (if upper is false).
func TerraformToPulumiName(name string, sch *schema.Schema, upper bool) string {
	var result string
	var nextCap bool
	var prev rune

	// Pluralize names that will become array-shaped Pulumi values
	if sch != nil && sch.MaxItems != 1 && (sch.Type == schema.TypeList || sch.Type == schema.TypeSet) {
		contract.Assertf(
			inflector.Pluralize(name) == name || inflector.Singularize(inflector.Pluralize(name)) == name,
			"expected to be able to safely pluralize name: %s", name)
		name = inflector.Pluralize(name)
	}

	casingActivated := false // tolerate leading underscores
	for i, c := range name {
		if c == '_' && casingActivated {
			// skip underscores and make sure the next one is capitalized.
			contract.Assertf(!nextCap, "Unexpected duplicate underscore: %v", name)
			nextCap = true
		} else {
			if c != '_' && !casingActivated {
				casingActivated = true // note that we've seen non-underscores, so we treat the right correctly.
			}
			if ((i == 0 && upper) || nextCap) && (c >= 'a' && c <= 'z') {
				// if we're at the start and upper was requested, or the next is meant to be a cap, capitalize it.
				result += string(unicode.ToUpper(c))
			} else {
				result += string(c)
			}
			nextCap = false
		}
		prev = c
	}
	if prev == '_' {
		// we had a next cap, but it wasn't realized.  propagate the _ after all.
		result += "_"
	}
	return result
}

// AutoName creates custom schema for a Terraform name property which is automatically populated from the
// resource's URN name, with an 8 character random suffix ("-"+7 random chars), and maximum length of maxlen.  This
// makes it easy to propagate the Pulumi resource's URN name part as the Terraform name as a convenient default, while
// still permitting it to be overridden.
func AutoName(name string, maxlen int) *SchemaInfo {
	return AutoNameTransform(name, maxlen, nil)
}

// AutoNameTransform creates custom schema for a Terraform name property which is automatically populated from the
// resource's URN name, with an 8 character random suffix ("-"+7 random chars), maximum length maxlen, and optional
// transformation function. This makes it easy to propagate the Pulumi resource's URN name part as the Terraform name
// as a convenient default, while still permitting it to be overridden.
func AutoNameTransform(name string, maxlen int, transform func(string) string) *SchemaInfo {
	return &SchemaInfo{
		Name: name,
		Default: &DefaultInfo{
			From: FromName(true, maxlen, transform),
		},
	}
}

// FromName automatically propagates a resource's URN onto the resulting default info.
func FromName(rand bool, maxlen int, transform func(string) string) func(res *PulumiResource) (interface{}, error) {
	return func(res *PulumiResource) (interface{}, error) {
		// Take the URN name part, transform it if required, and then append some unique characters.
		vs := string(res.URN.Name())
		if transform != nil {
			vs = transform(vs)
		}
		if rand {
			return resource.NewUniqueHex(vs+"-", 7, maxlen)
		}
		return vs, nil
	}
}
