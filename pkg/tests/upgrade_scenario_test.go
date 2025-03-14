// Copyright 2016-2025, Pulumi Corporation.
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

package tests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

type upgradeTestCase struct {
	resourceBefore upgradeTestResource
	resourceAfter  upgradeTestResource
}

type upgradeTestResource struct {
	schema      *schema.Resource
	info        *info.Resource
	yamlProgram string
}

func (tc upgradeTestCase) bridgedProvider(t *testing.T, resource upgradeTestResource) info.Provider {
	resMap := map[string]*schema.Resource{
		"prov_test": resource.schema,
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	p := pulcheck.BridgedProvider(t, "prov", tfp)

	rinfo := resource.info
	if rinfo == nil {
		rinfo = &info.Resource{Tok: "prov:index:Test"}
	}
	rinfo.Tok = "prov:index:Test"
	p.Resources = map[string]*info.Resource{
		"prov_test": rinfo,
	}
	return p
}

func (tc upgradeTestCase) prepare(t *testing.T) *pulumitest.PulumiTest {
	pt := pulcheck.PulCheck(t, tc.bridgedProvider(t, tc.resourceBefore), tc.resourceBefore.yamlProgram)
	pt.Up(t)
	state := pt.ExportStack(t)

	t.Logf("%s", string(state.Deployment))
	programAfter := tc.resourceBefore.yamlProgram
	if tc.resourceAfter.yamlProgram != "" {
		programAfter = tc.resourceAfter.yamlProgram
	}
	pt2 := pulcheck.PulCheck(t, tc.bridgedProvider(t, tc.resourceAfter), programAfter)
	pt2.ImportStack(t, state)
	return pt2
}

// When TF schema did not change, but Pulumi removes MaxItems=1, the bridged provider should not break.
func TestUpgrade_Pulumi_Removes_MaxItems1(t *testing.T) {
	programBefore := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            obj:
                str: "Hello"
                bool: true
`

	programAfter := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            objs:
                - str: "Hello"
                  bool: true
`
	r := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"obj": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"str": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"bool": {
							Type:     schema.TypeBool,
							Optional: true,
						},
					},
				},
			},
		},
	}

	trueBool := true

	tc := upgradeTestCase{
		resourceBefore: upgradeTestResource{
			yamlProgram: programBefore,
			schema:      r,
			info: &info.Resource{
				Fields: map[string]*info.Schema{
					"obj": {
						MaxItemsOne: &trueBool,
					},
				},
			},
		},
		resourceAfter: upgradeTestResource{
			yamlProgram: programAfter,
			schema:      r,
		},
	}

	test := tc.prepare(t)

	previewResult := test.Preview(t)
	t.Logf("PREVIEW: %v", previewResult.ChangeSummary)

	upResult := test.Up(t)
	t.Logf("UP: %v", upResult.Summary)
}
