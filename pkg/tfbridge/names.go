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
	"context"
	"fmt"
	"unicode"

	"github.com/pkg/errors"

	"github.com/gedex/inflector"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
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

// A helper method to perform TerraformToPulumiNameV2 translation at nested property positions easily. The path argument
// specifies the nested location of the current property within the schema; sch and ps arguments specify top-level
// resource, data source or provider SchemaMap and field SchemaInfo overrides. This method automatically finds the
// approproate nested SchemaMap and field overrides for the property and performs the name translation.
func TerraformToPulumiNameAtPath(
	path walk.SchemaPath,
	sch shim.SchemaMap,
	ps map[string]*SchemaInfo,
) (string, error) {
	if len(path) == 0 {
		return "", fmt.Errorf("TerraformToPulumiNameAtPath: path cannot be empty")
	}
	attr, ok := path[len(path)-1].(walk.GetAttrStep)
	if !ok {
		return "", fmt.Errorf("TerraformToPulumiNameAtPath: path must end with GetAttrStep")
	}

	// If the path is of length 1, this simply degenerates to TerraformToPulumiNameV2.
	if len(path) == 1 {
		return TerraformToPulumiNameV2(attr.Name, sch, ps), nil
	}

	// Otherwise we lookup parent object schema.
	objSchema, objInfos, err := LookupSchemas(path[0:len(path)-1], sch, ps)
	if err != nil {
		return "", fmt.Errorf("TerraformToPulumiNameAtPath failed to find parent object schema: %w", err)
	}

	var objSchemaMap shim.SchemaMap
	switch r := objSchema.Elem().(type) {
	case shim.Resource:
		objSchemaMap = r.Schema()
	}

	var objFields map[string]*SchemaInfo
	if objInfos != nil {
		objFields = objInfos.Fields
	}

	return TerraformToPulumiNameV2(attr.Name, objSchemaMap, objFields), nil
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

	tryPluralize := func() bool {
		tfs := sch.Get(name)
		if tfs == nil {
			// If we can't get type information, we don't attempt to pluralize.
			return false
		}
		switch tfs.Type() {
		// We only attempt to pluralize lists and sets.
		case shim.TypeSet, shim.TypeList:
			// If the user has provided a manual override for MaxItemsOne,
			// respect that.
			if psInfo != nil && psInfo.MaxItemsOne != nil {
				return !*psInfo.MaxItemsOne
			}
			// If the user has left MaxItemsOne unspecified, check the value
			// of MaxItems().
			return tfs.MaxItems() != 1
		default:
			return false
		}
	}

	// Pluralize names that will become array-shaped Pulumi values
	if sch != nil && tryPluralize() {
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

// AutoName configures a property to automatically populate with auto-computed names when no values are given to it by
// the user program.
//
// The auto-computed names will be based on the resource name extracted from the resource URN, and have a random suffix.
// The lifecycle of automatic names is tied to the Pulumi resource lifecycle, so the automatic name will not change
// during normal updates and will persist until the resource is replaced.
//
// If a required property is configured with AutoName, it becomes optional in the Pulumi Package Schema. Therefore
// removing AutoName from a required property is a breaking change.
//
// For a quick example, consider aws.ec2.Keypair that has this code in its definition:
//
//	ResourceInfo{
//	    	Fields: map[string]*SchemaInfo{
//	    		"key_name": AutoName("keyName", 255, "-"),
//	    	},
//	}
//
// Deploying a KeyPair allocates an automatic keyName without the user having to specify it:
//
//	const deployer = new aws.ec2.KeyPair("deployer", {publicKey: pubKey});
//	export const keyName = deployer.keyName;
//
// Running this example produces:
//
//	Outputs:
//	   keyName: "deployer-6587896"
//
// Note how the automatic name combines the resource name from the program with a random suffix.
func AutoName(name string, maxlength int, separator string) *SchemaInfo {
	autoNameOptions := AutoNameOptions{
		Separator: separator,
		Maxlen:    maxlength,
		Randlen:   7,
	}
	return &SchemaInfo{
		Name: name,
		Default: &DefaultInfo{
			AutoNamed: true,
			ComputeDefault: func(ctx context.Context, opts ComputeDefaultOptions) (interface{}, error) {
				return ComputeAutoNameDefault(ctx, autoNameOptions, opts)
			},
		},
	}
}

// AutoNameWithCustomOptions is similar to [AutoName] but exposes more options for configuring the generated names.
func AutoNameWithCustomOptions(name string, options AutoNameOptions) *SchemaInfo {
	return &SchemaInfo{
		Name: name,
		Default: &DefaultInfo{
			AutoNamed: true,
			ComputeDefault: func(ctx context.Context, opts ComputeDefaultOptions) (interface{}, error) {
				return ComputeAutoNameDefault(ctx, options, opts)
			},
		},
	}
}

// AutoNameTransform creates custom schema for a Terraform name property which is automatically populated from the
// resource's URN name, with an 8 character random suffix ("-"+7 random chars), maximum length maxlen, and optional
// transformation function. This makes it easy to propagate the Pulumi resource's URN name part as the Terraform name
// as a convenient default, while still permitting it to be overridden.
func AutoNameTransform(name string, maxlen int, transform func(string) string) *SchemaInfo {
	autoNameOptions := AutoNameOptions{
		Separator: "-",
		Maxlen:    maxlen,
		Randlen:   7,
		Transform: transform,
	}
	return &SchemaInfo{
		Name: name,
		Default: &DefaultInfo{
			AutoNamed: true,
			ComputeDefault: func(ctx context.Context, opts ComputeDefaultOptions) (interface{}, error) {
				return ComputeAutoNameDefault(ctx, autoNameOptions, opts)
			},
		},
	}
}

// FromName automatically propagates a resource's URN onto the resulting default info.
func FromName(options AutoNameOptions) func(res *PulumiResource) (interface{}, error) {
	return func(res *PulumiResource) (interface{}, error) {
		return ComputeAutoNameDefault(context.Background(), options, ComputeDefaultOptions{
			URN:        res.URN,
			Properties: res.Properties,
			Seed:       res.Seed,
		})
	}
}

func ComputeAutoNameDefault(
	ctx context.Context,
	options AutoNameOptions,
	defaultOptions ComputeDefaultOptions,
) (interface{}, error) {
	if defaultOptions.URN == "" {
		return nil, fmt.Errorf("AutoName is onnly supported for resources, expected Resource URN to be set")
	}
	// Take the URN name part, transform it if required, and then append some unique characters if requested.
	vs := string(defaultOptions.URN.Name())
	if options.Transform != nil {
		vs = options.Transform(vs)
	}
	if options.Randlen > 0 {
		uniqueHex, err := resource.NewUniqueName(
			defaultOptions.Seed, vs+options.Separator, options.Randlen, options.Maxlen, options.Charset)
		if err != nil {
			return uniqueHex, errors.Wrapf(err, "could not make instance of '%v'", defaultOptions.URN.Type())
		}
		vs = uniqueHex
	}
	if options.PostTransform != nil {
		return options.PostTransform(&PulumiResource{
			URN:        defaultOptions.URN,
			Properties: defaultOptions.Properties,
			Seed:       defaultOptions.Seed,
		}, vs)
	}
	return vs, nil
}
