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

// Helper code to drive Terraform CLI to run tests against an in-process provider.
package crosstests

import (
	"bytes"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl/hclwrite"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

type TfResDriver struct {
	driver *tfcheck.TFDriver
	res    *schema.Resource
}

func newTFResDriver(t T, dir, providerName, resName string, res *schema.Resource) *TfResDriver {
	p := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			resName: res,
		},
	}
	driver := tfcheck.NewTfDriver(t, dir, providerName, tfcheck.NewTFDriverOpts{SDKProvider: p})
	return &TfResDriver{
		driver: driver,
		res:    res,
	}
}

// coalesceInputs is a helper function to translate from previous cross-test
// input types (map[string]any, [tftypes.Value]) to the current representation:
// [cty.Value].
//
// As soon as we have completed the migration, we can remove this function.
func coalesceInputs(t T, schema map[string]*schema.Schema, config any) cty.Value {
	switch config := config.(type) {
	case nil:
		return cty.NullVal(cty.DynamicPseudoType)
	case cty.Value:
		return config
	case map[string]any:
		objectType := convert.InferObjectType(sdkv2.NewSchemaMap(schema), nil)
		for k := range objectType.AttributeTypes {
			objectType.OptionalAttributes[k] = struct{}{}
		}
		v := fromType(objectType).NewValue(config)
		return fromValue(v).ToCty()
	case tftypes.Value:
		return fromValue(config).ToCty()
	default:
		require.Failf(t, "unknown type", "unable to convert config type %T to %T", config, cty.Value{})
		return cty.Value{}
	}
}

type lifecycleArgs struct {
	CreateBeforeDestroy bool
}

func (d *TfResDriver) writePlan(
	t T,
	resourceSchema map[string]*schema.Schema,
	resourceType, resourceName string,
	config cty.Value,
	lifecycle lifecycleArgs,
) *tfcheck.TFPlan {
	plan, err := d.writePlanErr(t, resourceSchema, resourceType, resourceName, config, lifecycle)
	require.NoError(t, err)
	return plan
}

func (d *TfResDriver) writePlanErr(
	t T,
	resourceSchema map[string]*schema.Schema,
	resourceType, resourceName string,
	config cty.Value,
	lifecycle lifecycleArgs,
) (*tfcheck.TFPlan, error) {
	if !config.IsNull() {
		d.write(t, resourceSchema, resourceType, resourceName, config, lifecycle)
	} else {
		t.Logf("empty config file")
		d.driver.Write(t, "")
	}

	return d.driver.Plan(t)
}

func (d *TfResDriver) writePlanApply(
	t T,
	resourceSchema map[string]*schema.Schema,
	resourceType, resourceName string,
	config cty.Value,
	lifecycle lifecycleArgs,
) *tfcheck.TFPlan {
	plan := d.writePlan(t, resourceSchema, resourceType, resourceName, config, lifecycle)
	err := d.driver.ApplyPlan(t, plan)
	require.NoError(t, err)
	return plan
}

func (d *TfResDriver) refreshErr(
	t T,
	resourceSchema map[string]*schema.Schema,
	resourceType, resourceName string,
	config cty.Value,
	lifecycle lifecycleArgs,
) error {
	if !config.IsNull() {
		d.write(t, resourceSchema, resourceType, resourceName, config, lifecycle)
	} else {
		t.Logf("empty config file")
		d.driver.Write(t, "")
	}

	return d.driver.Refresh(t)
}

func (d *TfResDriver) write(
	t T,
	resourceSchema map[string]*schema.Schema,
	resourceType, resourceName string,
	config cty.Value,
	lifecycle lifecycleArgs,
) {
	var buf bytes.Buffer
	opts := []hclwrite.WriteResourceOption{}
	if lifecycle.CreateBeforeDestroy {
		opts = append(opts, hclwrite.WithCreateBeforeDestroy(true))
	}
	sch := hclSchemaSDKv2(resourceSchema)
	err := hclwrite.WriteResource(&buf, sch, resourceType, resourceName, config.AsValueMap(), opts...)
	require.NoError(t, err)
	t.Logf("HCL: \n%s\n", buf.String())
	d.driver.Write(t, buf.String())
}

func providerHCLProgram(t T, typ string, provider *schema.Provider, config cty.Value) string {
	var out bytes.Buffer
	sch := hclSchemaSDKv2(provider.Schema)
	require.NoError(t, hclwrite.WriteProvider(&out, sch, typ, config.AsValueMap()))

	res := provider.Resources()
	if l := len(res); l != 1 {
		require.FailNow(t, "Expected provider to have 1 resource (found %d), ambiguous resource choice", l)
	}

	require.NoError(t, hclwrite.WriteResource(&out, sch, res[0].Name, "res", map[string]cty.Value{}))

	return out.String()
}
