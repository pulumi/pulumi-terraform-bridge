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
	"fmt"
	"os"
	"strconv"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Transforms a PropertyMap to apply default values specified in DefaultInfo.
//
// These values are specified at the bridged provider configuration level, and are applied before any Terraform
// processing; therefore the function works at Pulumi level (transforming a PropertyMap).
//
// Note that resourceInstance need not be specified when applying defaults for Invoke or Configure processing.
func ApplyDefaultInfoValues(
	topSchemaMap shim.SchemaMap,
	topFieldInfos map[string]*tfbridge.SchemaInfo, // optional
	resourceInstance *tfbridge.PulumiResource, // optional
	props resource.PropertyMap,
) resource.PropertyMap {
	t := &defaultsTransform{
		resourceInstance: resourceInstance,
		topSchemaMap:     topSchemaMap,
		topFieldInfos:    topFieldInfos,
	}
	result := t.withDefaults(make(resource.PropertyPath, 0), resource.NewObjectProperty(props))
	if !result.IsObject() {
		contract.Failf("defaultsTransform.withDefaults returned a non-object value")
	}
	return result.ObjectValue()
}

func getDefaultValue(
	res *tfbridge.PulumiResource,
	fieldSchema shim.Schema,
	defaultInfo *tfbridge.DefaultInfo,
) (resource.PropertyValue, bool, error) {
	na := resource.NewNullProperty()

	// TODO handle defaultInfo.Config
	if defaultInfo == nil {
		return na, false, nil
	}
	if defaultInfo.From != nil && res != nil {
		raw, err := defaultInfo.From(res)
		if err != nil {
			return na, false, err
		}
		if raw == nil {
			return na, false, nil
		}
		return recoverDefaultValue(raw), true, nil

	} else if defaultInfo.EnvVars != nil {
		for _, n := range defaultInfo.EnvVars {
			if str, ok := os.LookupEnv(n); ok {
				// TODO what is the behavior of an env var set to an emtpy string? Is this the same as
				// unset, or the same as actually setting to empty?
				v, err := parseValueFromEnv(fieldSchema, str)
				return v, true, err
			}
		}
	}
	if defaultInfo.Value != nil {
		return recoverDefaultValue(defaultInfo.Value), true, nil
	}
	return na, false, nil
}

func parseValueFromEnv(sch shim.Schema, str string) (resource.PropertyValue, error) {
	var err error
	switch sch.Type() {
	case shim.TypeBool:
		v := false
		if str != "" {
			if v, err = strconv.ParseBool(str); err != nil {
				return resource.NewNullProperty(), err
			}
		}
		return resource.NewBoolProperty(v), nil
	case shim.TypeInt:
		v := int(0)
		if str != "" {
			iv, iverr := strconv.ParseInt(str, 0, 0)
			if iverr != nil {
				return resource.NewNullProperty(), iverr
			}
			v = int(iv)
		}
		return resource.NewNumberProperty(float64(v)), nil
	case shim.TypeFloat:
		v := float64(0.0)
		if str != "" {
			if v, err = strconv.ParseFloat(str, 64); err != nil {
				return resource.NewNullProperty(), err
			}
		}
		return resource.NewNumberProperty(float64(v)), nil
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
}

// Returns a non-nil resourceInstance only if the defaults are beeing applied to a resource at the top level.
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
	if info.Fields == nil {
		fields = map[string]*tfbridge.SchemaInfo{}
	}

	return objectSchema, fields, true
}

// Extends PropertyMap with Pulumi-specified default values.
func (du *defaultsTransform) extendPropertyMapWithDefaults(
	path resource.PropertyPath,
	props resource.PropertyMap,
) (resource.PropertyMap, error) {
	schemaMap, infos, ok := du.lookupSchemaByContext(path)
	if !ok {
		return props, nil
	}
	res := props.Copy()

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
		pv, gotDefault, err := getDefaultValue(du.resourceByPath(path), fieldSchema, fld.Default)
		if err != nil {
			return nil, fmt.Errorf("when computing a default for property '%s' %w", key, err)
		}
		if gotDefault {
			res[pk] = pv
		}
	}

	return res, nil
}

// This is mostly simply a recursion pattern on PropertyValue, can be abstracted out.
func (du *defaultsTransform) withDefaults(
	path resource.PropertyPath,
	value resource.PropertyValue,
) resource.PropertyValue {
	switch {
	case value.IsObject():
		pm := make(resource.PropertyMap)
		for k, v := range value.ObjectValue() {
			subPath := append(path, string(k))
			tv := du.withDefaults(subPath, v)
			pm[k] = tv
		}
		// After recurring on the elements, try to apply defaults here.
		if extended, err := du.extendPropertyMapWithDefaults(path, pm); err != nil {
			// TODO can we log the ignored error here
			value = resource.NewObjectProperty(pm)
		} else {
			value = resource.NewObjectProperty(extended)
		}
	case value.IsArray():
		av := value.ArrayValue()
		tvs := make([]resource.PropertyValue, 0, len(av))
		for i, v := range av {
			subPath := append(path, i)
			tv := du.withDefaults(subPath, v)
			tvs = append(tvs, tv)
		}
		value = resource.NewArrayProperty(tvs)
	case value.IsOutput():
		o := value.OutputValue()
		tv := du.withDefaults(path, o.Element)
		value = resource.NewOutputProperty(resource.Output{
			Element:      tv,
			Known:        o.Known,
			Secret:       o.Secret,
			Dependencies: o.Dependencies,
		})
	case value.IsSecret():
		s := value.SecretValue()
		newElement := du.withDefaults(path, s.Element)
		return resource.MakeSecret(newElement)
	}
	return value
}
