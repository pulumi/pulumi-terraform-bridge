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

// package fixup applies fixes to a [info.Provider] to ensure that it can generate a valid
// schema and that the schema can generate valid SDKs in all all languages.
//
// package fixup is still in development and may expose breaking changes in minor
// versions.
package fixup

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	tftokens "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// Default applies the default set of fixups to p.
//
// The set of fixups applied may expand over time, but it should not effect providers that
// correctly compile in all languages.
func Default(p *info.Provider) error {
	return errors.Join(
		fixPropertyConflict(p),
		fixMissingIDs(p),
		fixProviderResource(p),
	)
}

// fixProviderResource renames any resource that would otherwise be called `Provider`,
// since that conflicts with the package's actual `Provider` resource.
func fixProviderResource(p *info.Provider) error {
	tfToken := p.GetResourcePrefix() + "_provider"
	_, ok := p.P.ResourcesMap().GetOk(tfToken)
	if !ok {
		// No problematic Provider resource.
		return nil
	}

	res := ensureResources(p)[tfToken]
	if res == nil {
		res = &info.Resource{}
		ensureResources(p)[tfToken] = res
	}
	if res.Tok != "" {
		return nil // The token has already been renamed, so we are done.
	}

	// We need to rename the token.
	return tftokens.SingleModule(
		p.GetResourcePrefix(), "index", tftokens.MakeStandard(p.Name),
	).Resource(p.GetResourcePrefix()+"_"+p.Name+"_provider", res)
}

func fixMissingIDs(p *info.Provider) error {
	getIDType := func(r *info.Resource) tokens.Type {
		s := r.Fields["id"]
		if s == nil {
			return ""
		}
		return s.Type
	}
	return walkResources(p, func(r tfbridge.Resource) error {
		id, hasID := r.TF.Schema().GetOk("id")
		ok := hasID &&
			(id.Type() == shim.TypeString || getIDType(r.Schema) == "string") &&
			id.Computed()
		if !ok {
			r.Schema.ComputeID = missingID
		}
		return nil
	})
}

func missingID(context.Context, resource.PropertyMap) (resource.ID, error) {
	return "missing ID", nil
}

func fixPropertyConflict(p *info.Provider) error {
	if p.Name == "" {
		return fmt.Errorf("must set p.Name")
	}
	return walkResources(p, func(r tfbridge.Resource) error {
		var retError error
		r.TF.Schema().Range(func(key string, value shim.Schema) bool {
			if fix := badPropertyName(p.Name, key); fix != nil {
				if r.Schema.Fields == nil {
					r.Schema.Fields = map[string]*info.Schema{}
				}

				s, ok := r.Schema.Fields[key]
				if !ok {
					s = &info.Schema{}
				}

				if s.Name == "" {
					var err error
					s.Name, err = fix(r.TF)
					if err != nil {
						retError = fmt.Errorf("%q: %w", key, err)
					}
				}

				if !ok {
					r.Schema.Fields[key] = s
				}
			}

			return true
		})
		return retError
	})
}

func badPropertyName(providerName, key string) func(shim.Resource) (string, error) {
	switch key {
	case "urn":
		return newName(providerName)
	default:
		return nil
	}
}

func newName(providerName string) func(shim.Resource) (string, error) {
	return func(r shim.Resource) (string, error) {
		s := r.Schema()
		v := providerName + "_urn"
		if _, ok := s.GetOk(v); !ok {
			return tfbridge.TerraformToPulumiNameV2(v, s, nil), nil
		}
		return "", fmt.Errorf("no available new name, tried %q", v)
	}
}

func ensureResources(p *info.Provider) map[string]*info.Resource {
	if p.Resources == nil {
		p.Resources = map[string]*info.Resource{}
	}
	return p.Resources
}

func walkResources(p *info.Provider, f func(tfbridge.Resource) error) error {
	var errs []error

	p.P.ResourcesMap().Range(func(key string, tf shim.Resource) bool {
		res, isPresent := p.Resources[key]
		if !isPresent {
			res = &info.Resource{}
		}
		if err := f(tfbridge.Resource{
			Schema: res,
			TF:     tf,
			TFName: key,
		}); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}

		// If the res wasn't already present in the map and f made some change to
		// it, then we need to insert it back into p.Resources.
		//
		// If isPresent, then we don't need to make the insertion because res was
		// already in the map.
		//
		// If IsZero, then inserting res doesn't have any effect, so we skip it.
		if !isPresent && !reflect.ValueOf(*res).IsZero() {
			ensureResources(p)[key] = res
		}

		return true
	})

	return errors.Join(errs...)
}
