package crosstests

import (
	"bytes"
	"testing"

	prschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl/hclwrite"
)

func TestWritePFHCLProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  map[string]cty.Value
		schema prschema.Schema
		expect autogold.Value
	}{
		{
			name: "string-attr",
			value: map[string]cty.Value{
				"key": cty.StringVal("value"),
			},
			schema: prschema.Schema{
				Attributes: map[string]prschema.Attribute{
					"key": prschema.StringAttribute{Optional: true},
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
			schema: prschema.Schema{
				Attributes: map[string]prschema.Attribute{
					"key": prschema.BoolAttribute{Optional: true},
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
			schema: prschema.Schema{
				Attributes: map[string]prschema.Attribute{
					"key": prschema.StringAttribute{Optional: true},
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
			schema: prschema.Schema{
				Attributes: map[string]prschema.Attribute{
					"key": prschema.SingleNestedAttribute{
						Attributes: map[string]prschema.Attribute{
							"n1": prschema.NumberAttribute{Optional: true},
							"n2": prschema.SingleNestedAttribute{
								Attributes: map[string]prschema.Attribute{
									"dn1": prschema.BoolAttribute{Optional: true},
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
			schema: prschema.Schema{
				Blocks: map[string]prschema.Block{
					"key": prschema.SingleNestedBlock{
						Attributes: map[string]prschema.Attribute{
							"n1": prschema.NumberAttribute{Optional: true},
						},
						Blocks: map[string]prschema.Block{
							"n2": prschema.SingleNestedBlock{
								Attributes: map[string]prschema.Attribute{
									"dn1": prschema.BoolAttribute{Optional: true},
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
			schema: prschema.Schema{
				Blocks: map[string]prschema.Block{
					"key": prschema.ListNestedBlock{
						NestedObject: prschema.NestedBlockObject{
							Attributes: map[string]prschema.Attribute{
								"dn1": prschema.BoolAttribute{Optional: true},
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
			sch := hclSchemaPFProvider(tt.schema)
			err := hclwrite.WriteProvider(&actual, sch, "test", tt.value)
			require.NoError(t, err)
			tt.expect.Equal(t, actual.String())
		})
	}
}

func TestWritePFHCLResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  map[string]cty.Value
		schema rschema.Schema
		expect autogold.Value
	}{
		{
			name: "string-attr",
			value: map[string]cty.Value{
				"key": cty.StringVal("value"),
			},
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"key": rschema.StringAttribute{Optional: true},
				},
			},
			expect: autogold.Expect(`resource "testprovider_test" "test" {
  key = "value"
}
`),
		},
		{
			name: "bool-attr",
			value: map[string]cty.Value{
				"key": cty.BoolVal(true),
			},
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"key": rschema.BoolAttribute{Optional: true},
				},
			},
			expect: autogold.Expect(`resource "testprovider_test" "test" {
  key = true
}
`),
		},
		{
			name: "null-attr",
			value: map[string]cty.Value{
				"key": cty.NullVal(cty.String),
			},
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"key": rschema.StringAttribute{Optional: true},
				},
			},
			expect: autogold.Expect(`resource "testprovider_test" "test" {
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
			schema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"key": rschema.SingleNestedAttribute{
						Attributes: map[string]rschema.Attribute{
							"n1": rschema.NumberAttribute{Optional: true},
							"n2": rschema.SingleNestedAttribute{
								Attributes: map[string]rschema.Attribute{
									"dn1": rschema.BoolAttribute{Optional: true},
								},
							},
						},
					},
				},
			},
			expect: autogold.Expect(`resource "testprovider_test" "test" {
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
			schema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"key": rschema.SingleNestedBlock{
						Attributes: map[string]rschema.Attribute{
							"n1": rschema.NumberAttribute{Optional: true},
						},
						Blocks: map[string]rschema.Block{
							"n2": rschema.SingleNestedBlock{
								Attributes: map[string]rschema.Attribute{
									"dn1": rschema.BoolAttribute{Optional: true},
								},
							},
						},
					},
				},
			},
			expect: autogold.Expect(`resource "testprovider_test" "test" {
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
			schema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"key": rschema.ListNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"dn1": rschema.BoolAttribute{Optional: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(`resource "testprovider_test" "test" {
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
			sch := hclSchemaPFResource(tt.schema)
			err := hclwrite.WriteResource(&actual, sch, "testprovider_test", "test", tt.value)
			require.NoError(t, err)
			tt.expect.Equal(t, actual.String())
		})
	}
}
