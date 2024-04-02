package crosstests

import (
	"bytes"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestWriteHCL(t *testing.T) {
	type testCase struct {
		name   string
		value  cty.Value
		schema map[string]*schema.Schema
		expect autogold.Value
	}

	testCases := []testCase{
		{
			"simple",
			cty.ObjectVal(map[string]cty.Value{"x": cty.StringVal("OK")}),
			map[string]*schema.Schema{"x": {
				Type:     schema.TypeString,
				Optional: true,
			}},
			autogold.Expect(`
resource "res" "ex" {
  x = "OK"
}
`),
		},
		{
			"simple-null",
			cty.ObjectVal(map[string]cty.Value{"x": cty.NullVal(cty.String)}),
			map[string]*schema.Schema{"x": {
				Type:     schema.TypeString,
				Optional: true,
			}},
			autogold.Expect(`
resource "res" "ex" {
  x = null
}
`),
		},
		{
			"simple-missing",
			cty.ObjectVal(map[string]cty.Value{}),
			map[string]*schema.Schema{"x": {
				Type:     schema.TypeString,
				Optional: true,
			}},
			autogold.Expect(`
resource "res" "ex" {
}
`),
		},
		{
			"single-nested-block",
			cty.ObjectVal(map[string]cty.Value{
				"x": cty.StringVal("OK"),
				"y": cty.ObjectVal(map[string]cty.Value{
					"foo": cty.NumberIntVal(42),
				}),
			}),
			map[string]*schema.Schema{
				"x": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"y": {
					Type: schema.TypeMap,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"foo": {Type: schema.TypeInt, Required: true},
						},
					},
				},
			},
			autogold.Expect(`
resource "res" "ex" {
  x = "OK"
  y = {
    foo = 42
  }
}
`),
		},
		{
			"list-nested-block",
			cty.ObjectVal(map[string]cty.Value{
				"blk": cty.ListVal([]cty.Value{
					cty.ObjectVal(map[string]cty.Value{
						"foo": cty.NumberIntVal(1),
					}),
					cty.ObjectVal(map[string]cty.Value{
						"foo": cty.NumberIntVal(2),
					}),
				}),
			}),
			map[string]*schema.Schema{
				"blk": {
					Type: schema.TypeList,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"foo": {Type: schema.TypeInt, Required: true},
						},
					},
				},
			},
			autogold.Expect(`
resource "res" "ex" {
  blk {
    foo = 1
  }
  blk {
    foo = 2
  }
}
`),
		},
		{
			"set-nested-block",
			cty.ObjectVal(map[string]cty.Value{
				"blk": cty.ListVal([]cty.Value{
					cty.ObjectVal(map[string]cty.Value{
						"foo": cty.NumberIntVal(1),
					}),
					cty.ObjectVal(map[string]cty.Value{
						"foo": cty.NumberIntVal(2),
					}),
				}),
			}),
			map[string]*schema.Schema{
				"blk": {
					Type: schema.TypeSet,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"foo": {Type: schema.TypeInt, Required: true},
						},
					},
				},
			},
			autogold.Expect(`
resource "res" "ex" {
  blk {
    foo = 1
  }
  blk {
    foo = 2
  }
}
`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := WriteHCL(&out, tc.schema, "res", "ex", tc.value)
			require.NoError(t, err)
			tc.expect.Equal(t, "\n"+out.String())
		})
	}
}
