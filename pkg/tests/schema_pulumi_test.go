package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func ref[T any](t T) *T {
	return &t
}

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

	tfp := &schema.Provider{ResourcesMap: resMap}
	opts := []pulcheck.BridgedProviderOpt{}
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
