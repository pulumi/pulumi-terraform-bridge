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
)

// PulumiToTerraformName performs a standard transformation on the given name string, from Pulumi's PascalCasing or
// camelCasing, to Terraform's underscore_casing.
func PulumiToTerraformName(name string, tfs shim.SchemaMap, ps map[string]*SchemaInfo) string {

	if tfs != nil {
		// Inefficient but improves precision of inverting TerraformToPulumiName.
		t := pulumiToTerraformNameTable(tfs, ps)
		if result, ok := t[name]; ok {
			return result
		}
	}

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
	if singularResult := inflector.Singularize(result); singularResult != result && tfs != nil {

		// Check if the plural name points to an existing TF attribute.
		if _, existingPlural := tfs.GetOk(result); existingPlural {
			return result
		}

		// Note: If the name is not found in it's singular form in the schema map, that may be because the TF name was
		// already plural, and thus pluralization was a noop.  In this case, we know we should return the raw (plural)
		// result.
		var info *SchemaInfo
		sch, ok := tfs.GetOk(singularResult)
		if ps != nil {
			if p, ok := ps[singularResult]; ok {
				info = p
			}
		}

		if ok && checkTfMaxItems(sch, false) || isPulumiMaxItemsOne(info) {
			result = singularResult
		}
	}
	return result
}

// A strict inverse of TerraformToPulumiName(upper=false) that tabulates it based on the SchemaMap.
func pulumiToTerraformNameTable(schemaMap shim.SchemaMap, ps map[string]*SchemaInfo) map[string]string {
	m := map[string]string{}
	schemaMap.Range(func(key string, schema shim.Schema) bool {
		puName := TerraformToPulumiName(key, schema, ps[key], false)
		m[puName] = key
		return true
	})
	return m
}

func checkTfMaxItems(tfs shim.Schema, maxItemsOne bool) bool {
	if tfs == nil {
		return false
	}

	if tfs.Type() != shim.TypeList && tfs.Type() != shim.TypeSet {
		return false
	}

	return (tfs.MaxItems() == 1) == maxItemsOne
}

func isPulumiMaxItemsOne(ps *SchemaInfo) bool {
	return ps != nil && ps.MaxItemsOne != nil && *ps.MaxItemsOne
}

// TerraformToPulumiName performs a standard transformation on the given name string, from Terraform's underscore_casing
// to Pulumi's PascalCasing (if upper is true) or camelCasing (if upper is false).
func TerraformToPulumiName(name string, sch shim.Schema, ps *SchemaInfo, upper bool) string {
	var result string
	var nextCap bool
	var prev rune

	// Pluralize names that will become array-shaped Pulumi values
	if !isPulumiMaxItemsOne(ps) && checkTfMaxItems(sch, false) {
		pluralized := inflector.Pluralize(name)
		if inflector.Singularize(pluralized) == name {
			//			contract.Assertf(
			//				inflector.Pluralize(name) == name || inflector.Singularize(inflector.Pluralize(name)) == name,
			//				"expected to be able to safely pluralize name: %s (%s, %s)", name, inflector.Pluralize(name),
			//				inflector.Singularize(inflector.Pluralize(name)))

			name = pluralized
		}
		name = inflector.Pluralize(name)
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
