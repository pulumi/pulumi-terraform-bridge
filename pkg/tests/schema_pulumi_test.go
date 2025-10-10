package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func ref[T any](t T) *T {
	return &t
}

func TestSDKv2Basic(t *testing.T) {
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
	}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
	properties:
	  test: "hello"
outputs:
  testOut: ${mainRes.test}
`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	res := pt.Up(t)
	require.Equal(t, "hello", res.Outputs["testOut"].Value)
}

func TestBigIntOverride(t *testing.T) {
	t.Parallel()
	getZoneFromStack := func(data []byte) string {
		var stateMap map[string]interface{}
		err := json.Unmarshal(data, &stateMap)
		require.NoError(t, err)
		resourcesList := stateMap["resources"].([]interface{})
		// stack, provider, resource
		require.Len(t, resourcesList, 3)
		testResState := resourcesList[2].(map[string]interface{})
		resOutputs := testResState["outputs"].(map[string]interface{})
		return resOutputs["managedZoneId"].(string)
	}
	bigInt := 1<<62 + 1
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"prop": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"managed_zone_id": {
					Type:     schema.TypeInt,
					Computed: true,
				},
			},
			CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				rd.SetId("1")
				err := rd.Set("managed_zone_id", bigInt)
				require.NoError(t, err)
				return nil
			},
			UpdateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				require.Equal(t, bigInt, rd.Get("managed_zone_id").(int))
				return nil
			},
			UseJSONNumber: true,
		},
	}

	tfp := &schema.Provider{ResourcesMap: resMap}
	opts := []pulcheck.BridgedProviderOpt{}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, opts...)
	bridgedProvider.Resources["prov_test"] = &info.Resource{
		Tok: "prov:index:Test",
		Fields: map[string]*info.Schema{
			"managed_zone_id": {
				Type: "string",
			},
		},
	}

	program := `
name: test
runtime: yaml
resources:
    mainRes:
        type: prov:index:Test
        properties:
            prop: %s
`

	pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, "val"))
	pt.Up(t)

	// Check the state is correct
	stack := pt.ExportStack(t)
	data, err := stack.Deployment.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, fmt.Sprint(bigInt), getZoneFromStack(data))

	program2 := fmt.Sprintf(program, "val2")
	pulumiYamlPath := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
	err = os.WriteFile(pulumiYamlPath, []byte(program2), 0o600)
	require.NoError(t, err)

	pt.Up(t)
	// Check the state is correct
	stack = pt.ExportStack(t)
	data, err = stack.Deployment.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, fmt.Sprint(bigInt), getZoneFromStack(data))
}

func TestMakeTerraformResultNilVsEmptyMap(t *testing.T) {
	t.Parallel()
	// Nil and empty maps are not equal
	nilMap := resource.NewObjectProperty(nil)
	emptyMap := resource.NewObjectProperty(resource.PropertyMap{})

	assert.True(t, nilMap.DeepEquals(emptyMap))
	assert.NotEqual(t, emptyMap.ObjectValue(), nilMap.ObjectValue())

	// Check that MakeTerraformResult maintains that difference
	const resName = "prov_test"
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		},
	}

	prov := &schema.Provider{
		ResourcesMap: resMap,
	}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)

	ctx := context.Background()
	shimProv := bridgedProvider.P

	res := shimProv.ResourcesMap().Get(resName)

	t.Run("NilMap", func(t *testing.T) {
		// Create a resource with a nil map
		state, err := res.InstanceState("0", map[string]interface{}{}, map[string]interface{}{})
		assert.NoError(t, err)

		props, err := tfbridge.MakeTerraformResult(ctx, shimProv, state, res.Schema(), nil, nil, true)
		assert.NoError(t, err)
		assert.NotNil(t, props)
		assert.True(t, props["test"].V == nil)
	})

	t.Run("EmptyMap", func(t *testing.T) {
		// Create a resource with an empty map
		state, err := res.InstanceState("0", map[string]interface{}{"test": map[string]interface{}{}}, map[string]interface{}{})
		assert.NoError(t, err)

		props, err := tfbridge.MakeTerraformResult(ctx, shimProv, state, res.Schema(), nil, nil, true)
		assert.NoError(t, err)
		assert.NotNil(t, props)
		assert.True(t, props["test"].DeepEquals(emptyMap))
	})
}

func TestResourceInitFailure(t *testing.T) {
	t.Parallel()

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Required: true,
				},
			},
			CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				rd.SetId("1")
				return diag.Errorf("INIT TEST ERROR")
			},
		},
	}
	prov := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)

	pt := pulcheck.PulCheck(t, bridgedProvider, `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
	properties:
	  test: "hello"
`)

	_, err := pt.CurrentStack().Up(pt.Context())
	require.Error(t, err)
	require.ErrorContains(t, err, "INIT TEST ERROR")

	stack := pt.ExportStack(t)

	data, err := stack.Deployment.MarshalJSON()
	require.NoError(t, err)

	var stateMap map[string]interface{}
	err = json.Unmarshal(data, &stateMap)
	require.NoError(t, err)

	resourcesList := stateMap["resources"].([]interface{})
	require.Len(t, resourcesList, 3)
	mainResState := resourcesList[2].(map[string]interface{}) // stack, provider, resource
	initErrors := mainResState["initErrors"].([]interface{})
	require.Len(t, initErrors, 1)
	require.Contains(t, initErrors[0], "INIT TEST ERROR")
}

func TestUpdateResourceInitFailure(t *testing.T) {
	t.Parallel()

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Required: true,
				},
			},
			UpdateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				return diag.Errorf("UPDATE TEST ERROR")
			},
		},
	}
	prov := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      test: %s
`

	pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(program, "hello"))
	pt.Up(t)
	pt.WritePulumiYaml(t, fmt.Sprintf(program, "hello2"))

	_, err := pt.CurrentStack().Up(pt.Context())
	require.Error(t, err)
	require.ErrorContains(t, err, "UPDATE TEST ERROR")

	stack := pt.ExportStack(t)

	data, err := stack.Deployment.MarshalJSON()
	require.NoError(t, err)

	var stateMap map[string]interface{}
	err = json.Unmarshal(data, &stateMap)
	require.NoError(t, err)

	resourcesList := stateMap["resources"].([]interface{})
	// stack, provider, resource
	require.Len(t, resourcesList, 3)
	mainResState := resourcesList[2].(map[string]interface{})
	initErrors := mainResState["initErrors"].([]interface{})
	require.Len(t, initErrors, 1)
	require.Contains(t, initErrors[0], "UPDATE TEST ERROR")
}

func TestSDKv2AliasesSchemaUpgrade(t *testing.T) {
	t.Parallel()

	prov1 := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"test": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		},
	}
	bridgedProvider1 := pulcheck.BridgedProvider(t, "prov", prov1)

	prov2 := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test2": {Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
				},
			}},
		},
	}
	bridgedProvider2 := pulcheck.BridgedProvider(t, "prov", prov2, pulcheck.WithResourceInfo(map[string]*info.Resource{
		"prov_test2": {
			Aliases: []info.Alias{
				{
					Type: ref("prov:index/test:Test"),
				},
			},
		},
	}))

	pt := pulcheck.PulCheck(t, bridgedProvider1, `
    name: test
    runtime: yaml
    resources:
      mainRes:
        type: prov:index/test:Test
    	properties:
    	  test: "hello"
    `)

	pt.Up(t)
	stack := pt.ExportStack(t)

	yamlProgram := `
    name: test
    runtime: yaml
    resources:
      mainRes:
        type: prov:index/test2:Test2
    	properties:
    	  test: "hello"
    `

	pt2 := pulcheck.PulCheck(t, bridgedProvider2, yamlProgram)
	pt2.ImportStack(t, stack)

	res := pt2.Up(t)

	autogold.Expect(&map[string]int{"same": 2}).Equal(t, res.Summary.ResourceChanges)
}

func TestSDKv2AliasesRenameWithAlias(t *testing.T) {
	t.Parallel()

	prov1 := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"test": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		},
	}
	bridgedProvider1 := pulcheck.BridgedProvider(t, "prov", prov1)

	prov2 := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test2": {Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
				},
			}},
		},
	}
	bridgedProvider2 := pulcheck.BridgedProvider(t, "prov", prov2)
	bridgedProvider2.RenameResourceWithAlias(
		"prov_test2", "prov:index/test:Test", "prov:index/test2:Test2", "index", "index", nil)

	pt := pulcheck.PulCheck(t, bridgedProvider1, `
    name: test
    runtime: yaml
    resources:
      mainRes:
        type: prov:index/test:Test
    	properties:
    	  test: "hello"
    `)

	pt.Up(t)
	stack := pt.ExportStack(t)

	yamlProgram := `
    name: test
    runtime: yaml
    resources:
      mainRes:
        type: prov:index/test:Test
    	properties:
    	  test: "hello"
    `

	pt2 := pulcheck.PulCheck(t, bridgedProvider2, yamlProgram)
	pt2.ImportStack(t, stack)

	res := pt2.Up(t)

	autogold.Expect(&map[string]int{"same": 2}).Equal(t, res.Summary.ResourceChanges)
}

func TestPreviewSetElementWithUnknownBool(t *testing.T) {
	t.Parallel()

	fillBars := func(rd *schema.ResourceData) {
		setVal := rd.Get("test")
		if setVal == nil {
			return
		}
		set := setVal.(*schema.Set)
		setSlice := set.List()
		if len(setSlice) == 0 {
			return
		}
		newSetSlice := make([]interface{}, len(setSlice))
		for i, setElem := range setSlice {
			elemMap := setElem.(map[string]interface{})
			// bar is computed, so we need to set it
			elemMap["bar"] = false
			newSetSlice[i] = elemMap
		}
		setVal = schema.NewSet(set.F, newSetSlice)
		err := rd.Set("test", setVal)
		require.NoError(t, err)
	}

	prov := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {
				Schema: map[string]*schema.Schema{
					"test": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"bar": {
									Type:     schema.TypeBool,
									Computed: true,
									Optional: true,
								},
								"baz": {
									Type:     schema.TypeString,
									Optional: true,
								},
							},
						},
					},
				},
				CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
					rd.SetId("1")
					fillBars(rd)
					return nil
				},
				UpdateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
					fillBars(rd)
					return nil
				},
			},
		},
	}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)

	pt := pulcheck.PulCheck(t, bridgedProvider, `
    name: test
    runtime: yaml
    resources:
      mainRes:
        type: prov:index/test:Test
        properties:
          tests:
            - baz: "hello"`,
	)
	resUp := pt.Up(t)

	require.NotContains(t, resUp.StdErr, "Failed to calculate preview")
	require.NotContains(t, resUp.StdOut, "Failed to calculate preview")

	program2 := `
    name: test
    runtime: yaml
    resources:
      mainRes:
        type: prov:index/test:Test
        properties:
          tests:
            - baz: "hello"
            - baz: "world"`

	pt.WritePulumiYaml(t, program2)
	res := pt.Preview(t)

	require.NotContains(t, res.StdErr, "Failed to calculate preview")
	require.NotContains(t, res.StdOut, "Failed to calculate preview")
}

// The set hash function should never be called with nil, even if it is in the state.
func TestDiffSetHashFailsOnNil(t *testing.T) {
	t.Parallel()

	fillBars := func(rd *schema.ResourceData) {
		setVal := rd.Get("test")
		if setVal == nil {
			return
		}
		set := setVal.(*schema.Set)
		setSlice := set.List()
		if len(setSlice) == 0 {
			return
		}
		newSetSlice := make([]interface{}, len(setSlice))
		for i, setElem := range setSlice {
			elemMap := setElem.(map[string]interface{})
			// bar is computed, so we need to set it
			elemMap["bar"] = true
			newSetSlice[i] = elemMap
		}
		setVal = schema.NewSet(set.F, newSetSlice)
		err := rd.Set("test", setVal)
		require.NoError(t, err)
	}

	elemSch := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"bar": {
				Type:     schema.TypeBool,
				Computed: true,
				Optional: true,
			},
			"bar2": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"baz": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}

	res := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     elemSch,
				Set: func(v interface{}) int {
					for _, subV := range v.(map[string]interface{}) {
						if subV == nil {
							panic("nil value in hash func")
						}
					}
					return schema.HashResource(elemSch)(v)
				},
			},
		},
		CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
			rd.SetId("1")
			fillBars(rd)
			return nil
		},
		UpdateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
			fillBars(rd)
			return nil
		},
	}

	crosstests.Diff(t, &res, map[string]cty.Value{
		"test": cty.ListVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"baz": cty.StringVal("hello"),
			}),
		}),
	}, map[string]cty.Value{
		"test": cty.ListVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"baz": cty.StringVal("hello"),
			}),
			cty.ObjectVal(map[string]cty.Value{
				"baz": cty.StringVal("world"),
			}),
		}),
	},
	)
}

func TestDefaultsChanges(t *testing.T) {
	t.Parallel()

	for _, defVal := range []string{"hello", ""} {
		t.Run(fmt.Sprintf("default value %q", defVal), func(t *testing.T) {
			t.Parallel()

			for _, useDefaultFunc := range []bool{true, false} {
				t.Run(fmt.Sprintf("useDefaultFunc %t", useDefaultFunc), func(t *testing.T) {
					t.Parallel()
					defaultFunc := func() (interface{}, error) {
						return defVal, nil
					}

					var def any = defVal

					if !useDefaultFunc {
						defaultFunc = nil
					} else {
						def = nil
					}

					res := &schema.Resource{
						Schema: map[string]*schema.Schema{
							"test": {
								Type:        schema.TypeString,
								Optional:    true,
								DefaultFunc: defaultFunc,
								Default:     def,
							},
						},
					}

					configNoValue := map[string]cty.Value{}
					configWithValue := map[string]cty.Value{
						"test": cty.StringVal("world"),
					}
					configWithDefaultValue := map[string]cty.Value{
						"test": cty.StringVal(defVal),
					}

					t.Run("value removed from config", func(t *testing.T) {
						t.Parallel()

						crosstests.Diff(t, res, configWithValue, configNoValue)
					})

					t.Run("default value removed from config", func(t *testing.T) {
						t.Parallel()

						crosstests.Diff(t, res, configWithDefaultValue, configNoValue)
					})

					t.Run("default value added to config", func(t *testing.T) {
						t.Parallel()

						crosstests.Diff(t, res, configNoValue, configWithDefaultValue)
					})

					t.Run("value added to config", func(t *testing.T) {
						t.Parallel()

						crosstests.Diff(t, res, configNoValue, configWithValue)
					})
				})
			}
		})
	}
}

func TestDefaultSchemaChanged(t *testing.T) {
	t.Parallel()

	res1 := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeString,
				Default:  "hello",
				Optional: true,
			},
		},
	}

	res2 := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeString,
				Default:  "world",
				Optional: true,
			},
		},
	}

	res3 := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
	noValue := map[string]cty.Value{}
	default1Value := map[string]cty.Value{
		"test": cty.StringVal("hello"),
	}
	default2Value := map[string]cty.Value{
		"test": cty.StringVal("world"),
	}

	valuePairs := []struct {
		name   string
		before map[string]cty.Value
		after  map[string]cty.Value
	}{
		// TODO[pulumi/pulumi-terraform-bridge#3060]: Changing schema defaults is a DIFF_SOME in TF and a DIFF_NONE in Pulumi.
		// {name: "no value", before: noValue, after: noValue},
		{name: "no value, default1", before: noValue, after: default1Value},
		{name: "no value, default2", before: noValue, after: default2Value},
		{name: "default1, no value", before: default1Value, after: noValue},
		{name: "default1, default1", before: default1Value, after: default1Value},
		{name: "default1, default2", before: default1Value, after: default2Value},
		{name: "default2, no value", before: default2Value, after: noValue},
		{name: "default2, default1", before: default2Value, after: default1Value},
		{name: "default2, default2", before: default2Value, after: default2Value},
	}

	schemaPairs := []struct {
		name   string
		before *schema.Resource
		after  *schema.Resource
	}{
		{name: "res1, res1", before: res1, after: res1},
		{name: "res1, res2", before: res1, after: res2},
		{name: "res1, res3", before: res1, after: res3},
		{name: "res2, res1", before: res2, after: res1},
		{name: "res2, res2", before: res2, after: res2},
		{name: "res2, res3", before: res2, after: res3},
		{name: "res3, res1", before: res3, after: res1},
		{name: "res3, res2", before: res3, after: res2},
		{name: "res3, res3", before: res3, after: res3},
	}

	for _, valuePair := range valuePairs {
		for _, schemaPair := range schemaPairs {
			t.Run(fmt.Sprintf("%s, %s", valuePair.name, schemaPair.name), func(t *testing.T) {
				t.Parallel()

				crosstests.Diff(
					t,
					schemaPair.before,
					valuePair.before,
					valuePair.after,
					crosstests.DiffProviderUpgradedSchema(schemaPair.after),
				)
			})
		}
	}
}

func TestAssetDiff(t *testing.T) {
	t.Parallel()

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"test_path": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
	tfp := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": res,
		},
	}
	resInfo := &info.Resource{
		Fields: map[string]*info.Schema{
			"test_path": {
				Asset: &info.AssetTranslation{
					Kind: info.FileAsset,
				},
			},
		},
	}
	infoMap := map[string]*info.Resource{
		"prov_test": resInfo,
	}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.WithResourceInfo(infoMap))

	t.Run("file asset", func(t *testing.T) {
		tempDir := t.TempDir()
		assetPath := filepath.Join(tempDir, "asset.txt")
		err := os.WriteFile(assetPath, []byte("hello"), 0o600)
		require.NoError(t, err)

		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(`
        name: test
        runtime: yaml
        variables:
            fileAsset:
                fn::fileAsset: %s
        resources:
            mainRes:
                type: prov:index:Test
                properties:
                    testPath: ${fileAsset}
	`, assetPath))

		pt.Up(t)

		err = os.WriteFile(assetPath, []byte("world"), 0o600)
		require.NoError(t, err)

		prev := pt.Preview(t, optpreview.Diff())

		require.Contains(t, prev.StdOut, "- testPath: asset(file:2cf24db)")
		require.Contains(t, prev.StdOut, "+ testPath: asset(file:486ea46)")
		require.Contains(t, prev.StdOut, "~ 1 to update")

		pt.Up(t)
	})

	t.Run("string asset", func(t *testing.T) {
		pt := pulcheck.PulCheck(t, bridgedProvider, `
        name: test
        runtime: yaml
        variables:
            stringAsset:
                fn::stringAsset: hello
        resources:
            mainRes:
                type: prov:index:Test
                properties:
                    testPath: ${stringAsset}`)
		pt.Up(t)

		pt.WritePulumiYaml(t, `
        name: test
        runtime: yaml
        variables:
            stringAsset:
                fn::stringAsset: world
        resources:
            mainRes:
                type: prov:index:Test
                properties:
                    testPath: ${stringAsset}`)

		prev := pt.Preview(t, optpreview.Diff())

		autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      - testPath: asset(text:2cf24db) {
            <contents elided>
        }
      + testPath: asset(text:486ea46) {
            <contents elided>
        }
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, prev.StdOut)
		pt.Up(t)
	})
}

func TestArchiveDiff(t *testing.T) {
	t.Parallel()

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"test_path": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
	tfp := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": res,
		},
	}
	resInfo := &info.Resource{
		Fields: map[string]*info.Schema{
			"test_path": {
				Asset: &info.AssetTranslation{
					Kind:   info.FileArchive,
					Format: resource.ZIPArchive,
				},
			},
		},
	}
	infoMap := map[string]*info.Resource{
		"prov_test": resInfo,
	}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, pulcheck.WithResourceInfo(infoMap))

	t.Run("file archive", func(t *testing.T) {
		tempDir := t.TempDir()
		assetPath := filepath.Join(tempDir, "asset")
		err := os.Mkdir(assetPath, 0o700)
		require.NoError(t, err)
		file1Path := filepath.Join(assetPath, "file1.txt")
		file2Path := filepath.Join(assetPath, "file2.txt")
		err = os.WriteFile(file1Path, []byte("hello"), 0o600)
		require.NoError(t, err)
		err = os.WriteFile(file2Path, []byte("world"), 0o600)
		require.NoError(t, err)

		pt := pulcheck.PulCheck(t, bridgedProvider, fmt.Sprintf(`
        name: test
        runtime: yaml
        variables:
            archiveAsset:
                fn::fileArchive: %s
        resources:
            mainRes:
                type: prov:index:Test
                properties:
                    testPath: ${archiveAsset}`, assetPath))
		pt.Up(t)

		err = os.WriteFile(file1Path, []byte("hello1"), 0o600)
		require.NoError(t, err)
		err = os.WriteFile(file2Path, []byte("world1"), 0o600)
		require.NoError(t, err)

		prev := pt.Preview(t, optpreview.Diff())

		require.Contains(t, prev.StdOut, "~ testPath: archive(file:8933f25->e22d32b)")
		require.Contains(t, prev.StdOut, "~ 1 to update")

		pt.Up(t)
	})

	t.Run("asset archive", func(t *testing.T) {
		pt := pulcheck.PulCheck(t, bridgedProvider, `
        name: test
        runtime: yaml
        variables:
            assetArchive:
                fn::assetArchive:
                    file1:
                        fn::stringAsset: hello
                    file2:
                        fn::stringAsset: world
        resources:
            mainRes:
                type: prov:index:Test
                properties:
                    testPath: ${assetArchive}`)
		pt.Up(t)

		pt.WritePulumiYaml(t, `
        name: test
        runtime: yaml
        variables:
            assetArchive:
                fn::assetArchive:
                    file1:
                        fn::stringAsset: hello1
                    file2:
                        fn::stringAsset: world1
        resources:
            mainRes:
                type: prov:index:Test
                properties:
                    testPath: ${assetArchive}`)

		prev := pt.Preview(t, optpreview.Diff())

		autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::test::pulumi:pulumi:Stack::test-test]
    ~ prov:index/test:Test: (update)
        [id=newid]
        [urn=urn:pulumi:test::test::prov:index/test:Test::mainRes]
      ~ testPath: archive(assets:2a03253->4c74cbf) {
          ~ "file1": asset(text:2cf24db->91e9240) {"<contents elided>" => "<contents elided>"
            }
          ~ "file2": asset(text:486ea46->da4c6d4) {"<contents elided>" => "<contents elided>"
            }
        }
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, prev.StdOut)
		pt.Up(t)
	})
}

func TestTimeoutsHandling(t *testing.T) {
	t.Parallel()

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(time.Second * 10),
			Update: schema.DefaultTimeout(time.Second * 10),
		},
	}

	tfp := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": res,
		},
	}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)

	pt := pulcheck.PulCheck(t, bridgedProvider, `
        name: test
        runtime: yaml
        resources:
            mainRes:
                type: prov:index:Test
                properties:
                    test: hello`)
	// we just check that no errors occur.
	pt.Up(t)

	pt.WritePulumiYaml(t, `
        name: test
        runtime: yaml
        resources:
            mainRes:
                type: prov:index:Test
                properties:
                    test: hello1`)
	pt.Up(t)
}
