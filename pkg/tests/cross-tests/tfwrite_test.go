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
		{
			"empty-block-of-objects",
			cty.ObjectVal(map[string]cty.Value{"f0": cty.ListValEmpty(cty.EmptyObject)}),
			map[string]*schema.Schema{
				"f0": {
					Required: true,
					Type:     schema.TypeList,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{},
					},
					MaxItems: 1,
				},
			},
			autogold.Expect(`
resource "res" "ex" {
}
`),
		},
		{
			"block-of-empty-object",
			cty.ObjectVal(map[string]cty.Value{"f0": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{})})}),
			map[string]*schema.Schema{
				"f0": {
					Required: true,
					Type:     schema.TypeList,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{},
					},
					MaxItems: 1,
				},
			},
			autogold.Expect(`
resource "res" "ex" {
  f0 {
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
