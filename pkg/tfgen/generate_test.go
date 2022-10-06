package tfgen

import (
	schemav2 "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
	"github.com/stretchr/testify/assert"
)

func Test_DeprecationFromTFSchema(t *testing.T) {
	v := &variable{
		name:   "v",
		schema: shimv1.NewSchema(&schema.Schema{Type: schema.TypeString, Deprecated: "This is deprecated"}),
	}

	deprecationMessage := v.deprecationMessage()
	assert.Equal(t, "This is deprecated", deprecationMessage)
}

func Test_ForceNew(t *testing.T) {
	cases := []struct {
		Name           string
		Var            variable
		ShouldForceNew bool
	}{
		{Name: "Pulumi Schema with ForceNew Override ShouldForceNew true",
			Var: variable{
				name: "v",
				schema: shimv1.NewSchema(&schema.Schema{
					Type: schema.TypeString,
				}),
				info: &tfbridge.SchemaInfo{
					ForceNew: tfbridge.True(),
				},
			},
			ShouldForceNew: true,
		},
		{
			Name: "TF Schema ForceNew ShouldForceNew true",
			Var: variable{
				name: "v",
				schema: shimv1.NewSchema(&schema.Schema{
					Type:     schema.TypeString,
					ForceNew: true,
				}),
			},
			ShouldForceNew: true,
		},
		{
			Name: "Output Parameter ShouldForceNew false",
			Var: variable{
				out: true,
			},
			ShouldForceNew: false,
		},
		{
			Name: "Input Non ForceNew Parameter ShouldForceNew false",
			Var: variable{
				name: "v",
				schema: shimv1.NewSchema(&schema.Schema{
					Type: schema.TypeString,
				}),
			},
			ShouldForceNew: false,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			v := &test.Var
			actuallyForcesNew := v.forceNew()
			assert.Equal(t, test.ShouldForceNew, actuallyForcesNew)
		})
	}
}

func TestMakeObjectPropertyType_InputTypes(t *testing.T) {

	vpcConfig := mockResource{
		schema: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"security_group_ids": {
				Type:     7,
				Required: true,
				Optional: false,
				Computed: false,
				Elem: &schema.Schema{
					Type:     4,
					Required: false,
					Optional: false,
					Computed: false,
				},
			},
			"subnet_ids": {
				Type:     7,
				Required: true,
				Optional: false,
				Computed: false,
				Elem: &schema.Schema{
					Type:       4,
					ConfigMode: 0,
					Required:   false,
					Optional:   false,
					Computed:   false,
				},
			},
			// This should only appear in the output type, not the input type:
			"vpc_id": {
				Type:       4,
				ConfigMode: 0,
				Required:   false,
				Optional:   false,
				Computed:   true,
			},
		}),
	}

	inputResult := makeObjectPropertyType("vpc_config", vpcConfig, nil, false, entityDocs{}, "")
	sort.Slice(inputResult.properties, func(i, j int) bool {
		return inputResult.properties[i].name < inputResult.properties[j].name
	})

	assert.Equal(t, 3, len(inputResult.properties))
	assert.Equal(t, "securityGroupIds", inputResult.properties[0].name)
	assert.Equal(t, false, inputResult.properties[0].out)
	assert.Equal(t, "subnetIds", inputResult.properties[1].name)
	assert.Equal(t, false, inputResult.properties[1].out)
	assert.Equal(t, "vpcId", inputResult.properties[2].name)
	assert.Equal(t, true, inputResult.properties[2].out)
}
