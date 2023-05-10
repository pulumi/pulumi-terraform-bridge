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

	"github.com/pkg/errors"

	"github.com/gedex/inflector"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

// PulumiToTerraformName performs a standard transformation on the given name string, from Pulumi's PascalCasing or
// camelCasing, to Terraform's underscore_casing.
func PulumiToTerraformName(name string, tfs shim.SchemaMap, ps map[string]*SchemaInfo) string {
	var result string
	// First, check if any tf name points to this one.
	if tfs != nil {
		tfs.Range(func(key string, value shim.Schema) bool {
			v := TerraformToPulumiNameV2(key, tfs, ps)
			if v == name {
				result = key
			}
			return result == ""
		})
		if result != "" {
			return result
		}
	}

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

	return result
}

func isTfPlural(tfs shim.Schema) bool {
	if tfs == nil {
		return false
	}

	switch tfs.Type() {
	case shim.TypeSet, shim.TypeList:
		return tfs.MaxItems() != 1
	default:
		return false
	}
}

func isPulumiMaxItemsOne(ps *SchemaInfo) bool {
	return ps != nil && ps.MaxItemsOne != nil && *ps.MaxItemsOne
}

// TerraformToPulumiNameV2 performs a standard transformation on the given name string,
// from Terraform's underscore_casing to Pulumi's camelCasing.
func TerraformToPulumiNameV2(name string, sch shim.SchemaMap, ps map[string]*SchemaInfo) string {
	return terraformToPulumiName(name, sch, ps, false)
}

// TerraformToPulumiName performs a standard transformation on the given name
// string, from Terraform's underscore_casing to Pulumi's PascalCasing (if upper is true)
// or camelCasing (if upper is false).
//
// Deprecated: Convert to TerraformToPulumiNameV2, see comment for conversion instructions.
//
// TerraformToPulumiNameV2 includes enough information for a bijective mapping between
// Terraform attributes and Pulumi properties. If the full naming context cannot be
// acquired, you can construct a partial naming context:
//
//	TerraformToPulumiNameV2(name,
//		schema.SchemaMap(map[string]shim.Schema{name: sch}),
//		map[string]*SchemaInfo{name: ps})
//
// If the previous (non-invertable) camel casing is necessary, it must be implemented
// manually:
//
//	name := TerraformToPulumiNameV2(key, sch, ps)
//	name = strings(uniode.ToUpper(rune(name[0]))) + name[1:]
func TerraformToPulumiName(name string, sch shim.Schema, ps *SchemaInfo, upper bool) string {
	return terraformToPulumiName(name,
		schema.SchemaMap(map[string]shim.Schema{name: sch}),
		map[string]*SchemaInfo{name: ps},
		upper)
}

func terraformToPulumiName(name string, sch shim.SchemaMap, ps map[string]*SchemaInfo, upper bool) string {
	var result string
	var nextCap bool
	var prev rune

	var psInfo *SchemaInfo
	if ps != nil {
		psInfo = ps[name]
	}

	if psInfo != nil {
		if name := psInfo.Name; name != "" {
			return name
		}
	}

	// Pluralize names that will become array-shaped Pulumi values
	if sch != nil && !isPulumiMaxItemsOne(psInfo) && isTfPlural(sch.Get(name)) {
		candidate := inflector.Pluralize(name)
		// We don't assign a plural name if there is another key in the namespace that
		// would conflict with our name... unless that key is manually assigned a .Name
		// that prevents the conflict.
		//
		// NOTE Without full cycle analysis, it is possible to get a non-bijective
		// mapping when there is another key that is manually mapped to a conflicting
		// value.
		//
		// This will be non-bijective:
		//
		//	[
		//		{key: "key", type: List},        // Maps to "keys"
		//		{key: "conflict", Name: "keys"}, // Set to "keys"
		//	]
		//
		// The non-bijectivity will be caught at tfgen time and a warning will be emitted.

		_, conflict := sch.GetOk(candidate)

		// A conflict at the `sch` level doesn't necessarily mean that it is
		// unsafe to pluralize. It is possible that the potentially conflicting
		// field had its name manually set to another value.
		conflictSafe := (ps[candidate] != nil &&
			ps[candidate].Name != "" &&
			ps[candidate].Name != candidate)
		if !conflict || conflictSafe {
			name = candidate
		}
	}

	casingActivated := false // tolerate leading underscores
	for i, c := range name {
		if c == '_' && casingActivated {
			// any number of consecutive underscores in a string, e.g. foo__dot__bar, result in capitalization
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

// AutoNameOptions provides parameters to AutoName to control how names will be generated
type AutoNameOptions struct {
	// A separator between name and random portions of the
	Separator string
	// Maximum length of the generated name
	Maxlen int
	// Number of random characters to add to the name
	Randlen int
	// What characters to use for the random portion of the name, defaults to hex digits
	Charset []rune
	// A transform to apply to the name prior to adding random characters
	Transform func(string) string
	// A transform to apply after the auto naming has been computed
	PostTransform func(res *PulumiResource, name string) (string, error)
}

// AutoName creates custom schema for a Terraform name property which is automatically populated
// from the resource's URN name, and transformed based on the provided options.
func AutoName(name string, maxlength int, separator string) *SchemaInfo {
	return &SchemaInfo{
		Name: name,
		Default: &DefaultInfo{
			AutoNamed: true,
			From: FromName(AutoNameOptions{
				Separator: separator,
				Maxlen:    maxlength,
				Randlen:   7,
			}),
		},
	}
}

// AutoNameWithCustomOptions creates a custom schema for a Terraform name property and allows setting options to allow
// transforms, custom separators and maxLength combinations.
func AutoNameWithCustomOptions(name string, options AutoNameOptions) *SchemaInfo {
	return &SchemaInfo{
		Name: name,
		Default: &DefaultInfo{
			AutoNamed: true,
			From:      FromName(options),
		},
	}
}

// AutoNameTransform creates custom schema for a Terraform name property which is automatically populated from the
// resource's URN name, with an 8 character random suffix ("-"+7 random chars), maximum length maxlen, and optional
// transformation function. This makes it easy to propagate the Pulumi resource's URN name part as the Terraform name
// as a convenient default, while still permitting it to be overridden.
func AutoNameTransform(name string, maxlen int, transform func(string) string) *SchemaInfo {
	return &SchemaInfo{
		Name: name,
		Default: &DefaultInfo{
			AutoNamed: true,
			From: FromName(AutoNameOptions{
				Separator: "-",
				Maxlen:    maxlen,
				Randlen:   7,
				Transform: transform,
			}),
		},
	}
}

// FromName automatically propagates a resource's URN onto the resulting default info.
func FromName(options AutoNameOptions) func(res *PulumiResource) (interface{}, error) {
	return func(res *PulumiResource) (interface{}, error) {
		// Take the URN name part, transform it if required, and then append some unique characters if requested.
		vs := string(res.URN.Name())
		if options.Transform != nil {
			vs = options.Transform(vs)
		}
		if options.Randlen > 0 {
			uniqueHex, err := resource.NewUniqueName(
				res.Seed, vs+options.Separator, options.Randlen, options.Maxlen, options.Charset)
			if err != nil {
				return uniqueHex, errors.Wrapf(err, "could not make instance of '%v'", res.URN.Type())
			}
			vs = uniqueHex
		}
		if options.PostTransform != nil {
			return options.PostTransform(res, vs)
		}
		return vs, nil
	}
}
