package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/require"
)

func TestUnknownHandling(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
		"prov_aux": {
			Schema: map[string]*schema.Schema{
				"aux": {
					Type:     schema.TypeString,
					Computed: true,
					Optional: true,
				},
			},
			CreateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				d.SetId("aux")
				err := d.Set("aux", "aux")
				require.NoError(t, err)
				return nil
			},
		},
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
	program := `
name: test
runtime: yaml
resources:
  auxRes:
    type: prov:index:Aux
  mainRes:
    type: prov:index:Test
    properties:
      test: ${auxRes.aux}
outputs:
  testOut: ${mainRes.test}
`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	res := pt.Preview(t, optpreview.Diff())
	// Test that the test property is unknown at preview time
	require.Contains(t, res.StdOut, "test      : output<string>")
	resUp := pt.Up(t)
	// assert that the property gets resolved
	require.Equal(t, "aux", resUp.Outputs["testOut"].Value)
}

func trimDiff(t *testing.T, diff string) string {
	urnLine := "    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]"
	resourcesSummaryLine := "Resources:\n"
	require.Contains(t, diff, urnLine)
	require.Contains(t, diff, resourcesSummaryLine)

	// trim the diff to only include the contents after the URN line and before the summary
	urnIndex := strings.Index(diff, urnLine)
	resourcesSummaryIndex := strings.Index(diff, resourcesSummaryLine)
	return diff[urnIndex+len(urnLine) : resourcesSummaryIndex]
}

func TestUnknownBlocks(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"test_prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
		},
		"prov_nested_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested_prop": {
								Type:     schema.TypeList,
								Optional: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"test_prop": {
											Type:     schema.TypeList,
											Optional: true,
											Elem: &schema.Schema{
												Type: schema.TypeString,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"prov_aux": {
			Schema: map[string]*schema.Schema{
				"aux": {
					Type:     schema.TypeList,
					Computed: true,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"test_prop": {
								Type:     schema.TypeString,
								Computed: true,
								Optional: true,
							},
						},
					},
				},
				"nested_aux": {
					Type:     schema.TypeList,
					Optional: true,
					Computed: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested_prop": {
								Type:     schema.TypeList,
								Optional: true,
								Computed: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"test_prop": {
											Type:     schema.TypeList,
											Optional: true,
											Computed: true,
											Elem: &schema.Schema{
												Type: schema.TypeString,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			CreateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				d.SetId("aux")
				if d.Get("aux") == nil {
					err := d.Set("aux", []map[string]interface{}{{"test_prop": "aux"}})
					require.NoError(t, err)
				}
				if d.Get("nested_aux") == nil {
					err := d.Set("nested_aux", []map[string]interface{}{
						{
							"nested_prop": []map[string]interface{}{
								{"test_prop": []string{"aux"}},
							},
						},
					})
					require.NoError(t, err)
				}
				return nil
			},
		},
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)

	provTestKnownProgram := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            tests:
                - testProp: "known_val"
`
	nestedProvTestKnownProgram := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - nestedProps:
                    - testProps:
                        - "known_val"
`

	for _, tc := range []struct {
		name                string
		program             string
		initialKnownProgram string
		expectedInitial     autogold.Value
		expectedUpdate      autogold.Value
	}{
		{
			"list of objects",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:Test
        properties:
            tests: ${auxRes.auxes}
`,
			provTestKnownProgram,
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/test:Test: (create)
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
        tests     : output<string>
`),
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - testProp: "known_val"
            }
        ]
      + tests: output<string>
`),
		},
		{
			"unknown object",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:Test
        properties:
            tests:
                - ${auxRes.auxes[0]}
`,
			provTestKnownProgram,

			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/test:Test: (create)
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
        tests     : [
            [0]: output<string>
        ]
`),
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - testProp: "known_val"
                }
          + [0]: output<string>
        ]
`),
		},
		{
			"unknown object with others",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:Test
        properties:
            tests:
                - ${auxRes.auxes[0]}
                - {"testProp": "val"}
`,
			provTestKnownProgram,

			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/test:Test: (create)
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
        tests     : [
            [0]: output<string>
            [1]: {
                testProp  : "val"
            }
        ]
`),
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - testProp: "known_val"
                }
          + [0]: output<string>
          + [1]: {
                  + testProp  : "val"
                }
        ]
`),
		},
		{
			"unknown nested",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests: ${auxRes.nestedAuxes}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : output<string>
`),
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      - tests: [
      -     [0]: {
              - nestedProps: [
              -     [0]: {
                      - testProps: [
                      -     [0]: "known_val"
                        ]
                    }
                ]
            }
        ]
      + tests: output<string>
`),
		},
		{
			"unknown nested level 1",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - ${auxRes.nestedAuxes[0]}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: output<string>
        ]
`),
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      ~ tests: [
          - [0]: {
                  - nestedProps: [
                  -     [0]: {
                          - testProps: [
                          -     [0]: "known_val"
                            ]
                        }
                    ]
                }
          + [0]: output<string>
        ]
`),
		},
		{
			"unknown nested level 2",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - nestedProps: ${auxRes.nestedAuxes[0].nestedProps}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: {
                nestedProps: output<string>
            }
        ]
`),
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      ~ tests: [
          ~ [0]: {
                  - nestedProps: [
                  -     [0]: {
                          - testProps: [
                          -     [0]: "known_val"
                            ]
                        }
                    ]
                  + nestedProps: output<string>
                }
        ]
`),
		},
		{
			"unknown nested level 3",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - nestedProps:
                    - ${auxRes.nestedAuxes[0].nestedProps[0]}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: {
                nestedProps: [
                    [0]: output<string>
                ]
            }
        ]
`),
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nestedProps: [
                      - [0]: {
                              - testProps: [
                              -     [0]: "known_val"
                                ]
                            }
                      + [0]: output<string>
                    ]
                }
        ]
`),
		},
		{
			"unknown nested level 4",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - nestedProps:
                    - testProps: ${auxRes.nestedAuxes[0].nestedProps[0].testProps}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: {
                nestedProps: [
                    [0]: {
                        testProps : output<string>
                    }
                ]
            }
        ]
`),
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nestedProps: [
                      ~ [0]: {
                              - testProps: [
                              -     [0]: "known_val"
                                ]
                              + testProps: output<string>
                            }
                    ]
                }
        ]
`),
		},
		{
			"unknown nested level 5",
			`
name: test
runtime: yaml
resources:
    auxRes:
        type: prov:index:Aux
        properties:
            auxes: %s
            nestedAuxes: %s
    mainRes:
        type: prov:index:NestedTest
        properties:
            tests:
                - nestedProps:
                    - testProps:
                        - ${auxRes.nestedAuxes[0].nestedProps[0].testProps[0]}
`,
			nestedProvTestKnownProgram,
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    + prov:index/nestedTest:NestedTest: (create)
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
        tests     : [
            [0]: {
                nestedProps: [
                    [0]: {
                        testProps : [
                            [0]: output<string>
                        ]
                    }
                ]
            }
        ]
`),
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/nestedTest:NestedTest: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/nestedTest:NestedTest::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ nestedProps: [
                      ~ [0]: {
                              ~ testProps: [
                                  ~ [0]: "known_val" => output<string>
                                ]
                            }
                    ]
                }
        ]
`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			computedProgram := fmt.Sprintf(tc.program, "null", "null")

			t.Run("initial preview", func(t *testing.T) {
				pt := pulcheck.PulCheck(t, bridgedProvider, computedProgram)
				res := pt.Preview(t, optpreview.Diff())
				t.Log(res.StdOut)

				tc.expectedInitial.Equal(t, trimDiff(t, res.StdOut))
			})

			t.Run("update preview", func(t *testing.T) {
				t.Parallel()
				t.Skipf("Skipping this test as it this case is not handled by the TF plugin sdk")
				// The TF plugin SDK does not handle removing an input for a computed value, even if the provider implements it.
				// The plugin SDK always fills an empty Computed property with the value from the state.
				// Diff in these cases always returns no diff and the old state value is used.
				nonComputedProgram := fmt.Sprintf(tc.program, "[{testProp: \"val1\"}]", "[{nestedProps: [{testProps: [\"val1\"]}]}]")
				pt := pulcheck.PulCheck(t, bridgedProvider, nonComputedProgram)
				pt.Up(t)

				pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

				err := os.WriteFile(pulumiYamlPath, []byte(computedProgram), 0o600)
				require.NoError(t, err)

				res := pt.Preview(t, optpreview.Diff())
				t.Log(res.StdOut)
				tc.expectedUpdate.Equal(t, trimDiff(t, res.StdOut))
			})

			t.Run("update preview with computed", func(t *testing.T) {
				t.Parallel()
				pt := pulcheck.PulCheck(t, bridgedProvider, tc.initialKnownProgram)
				pt.Up(t)

				pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

				err := os.WriteFile(pulumiYamlPath, []byte(computedProgram), 0o600)
				require.NoError(t, err)

				res := pt.Preview(t, optpreview.Diff())
				t.Log(res.StdOut)
				tc.expectedUpdate.Equal(t, trimDiff(t, res.StdOut))
			})
		})
	}
}

func TestDetailedDiffPlainTypes(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"string_prop": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"list_prop": {
					Type:     schema.TypeList,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"set_prop": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"map_prop": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem:     &schema.Schema{Type: schema.TypeString},
				},
				"list_block": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
				"set_block": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
				"max_items_one_block": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
		},
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties: %s
`

	for _, tc := range []struct {
		name     string
		props1   interface{}
		props2   interface{}
		expected autogold.Value
	}{
		{
			"string unchanged",
			map[string]interface{}{"stringProp": "val"},
			map[string]interface{}{"stringProp": "val"},
			autogold.Expect("\n"),
		},
		{
			"string added",
			map[string]interface{}{},
			map[string]interface{}{"stringProp": "val"},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + stringProp: "val"
`),
		},
		{
			"string removed",
			map[string]interface{}{"stringProp": "val1"},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - stringProp: "val1"
`),
		},
		{
			"string changed",
			map[string]interface{}{"stringProp": "val1"},
			map[string]interface{}{"stringProp": "val2"},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ stringProp: "val1" => "val2"
`),
		},
		{
			"list unchanged",
			map[string]interface{}{"listProps": []interface{}{"val"}},
			map[string]interface{}{"listProps": []interface{}{"val"}},
			autogold.Expect("\n"),
		},
		{
			"list added",
			map[string]interface{}{},
			map[string]interface{}{"listProps": []interface{}{"val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + listProps: [
      +     [0]: "val"
        ]
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"list added empty",
			map[string]interface{}{},
			map[string]interface{}{"listProps": []interface{}{}},
			autogold.Expect("\n"),
		},
		{
			"list removed",
			map[string]interface{}{"listProps": []interface{}{"val"}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - listProps: [
      -     [0]: "val"
        ]
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"list removed empty",
			map[string]interface{}{"listProps": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect("\n"),
		},
		{
			"list element added front",
			map[string]interface{}{"listProps": []interface{}{"val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [0]: "val2" => "val1"
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
`),
		},
		{
			"list element added back",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          + [2]: "val3"
        ]
`),
		},
		{
			"list element added middle",
			map[string]interface{}{"listProps": []interface{}{"val1", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
`),
		},
		{
			"list element removed front",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [0]: "val1" => "val2"
          ~ [1]: "val2" => "val3"
          - [2]: "val3"
        ]
`),
		},
		{
			"list element removed back",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          - [2]: "val3"
        ]
`),
		},
		{
			"list element removed middle",
			map[string]interface{}{"listProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"listProps": []interface{}{"val1", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [1]: "val2" => "val3"
          - [2]: "val3"
        ]
`),
		},
		{
			"list element changed",
			map[string]interface{}{"listProps": []interface{}{"val1"}},
			map[string]interface{}{"listProps": []interface{}{"val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listProps: [
          ~ [0]: "val1" => "val2"
        ]
`),
		},
		{
			"set unchanged",
			map[string]interface{}{"setProps": []interface{}{"val"}},
			map[string]interface{}{"setProps": []interface{}{"val"}},
			autogold.Expect("\n"),
		},
		{
			"set added",
			map[string]interface{}{},
			map[string]interface{}{"setProps": []interface{}{"val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + setProps: [
      +     [0]: "val"
        ]
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"set added empty",
			map[string]interface{}{},
			map[string]interface{}{"setProps": []interface{}{}},
			autogold.Expect("\n"),
		},
		{
			"set removed",
			map[string]interface{}{"setProps": []interface{}{"val"}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - setProps: [
      -     [0]: "val"
        ]
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"set removed empty",
			map[string]interface{}{"setProps": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect("\n"),
		},
		{
			"set element added front",
			map[string]interface{}{"setProps": []interface{}{"val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          + [0]: "val1"
        ]
`),
		},
		{
			"set element added back",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          + [2]: "val3"
        ]
`),
		},
		{
			"set element added middle",
			map[string]interface{}{"setProps": []interface{}{"val1", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          + [1]: "val2"
        ]
`),
		},
		{
			"set element removed front",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val2", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          - [0]: "val1"
        ]
`),
		},
		{
			"set element removed back",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          - [2]: "val3"
        ]
`),
		},
		{
			"set element removed middle",
			map[string]interface{}{"setProps": []interface{}{"val1", "val2", "val3"}},
			map[string]interface{}{"setProps": []interface{}{"val1", "val3"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          - [1]: "val2"
        ]
`),
		},
		{
			"set element changed",
			map[string]interface{}{"setProps": []interface{}{"val1"}},
			map[string]interface{}{"setProps": []interface{}{"val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setProps: [
          ~ [0]: "val1" => "val2"
        ]
`),
		},
		{
			"map unchanged",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			autogold.Expect("\n"),
		},
		{
			"map added",
			map[string]interface{}{},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + mapProp: {
          + key: "val"
        }
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"map added empty",
			map[string]interface{}{},
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			autogold.Expect("\n"),
		},
		{
			"map removed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - mapProp: {
          - key: "val"
        }
`),
		},
		// pulumi/pulumi-terraform-bridge#2233: This is the intended behavior
		{
			"map removed empty",
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			map[string]interface{}{},
			autogold.Expect("\n"),
		},
		{
			"map element added",
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + mapProp: {
          + key: "val"
        }
`),
		},
		{
			"map element removed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val"}},
			map[string]interface{}{"mapProp": map[string]interface{}{}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ mapProp: {
          - key: "val"
        }
`),
		},
		{
			"map value changed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val1"}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key": "val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ mapProp: {
          ~ key: "val1" => "val2"
        }
`),
		},
		{
			"map key changed",
			map[string]interface{}{"mapProp": map[string]interface{}{"key1": "val"}},
			map[string]interface{}{"mapProp": map[string]interface{}{"key2": "val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ mapProp: {
          - key1: "val"
          + key2: "val"
        }
`),
		},
		{
			"list block unchanged",
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect("\n"),
		},
		{
			"list block added",
			map[string]interface{}{},
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + listBlocks: [
      +     [0]: {
              + prop      : "val"
            }
        ]
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"list block added empty",
			map[string]interface{}{},
			map[string]interface{}{"listBlocks": []interface{}{}},
			autogold.Expect("\n"),
		},
		{
			"list block added empty object",
			map[string]interface{}{},
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{}}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + listBlocks: [
      +     [0]: {
            }
        ]
`),
		},
		{
			"list block removed",
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - listBlocks: [
      -     [0]: {
              - prop: "val"
            }
        ]
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"list block removed empty",
			map[string]interface{}{"listBlocks": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect("\n"),
		},
		{
			"list block removed empty object",
			map[string]interface{}{"listBlocks": []interface{}{map[string]interface{}{}}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - listBlocks: [
      -     [0]: {
              - prop: <null>
            }
        ]
`),
		},
		{
			"list block element added front",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [0]: {
                  ~ prop: "val2" => "val1"
                }
          ~ [1]: {
                  ~ prop: "val3" => "val2"
                }
          + [2]: {
                  + prop      : "val3"
                }
        ]
`),
		},
		{
			"list block element added back",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          + [2]: {
                  + prop      : "val3"
                }
        ]
`),
		},
		{
			"list block element added middle",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [1]: {
                  ~ prop: "val3" => "val2"
                }
          + [2]: {
                  + prop      : "val3"
                }
        ]
`),
		},
		{
			"list block element removed front",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [0]: {
                  ~ prop: "val1" => "val2"
                }
          ~ [1]: {
                  ~ prop: "val2" => "val3"
                }
          - [2]: {
                  - prop: "val3"
                }
        ]
`),
		},
		{
			"list block element removed back",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          - [2]: {
                  - prop: "val3"
                }
        ]
`),
		},
		{
			"list block element removed middle",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [1]: {
                  ~ prop: "val2" => "val3"
                }
          - [2]: {
                  - prop: "val3"
                }
        ]
`),
		},
		{
			"list block element changed",
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
			}},
			map[string]interface{}{"listBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ listBlocks: [
          ~ [0]: {
                  ~ prop: "val1" => "val2"
                }
        ]
`),
		},
		{
			"set block unchanged",
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect("\n"),
		},
		{
			"set block added",
			map[string]interface{}{},
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + setBlocks: [
      +     [0]: {
              + prop      : "val"
            }
        ]
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"set block added empty",
			map[string]interface{}{},
			map[string]interface{}{"setBlocks": []interface{}{}},
			autogold.Expect("\n"),
		},
		{
			"set block added empty object",
			map[string]interface{}{},
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{}}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + setBlocks: [
      +     [0]: {
            }
        ]
`),
		},
		{
			"set block removed",
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{"prop": "val"}}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - setBlocks: [
      -     [0]: {
              - prop: "val"
            }
        ]
`),
		},
		// This is expected to be a no-op because blocks can not be nil in TF
		{
			"set block removed empty",
			map[string]interface{}{"setBlocks": []interface{}{}},
			map[string]interface{}{},
			autogold.Expect("\n"),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2399] nested prop diff
		{
			"set block removed empty object",
			map[string]interface{}{"setBlocks": []interface{}{map[string]interface{}{}}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - setBlocks: [
      -     [0]: {
              - prop: ""
            }
        ]
`),
		},
		{
			"set block element added front",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          + [0]: {
                  + prop      : "val1"
                }
        ]
`),
		},
		{
			"set block element added back",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          + [2]: {
                  + prop      : "val3"
                }
        ]
`),
		},
		{
			"set block element added middle",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          + [1]: {
                  + prop      : "val2"
                }
        ]
`),
		},
		{
			"set block element removed front",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          - [0]: {
                  - prop: "val1"
                }
        ]
`),
		},
		{
			"set block element removed back",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          - [2]: {
                  - prop: "val3"
                }
        ]
`),
		},
		{
			"set block element removed middle",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val2"},
				map[string]interface{}{"prop": "val3"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
				map[string]interface{}{"prop": "val3"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          - [1]: {
                  - prop: "val2"
                }
        ]
`),
		},
		{
			"set block element changed",
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val1"},
			}},
			map[string]interface{}{"setBlocks": []interface{}{
				map[string]interface{}{"prop": "val2"},
			}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ setBlocks: [
          ~ [0]: {
                  ~ prop: "val1" => "val2"
                }
        ]
`),
		},
		{
			"maxItemsOne block unchanged",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			autogold.Expect("\n"),
		},
		{
			"maxItemsOne block added",
			map[string]interface{}{},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + maxItemsOneBlock: {
          + prop      : "val"
        }
`),
		},
		{
			"maxItemsOne block added empty",
			map[string]interface{}{},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + maxItemsOneBlock: {
        }
`),
		},
		{
			"maxItemsOne block removed",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val"}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - maxItemsOneBlock: {
          - prop: "val"
        }
`),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2399] nested prop diff
		{
			"maxItemsOne block removed empty",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{}},
			map[string]interface{}{},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - maxItemsOneBlock: {
          - prop: <null>
        }
`),
		},
		{
			"maxItemsOne block changed",
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val1"}},
			map[string]interface{}{"maxItemsOneBlock": map[string]interface{}{"prop": "val2"}},
			autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ maxItemsOneBlock: {
          ~ prop: "val1" => "val2"
        }
`),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			props1, err := json.Marshal(tc.props1)
			require.NoError(t, err)
			program1 := fmt.Sprintf(program, string(props1))
			props2, err := json.Marshal(tc.props2)
			require.NoError(t, err)
			program2 := fmt.Sprintf(program, string(props2))
			pt := pulcheck.PulCheck(t, bridgedProvider, program1)
			pt.Up(t)

			pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

			err = os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
			require.NoError(t, err)

			res := pt.Preview(t, optpreview.Diff())
			t.Log(res.StdOut)
			tc.expected.Equal(t, trimDiff(t, res.StdOut))
		})
	}
}

func TestUnknownCollectionForceNewDetailedDiff(t *testing.T) {
	t.Parallel()
	collectionForceNewResource := func(typ schema.ValueType) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     typ,
					Optional: true,
					ForceNew: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
		}
	}

	propertyForceNewResource := func(typ schema.ValueType) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     typ,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
								ForceNew: true,
							},
						},
					},
				},
			},
		}
	}

	auxResource := func(typ schema.ValueType) *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"aux": {
					Type:     typ,
					Computed: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"prop": {
								Type:     schema.TypeString,
								Computed: true,
							},
						},
					},
				},
			},
			CreateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				d.SetId("aux")
				err := d.Set("aux", []map[string]interface{}{{"prop": "aux"}})
				require.NoError(t, err)
				return nil
			},
		}
	}

	initialProgram := `
    name: test
    runtime: yaml
    resources:
      mainRes:
        type: prov:index:Test
        properties:
          tests: [{prop: 'value'}]
`

	program := `
    name: test
    runtime: yaml
    resources:
      auxRes:
        type: prov:index:Aux
      mainRes:
        type: prov:index:Test
        properties:
          tests: %s
`

	runTest := func(t *testing.T, program2 string, bridgedProvider info.Provider, expectedOutput autogold.Value) {
		pt := pulcheck.PulCheck(t, bridgedProvider, initialProgram)
		pt.Up(t)
		pt.WritePulumiYaml(t, program2)

		res := pt.Preview(t, optpreview.Diff())

		expectedOutput.Equal(t, trimDiff(t, res.StdOut))
	}

	t.Run("list force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": collectionForceNewResource(schema.TypeList),
			"prov_aux":  auxResource(schema.TypeList),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())
		runTest := func(t *testing.T, program2 string, expectedOutput autogold.Value) {
			runTest(t, program2, bridgedProvider, expectedOutput)
		}

		t.Run("unknown plain property", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[{prop: \"${auxRes.auxes[0].prop}\"}]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ prop: "value" => output<string>
                }
        ]
`))
		})

		t.Run("unknown object", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[\"${auxRes.auxes[0]}\"]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - prop: "value"
                }
          + [0]: output<string>
        ]
`))
		})

		t.Run("unknown collection", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "\"${auxRes.auxes}\"")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - prop: "value"
            }
        ]
      + tests: output<string>
`))
		})
	})

	t.Run("list property force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": propertyForceNewResource(schema.TypeList),
			"prov_aux":  auxResource(schema.TypeList),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())
		runTest := func(t *testing.T, program2 string, expectedOutput autogold.Value) {
			runTest(t, program2, bridgedProvider, expectedOutput)
		}

		t.Run("unknown plain property", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[{prop: \"${auxRes.auxes[0].prop}\"}]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ prop: "value" => output<string>
                }
        ]
`))
		})

		t.Run("unknown object", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[\"${auxRes.auxes[0]}\"]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - prop: "value"
                }
          + [0]: output<string>
        ]
`))
		})

		t.Run("unknown collection", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "\"${auxRes.auxes}\"")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - prop: "value"
            }
        ]
      + tests: output<string>
`))
		})
	})

	t.Run("set force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": collectionForceNewResource(schema.TypeSet),
			"prov_aux":  auxResource(schema.TypeSet),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())
		runTest := func(t *testing.T, program2 string, expectedOutput autogold.Value) {
			runTest(t, program2, bridgedProvider, expectedOutput)
		}

		t.Run("unknown plain property", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[{prop: \"${auxRes.auxes[0].prop}\"}]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ prop: "value" => output<string>
                }
        ]
`))
		})

		t.Run("unknown object", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[\"${auxRes.auxes[0]}\"]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - prop: "value"
                }
          + [0]: output<string>
        ]
`))
		})

		t.Run("unknown collection", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "\"${auxRes.auxes}\"")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - prop: "value"
            }
        ]
      + tests: output<string>
`))
		})
	})

	t.Run("set property force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": propertyForceNewResource(schema.TypeSet),
			"prov_aux":  auxResource(schema.TypeSet),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())
		runTest := func(t *testing.T, program2 string, expectedOutput autogold.Value) {
			runTest(t, program2, bridgedProvider, expectedOutput)
		}

		t.Run("unknown plain property", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[{prop: \"${auxRes.auxes[0].prop}\"}]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: {
                  ~ prop: "value" => output<string>
                }
        ]
`))
		})

		t.Run("unknown object", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[\"${auxRes.auxes[0]}\"]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - prop: "value"
                }
          + [0]: output<string>
        ]
`))
		})

		t.Run("unknown collection", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "\"${auxRes.auxes}\"")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    +-prov:index/test:Test: (replace)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - prop: "value"
            }
        ]
      + tests: output<string>
`))
		})
	})
}
