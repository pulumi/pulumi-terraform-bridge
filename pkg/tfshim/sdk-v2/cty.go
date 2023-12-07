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

package sdkv2

import (
	"fmt"
	"math/big"

	"github.com/golang/glog"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

const (
	terraformUnknownVariableValue = "74D93920-ED26-11E3-AC10-0800200C9A66"
)

// makeResourceRawConfig converts the decoded Go values in a terraform.ResourceConfig into a cty.Value that is
// appropriate for Instance{Diff,State}.RawConfig.
func makeResourceRawConfig(
	strategy DiffStrategy,
	config *terraform.ResourceConfig,
	resource *schema.Resource,
) cty.Value {
	if strategy == ClassicDiff {
		return makeResourceRawConfigClassic(config, resource)
	}

	value, err := recoverAndCoerceCtyValue(resource, config.Raw)
	if err == nil {
		return value
	}

	// This should never happen in practice, but following the original design of this method error recovery
	// is attempted by using approximate methods as it might be better to proceed than to fail fast.
	glog.V(9).Infof("failed to recover resource config value from data, "+
		"falling back to approximate methods: %v", err)

	return makeResourceRawConfigClassic(config, resource)
}

func makeResourceRawConfigClassic(config *terraform.ResourceConfig, resource *schema.Resource) cty.Value {
	// Unlike schema.JSONMapToStateValue, schema.HCL2ValueFromConfigValue is an approximate method as it does not
	// consult the type of the resource. This causes problems such as lists being decoded as Tuple when the schema
	// wants a Set. The problems cause CoerceValue to fail.
	original := schema.HCL2ValueFromConfigValue(config.Raw)
	coerced, err := resource.CoreConfigSchema().CoerceValue(original)
	if err != nil {
		// Once more, choosing to proceed with a slightly incorrect value rather than fail fast.
		glog.V(9).Infof("failed to coerce config: %v", err)
		return original
	}
	return coerced
}

// This method started off calling [schema.JSONMapToStateValue], but it turns out that it is not
// suitable for processing unknown values encoded as [terraformUnknownVariableValue]. The latest
// implementation retains CoerceValue call from the JSONMapToStateValue, but does an explicit
// type-driven recursive recovery pass from any to cty.Value first, instead of doing this
// transformation via an intermediate JSON form. The pass is made aware of
// terraformUnknownVariableValue.
//
// One possibility to refactor this code is to have MakeTerraformInput return cty.Value directly
// instead of implicit any encoding.
func recoverAndCoerceCtyValue(resource *schema.Resource, value any) (cty.Value, error) {
	c, err := recoverCtyValue(resource.CoreConfigSchema().ImpliedType(), value)
	if err != nil {
		return cty.NilVal, fmt.Errorf("recoverCtyValue failed: %w", err)
	}
	cv, err := resource.CoreConfigSchema().CoerceValue(c)
	if err != nil {
		return cty.NilVal, fmt.Errorf("CoerceValue failed: %w", err)
	}
	return cv, nil
}

// See [recoverAndCoerceCtyValue] as the main use case. Takes a value produced by
// tfbridge.MakeTerraformInput and converts it to a cty.Value of the desired type dT. This recursive
// method tracks desired type dT through containers such as objects, maps and lists. For scalars, it
// recognizes unknowns encoded as terraformUnknownVariableValue. Missing values for object maps are
// populated with nulls (important to avoid panics in TF internal code), and extraneous values
// discarded. No type conversions such as string to bool are attempted, as the code calls
// CoerceValue on the result of this method and that traversal takes care of that.
func recoverCtyValue(dT cty.Type, value interface{}) (cty.Value, error) {
	switch value := value.(type) {
	case nil:
		return cty.NullVal(dT), nil
	case map[string]interface{}:
		switch {
		case dT.IsMapType():
			return recoverCtyValueOfMapType(dT, value)
		case dT.IsObjectType():
			return recoverCtyValueOfObjectType(dT, value)
		default:
			return cty.NilVal, fmt.Errorf("Cannot reconcile map %v to %v", value, dT)
		}
	case []interface{}:
		switch {
		case dT.IsTupleType():
			return recoverCtyValueOfTupleType(dT, value)
		case dT.IsSetType():
			return recoverCtyValueOfSetType(dT, value)
		case dT.IsListType():
			return recoverCtyValueOfListType(dT, value)
		default:
			return cty.NilVal, fmt.Errorf("Cannot reconcile slice %v to %v", value, dT)
		}
	case string:
		if value == terraformUnknownVariableValue {
			return cty.UnknownVal(dT), nil
		}
		return cty.StringVal(value), nil
	case bool:
		return cty.BoolVal(value), nil
	case int:
		return cty.NumberIntVal(int64(value)), nil
	case int64:
		return cty.NumberIntVal(value), nil
	case uint64:
		return cty.NumberUIntVal(value), nil
	case float64:
		return cty.NumberFloatVal(value), nil
	case *big.Float:
		return cty.NumberVal(value), nil
	case uint8:
		return cty.NumberIntVal(int64(value)), nil
	case uint16:
		return cty.NumberIntVal(int64(value)), nil
	case uint32:
		return cty.NumberIntVal(int64(value)), nil
	case int8:
		return cty.NumberIntVal(int64(value)), nil
	case int16:
		return cty.NumberIntVal(int64(value)), nil
	case int32:
		return cty.NumberIntVal(int64(value)), nil
	case float32:
		return cty.NumberFloatVal(float64(value)), nil
	default:
		return cty.NilVal, fmt.Errorf("Cannot recover cty.Value\n"+
			"  Desired type: %v\n"+
			"  Value: %v\n"+
			"  Actual type: %#T\n",
			dT.GoString(), value, value)
	}
}

func recoverCtyValueOfMapType(dT cty.Type, value map[string]interface{}) (cty.Value, error) {
	if !dT.IsMapType() {
		return cty.NilVal, fmt.Errorf("recoverCtyValueOfMapType expected a Map, got %v", dT)
	}
	eT := dT.ElementType()
	attrs := map[string]cty.Value{}
	for k, v := range value {
		var err error
		attrs[k], err = recoverCtyValue(eT, v)
		if err != nil {
			return cty.NilVal, err
		}
	}
	return cty.ObjectVal(attrs), nil
}

func recoverCtyValueOfObjectType(dT cty.Type, value map[string]interface{}) (cty.Value, error) {
	if !dT.IsObjectType() {
		return cty.NilVal,
			fmt.Errorf("recoverCtyValueOfObjectType expected an Object, got %v", dT)
	}
	aT := dT.AttributeTypes()
	if len(aT) == 0 {
		return cty.EmptyObjectVal, nil
	}
	attrs := map[string]cty.Value{}
	for attrName, attrType := range aT {
		if v, ok := value[attrName]; ok {
			var err error
			attrs[attrName], err = recoverCtyValue(attrType, v)
			if err != nil {
				return cty.NilVal, err
			}
		} else {
			attrs[attrName] = cty.NullVal(attrType)
		}
	}
	return cty.ObjectVal(attrs), nil
}

func recoverCtyValueOfListType(dT cty.Type, values []interface{}) (cty.Value, error) {
	if !dT.IsListType() {
		return cty.NilVal,
			fmt.Errorf("recoverCtyValueOfListType expected a List, got %v", dT)
	}
	eT := dT.ElementType()
	vals := []cty.Value{}
	for _, v := range values {
		rv, err := recoverCtyValue(eT, v)
		if err != nil {
			return cty.NilVal, err
		}
		vals = append(vals, rv)
	}
	if len(vals) == 0 {
		return cty.ListValEmpty(dT), nil
	}
	return cty.ListVal(vals), nil
}

func recoverCtyValueOfSetType(dT cty.Type, values []interface{}) (cty.Value, error) {
	if !dT.IsSetType() {
		return cty.NilVal, fmt.Errorf("recoverCtyValueOfSetType expected a Set, got %v", dT)
	}
	eT := dT.ElementType()
	vals := []cty.Value{}
	for _, v := range values {
		rv, err := recoverCtyValue(eT, v)
		if err != nil {
			return cty.NilVal, err
		}
		vals = append(vals, rv)
	}
	if len(vals) == 0 {
		return cty.SetValEmpty(dT), nil
	}
	return cty.SetVal(vals), nil
}

func recoverCtyValueOfTupleType(dT cty.Type, values []interface{}) (cty.Value, error) {
	if !dT.IsTupleType() {
		return cty.NilVal, fmt.Errorf(
			"recoverCtyValueOfTulpeType expected a Tuple, got %v", dT)
	}
	vals := []cty.Value{}
	for i, v := range values {
		rv, err := recoverCtyValue(dT.TupleElementType(i), v)
		if err != nil {
			return cty.NilVal, err
		}
		vals = append(vals, rv)
	}
	if len(vals) == 0 {
		return cty.EmptyTupleVal, nil
	}
	return cty.TupleVal(vals), nil
}
