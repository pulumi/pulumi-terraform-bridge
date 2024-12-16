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
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
)

func runDetailedDiffTest(
	t *testing.T, resMap map[string]*schema.Resource, program1, program2 string,
) (string, map[string]interface{}) {
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())
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
      +     [0]: output<string>
        ]
    --outputs:--
  + testOut: output<string>
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
          ~ [0]: "val1" => output<string>
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
          ~ [0]: "val2" => output<string>
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
          ~ [1]: "val3" => output<string>
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
          + [2]: output<string>
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
          ~ [1]: "val2" => output<string>
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
          ~ [0]: "val2" => output<string>
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
          ~ [1]: "val3" => output<string>
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
          + [2]: output<string>
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
      + tests: output<string>
    --outputs:--
  + testOut: output<string>
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
      + tests: output<string>
`),
			autogold.Expect(map[string]interface{}{"tests": map[string]interface{}{"kind": "UPDATE"}}),
		)
	})
}

func TestDetailedDiffSetDuplicates(t *testing.T) {
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
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.EnableAccurateBridgePreviews())

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      tests: %s`

	t.Run("pulumi", func(t *testing.T) {
		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, `["a", "a"]`))
		pt.Up(t)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, `["b", "b", "a", "a", "c"]`))

		res := pt.Preview(t, optpreview.Diff())

		autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: "b"
          + [4]: "c"
        ]
`).Equal(t, trimDiff(t, res.StdOut))
	})

	t.Run("terraform", func(t *testing.T) {
		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfp)
		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test = ["a", "a"]
}`)

		plan, err := tfdriver.Plan(t)
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)

		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test = ["b", "b", "a", "a", "c"]
}`)

		plan, err = tfdriver.Plan(t)
		require.NoError(t, err)

		autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # prov_test.mainRes will be updated in-place
  ~ resource "prov_test" "mainRes" {
        id   = "newid"
      ~ test = [
          + "b",
          + "c",
            # (1 unchanged element hidden)
        ]
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, plan.StdOut)
	})
}

func TestDetailedDiffSetNestedAttributeUpdated(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"nested2": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"nested3": {
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
    properties:
      tests: %s`

	t.Run("pulumi", func(t *testing.T) {
		props1 := []map[string]string{
			{"nested": "b", "nested2": "b", "nested3": "b"},
			{"nested": "a", "nested2": "a", "nested3": "a"},
			{"nested": "c", "nested2": "c", "nested3": "c"},
		}
		props2 := []map[string]string{
			{"nested": "b", "nested2": "b", "nested3": "b"},
			{"nested": "d", "nested2": "a", "nested3": "a"},
			{"nested": "c", "nested2": "c", "nested3": "c"},
		}

		props1JSON, err := json.Marshal(props1)
		require.NoError(t, err)

		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, string(props1JSON)))
		pt.Up(t)

		props2JSON, err := json.Marshal(props2)
		require.NoError(t, err)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, string(props2JSON)))

		res := pt.Preview(t, optpreview.Diff())

		autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          - [0]: {
                  - nested : "a"
                  - nested2: "a"
                  - nested3: "a"
                }
          + [1]: {
                  + nested    : "d"
                  + nested2   : "a"
                  + nested3   : "a"
                }
        ]
`).Equal(t, trimDiff(t, res.StdOut))
	})

	t.Run("terraform", func(t *testing.T) {
		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfp)
		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "b"
	  nested2 = "b"
	  nested3 = "b"
	}
 test {
	  nested = "a"	
	  nested2 = "a"
	  nested3 = "a"
	}
 test {
	  nested = "c"
	  nested2 = "c"
	  nested3 = "c"
	}
}`)

		plan, err := tfdriver.Plan(t)
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)

		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "b"
	  nested2 = "b"
	  nested3 = "b"
	}
 test {
	  nested = "d"	
	  nested2 = "a"
	  nested3 = "a"
	}
 test {
	  nested = "c"
	  nested2 = "c"
	  nested3 = "c"
	}
}`)

		plan, err = tfdriver.Plan(t)
		require.NoError(t, err)

		autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # prov_test.mainRes will be updated in-place
  ~ resource "prov_test" "mainRes" {
        id = "newid"

      - test {
          - nested  = "a" -> null
          - nested2 = "a" -> null
          - nested3 = "a" -> null
        }
      + test {
          + nested  = "d"
          + nested2 = "a"
          + nested3 = "a"
        }

        # (2 unchanged blocks hidden)
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, plan.StdOut)
	})
}

func TestDetailedDiffSetComputedNestedAttribute(t *testing.T) {
	t.Parallel()
	resCount := 0
	setComputedProp := func(t *testing.T, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
		testSet := d.Get("test").(*schema.Set)
		testVals := testSet.List()
		newTestVals := make([]interface{}, len(testVals))
		for i, v := range testVals {
			val := v.(map[string]interface{})
			if val["computed"] == nil || val["computed"] == "" {
				val["computed"] = fmt.Sprint(resCount)
				resCount++
			}
			newTestVals[i] = val
		}

		err := d.Set("test", schema.NewSet(testSet.F, newTestVals))
		require.NoError(t, err)
		return nil
	}

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"computed": {
								Type:     schema.TypeString,
								Optional: true,
								Computed: true,
							},
						},
					},
				},
			},
			CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
				d.SetId("id")
				return setComputedProp(t, d, i)
			},
			UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
				return setComputedProp(t, d, i)
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
    properties:
      tests: %s`

	t.Run("pulumi", func(t *testing.T) {
		props1 := []map[string]string{
			{"nested": "a", "computed": "b"},
		}
		props1JSON, err := json.Marshal(props1)
		require.NoError(t, err)

		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, string(props1JSON)))
		pt.Up(t)

		props2 := []map[string]string{
			{"nested": "a"},
			{"nested": "a", "computed": "b"},
		}
		props2JSON, err := json.Marshal(props2)
		require.NoError(t, err)

		pt.WritePulumiYaml(t, fmt.Sprintf(program, string(props2JSON)))
		res := pt.Preview(t, optpreview.Diff())

		// TODO[pulumi/pulumi-terraform-bridge#2528]: The preview is wrong here because of the computed property
		autogold.Expect(`
    ~ prov:index/test:Test: (update)
        [id=id]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ tests: [
          + [1]: {
                  + computed  : "b"
                  + nested    : "a"
                }
        ]
`).Equal(t, trimDiff(t, res.StdOut))
	})

	t.Run("terraform", func(t *testing.T) {
		resCount = 0
		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", tfp)
		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "a"
	  computed = "b"
	}
}`)

		plan, err := tfdriver.Plan(t)
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)

		tfdriver.Write(t, `
resource "prov_test" "mainRes" {
  test {
	  nested = "a"
	  computed = "b"
	}
  test {
	  nested = "a"
	}
}`)
		plan, err = tfdriver.Plan(t)
		require.NoError(t, err)

		autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # prov_test.mainRes will be updated in-place
  ~ resource "prov_test" "mainRes" {
        id = "id"

      + test {
          + computed = (known after apply)
          + nested   = "a"
        }

        # (1 unchanged block hidden)
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, plan.StdOut)
	})
}

func TestDetailedDiffSet(t *testing.T) {
	t.Parallel()

	attributeSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}

	attributeSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				ForceNew: true,
			},
		},
	}

	blockSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	blockSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	blockSchemaNestedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
		},
	}

	attrList := func(arr *[]string) cty.Value {
		if arr == nil {
			return cty.NullVal(cty.DynamicPseudoType)
		}
		slice := make([]cty.Value, len(*arr))
		for i, v := range *arr {
			slice[i] = cty.StringVal(v)
		}
		if len(slice) == 0 {
			return cty.ListValEmpty(cty.String)
		}
		return cty.ListVal(slice)
	}

	nestedAttrList := func(arr *[]string) cty.Value {
		if arr == nil {
			return cty.NullVal(cty.DynamicPseudoType)
		}
		slice := make([]cty.Value, len(*arr))
		for i, v := range *arr {
			slice[i] = cty.ObjectVal(
				map[string]cty.Value{
					"nested": cty.StringVal(v),
				},
			)
		}
		if len(slice) == 0 {
			return cty.ListValEmpty(cty.Object(map[string]cty.Type{"nested": cty.String}))
		}
		return cty.ListVal(slice)
	}

	schemaValueMakerPairs := []struct {
		name       string
		res        schema.Resource
		valueMaker func(*[]string) cty.Value
	}{
		{"attribute no force new", attributeSchema, attrList},
		{"block no force new", blockSchema, nestedAttrList},
		{"attribute force new", attributeSchemaForceNew, attrList},
		{"block force new", blockSchemaForceNew, nestedAttrList},
		{"block nested force new", blockSchemaNestedForceNew, nestedAttrList},
	}

	scenarios := []struct {
		name         string
		initialValue *[]string
		changeValue  *[]string
	}{
		{"unchanged non-empty", &[]string{"value"}, &[]string{"value"}},
		{"unchanged empty", &[]string{}, &[]string{}},
		{"unchanged null", nil, nil},

		{"changed non-null", &[]string{"value"}, &[]string{"value1"}},
		{"changed null to non-null", nil, &[]string{"value"}},
		{"changed non-null to null", &[]string{"value"}, nil},
		{"changed null to empty", nil, &[]string{}},
		{"changed empty to null", &[]string{}, nil},

		{"added", &[]string{}, &[]string{"value"}},
		{"removed", &[]string{"value"}, &[]string{}},

		{"removed front", &[]string{"val1", "val2", "val3"}, &[]string{"val2", "val3"}},
		{"removed front unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val3", "val1"}},
		{"removed middle", &[]string{"val1", "val2", "val3"}, &[]string{"val1", "val3"}},
		{"removed middle unordered", &[]string{"val3", "val1", "val2"}, &[]string{"val3", "val1"}},
		{"removed end", &[]string{"val1", "val2", "val3"}, &[]string{"val1", "val2"}},
		{"removed end unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val2", "val3"}},

		{"added front", &[]string{"val2", "val3"}, &[]string{"val1", "val2", "val3"}},
		{"added front unordered", &[]string{"val3", "val1"}, &[]string{"val2", "val3", "val1"}},
		{"added middle", &[]string{"val1", "val3"}, &[]string{"val1", "val2", "val3"}},
		{"added middle unordered", &[]string{"val2", "val1"}, &[]string{"val2", "val3", "val1"}},
		{"added end", &[]string{"val1", "val2"}, &[]string{"val1", "val2", "val3"}},
		{"added end unordered", &[]string{"val2", "val3"}, &[]string{"val2", "val3", "val1"}},

		{"same element updated", &[]string{"val1", "val2", "val3"}, &[]string{"val1", "val4", "val3"}},
		{"same element updated unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val2", "val4", "val1"}},

		{"shuffled", &[]string{"val1", "val2", "val3"}, &[]string{"val3", "val1", "val2"}},
		{"shuffled unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val3", "val1", "val2"}},
		{"shuffled with duplicates", &[]string{"val1", "val2", "val3"}, &[]string{"val3", "val1", "val2", "val3"}},
		{"shuffled with duplicates unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val3", "val1", "val2", "val3"}},

		{"shuffled added front", &[]string{"val2", "val3"}, &[]string{"val1", "val3", "val2"}},
		{"shuffled added middle", &[]string{"val1", "val3"}, &[]string{"val3", "val2", "val1"}},
		{"shuffled added end", &[]string{"val1", "val2"}, &[]string{"val2", "val1", "val3"}},

		{"shuffled removed front", &[]string{"val1", "val2", "val3"}, &[]string{"val3", "val2"}},
		{"shuffled removed middle", &[]string{"val1", "val2", "val3"}, &[]string{"val3", "val1"}},
		{"shuffled removed end", &[]string{"val1", "val2", "val3"}, &[]string{"val2", "val1"}},

		{"two added", &[]string{"val1", "val2"}, &[]string{"val1", "val2", "val3", "val4"}},
		{"two removed", &[]string{"val1", "val2", "val3", "val4"}, &[]string{"val1", "val2"}},
		{"two added and two removed", &[]string{"val1", "val2", "val3", "val4"}, &[]string{"val1", "val2", "val5", "val6"}},
		{"two added and two removed shuffled, one overlaps", &[]string{"val1", "val2", "val3", "val4"}, &[]string{"val1", "val5", "val6", "val2"}},
		{"two added and two removed shuffled, no overlaps", &[]string{"val1", "val2", "val3", "val4"}, &[]string{"val5", "val6", "val1", "val2"}},
		{"two added and two removed shuffled, with duplicates", &[]string{"val1", "val2", "val3", "val4"}, &[]string{"val1", "val5", "val6", "val2", "val1", "val2"}},
	}

	type testOutput struct {
		initialValue *[]string
		changeValue  *[]string
		tfOut        string
		pulumiOut    string
		detailedDiff map[string]any
	}

	runTest := func(t *testing.T, schema schema.Resource, valueMaker func(*[]string) cty.Value, val1 *[]string, val2 *[]string) {
		initialValue := valueMaker(val1)
		changeValue := valueMaker(val2)

		diff := crosstests.Diff(t, &schema, map[string]cty.Value{"test": initialValue}, map[string]cty.Value{"test": changeValue})

		autogold.ExpectFile(t, testOutput{
			initialValue: val1,
			changeValue:  val2,
			tfOut:        diff.TFOut,
			pulumiOut:    diff.PulumiOut,
			detailedDiff: diff.PulumiDiff.DetailedDiff,
		})
	}

	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					runTest(t, schemaValueMakerPair.res, schemaValueMakerPair.valueMaker, scenario.initialValue, scenario.changeValue)
				})
			}
		})
	}
}
