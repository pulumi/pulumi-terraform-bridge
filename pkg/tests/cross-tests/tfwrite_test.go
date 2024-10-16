// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crosstests

import (
	"bytes"
	"testing"

	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
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
		{
			"list-list-nested-block",
			cty.ObjectVal(map[string]cty.Value{
				"blk": cty.ListVal([]cty.Value{
					cty.ObjectVal(map[string]cty.Value{
						"foo": cty.ListVal([]cty.Value{
							cty.ObjectVal(map[string]cty.Value{
								"bar": cty.NumberIntVal(1),
							}),
							cty.ObjectVal(map[string]cty.Value{
								"bar": cty.NumberIntVal(2),
							}),
						}),
					}),
					cty.ObjectVal(map[string]cty.Value{
						"foo": cty.ListVal([]cty.Value{
							cty.ObjectVal(map[string]cty.Value{
								"bar": cty.NumberIntVal(4),
							}),
							cty.ObjectVal(map[string]cty.Value{
								"bar": cty.NumberIntVal(3),
							}),
						}),
					}),
				}),
			}),
			map[string]*schema.Schema{
				"blk": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"foo": {
								Optional: true,
								Type:     schema.TypeList,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"bar": {Type: schema.TypeInt, Required: true},
									},
								},
							},
						},
					},
				},
			},
			autogold.Expect(`
resource "res" "ex" {
  blk {
    foo {
      bar = 1
    }
    foo {
      bar = 2
    }
  }
  blk {
    foo {
      bar = 4
    }
    foo {
      bar = 3
    }
  }
}
`),
		},
		{
			"explicit-null-val",
			cty.ObjectVal(map[string]cty.Value{"f0": cty.NilVal}),
			map[string]*schema.Schema{
				"f0": {
					Optional: true,
					Type:     schema.TypeList,
					Elem: &schema.Schema{
						Type:     schema.TypeMap,
						Optional: true,
						Computed: true,
						Elem: &schema.Schema{
							Type:      schema.TypeInt,
							Optional:  true,
							Sensitive: true,
						},
					},
				},
			},
			autogold.Expect(`
resource "res" "ex" {
  f0 = null
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
