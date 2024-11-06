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

package info

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// SetAutonaming auto-names all resource properties that are literally called "name".
//
// The effect is identical to configuring each matching property with [AutoName]. Pulumi will propose an auto-computed
// value for these properties when no value is given by the user program. If a property was required before auto-naming,
// it becomes optional.
//
// The maxLength and separator parameters configure how AutoName generates default values. See [AutoNameOptions].
//
// SetAutonaming will skip properties that already have a [SchemaInfo] entry in [ResourceInfo.Fields], assuming those
// are already customized by the user. If those properties need AutoName functionality, please use AutoName directly to
// populate their SchemaInfo entry.
//
// Note that when constructing a ProviderInfo incrementally, some care is required to make sure SetAutonaming is called
// after [ProviderInfo.Resources] map is fully populated, as it relies on this map to find resources to auto-name.
func (p *Provider) SetAutonaming(maxLength int, separator string) {
	if p.P == nil {
		glog.Warningln("SetAutonaming found a `ProviderInfo.P` nil. No Autonames were applied.")
		return
	}

	const nameProperty = "name"
	for resname, res := range p.Resources {
		if schema := p.P.ResourcesMap().Get(resname); schema != nil {
			// Only apply auto-name to input properties (Optional || Required)
			// of type `string` named `name`
			if sch, hasName := schema.Schema().GetOk(nameProperty); hasName &&
				(sch.Optional() || sch.Required()) && // Is an input type
				sch.Type() == shim.TypeString { // has type string
				if _, hasfield := res.Fields[nameProperty]; !hasfield {
					ensureMap(&res.Fields)[nameProperty] = AutoName(nameProperty, maxLength, separator)
				}
			}
		}
	}
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
func AutoName(name string, maxlength int, separator string) *Schema {
	autoNameOptions := AutoNameOptions{
		Separator: separator,
		Maxlen:    maxlength,
		Randlen:   7,
	}
	return &Schema{
		Name: name,
		Default: &Default{
			AutoNamed: true,
			ComputeDefault: func(ctx context.Context, opts ComputeDefaultOptions) (interface{}, error) {
				return ComputeAutoNameDefault(ctx, autoNameOptions, opts)
			},
		},
	}
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

func ComputeAutoNameDefault(
	ctx context.Context,
	options AutoNameOptions,
	defaultOptions ComputeDefaultOptions,
) (interface{}, error) {
	if defaultOptions.URN == "" {
		return nil, fmt.Errorf("AutoName is only supported for resources, expected Resource URN to be set")
	}

	// Reuse the value from prior state if available. Note that this code currently only runs for Plugin Framework
	// resources, as SDKv2 based resources avoid calling ComputedDefaults in the first place in update situations.
	// To do that SDKv2 based resources track __defaults meta-key to distinguish between values originating from
	// defaulting machinery from values originating from user code. Unfortunately Plugin Framework cannot reliably
	// disinguish default values, therefore it always calls ComputedDefaults. To compensate, this code block avoids
	// re-generating the auto-name if it is located in PriorState and reuses the old one; this avoids generating a
	// fresh random value and causing a replace plan.
	if defaultOptions.PriorState != nil && defaultOptions.PriorValue.V != nil {
		if defaultOptions.PriorValue.IsString() {
			return defaultOptions.PriorValue.StringValue(), nil
		}
	}

	// Take the URN name part, transform it if required, and then append some unique characters if requested.
	vs := defaultOptions.URN.Name()
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

func ensureMap[K comparable, V any](m *map[K]V) map[K]V {
	if *m == nil {
		*m = map[K]V{}
	}
	return *m
}
