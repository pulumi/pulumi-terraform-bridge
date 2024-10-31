package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
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

	runTest := func(t *testing.T, PRC bool) {
		tfp := &schema.Provider{ResourcesMap: resMap}
		opts := []pulcheck.BridgedProviderOpt{}
		if !PRC {
			opts = append(opts, pulcheck.DisablePlanResourceChange())
		}
		bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp, opts...)
		bridgedProvider.Resources["prov_test"] = &tfbridge.ResourceInfo{
			Tok: "prov:index:Test",
			Fields: map[string]*tfbridge.SchemaInfo{
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

	t.Run("PRC disabled", func(t *testing.T) {
		runTest(t, false)
	})

	t.Run("PRC enabled", func(t *testing.T) {
		runTest(t, true)
	})
}

func TestCreateCustomTimeoutsCrossTest(t *testing.T) {
	t.Parallel()
	test := func(
		t *testing.T,
		schemaCreateTimeout *time.Duration,
		programTimeout *string,
		expected time.Duration,
		ExpectFail bool,
	) {
		var pulumiCapturedTimeout *time.Duration
		var tfCapturedTimeout *time.Duration
		prov := &schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"prov_test": {
					Schema: map[string]*schema.Schema{
						"prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
					CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
						t := rd.Timeout(schema.TimeoutCreate)
						if pulumiCapturedTimeout == nil {
							pulumiCapturedTimeout = &t
						} else {
							tfCapturedTimeout = &t
						}
						rd.SetId("id")
						return diag.Diagnostics{}
					},
					Timeouts: &schema.ResourceTimeout{
						Create: schemaCreateTimeout,
					},
				},
			},
		}

		bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)
		pulumiTimeout := `""`
		if programTimeout != nil {
			pulumiTimeout = fmt.Sprintf(`"%s"`, *programTimeout)
		}

		tfTimeout := "null"
		if programTimeout != nil {
			tfTimeout = fmt.Sprintf(`"%s"`, *programTimeout)
		}

		program := fmt.Sprintf(`
name: test
runtime: yaml
resources:
	mainRes:
		type: prov:Test
		properties:
			prop: "val"
		options:
			customTimeouts:
				create: %s
`, pulumiTimeout)

		pt := pulcheck.PulCheck(t, bridgedProvider, program)
		pt.Up(t)
		// We pass custom timeouts in the program if the resource does not support them.

		require.NotNil(t, pulumiCapturedTimeout)
		require.Nil(t, tfCapturedTimeout)

		tfProgram := fmt.Sprintf(`
resource "prov_test" "mainRes" {
	prop = "val"
	timeouts {
		create = %s
	}
}`, tfTimeout)

		tfdriver := tfcheck.NewTfDriver(t, t.TempDir(), "prov", prov)
		tfdriver.Write(t, tfProgram)

		plan, err := tfdriver.Plan(t)
		if ExpectFail {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		err = tfdriver.Apply(t, plan)
		require.NoError(t, err)
		require.NotNil(t, tfCapturedTimeout)

		assert.Equal(t, *pulumiCapturedTimeout, *tfCapturedTimeout)
		assert.Equal(t, *pulumiCapturedTimeout, expected)
	}

	oneSecString := "1s"
	oneSec := 1 * time.Second
	// twoSecString := "2s"
	twoSec := 2 * time.Second

	tests := []struct {
		name                string
		schemaCreateTimeout *time.Duration
		programTimeout      *string
		expected            time.Duration
		expectFail          bool
	}{
		{
			"schema specified timeout",
			&oneSec,
			nil,
			oneSec,
			false,
		},
		{
			"program specified timeout",
			&twoSec,
			&oneSecString,
			oneSec,
			false,
		},
		{
			"program specified without schema timeout",
			nil,
			&oneSecString,
			oneSec,
			true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			test(t, tc.schemaCreateTimeout, tc.programTimeout, tc.expected, tc.expectFail)
		})
	}
}

func TestStateFunc(t *testing.T) {
	t.Parallel()
	resMap := map[string]*schema.Resource{
		"prov_test": {
			CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
				d.SetId("id")
				var diags diag.Diagnostics
				v, ok := d.GetOk("test")
				assert.True(t, ok, "test property not set")

				err := d.Set("test", v.(string)+" world")
				require.NoError(t, err)
				return diags
			},
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Optional: true,
					ForceNew: true,
					StateFunc: func(v interface{}) string {
						return v.(string) + " world"
					},
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
	require.Equal(t, "hello world", res.Outputs["testOut"].Value)
	pt.Preview(t, optpreview.ExpectNoChanges())
}

// TestPlanStateEdit tests that [shimv2.WithPlanStateEdit] can be used to effectively edit
// planed state.
//
// The test is set up to reproduce https://github.com/pulumi/pulumi-gcp/issues/2372.
func TestPlanStateEdit(t *testing.T) {
	t.Parallel()
	setLabelsDiff := func(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
		raw := d.Get("labels")
		if raw == nil {
			return nil
		}

		if d.Get("terraform_labels") == nil {
			return fmt.Errorf("`terraform_labels` field is not present in the resource schema")
		}

		// If "labels" field is computed, set "terraform_labels" and "effective_labels" to computed.
		// https://github.com/hashicorp/terraform-provider-google/issues/16217
		if !d.GetRawPlan().GetAttr("labels").IsWhollyKnown() {
			if err := d.SetNewComputed("terraform_labels"); err != nil {
				return fmt.Errorf("error setting terraform_labels to computed: %w", err)
			}

			return nil
		}

		// Merge provider default labels with the user defined labels in the resource to get terraform managed labels
		terraformLabels := make(map[string]string)

		labels := raw.(map[string]interface{})
		for k, v := range labels {
			terraformLabels[k] = v.(string)
		}

		if err := d.SetNew("terraform_labels", terraformLabels); err != nil {
			return fmt.Errorf("error setting new terraform_labels diff: %w", err)
		}

		return nil
	}

	const tfLabelsKey = "terraform_labels"

	fixEmptyLabels := func(ctx context.Context, req shimv2.PlanStateEditRequest) (cty.Value, error) {
		tfbridge.GetLogger(ctx).Debug("Invoked") // ctx is correctly passed and the logger is available

		assert.Equal(t, "prov_test", req.TfToken)
		assert.Equal(t, resource.PropertyMap{
			"__defaults": resource.NewProperty([]resource.PropertyValue{}),
			"labels": resource.NewProperty(resource.PropertyMap{
				"empty": resource.NewProperty(""),
				"key":   resource.NewProperty("val"),
			}),
		}, req.NewInputs)
		assert.Equal(t, resource.PropertyMap{
			"configValue": resource.NewProperty("configured"),
		}, req.ProviderConfig)

		m := req.PlanState.AsValueMap()
		effectiveLabels := m[tfLabelsKey].AsValueMap()
		effectiveLabels["empty"] = cty.StringVal("")
		m[tfLabelsKey] = cty.MapVal(effectiveLabels)
		return cty.ObjectVal(m), nil
	}

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"labels": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			tfLabelsKey: {
				Type:     schema.TypeMap,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
		CustomizeDiff: setLabelsDiff,
	}

	tfp := &schema.Provider{
		Schema: map[string]*schema.Schema{"config_value": {
			Type:     schema.TypeString,
			Optional: true,
		}},
		ResourcesMap: map[string]*schema.Resource{"prov_test": res},
	}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp,
		pulcheck.WithStateEdit(fixEmptyLabels))
	program := `
name: test
runtime: yaml
resources:
  _:
    type: pulumi:providers:prov
    properties:
        configValue: "configured"
    defaultProvider: true
  mainRes:
    type: prov:index:Test
    properties:
      labels: { "key": "val", "empty": "" }
outputs:
  keyValue: ${mainRes.terraformLabels["key"]}
  emptyValue: ${mainRes.terraformLabels["empty"]}`
	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	out := pt.Up(t)

	assert.Equal(t, "val", out.Outputs["keyValue"].Value)
	assert.Equal(t, "", out.Outputs["emptyValue"].Value)
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

func TestTypeChecker(t *testing.T) {
	t.Setenv("PULUMI_DEBUG_YAML_DISABLE_TYPE_CHECKING", "true")
	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"tags": {
					Type:     schema.TypeMap,
					Elem:     &schema.Schema{Type: schema.TypeString},
					Optional: true,
				},
				"network_configuration": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"assign_public_ip": {
								Type:     schema.TypeBool,
								Optional: true,
								Default:  false,
							},
							"security_groups": {
								Type:     schema.TypeSet,
								Optional: true,
								Elem:     &schema.Schema{Type: schema.TypeString},
							},
							"subnets": {
								Type:     schema.TypeSet,
								Optional: true,
								Elem:     &schema.Schema{Type: schema.TypeString},
							},
						},
					},
				},
			},
		},
	}

	runTest := func(t *testing.T, resMap map[string]*schema.Resource, props interface{}, expectedError string) {
		propsJSON, err := json.Marshal(props)
		require.NoError(t, err)
		program := fmt.Sprintf(`
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
	properties: %s`, propsJSON)

		bridgedProvider := pulcheck.BridgedProvider(t, "prov", &schema.Provider{ResourcesMap: resMap})
		pt := pulcheck.PulCheck(t, bridgedProvider, program)
		_, err = pt.CurrentStack().Up(pt.Context())

		require.ErrorContains(t, err, "Unexpected type at field")
		require.ErrorContains(t, err, expectedError)
	}

	t.Run("flat type instead of array", func(t *testing.T) {
		runTest(t, resMap, map[string]interface{}{"networkConfiguration": map[string]any{"subnets": "subnet"}}, "expected array type, got")
	})

	t.Run("flat type instead of map", func(t *testing.T) {
		runTest(t, resMap, map[string]interface{}{"tags": "tag"}, "expected object type, got")
	})

	t.Run("flat type instead of object", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		runTest(t, resMap, map[string]interface{}{"network_configuration": "config"}, "expected object type, got")
	})

	t.Run("array instead of object", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		runTest(t, resMap, map[string]interface{}{"network_configuration": []string{"config"}}, "expected object type, got")
	})

	t.Run("array instead of map", func(t *testing.T) {
		runTest(t, resMap, map[string]interface{}{"tags": []string{"tag"}}, "expected object type, got")
	})

	t.Run("array instead of flat type", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		runTest(t, resMap, map[string]interface{}{"network_configuration": map[string]interface{}{"assign_public_ip": []any{true}}}, "expected array type, got")
	})

	t.Run("map instead of array", func(t *testing.T) {
		runTest(t, resMap,
			map[string]interface{}{"networkConfiguration": map[string]any{"subnets": map[string]any{"sub": "sub"}}},
			"expected array type, got")
	})

	t.Run("map instead of flat type", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		runTest(t, resMap, map[string]interface{}{"network_configuration": map[string]interface{}{"assign_public_ip": map[string]any{"val": true}}}, "expected array type, got")
	})
}
