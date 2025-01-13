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
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

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
		var retError []error
		r.TF.Schema().Range(func(key string, value shim.Schema) bool {
			if fix := badPropertyName(p.Name, p.GetResourcePrefix(), key); fix != nil {
				err := fix(r)
				if err != nil {
					retError = append(retError, fmt.Errorf("%s: %w", key, err))
				}
			}

			return true
		})
		return errors.Join(retError...)
	})
}

func getField(i *map[string]*info.Schema, name string) *info.Schema {
	contract.Assertf(i != nil, "i cannot be nil")
	if *i == nil {
		*i = map[string]*info.Schema{}
	}
	v, ok := (*i)[name]
	if !ok {
		v = new(info.Schema)
		(*i)[name] = v
	}
	return v
}

type fixupProperty = func(tfbridge.Resource) error

func badPropertyName(providerName, tokenPrefix, key string) fixupProperty {
	switch key {
	case "urn":
		return fixURN(providerName)
	case "id":
		return fixID(providerName, tokenPrefix)
	default:
		return nil
	}
}

func fixID(providerName string, tokenPrefix string) fixupProperty {
	getResourceName := func(tk string) string {
		name := strings.TrimPrefix(tk, tokenPrefix)
		if name == "" {
			name = tk
		}
		return strings.TrimLeft(name, "_")
	}
	return func(r tfbridge.Resource) error {
		tfSchema := r.TF.Schema()
		tfIDProperty, ok := tfSchema.GetOk("id")
		if !ok {
			// could not find an ID
			return nil
		}

		// If the user has over-written the field, don't change that.
		if f := r.Schema.Fields["id"]; f != nil && f.Name != "" {
			return nil
		}

		// We have an ordered list of names to attempt when we alias ID.
		//
		// 1. "<resource_name>_id"
		// 2. "<provider_name>_<resource_name>_id"
		// 4. "resource_id"
		// 3. "<provider_name>_id"
		candidateNames := []string{
			getResourceName(r.TFName) + "_id",
			strings.ReplaceAll(providerName, "-", "_") + "_" + getResourceName(r.TFName) + "_id",
			"resource_id",
			strings.ReplaceAll(providerName, "-", "_") + "_id",
		}

		for _, proposedIDFieldName := range candidateNames {
			if _, ok := tfSchema.GetOk(proposedIDFieldName); ok {
				continue
			}

			// If either id.Optional or id.Required are set, then the provider allows
			// (or requires) the user to set "id" as an input. Pulumi does not allow
			// that, so we alias "id".
			// Alternatively if the type of id is not string, we'll replace the property
			// so we need to alias the original property.
			if !tfIDProperty.Optional() && !tfIDProperty.Required() && tfIDProperty.Type() == shim.TypeString {
				return nil
			}

			newIDField := tfbridge.TerraformToPulumiNameV2(proposedIDFieldName, tfSchema, r.Schema.Fields)
			getField(&r.Schema.Fields, "id").Name = newIDField

			// We are altering the original ID because it is valid for the
			// user to set it. As long as it will be present as an output, we
			// should still be able to use it as the actual ID.
			//
			// We expect newIDField to be present in the output space when:
			//
			// 1. The user *must* set it.
			// 2. The user *may* set it, but if they don't then the provider will.
			if tfIDProperty.Required() || (tfIDProperty.Optional() && tfIDProperty.Computed()) {
				r.Schema.ComputeID = tfbridge.DelegateIDField(resource.PropertyKey(newIDField), providerName,
					"https://github.com/pulumi/pulumi-terraform-provider")
			}

			return nil
		}

		return fmt.Errorf("no available new name, tried %#v", candidateNames)
	}
}

func fixURN(providerName string) fixupProperty {
	return func(r tfbridge.Resource) error {
		s := r.TF.Schema()
		v := providerName + "_urn"
		if _, ok := s.GetOk(v); ok {
			return fmt.Errorf("no available new name, tried %q", v)
		}
		if f := getField(&r.Schema.Fields, "urn"); f.Name == "" {
			f.Name = tfbridge.TerraformToPulumiNameV2(v, s, r.Schema.Fields)
		}
		return nil
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
