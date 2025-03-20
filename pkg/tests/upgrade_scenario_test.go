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
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// A schema change with state ugpraders should work as expected.
func TestUpgrade_StateUpgraders(t *testing.T) {
	t.Skip("TODO[pulumi/pulumi-terraform-bridge#1667]")
	t.Parallel()

	resourceBefore := &schema.Resource{
		CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
			rd.SetId("id-0")
			return nil
		},
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}

	resourceAfter := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
			},
		},
		SchemaVersion: 1,
		CustomizeDiff: func(ctx context.Context, rd *schema.ResourceDiff, i interface{}) error {
			// Diff is receiving already upgraded data.
			autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id-0"), "prop":cty.ListVal([]cty.Value{cty.NumberIntVal(1), cty.NumberIntVal(2), cty.NumberIntVal(3)})})`).Equal(t, rd.GetRawState().GoString())
			return nil
		},
		StateUpgraders: []schema.StateUpgrader{{
			Version: 0,
			Type:    cty.Object(map[string]cty.Type{"prop": cty.String}),
			Upgrade: func(
				ctx context.Context,
				rawState map[string]interface{},
				meta interface{},
			) (map[string]interface{}, error) {
				// Upgrade function is receiving the data as it was written.
				autogold.Expect(map[string]interface{}{"id": "id-0", "prop": "one,two,three"}).Equal(t, rawState)

				s := rawState["prop"].(string)
				parts := strings.Split(s, ",")
				partsN := []int{}
				for _, p := range parts {
					parsed := map[string]int{"one": 1, "two": 2, "three": 3}
					partsN = append(partsN, parsed[p])
				}
				return map[string]any{"prop": partsN, "id": rawState["id"]}, nil
			},
		}},
	}

	programBefore := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            prop: "one,two,three"
`

	programAfter := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            props:
                - 1
                - 2
                - 3
`

	tc := upgradeTestCase{
		resourceBefore: upgradeTestResource{
			schema:      resourceBefore,
			yamlProgram: programBefore,
		},
		resourceAfter: upgradeTestResource{
			schema:      resourceAfter,
			yamlProgram: programAfter,
		},
	}

	test := tc.prepare(t, false /*refresh*/)

	previewResult := test.Preview(t, optpreview.Diff())

	t.Logf("%s", previewResult.StdOut)

	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, previewResult.ChangeSummary)

	upResult := test.Up(t)

	autogold.Expect(&map[string]int{"same": 2}).Equal(t, upResult.Summary.ResourceChanges)
}

// When TF schema did not change, but Pulumi removes MaxItems=1, the bridged provider should not break.
func TestUpgrade_Pulumi_Removes_MaxItems1(t *testing.T) {
	t.Skip("TODO[pulumi/pulumi-terraform-bridge#1667]")
	t.Parallel()

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
		CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
			rd.SetId("id")
			require.Truef(t, rd.GetRawState().IsNull(), "RawState is null at create")
			autogold.Expect([]interface{}{map[string]interface{}{"bool": true, "str": "Hello"}}).Equal(t, rd.Get("obj"))
			return diag.Diagnostics{}
		},
		CustomizeDiff: func(ctx context.Context, rd *schema.ResourceDiff, i interface{}) error {
			if rd.GetRawState().IsNull() {
				// During Create, GetRawState is Null; nothing to check.
				return nil
			}

			// Check GetRawState() during update.
			autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id"), "obj":cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"bool":cty.True, "str":cty.StringVal("Hello")})})})`).Equal(t, rd.GetRawState().GoString())
			return nil
		},
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

	test := tc.prepare(t, false /*refresh*/)

	previewResult := test.Preview(t)

	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, previewResult.ChangeSummary)

	upResult := test.Up(t)
	autogold.Expect(&map[string]int{"same": 2}).Equal(t, upResult.Summary.ResourceChanges)
}

// Here underlying schema is not changed, but Pulumi is adding a MaxItems=1 marker.
func TestUpgrade_Pulumi_Adds_MaxItems1(t *testing.T) {
	t.Skip("TODO[pulumi/pulumi-terraform-bridge#1667]")
	t.Parallel()

	programBefore := `
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

	programAfter := `
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

	r := &schema.Resource{
		CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
			rd.SetId("id")
			require.Truef(t, rd.GetRawState().IsNull(), "RawState is null at create")
			autogold.Expect([]interface{}{map[string]interface{}{"bool": true, "str": "Hello"}}).Equal(t, rd.Get("obj"))
			return diag.Diagnostics{}
		},
		CustomizeDiff: func(ctx context.Context, rd *schema.ResourceDiff, i interface{}) error {
			if rd.GetRawState().IsNull() {
				// During Create, GetRawState is Null; nothing to check.
				return nil
			}

			// Check GetRawState() during update.
			autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id"), "obj":cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"bool":cty.True, "str":cty.StringVal("Hello")})})})`).Equal(t, rd.GetRawState().GoString())
			return nil
		},
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
		},
		resourceAfter: upgradeTestResource{
			yamlProgram: programAfter,
			schema:      r,
			info: &info.Resource{
				Fields: map[string]*info.Schema{
					"obj": {
						MaxItemsOne: &trueBool,
					},
				},
			},
		},
	}

	test := tc.prepare(t, false /*refresh*/)

	previewResult := test.Preview(t)

	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, previewResult.ChangeSummary)

	upResult := test.Up(t)
	autogold.Expect(&map[string]int{"same": 2}).Equal(t, upResult.Summary.ResourceChanges)
}

func TestUpgrade_Upstream_Adds_MaxItems1(t *testing.T) {
	t.Skip("TODO[pulumi/pulumi-terraform-bridge#1667]")
	t.Parallel()

	testUpgradeUpstreamAddsMaxItems1(t, false /*refresh*/)
}

// Testing refresh is an important part of the upgrade story as the refreshed state needs to contain enough markers to
// reconstruct the expected raw state for the upgrade to go smoothly. Reuse one of the existing test cases here to test
// the refresh path.
func TestUpgrade_Refresh(t *testing.T) {
	t.Skip("TODO[pulumi/pulumi-terraform-bridge#1667]")
	t.Parallel()
	testUpgradeUpstreamAddsMaxItems1(t, true /*refresh*/)
}

func testUpgradeUpstreamAddsMaxItems1(t *testing.T, refresh bool) {
	programBefore := `
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

	programAfter := `
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

	r := func(maxItems int) *schema.Resource {
		return &schema.Resource{
			CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				rd.SetId("id")
				require.Truef(t, rd.GetRawState().IsNull(), "RawState is null at create")
				autogold.Expect([]interface{}{map[string]interface{}{"bool": true, "str": "Hello"}}).Equal(t, rd.Get("obj"))
				return diag.Diagnostics{}
			},
			CustomizeDiff: func(ctx context.Context, rd *schema.ResourceDiff, i interface{}) error {
				if rd.GetRawState().IsNull() {
					// During Create, GetRawState is Null; nothing to check.
					return nil
				}

				// Check GetRawState() during update.
				autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id"), "obj":cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"bool":cty.True, "str":cty.StringVal("Hello")})})})`).Equal(t, rd.GetRawState().GoString())
				return nil
			},
			Schema: map[string]*schema.Schema{
				"obj": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: maxItems,
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
	}

	tc := upgradeTestCase{
		resourceBefore: upgradeTestResource{
			yamlProgram: programBefore,
			schema:      r(0),
		},
		resourceAfter: upgradeTestResource{
			yamlProgram: programAfter,
			schema:      r(1),
		},
	}

	test := tc.prepare(t, refresh)

	previewResult := test.Preview(t)

	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, previewResult.ChangeSummary)

	upResult := test.Up(t)
	autogold.Expect(&map[string]int{"same": 2}).Equal(t, upResult.Summary.ResourceChanges)
}

func TestUpgrade_Upstream_Removes_MaxItems1(t *testing.T) {
	t.Skip("TODO[pulumi/pulumi-terraform-bridge#1667]")

	t.Parallel()

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

	r := func(maxItems int) *schema.Resource {
		return &schema.Resource{
			CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				rd.SetId("id")
				require.Truef(t, rd.GetRawState().IsNull(), "RawState is null at create")
				autogold.Expect([]interface{}{map[string]interface{}{"bool": true, "str": "Hello"}}).Equal(t, rd.Get("obj"))
				return diag.Diagnostics{}
			},
			CustomizeDiff: func(ctx context.Context, rd *schema.ResourceDiff, i interface{}) error {
				if rd.GetRawState().IsNull() {
					// During Create, GetRawState is Null; nothing to check.
					return nil
				}

				// Check GetRawState() during update.
				autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id"), "obj":cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"bool":cty.True, "str":cty.StringVal("Hello")})})})`).Equal(t, rd.GetRawState().GoString())
				return nil
			},
			Schema: map[string]*schema.Schema{
				"obj": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: maxItems,
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
	}

	tc := upgradeTestCase{
		resourceBefore: upgradeTestResource{
			yamlProgram: programBefore,
			schema:      r(1),
		},
		resourceAfter: upgradeTestResource{
			yamlProgram: programAfter,
			schema:      r(0),
		},
	}

	test := tc.prepare(t, false /*refresh*/)

	previewResult := test.Preview(t)

	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, previewResult.ChangeSummary)

	upResult := test.Up(t)
	autogold.Expect(&map[string]int{"same": 2}).Equal(t, upResult.Summary.ResourceChanges)
}

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

func (tc upgradeTestCase) prepare(t *testing.T, refresh bool) *pulumitest.PulumiTest {
	pt := pulcheck.PulCheck(t, tc.bridgedProvider(t, tc.resourceBefore), tc.resourceBefore.yamlProgram)
	pt.Up(t)

	if refresh {
		pt.Refresh(t)
	}

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
