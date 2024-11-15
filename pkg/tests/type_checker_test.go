package tests

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/stretchr/testify/require"
)

func TestTypeChecker(t *testing.T) {
	t.Parallel()
	makeResMap := func(sch map[string]*schema.Schema) map[string]*schema.Resource {
		return map[string]*schema.Resource{
			"prov_test": {Schema: sch},
		}
	}

	runTest := func(t *testing.T, resMap map[string]*schema.Resource, props interface{}, expectedError string) {
		t.Helper()
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
		pt := pulcheck.PulCheck(t, bridgedProvider, program,
			opttest.Env("PULUMI_DEBUG_YAML_DISABLE_TYPE_CHECKING", "true"),
			opttest.Env("PULUMI_ERROR_TYPE_CHECKER", "true"),
		)
		_, err = pt.CurrentStack().Up(pt.Context())

		require.ErrorContains(t, err, "Unexpected type at field")
		require.ErrorContains(t, err, expectedError)
	}

	t.Run("flat type instead of array", func(t *testing.T) {
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnets": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		})
		runTest(t, resMap, map[string]interface{}{"networkConfiguration": map[string]any{"subnets": "subnet"}}, "expected array type, got")
	})

	t.Run("flat type instead of map", func(t *testing.T) {
		resMap := makeResMap(map[string]*schema.Schema{
			"tags": {
				Type:     schema.TypeMap,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
			},
		})
		runTest(t, resMap, map[string]interface{}{"tags": "tag"}, "expected object type, got")
	})

	t.Run("flat type instead of object", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnets": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		})
		runTest(t, resMap, map[string]interface{}{"network_configuration": "config"}, "expected object type, got")
	})

	t.Run("array instead of object", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnets": {
							Type: schema.TypeSet,
							Elem: &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		})
		runTest(t, resMap, map[string]interface{}{"network_configuration": []string{"config"}}, "expected object type, got")
	})

	t.Run("array instead of map", func(t *testing.T) {
		resMap := makeResMap(map[string]*schema.Schema{
			"tags": {
				Type:     schema.TypeMap,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
			},
		})
		runTest(t, resMap, map[string]interface{}{"tags": []string{"tag"}}, "expected object type, got")
	})

	t.Run("array instead of flat type", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"assign_public_ip": {
							Type:     schema.TypeBool,
							Optional: true,
						},
					},
				},
			},
		})
		runTest(t, resMap, map[string]interface{}{"network_configuration": map[string]interface{}{"assign_public_ip": []any{true}}}, "expected array type, got")
	})

	t.Run("map instead of array", func(t *testing.T) {
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnets": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		})
		runTest(t, resMap,
			map[string]interface{}{"networkConfiguration": map[string]any{"subnets": map[string]any{"sub": "sub"}}},
			"expected array type, got")
	})

	t.Run("map instead of flat type", func(t *testing.T) {
		t.Skip("This is caught by the YAML runtime, not the type checker")
		resMap := makeResMap(map[string]*schema.Schema{
			"network_configuration": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"assign_public_ip": {
							Type: schema.TypeBool,
						},
					},
				},
			},
		})
		runTest(t, resMap, map[string]interface{}{"network_configuration": map[string]interface{}{"assign_public_ip": map[string]any{"val": true}}}, "expected array type, got")
	})
}