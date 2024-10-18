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
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

type TfResDriver struct {
	driver *tfcheck.TfDriver
	res    *schema.Resource
}

func newTFResDriver(t T, dir, providerName, resName string, res *schema.Resource) *TfResDriver {
	p := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			resName: res,
		},
	}
	driver := tfcheck.NewTfDriver(t, dir, providerName, p)
	return &TfResDriver{
		driver: driver,
		res:    res,
	}
}

func (d *TfResDriver) coalesce(t T, x any) *tftypes.Value {
	if x == nil {
		return nil
	}
	objectType := convert.InferObjectType(sdkv2.NewSchemaMap(d.res.Schema), nil)
	for k := range objectType.AttributeTypes {
		objectType.OptionalAttributes[k] = struct{}{}
	}
	t.Logf("infer object type: %v", objectType)
	v := fromType(objectType).NewValue(x)
	return &v
}

type lifecycleArgs struct {
	CreateBeforeDestroy bool
}

func (d *TfResDriver) writePlanApply(
	t T,
	resourceSchema map[string]*schema.Schema,
	resourceType, resourceName string,
	rawConfig any,
	lifecycle lifecycleArgs,
) *tfcheck.TfPlan {
	config := d.coalesce(t, rawConfig)
	if config != nil {
		d.write(t, resourceSchema, resourceType, resourceName, *config, lifecycle)
	} else {
		t.Logf("empty config file")
		d.driver.Write(t, "")
	}
	plan, err := d.driver.Plan(t)
	require.NoError(t, err)
	err = d.driver.Apply(t, plan)
	require.NoError(t, err)
	return plan
}

func (d *TfResDriver) write(
	t T,
	resourceSchema map[string]*schema.Schema,
	resourceType, resourceName string,
	config tftypes.Value,
	lifecycle lifecycleArgs,
) {
	var buf bytes.Buffer
	ctyConfig := fromValue(config).ToCty()
	if lifecycle.CreateBeforeDestroy {
		ctyMap := ctyConfig.AsValueMap()
		if ctyMap == nil {
			ctyMap = make(map[string]cty.Value)
		}
		ctyMap["lifecycle"] = cty.ObjectVal(
			map[string]cty.Value{
				"create_before_destroy": cty.True,
			},
		)
		ctyConfig = cty.ObjectVal(ctyMap)
	}
	err := WriteHCL(&buf, resourceSchema, resourceType, resourceName, ctyConfig)
	require.NoError(t, err)
	t.Logf("HCL: \n%s\n", buf.String())
	d.driver.Write(t, buf.String())
}

type tfChange struct {
	Actions []string       `json:"actions"`
	Before  map[string]any `json:"before"`
	After   map[string]any `json:"after"`
}

// Still discovering the structure of JSON-serialized TF plans. The information required from these is, primarily, is
// whether the resource is staying unchanged, being updated or replaced. Secondarily, would be also great to know
// detailed paths of properties causing the change, though that is more difficult to cross-compare with Pulumi.
//
// For now this is code is similar to `jq .resource_changes[0].change.actions[0] plan.json`.
func (*TfResDriver) parseChangesFromTFPlan(plan tfcheck.TfPlan) tfChange {
	type p struct {
		ResourceChanges []struct {
			Change tfChange `json:"change"`
		} `json:"resource_changes"`
	}
	jb, err := json.Marshal(plan.RawPlan)
	contract.AssertNoErrorf(err, "failed to marshal terraform plan")
	var pp p
	err = json.Unmarshal(jb, &pp)
	contract.AssertNoErrorf(err, "failed to unmarshal terraform plan")
	contract.Assertf(len(pp.ResourceChanges) == 1, "expected exactly one resource change")
	return pp.ResourceChanges[0].Change
}
