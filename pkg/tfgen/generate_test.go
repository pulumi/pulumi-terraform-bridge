package tfgen

import (
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
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			v := &test.Var
			actuallyForcesNew := v.forceNew()
			assert.Equal(t, test.ShouldForceNew, actuallyForcesNew)
		})
	}
}
