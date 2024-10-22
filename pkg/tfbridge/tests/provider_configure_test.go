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

package tfbridgetests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
)

func TestConfigureSimpleValues(t *testing.T) {
	t.Run("string", crosstests.MakeConfigure(map[string]*schema.Schema{
		"f0": {Type: schema.TypeString, Required: true},
	}, cty.ObjectVal(map[string]cty.Value{
		"f0": cty.StringVal("v0"),
	}), crosstests.InferPulumiValue()))

	t.Run("bool", crosstests.MakeConfigure(map[string]*schema.Schema{
		"f0": {Type: schema.TypeBool, Required: true},
		"f1": {Type: schema.TypeBool, Required: true},
	}, cty.ObjectVal(map[string]cty.Value{
		"f0": cty.BoolVal(false),
		"f1": cty.BoolVal(true),
	}), crosstests.InferPulumiValue()))

	t.Run("int", crosstests.MakeConfigure(map[string]*schema.Schema{
		"f0": {Type: schema.TypeInt, Required: true},
	}, cty.ObjectVal(map[string]cty.Value{
		"f0": cty.NumberIntVal(123456),
	}), crosstests.InferPulumiValue()))

	t.Run("float64", func(t *testing.T) {
		t.Skip("TODO: Float64 does not pass cross-tests")
		crosstests.Configure(t, map[string]*schema.Schema{
			"f0": {Type: schema.TypeFloat, Required: true},
		}, cty.ObjectVal(map[string]cty.Value{
			"f0": cty.NumberFloatVal(123.456),
		}), crosstests.InferPulumiValue())
	})
}

func TestConfigureSimpleSecretValues(t *testing.T) {
	t.Run("string", crosstests.MakeConfigure(map[string]*schema.Schema{
		"f0": {Type: schema.TypeString, Required: true},
	}, cty.ObjectVal(map[string]cty.Value{
		"f0": cty.StringVal("v0"),
	}), resource.PropertyMap{
		"f0": resource.MakeSecret(resource.NewProperty("v0")),
	}))
}
