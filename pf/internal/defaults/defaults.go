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

package defaults

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
)

type ApplyDefaultInfoValuesArgs struct {
	// Required. The configuration property map to extend with defaults.
	PropertyMap resource.PropertyMap

	// Toplevel schema map for the resource, data source or provider.
	SchemaMap shim.SchemaMap

	// Toplevel SchemaInfo configuration matching TopSchemaMap.
	SchemaInfos map[string]*tfbridge.SchemaInfo

	// Optional. Note that ResourceInstance need not be specified for Invoke or Configure processing.
	ResourceInstance *tfbridge.PulumiResource

	// Optional. If known, these are the provider-level configuration values, to support DefaultInfo.Config.
	ProviderConfig resource.PropertyMap
}

// Transforms a PropertyMap to apply default values specified in DefaultInfo.
//
// These values are specified at the bridged provider configuration level, and are applied before any Terraform
// processing; therefore the function works at Pulumi level (transforming a PropertyMap).
func ApplyDefaultInfoValues(ctx context.Context, args ApplyDefaultInfoValuesArgs) resource.PropertyMap {

	// Can short-circuit the entire processing if there are no matching SchemaInfo entries, and therefore no
	// matching DefaultInfo entries at all.
	if args.SchemaInfos == nil {
		return args.PropertyMap
	}

	t := &defaultsTransform{
		resourceInstance: args.ResourceInstance,
		topSchemaMap:     args.SchemaMap,
		topFieldInfos:    args.SchemaInfos,
		providerConfig:   args.ProviderConfig,
	}
	result := t.withDefaults(ctx, make(resource.PropertyPath, 0), resource.NewObjectProperty(args.PropertyMap))
	contract.Assertf(result.IsObject(), "defaultsTransform.withDefaults returned a non-object value")
	return result.ObjectValue()
}

func getDefaultValue(
	ctx context.Context,
	property resource.PropertyKey,
	res *tfbridge.PulumiResource,
	fieldSchema shim.Schema,
	defaultInfo *tfbridge.DefaultInfo,
	providerConfig resource.PropertyMap,
) (resource.PropertyValue, bool) {
	na := resource.NewNullProperty()

	if defaultInfo == nil {
		return na, false
	}

	// Conditional order follows old code v3/tfbridge but may be relaxed in the future, for instance allowing
	// defaultInfo.Value to kick in as fallback when Config is specified but does not match.
	if len(defaultInfo.EnvVars) != 0 {
		for _, n := range defaultInfo.EnvVars {
			// Following code in v3/tfbridge, ignoring set but empty env vars.
			if str, ok := os.LookupEnv(n); ok && str != "" {

				v, err := parseValueFromEnv(fieldSchema, str)
				if err != nil {
					msg := fmt.Errorf("Cannot parse the value of environment variable '%s'"+
						" to populate property '%s' with a default value: %w",
						n, string(property), err)
					tflog.Error(ctx, msg.Error())
					return na, false
				}

				tflog.Trace(ctx, "DefaultInfo.EnvVars applied a default from an environment variable",
					map[string]any{"envvar": n, "property": string(property)})

				return v, true
			}
		}

		// Value is allowed together with EnvVars but serves as a fallback.
		if defaultInfo.Value != nil {
			tflog.Trace(ctx, "DefaultInfo.Value applied a default value",
				map[string]any{
					"property": string(property),
				})
			return recoverDefaultValue(defaultInfo.Value), true
		}
	} else if defaultInfo.Config != "" {
		pk := resource.PropertyKey(defaultInfo.Config)
		if providerConfig != nil {
			if pv, ok := providerConfig[pk]; ok {
				tflog.Trace(ctx, "DefaultInfo.Config inherited a value from provider config",
					map[string]any{
						"key":      defaultInfo.Config,
						"property": string(property),
					})
				return pv, true
			}
		}
	} else if defaultInfo.Value != nil {
		tflog.Trace(ctx, "DefaultInfo.Value applied a default value",
			map[string]any{
				"property": string(property),
			})
		return recoverDefaultValue(defaultInfo.Value), true
	} else if defaultInfo.From != nil && res != nil {
		raw, err := defaultInfo.From(res)
		if err != nil {
			msg := fmt.Errorf("Failed computing a default value for property '%s': %w",
				string(property), err)
			tflog.Error(ctx, msg.Error())
			return na, false
		}
		if raw == nil {
			return na, false
		}
		tflog.Trace(ctx, "DefaultInfo.From applied a computed default value",
			map[string]any{"property": string(property)})
		return recoverDefaultValue(raw), true
	}

	return na, false
}

func parseValueFromEnv(sch shim.Schema, str string) (resource.PropertyValue, error) {
	contract.Assertf(str != "", "parseValueFromEnv only works on non-empty strings")

	switch sch.Type() {
	case shim.TypeBool:
		v, err := strconv.ParseBool(str)
		if err != nil {
			return resource.NewNullProperty(), err
		}
		return resource.NewBoolProperty(v), nil
	case shim.TypeInt:
		iv, iverr := strconv.ParseInt(str, 0, 0)
		if iverr != nil {
			return resource.NewNullProperty(), iverr
		}
		v := int(iv)
		return resource.NewNumberProperty(float64(v)), nil
	case shim.TypeFloat:
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return resource.NewNullProperty(), err
		}
		return resource.NewNumberProperty(v), nil
	case shim.TypeString:
		return resource.NewStringProperty(str), nil
	default:
		return resource.NewNullProperty(), fmt.Errorf("unknown type for default value: %v", sch.Type())
	}
}

func recoverDefaultValue(defaultValue any) resource.PropertyValue {
	if pv, alreadyPV := defaultValue.(resource.PropertyValue); alreadyPV {
		return pv
	}
	return resource.NewPropertyValue(defaultValue)
}

type defaultsTransform struct {
	topSchemaMap     shim.SchemaMap
	topFieldInfos    map[string]*tfbridge.SchemaInfo // optional
	resourceInstance *tfbridge.PulumiResource        // optional
	providerConfig   resource.PropertyMap            // optional
}

// Returns a non-nil resourceInstance only if the defaults are being applied to a resource at the top level.
func (du *defaultsTransform) resourceByPath(path resource.PropertyPath) *tfbridge.PulumiResource {
	var res *tfbridge.PulumiResource
	if len(path) == 0 {
		res = du.resourceInstance
	}
	return res
}

// Returns matching object schema for a context determined by the PropertyPath, if any.
func (du *defaultsTransform) lookupSchemaByContext(
	path resource.PropertyPath,
) (shim.SchemaMap, map[string]*tfbridge.SchemaInfo, bool) {
	if len(path) == 0 {
		return du.topSchemaMap, du.topFieldInfos, true
	}

	schemaPath := tfbridge.PropertyPathToSchemaPath(path, du.topSchemaMap, du.topFieldInfos)
	if schemaPath == nil {
		return nil, nil, false
	}

	schema, info, err := tfbridge.LookupSchemas(schemaPath, du.topSchemaMap, du.topFieldInfos)
	if err != nil {
		return nil, nil, false
	}

	encodedObjectSchema, ok := schema.Elem().(shim.Resource)
	if !ok {
		return nil, nil, false
	}

	objectSchema := encodedObjectSchema.Schema()

	var fields map[string]*tfbridge.SchemaInfo
	if info == nil || info.Fields == nil {
		fields = map[string]*tfbridge.SchemaInfo{}
	} else {
		fields = info.Fields
	}

	return objectSchema, fields, true
}

// Extends PropertyMap with Pulumi-specified default values.
func (du *defaultsTransform) extendPropertyMapWithDefaults(
	ctx context.Context,
	path resource.PropertyPath,
	props resource.PropertyMap,
) resource.PropertyMap {
	schemaMap, infos, ok := du.lookupSchemaByContext(path)
	if !ok {
		return props
	}
	res := props.Copy() // take a shallow copy

	// Can iterate over SchemaInfo.Fields instead of iterating over every field in the schema, since the algorithm
	// is only interested in properties where there is a SchemaInfo.Field specifying a default.
	for key, fld := range infos {
		if fld == nil || fld.Default == nil {
			continue
		}
		fieldSchema, knownField := schemaMap.GetOk(key)
		if !knownField {
			continue
		}

		pulumiName := tfbridge.TerraformToPulumiNameV2(key, schemaMap, infos)
		pk := resource.PropertyKey(pulumiName)

		if _, setAlready := res[pk]; setAlready {
			continue
		}

		// using default value for empty property
		pv, gotDefault := getDefaultValue(ctx,
			pk,
			du.resourceByPath(path),
			fieldSchema,
			fld.Default,
			du.providerConfig)
		if gotDefault {
			res[pk] = pv
		}
	}

	return res
}

// This is mostly simply a recursion pattern on PropertyValue, can be abstracted out.
func (du *defaultsTransform) withDefaults(
	ctx context.Context,
	path resource.PropertyPath,
	value resource.PropertyValue,
) resource.PropertyValue {
	tr := func(path resource.PropertyPath, value resource.PropertyValue) (resource.PropertyValue, error) {
		if !value.IsObject() {
			return value, nil
		}
		pm := value.ObjectValue()
		// After recurring on the elements, try to apply defaults here.
		extended := du.extendPropertyMapWithDefaults(ctx, path, pm)
		return resource.NewObjectProperty(extended), nil /* no errors here */
	}
	transformed, err := propertyvalue.TransformPropertyValue(path, tr, value)
	contract.AssertNoErrorf(err, "TransformPropertyValue should not return errors here")
	return transformed
}
