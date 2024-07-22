// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This is a test utility for converting sdkv2 schemas to sdkv1 to allow testing both without
// having to specify the schema twice.
// Only works with a part of the schema, will throw errors on unsupported features.
package schemaconvert

import (
	v1Schema "github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	v2Schema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func sdkv2ToV1Type(t v2Schema.ValueType) v1Schema.ValueType {
	return v1Schema.ValueType(t)
}

func sdkv2ToV1SchemaOrResource(elem interface{}) interface{} {
	switch elem := elem.(type) {
	case nil:
		return nil
	case *v2Schema.Schema:
		return Sdkv2ToV1Schema(elem)
	case *v2Schema.Resource:
		return sdkv2ToV1Resource(elem)
	default:
		contract.Failf("unexpected type %T", elem)
		return nil
	}
}

func sdkv2ToV1Resource(sch *v2Schema.Resource) *v1Schema.Resource {
	//nolint:staticcheck // deprecated
	contract.Assertf(sch.MigrateState == nil, "MigrateState is not supported in conversion")
	contract.Assertf(sch.StateUpgraders == nil, "StateUpgraders is not supported in conversion")

	//nolint:staticcheck // deprecated
	contract.Assertf(sch.Create == nil && sch.Read == nil && sch.Update == nil && sch.Delete == nil && sch.Exists == nil &&
		sch.CreateContext == nil && sch.ReadContext == nil && sch.UpdateContext == nil &&
		sch.DeleteContext == nil && sch.Importer == nil,
		"runtime methods not supported in conversion")

	contract.Assertf(sch.CustomizeDiff == nil, "CustomizeDiff is not supported in conversion")

	var timeouts *v1Schema.ResourceTimeout
	if sch.Timeouts != nil {
		timeouts = &v1Schema.ResourceTimeout{
			Create:  sch.Timeouts.Create,
			Read:    sch.Timeouts.Read,
			Update:  sch.Timeouts.Update,
			Delete:  sch.Timeouts.Delete,
			Default: sch.Timeouts.Default,
		}
	}

	return &v1Schema.Resource{
		Schema:             Sdkv2ToV1SchemaMap(sch.Schema),
		SchemaVersion:      sch.SchemaVersion,
		DeprecationMessage: sch.DeprecationMessage,
		Timeouts:           timeouts,
	}
}

func Sdkv2ToV1Schema(sch *v2Schema.Schema) *v1Schema.Schema {
	if sch.DiffSuppressFunc != nil {
		contract.Failf("DiffSuppressFunc is not supported in conversion")
	}

	defaultFunc := v1Schema.SchemaDefaultFunc(nil)
	if sch.DefaultFunc != nil {
		defaultFunc = func() (interface{}, error) {
			return sch.DefaultFunc()
		}
	}

	stateFunc := v1Schema.SchemaStateFunc(nil)
	if sch.StateFunc != nil {
		stateFunc = func(i interface{}) string {
			return sch.StateFunc(i)
		}
	}

	set := v1Schema.SchemaSetFunc(nil)
	if sch.Set != nil {
		set = func(i interface{}) int {
			return sch.Set(i)
		}
	}

	validateFunc := v1Schema.SchemaValidateFunc(nil)
	if sch.ValidateFunc != nil {
		validateFunc = func(i interface{}, s string) ([]string, []error) {
			return sch.ValidateFunc(i, s)
		}
	}

	return &v1Schema.Schema{
		Type:         sdkv2ToV1Type(sch.Type),
		Optional:     sch.Optional,
		Required:     sch.Required,
		Default:      sch.Default,
		DefaultFunc:  defaultFunc,
		Description:  sch.Description,
		InputDefault: sch.InputDefault,
		Computed:     sch.Computed,
		ForceNew:     sch.ForceNew,
		StateFunc:    stateFunc,
		Elem:         sdkv2ToV1SchemaOrResource(sch.Elem),
		MaxItems:     sch.MaxItems,
		MinItems:     sch.MinItems,
		Set:          set,
		//nolint:staticcheck
		ComputedWhen:  sch.ComputedWhen,
		ConflictsWith: sch.ConflictsWith,
		ExactlyOneOf:  sch.ExactlyOneOf,
		AtLeastOneOf:  sch.AtLeastOneOf,
		Deprecated:    sch.Deprecated,
		ValidateFunc:  validateFunc,
		Sensitive:     sch.Sensitive,
	}
}

func Sdkv2ToV1SchemaMap(sch map[string]*v2Schema.Schema) map[string]*v1Schema.Schema {
	res := make(map[string]*v1Schema.Schema)
	for k, v := range sch {
		res[k] = Sdkv2ToV1Schema(v)
	}
	return res
}
