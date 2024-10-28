package crosstests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestGenerateYaml(t *testing.T) {
	providerShortName := "crossprovider"
	rtype := "crossprovider_testres"
	rtok := "TestRes"
	rtoken := providerShortName + ":index:" + rtok
	for _, tc := range []struct {
		name     string
		schema   map[string]*schema.Schema
		tfConfig any
		expect   autogold.Value
	}{
		{
			"simple",
			map[string]*schema.Schema{"x": {
				Type:     schema.TypeString,
				Optional: true,
			}},
			map[string]interface{}{"x": "OK"},
			autogold.Expect(`backend:
    url: file://./data
name: project
resources:
    example:
        properties:
            x: OK
        type: crossprovider:index:TestRes
runtime: yaml
`),
		},
		{
			"simple-null",
			map[string]*schema.Schema{"x": {
				Type:     schema.TypeString,
				Optional: true,
			}},
			map[string]interface{}{"x": nil},
			autogold.Expect(`backend:
    url: file://./data
name: project
resources:
    example:
        properties: {}
        type: crossprovider:index:TestRes
runtime: yaml
`),
		},
		{
			"simple-missing",
			map[string]*schema.Schema{"x": {
				Type:     schema.TypeString,
				Optional: true,
			}},
			map[string]interface{}{},
			autogold.Expect(`backend:
    url: file://./data
name: project
resources:
    example:
        properties: {}
        type: crossprovider:index:TestRes
runtime: yaml
`),
		},
		{
			"single-nested-block",
			map[string]*schema.Schema{
				"x": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"y": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem: &schema.Schema{
						Type:     schema.TypeInt,
						Required: true,
					},
				},
			},
			map[string]interface{}{
				"x": "OK",
				"y": map[string]interface{}{
					"foo": 42,
				},
			},
			autogold.Expect(`backend:
    url: file://./data
name: project
resources:
    example:
        properties:
            x: OK
            "y":
                foo: 42
        type: crossprovider:index:TestRes
runtime: yaml
`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tfp := &schema.Provider{
				ResourcesMap: map[string]*schema.Resource{
					rtype: {
						Schema: tc.schema,
					},
				},
			}
			shimProvider := shimv2.NewProvider(tfp, shimv2.WithPlanResourceChange(
				func(tfResourceType string) bool { return true },
			))
			schema := shimProvider.ResourcesMap().Get(rtype).Schema()
			out, err := generateYaml(t, rtoken,
				inferPulumiValue(t, schema, nil, coalesceInputs(t, tc.schema, tc.tfConfig)))
			require.NoError(t, err)
			b, err := yaml.Marshal(out)
			require.NoError(t, err)
			tc.expect.Equal(t, string(b[:]))
		})
	}
}
