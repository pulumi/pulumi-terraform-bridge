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

package internal

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

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
	if defaultInfo.From != nil {
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

// TODO this needs to handle the nested case, setting defaults for properties of object-typed properties.
func SetDefaultValues(
	res *tfbridge.PulumiResource,
	resShim shim.Resource,
	fields map[string]*tfbridge.SchemaInfo,
) (finalError error) {
	for key, fld := range fields {
		if fld == nil || fld.Default == nil {
			continue
		}
		fieldSchema, knownField := resShim.Schema().GetOk(key)
		if !knownField {
			continue
		}

		pulumiName := tfbridge.TerraformToPulumiNameV2(key, resShim.Schema(), fields)
		pk := resource.PropertyKey(pulumiName)

		if _, setAlready := res.Properties[pk]; setAlready {
			continue
		}

		// using default value for empty property
		pv, gotDefault, err := getDefaultValue(res, fieldSchema, fld.Default)
		if err != nil {
			return err
		}
		if gotDefault {
			res.Properties[pk] = pv
		}
	}
	return
}
