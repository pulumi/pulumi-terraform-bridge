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

func TestGetArgDescription(t *testing.T) {
	// The structure we get from parsing manually-generated docs:
	entityDocsManual := entityDocs{
		Arguments: map[string]*argumentDocs{
			"not_nested": {
				description: "not_nested_desc",
			},
			"nested": {
				description: "nested_desc",
				arguments: map[string]*argumentDocs{
					"nested_l2": {
						description: "nested_l2_desc",
						arguments: map[string]*argumentDocs{
							"nested_l3": {
								description: "nested_l3_desc",
							},
						},
					},
				},
			},
		},
		Attributes: map[string]string{
			"attrib": "attrib_desc",
		},
	}

	// The structure we get from parsing auto-generated docs. Note that argumentDocs.arguments are populated by the code
	// that parses this type of documentation, but are not used by the function under test, so we do not add them to
	// the test data:
	entityDocsAuto := entityDocs{
		Arguments: map[string]*argumentDocs{
			"not_nested": {
				description: "not_nested_desc",
			},
			"nested": {
				description: "nested_desc",
			},
			"nested.nested_l2": {
				description: "nested_l2_desc",
			},
			"nested.nested_l2.nested_l3": {
				description: "nested_l3_desc",
			},
		},
		Attributes: map[string]string{
			"attrib": "attrib_desc",
		},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"not_found", ""},
		{"not_found.not_found", ""},
		{"not_found.not_found.not_found", ""},
		{"not_nested", "not_nested_desc"},
		{"nested", "nested_desc"},
		{"nested.nested_l2", "nested_l2_desc"},
		{"nested.nested_l2.not_found", ""},
		{"nested.nested_l2.attrib", "attrib_desc"},
		{"nested.nested_l2.nested_l3", "nested_l3_desc"},
		{"attrib", "attrib_desc"},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, entityDocsManual.getArgDescription(test.input))
		assert.Equal(t, test.expected, entityDocsAuto.getArgDescription(test.input))
	}
}
