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
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
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
	require.Contains(t, res.StdOut, "test      : [unknown]")
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
        tests     : [unknown]
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
      + tests: [unknown]
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
            [0]: [unknown]
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
          + [0]: [unknown]
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
            [0]: [unknown]
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
          + [0]: [unknown]
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
        tests     : [unknown]
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
      + tests: [unknown]
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
            [0]: [unknown]
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
          + [0]: [unknown]
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
                nestedProps: [unknown]
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
                  + nestedProps: [unknown]
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
                    [0]: [unknown]
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
                      + [0]: [unknown]
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
                        testProps : [unknown]
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
                              + testProps: [unknown]
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
                            [0]: [unknown]
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
                                  ~ [0]: "known_val" => [unknown]
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
				t.Skipf("Skipping this test as it this case is not handled by the TF plugin sdk")
				t.Parallel()
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
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
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
                  ~ prop: "value" => [unknown]
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
          + [0]: [unknown]
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
      + tests: [unknown]
`))
		})
	})

	t.Run("list property force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": propertyForceNewResource(schema.TypeList),
			"prov_aux":  auxResource(schema.TypeList),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
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
                  ~ prop: "value" => [unknown]
                }
        ]
`))
		})

		t.Run("unknown object", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[\"${auxRes.auxes[0]}\"]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - prop: "value"
                }
          + [0]: [unknown]
        ]
`))
		})

		t.Run("unknown collection", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "\"${auxRes.auxes}\"")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - prop: "value"
            }
        ]
      + tests: [unknown]
`))
		})
	})

	t.Run("set force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": collectionForceNewResource(schema.TypeSet),
			"prov_aux":  auxResource(schema.TypeSet),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
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
                  ~ prop: "value" => [unknown]
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
          + [0]: [unknown]
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
      + tests: [unknown]
`))
		})
	})

	t.Run("set property force new", func(t *testing.T) {
		resMap := map[string]*schema.Resource{
			"prov_test": propertyForceNewResource(schema.TypeSet),
			"prov_aux":  auxResource(schema.TypeSet),
		}

		tfp := &schema.Provider{ResourcesMap: resMap}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
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
                  ~ prop: "value" => [unknown]
                }
        ]
`))
		})

		t.Run("unknown object", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "[\"${auxRes.auxes[0]}\"]")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - prop: "value"
                }
          + [0]: [unknown]
        ]
`))
		})

		t.Run("unknown collection", func(t *testing.T) {
			program2 := fmt.Sprintf(program, "\"${auxRes.auxes}\"")
			runTest(t, program2, autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: {
              - prop: "value"
            }
        ]
      + tests: [unknown]
`))
		})
	})
}

func runDetailedDiffTest(
	t *testing.T, resMap map[string]*schema.Resource, program1, program2 string,
) (string, map[string]interface{}) {
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
	pt := pulcheck.PulCheck(t, bridgedProvider, program1)
	pt.Up(t)
	pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")

	err := os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
	require.NoError(t, err)

	pt.ClearGrpcLog(t)
	res := pt.Preview(t, optpreview.Diff())
	t.Log(res.StdOut)

	diffResponse := struct {
		DetailedDiff map[string]interface{} `json:"detailedDiff"`
	}{}

	for _, entry := range pt.GrpcLog(t).Entries {
		if entry.Method == "/pulumirpc.ResourceProvider/Diff" {
			err := json.Unmarshal(entry.Response, &diffResponse)
			require.NoError(t, err)
		}
	}

	return res.StdOut, diffResponse.DetailedDiff
}

// "UNKNOWN" for unknown values
func testDetailedDiffWithUnknowns(t *testing.T, resMap map[string]*schema.Resource, unknownString string, props1, props2 interface{}, expected, expectedDetailedDiff autogold.Value) {
	originalProgram := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
	properties:
		tests: %s
outputs:
  testOut: ${mainRes.tests}
	`
	props1JSON, err := json.Marshal(props1)
	require.NoError(t, err)
	program1 := fmt.Sprintf(originalProgram, string(props1JSON))

	programWithUnknown := `
name: test
runtime: yaml
resources:
  auxRes:
    type: prov:index:Aux
  mainRes:
    type: prov:index:Test
    properties:
      tests: %s
outputs:
  testOut: ${mainRes.tests}
`
	props2JSON, err := json.Marshal(props2)
	require.NoError(t, err)
	program2 := fmt.Sprintf(programWithUnknown, string(props2JSON))
	program2 = strings.ReplaceAll(program2, "UNKNOWN", unknownString)

	out, detailedDiff := runDetailedDiffTest(t, resMap, program1, program2)
	expected.Equal(t, trimDiff(t, out))
	expectedDetailedDiff.Equal(t, detailedDiff)
}

func TestDetailedDiffUnknownSetAttributeElement(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
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

	t.Run("empty to unknown element", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{},
			[]interface{}{"UNKNOWN"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [
      +     [0]: [unknown]
        ]
    --outputs:--
  + testOut: [unknown]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}))
	})

	t.Run("non-empty to unknown element", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1"},
			[]interface{}{"UNKNOWN"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => [unknown]
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}))
	})

	t.Run("unknown element added front", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val2", "val3"},
			[]interface{}{"UNKNOWN", "val2", "val3"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val2" => [unknown]
          ~ [1]: "val3" => "val2"
          + [2]: "val3"
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("unknown element added middle", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val3"},
			[]interface{}{"val1", "UNKNOWN", "val3"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
            [0]: "val1"
          ~ [1]: "val3" => [unknown]
          + [2]: "val3"
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("unknown element added end", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2"},
			[]interface{}{"val1", "val2", "UNKNOWN"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
            [0]: "val1"
            [1]: "val2"
          + [2]: [unknown]
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("element updated to unknown", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2", "val3"},
			[]interface{}{"val1", "UNKNOWN", "val3"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
            [0]: "val1"
          ~ [1]: "val2" => [unknown]
            [2]: "val3"
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added front", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val2", "val3"},
			[]interface{}{"UNKNOWN", "val3", "val2"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val2" => [unknown]
            [1]: "val3"
          + [2]: "val2"
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added middle", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val3"},
			[]interface{}{"val3", "UNKNOWN", "val1"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val3"
          ~ [1]: "val3" => [unknown]
          + [2]: "val1"
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})

	t.Run("shuffled unknown added end", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.aux}",
			[]interface{}{"val1", "val2"},
			[]interface{}{"val2", "val1", "UNKNOWN"},
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          ~ [0]: "val1" => "val2"
          ~ [1]: "val2" => "val1"
          + [2]: [unknown]
        ]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})
}

func TestUnknownSetAttributeDiff(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		},
		"prov_aux": {
			Schema: map[string]*schema.Schema{
				"aux": {
					Type:     schema.TypeSet,
					Computed: true,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			CreateContext: func(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
				d.SetId("aux")
				err := d.Set("aux", []interface{}{"aux"})
				require.NoError(t, err)
				return nil
			},
		},
	}

	t.Run("empty to unknown set", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.auxes}",
			[]interface{}{},
			"UNKNOWN",
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      + tests: [unknown]
    --outputs:--
  + testOut: [unknown]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{}}),
		)
	})

	t.Run("non-empty to unknown set", func(t *testing.T) {
		testDetailedDiffWithUnknowns(t, resMap, "${auxRes.auxes}",
			[]interface{}{"val"},
			"UNKNOWN",
			autogold.Expect(`
    + prov:index/aux:Aux: (create)
        [urn=urn:pulumi:test::test::prov:index/aux:Aux::auxRes]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - tests: [
      -     [0]: "val"
        ]
      + tests: [unknown]
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})
}
