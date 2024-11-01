package crosstests

import (
	"bytes"
	"testing"

	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestWritePFHCLProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  map[string]cty.Value
		schema pschema.Schema
		expect autogold.Value
	}{
		{
			name: "string-attr",
			value: map[string]cty.Value{
				"key": cty.StringVal("value"),
			},
			schema: pschema.Schema{
				Attributes: map[string]pschema.Attribute{
					"key": pschema.StringAttribute{Optional: true},
				},
			},
			expect: autogold.Expect(`provider "test" {
  key = "value"
}
`),
		},
		{
			name: "bool-attr",
			value: map[string]cty.Value{
				"key": cty.BoolVal(true),
			},
			schema: pschema.Schema{
				Attributes: map[string]pschema.Attribute{
					"key": pschema.BoolAttribute{Optional: true},
				},
			},
			expect: autogold.Expect(`provider "test" {
  key = true
}
`),
		},
		{
			name: "null-attr",
			value: map[string]cty.Value{
				"key": cty.NullVal(cty.String),
			},
			schema: pschema.Schema{
				Attributes: map[string]pschema.Attribute{
					"key": pschema.StringAttribute{Optional: true},
				},
			},
			expect: autogold.Expect(`provider "test" {
  key = null
}
`),
		},
		{
			name: "single-nested-attr",
			value: map[string]cty.Value{
				"key": cty.ObjectVal(map[string]cty.Value{
					"n1": cty.NumberIntVal(123),
					"n2": cty.ObjectVal(map[string]cty.Value{
						"dn1": cty.BoolVal(true),
					}),
				}),
			},
			schema: pschema.Schema{
				Attributes: map[string]pschema.Attribute{
					"key": pschema.SingleNestedAttribute{
						Attributes: map[string]pschema.Attribute{
							"n1": pschema.NumberAttribute{Optional: true},
							"n2": pschema.SingleNestedAttribute{
								Attributes: map[string]pschema.Attribute{
									"dn1": pschema.BoolAttribute{Optional: true},
								},
							},
						},
					},
				},
			},
			expect: autogold.Expect(`provider "test" {
  key = {
    n1 = 123
    n2 = {
      dn1 = true
    }
  }
}
`),
		},
		{
			name: "single-nested-block",
			value: map[string]cty.Value{
				"key": cty.ObjectVal(map[string]cty.Value{
					"n1": cty.NumberIntVal(123),
					"n2": cty.ObjectVal(map[string]cty.Value{
						"dn1": cty.BoolVal(true),
					}),
				}),
			},
			schema: pschema.Schema{
				Blocks: map[string]pschema.Block{
					"key": pschema.SingleNestedBlock{
						Attributes: map[string]pschema.Attribute{
							"n1": pschema.NumberAttribute{Optional: true},
						},
						Blocks: map[string]pschema.Block{
							"n2": pschema.SingleNestedBlock{
								Attributes: map[string]pschema.Attribute{
									"dn1": pschema.BoolAttribute{Optional: true},
								},
							},
						},
					},
				},
			},
			expect: autogold.Expect(`provider "test" {
  key {
    n1 = 123
    n2 {
      dn1 = true
    }
  }
}
`),
		},
		{
			name: "list-nested-block",
			value: map[string]cty.Value{
				"key": cty.ListVal([]cty.Value{
					cty.ObjectVal(map[string]cty.Value{
						"dn1": cty.BoolVal(true),
					}),
					cty.ObjectVal(map[string]cty.Value{
						"dn1": cty.BoolVal(false),
					}),
				}),
			},
			schema: pschema.Schema{
				Blocks: map[string]pschema.Block{
					"key": pschema.ListNestedBlock{
						NestedObject: pschema.NestedBlockObject{
							Attributes: map[string]pschema.Attribute{
								"dn1": pschema.BoolAttribute{Optional: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(`provider "test" {
  key {
    dn1 = true
  }
  key {
    dn1 = false
  }
}
`),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var actual bytes.Buffer
			err := WritePF(&actual).Provider(tt.schema, "test", tt.value)
			require.NoError(t, err)
			tt.expect.Equal(t, actual.String())
		})
	}
}
