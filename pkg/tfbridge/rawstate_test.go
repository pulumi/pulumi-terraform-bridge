// Copyright 2016-2025, Pulumi Corporation.
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

package tfbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func Test_rawstate_inflections_turnaround(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		schema *schema.Schema
		pv     resource.PropertyValue
		cv     cty.Value
	}

	testCases := []testCase{
		{
			name: "null-string",
			pv:   resource.NewNullProperty(),
			cv:   cty.NullVal(cty.String),
		},
		{
			name: "null-number",
			pv:   resource.NewNumberProperty(42.5),
			cv:   cty.NumberFloatVal(42.5),
		},
		{
			name: "empty-string",
			pv:   resource.NewStringProperty(""),
			cv:   cty.StringVal(""),
		},
		{
			name: "simple-string",
			pv:   resource.NewStringProperty("simple"),
			cv:   cty.StringVal("simple"),
		},
		{
			name: "simple-bool",
			pv:   resource.NewBoolProperty(true),
			cv:   cty.BoolVal(true),
		},
		{
			name: "maxitems1-flat-object",
			schema: &schema.Schema{
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"str": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			pv: resource.NewObjectProperty(resource.PropertyMap{
				"str": resource.NewStringProperty("OK"),
			}),
			cv: cty.ListVal([]cty.Value{
				cty.ObjectVal(map[string]cty.Value{
					"str": cty.StringVal("OK"),
				}),
			}),
		},
		{
			name: "maxitems1-flat-object-nil",
			schema: &schema.Schema{
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"str": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			pv: resource.NewNullProperty(),
			cv: cty.ListValEmpty(cty.Object(map[string]cty.Type{
				"str": cty.String,
			})),
		},
		{
			name: "maxitems1-flat-set-object",
			schema: &schema.Schema{
				Type:     schema.TypeSet,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"str": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			pv: resource.NewObjectProperty(resource.PropertyMap{
				"str": resource.NewStringProperty("OK"),
			}),
			cv: cty.SetVal([]cty.Value{
				cty.ObjectVal(map[string]cty.Value{
					"str": cty.StringVal("OK"),
				}),
			}),
		},
		{
			name: "maxitems1-flat-set-nil",
			schema: &schema.Schema{
				Type:     schema.TypeSet,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"str": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			pv: resource.NewNullProperty(),
			cv: cty.SetValEmpty(cty.Object(map[string]cty.Type{
				"str": cty.String,
			})),
		},
		{
			name: "bigint",
			schema: &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			pv: resource.NewStringProperty("12345678901234567890"),
			cv: cty.MustParseNumberVal("12345678901234567890"),
		},
		{
			name: "bignum",
			schema: &schema.Schema{
				Type:     schema.TypeFloat,
				Optional: true,
			},
			pv: resource.NewStringProperty("12345678.901234567890"),
			cv: cty.MustParseNumberVal("12345678.901234567890"),
		},
		{
			name: "empty-set",
			schema: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			pv: resource.NewArrayProperty([]resource.PropertyValue{}),
			cv: cty.SetValEmpty(cty.String),
		},
		{
			name: "empty-list",
			schema: &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			pv: resource.NewArrayProperty([]resource.PropertyValue{}),
			cv: cty.ListValEmpty(cty.String),
		},
		{
			name: "empty-map",
			schema: &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			pv: resource.NewObjectProperty(resource.PropertyMap{}),
			cv: cty.MapValEmpty(cty.String),
		},
		{
			name: "inflect-inside-list",
			schema: &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
			},
			pv: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("12345678901234567890"),
			}),
			cv: cty.ListVal([]cty.Value{cty.MustParseNumberVal("12345678901234567890")}),
		},
		{
			name: "inflect-inside-set",
			schema: &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
			},
			pv: resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("12345678901234567890"),
			}),
			cv: cty.SetVal([]cty.Value{cty.MustParseNumberVal("12345678901234567890")}),
		},
		{
			name: "inflect-inside-map",
			schema: &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
				},
			},
			pv: resource.NewObjectProperty(resource.PropertyMap{
				"x": resource.NewStringProperty("12345678901234567890"),
			}),
			cv: cty.MapVal(map[string]cty.Value{"x": cty.MustParseNumberVal("12345678901234567890")}),
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()
			cv := tcase.cv
			pv := tcase.pv

			ih := inflectHelper{}

			if tcase.schema != nil {
				ih.schemaMap = sdkv2.NewSchemaMap(map[string]*schema.Schema{
					"prop": tcase.schema,
				})

				cv = cty.ObjectVal(map[string]cty.Value{"prop": cv})
				pv = resource.NewObjectProperty(resource.PropertyMap{"prop": pv})

				ih.schemaInfos = map[string]*info.Schema{
					"prop": {Name: "prop"},
				}
			}

			t.Logf("pv: %v", pv.String())
			t.Logf("cv: %v", cv.GoString())

			infl, err := ih.inflections(pv, cv)
			require.NoError(t, err)

			infle, err := rawStateEncodeInflections(infl)
			require.NoError(t, err)

			t.Logf("inflections: %#v", infle)

			recoveredCtyValue, err := rawStateRecover(pv, infl)
			require.NoError(t, err)

			t.Logf("cv2:%v", recoveredCtyValue.GoString())

			require.Truef(t, recoveredCtyValue.RawEquals(cv),
				"cty.Value failed to turn around\nExpected: %s\nActual:   %s\n",
				cv.GoString(),
				recoveredCtyValue.GoString(),
			)
		})
	}
}

func Test_rawstate_inflections_serialization(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		infl   rawStateInflections
		expect autogold.Value
	}

	testCases := []testCase{
		{
			name: "typedNull",
			infl: rawStateInflections{TypedNull: &typedNull{T: cty.Object(map[string]cty.Type{
				"x": cty.String,
				"y": cty.Number,
			})}},
			expect: autogold.Expect(`{
 "null": {
  "t": [
   "object",
   {
    "x": "string",
    "y": "number"
   }
  ]
 }
}`),
		},
		{
			name: "pluralize-null",
			infl: rawStateInflections{Pluralize: &pluralize{ElementType: &cty.String}},
			expect: autogold.Expect(`{
 "plu": {
  "i": {},
  "t": "string"
 }
}`),
		},
		{
			name: "pluralize-inner",
			infl: rawStateInflections{Pluralize: &pluralize{
				Inner: rawStateInflections{TypedNull: &typedNull{T: cty.String}},
			}},
			expect: autogold.Expect(`{
 "plu": {
  "i": {
   "null": {
    "t": "string"
   }
  }
 }
}`),
		},
		{
			name: "map-empty",
			infl: rawStateInflections{Map: &mapInflections{
				T: &cty.String,
			}},
			expect: autogold.Expect(`{
 "map": {
  "t": "string"
 }
}`),
		},
		{
			name: "map-regular",
			infl: rawStateInflections{
				Map: &mapInflections{
					ElementInflections: map[resource.PropertyKey]rawStateInflections{
						"x": {TypedNull: &typedNull{T: cty.Bool}},
					},
				},
			},
			expect: autogold.Expect(`{
 "map": {
  "m": {
   "x": {
    "null": {
     "t": "bool"
    }
   }
  }
 }
}`),
		},
		{
			name: "obj",
			infl: rawStateInflections{
				Obj: &objInflections{
					Ignored: map[resource.PropertyKey]struct{}{
						"__meta": {},
					},
					Renamed: map[resource.PropertyKey]string{
						"fooBar": "foo_bar",
					},
					ElementInflections: map[resource.PropertyKey]rawStateInflections{
						"fooBar": {
							TypedNull: &typedNull{
								T: cty.Bool,
							},
						},
					},
				},
			},
			expect: autogold.Expect(`{
 "obj": {
  "ignored": {
   "__meta": {}
  },
  "o": {
   "fooBar": {
    "null": {
     "t": "bool"
    }
   }
  },
  "renamed": {
   "fooBar": "foo_bar"
  }
 }
}`),
		},
		{
			name: "array-empty",
			infl: rawStateInflections{
				Array: &arrayInflections{
					T: &cty.Bool,
				},
			},
			expect: autogold.Expect(`{
 "arr": {
  "arr": null,
  "t": "bool"
 }
}`),
		},
		{
			name: "array-regular",
			infl: rawStateInflections{
				Array: &arrayInflections{
					ElementInflections: map[int]rawStateInflections{
						1: {
							TypedNull: &typedNull{T: cty.String},
						},
					},
				},
			},
			expect: autogold.Expect(`{
 "arr": {
  "arr": {
   "1": {
    "null": {
     "t": "string"
    }
   }
  }
 }
}`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := rawStateEncodeInflections(tc.infl)
			require.NoError(t, err)

			t.Logf("encoded: %#v", encoded)

			back, err := rawStateParseInflections(encoded)
			require.NoError(t, err)

			require.Equalf(t, tc.infl, back, "turnaround")

			encodedJ, err := json.MarshalIndent(encoded, "", " ")
			require.NoError(t, err)

			tc.expect.Equal(t, string(encodedJ))
		})
	}
}

func Test_rawStateReducePrecision(t *testing.T) {
	t.Parallel()

	a := cty.NumberFloatVal(1.1)
	b := cty.MustParseNumberVal("1.1")

	assert.Equal(t, false, a.RawEquals(b))
	assert.Equal(t, true, rawStateReducePrecision(a).RawEquals(rawStateReducePrecision(b)))
	assert.Equal(t, false, rawStateReducePrecision(a).RawEquals(cty.NumberFloatVal(1.0999)))
	assert.Equal(t, cty.NullVal(cty.Number), rawStateReducePrecision(cty.NullVal(cty.Number)))

	// Check that it does not mutate the inputs.
	fl := big.NewFloat(1.252235135353451351345134)
	rawStateReducePrecision(cty.NumberVal(fl))
	assert.Equal(t, 0, big.NewFloat(1.252235135353451351345134).Cmp(fl))
}

// For each situation when MakeTerraformResult introduces a distortion between the natural encoding of a TF value as a
// Pulumi value, rawstate needs to be able to compute inflections to reverse the process and reconstruct the TF value.
//
// It is useful to look at coverage reports produced solely from this test matrix to check that it covers interesting
// branches in [MakeTerraformOutput].
func Test_rawstate_against_MakeTerraformResult(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	type testCase struct {
		name    string
		inputs  resource.PropertyMap
		tfs     map[string]*schema.Schema
		ps      map[string]*SchemaInfo
		tfState autogold.Value
		infl    autogold.Value
	}

	testCases := []testCase{
		{
			name: "string",
			tfs: map[string]*schema.Schema{
				"str": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			inputs: resource.PropertyMap{"str": resource.NewStringProperty("OK")},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "str":cty.StringVal("OK")})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    }
  }
}`),
		},
		{
			name: "bool",
			tfs: map[string]*schema.Schema{
				"b": {
					Type:     schema.TypeBool,
					Optional: true,
				},
			},
			inputs:  resource.PropertyMap{"b": resource.NewBoolProperty(true)},
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"b":cty.True, "id":cty.StringVal("id0")})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    }
  }
}`),
		},
		{
			name: "int",
			tfs: map[string]*schema.Schema{
				"i": {
					Type:     schema.TypeInt,
					Optional: true,
				},
			},
			inputs:  resource.PropertyMap{"i": resource.NewNumberProperty(42)},
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"i":cty.NumberIntVal(42), "id":cty.StringVal("id0")})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    }
  }
}`),
		},
		{
			name: "float",
			tfs: map[string]*schema.Schema{
				"f": {
					Type:     schema.TypeFloat,
					Optional: true,
				},
			},
			inputs: resource.PropertyMap{"f": resource.NewNumberProperty(3.14)},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"f":cty.MustParseNumberVal("3.14"), "id":cty.StringVal("id0")})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    }
  }
}`),
		},
		{
			name: "coerced-bool",
			tfs: map[string]*schema.Schema{
				"b": {
					Type:     schema.TypeBool,
					Optional: true,
				},
			},
			inputs:  resource.PropertyMap{"b": resource.NewStringProperty("true")},
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"b":cty.True, "id":cty.StringVal("id0")})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    }
  }
}`),
		},
		{
			name: "coerced-int",
			tfs: map[string]*schema.Schema{
				"i": {
					Type:     schema.TypeInt,
					Optional: true,
				},
			},
			inputs:  resource.PropertyMap{"i": resource.NewStringProperty("42")},
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"i":cty.NumberIntVal(42), "id":cty.StringVal("id0")})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    }
  }
}`),
		},
		{
			name: "coerced-float",
			tfs: map[string]*schema.Schema{
				"f": {
					Type:     schema.TypeFloat,
					Optional: true,
				},
			},
			inputs: resource.PropertyMap{"f": resource.NewStringProperty("3.14")},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"f":cty.MustParseNumberVal("3.14"), "id":cty.StringVal("id0")})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    }
  }
}`),
		},
		{
			name: "list",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("A"),
				resource.NewStringProperty("B"),
				resource.NewStringProperty("C"),
			})},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.ListVal([]cty.Value{cty.StringVal("A"), cty.StringVal("B"), cty.StringVal("C")})})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "arr": {
          "arr": null
        }
      }
    }
  }
}`),
		},
		{
			name: "list-empty",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewArrayProperty([]resource.PropertyValue{})},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.ListValEmpty(cty.String)})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "arr": {
          "arr": null,
          "t": "string"
        }
      }
    }
  }
}`),
		},
		{
			name: "list-null",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewNullProperty()},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.NullVal(cty.List(cty.String))})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "null": {
          "t": [
            "list",
            "string"
          ]
        }
      }
    }
  }
}`),
		},
		{
			name: "list-maxitems1",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"x": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewObjectProperty(resource.PropertyMap{
				"x": resource.NewStringProperty("OK"),
			})},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"x":cty.StringVal("OK")})})})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "plu": {
          "i": {
            "obj": {}
          }
        }
      }
    }
  }
}`),
		},
		{
			name: "list-maxitems1-empty",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"x": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewNullProperty()},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.ListValEmpty(cty.Object(map[string]cty.Type{"x":cty.String}))})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "plu": {
          "i": {},
          "t": [
            "object",
            {
              "x": "string"
            }
          ]
        }
      }
    }
  }
}`),
		},
		{
			name: "set",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("A"),
				resource.NewStringProperty("B"),
				resource.NewStringProperty("C"),
			})},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.SetVal([]cty.Value{cty.StringVal("A"), cty.StringVal("B"), cty.StringVal("C")})})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "set": {
          "set": null
        }
      }
    }
  }
}`),
		},
		{
			name: "set-empty",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewArrayProperty([]resource.PropertyValue{})},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.NullVal(cty.Set(cty.String))})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "null": {
          "t": [
            "set",
            "string"
          ]
        }
      }
    }
  }
}`),
		},
		{
			name: "set-null",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewNullProperty()},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.NullVal(cty.Set(cty.String))})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "null": {
          "t": [
            "set",
            "string"
          ]
        }
      }
    }
  }
}`),
		},
		{
			name: "set-maxitems1",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeSet,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"x": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewObjectProperty(resource.PropertyMap{
				"x": resource.NewStringProperty("OK"),
			})},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.SetVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"x":cty.StringVal("OK")})})})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "plu": {
          "i": {
            "obj": {}
          },
          "set": true
        }
      }
    }
  }
}`),
		},
		{
			name: "set-maxitems1-empty",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeSet,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"x": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewNullProperty()},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.SetValEmpty(cty.Object(map[string]cty.Type{"x":cty.String}))})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "plu": {
          "i": {},
          "set": true,
          "t": [
            "object",
            {
              "x": "string"
            }
          ]
        }
      }
    }
  }
}`),
		},
		{
			name: "map",
			tfs: map[string]*schema.Schema{
				"ls": {
					Type:     schema.TypeMap,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeInt,
					},
				},
			},
			inputs: resource.PropertyMap{"ls": resource.NewObjectProperty(resource.PropertyMap{
				"one": resource.NewNumberProperty(1),
				"two": resource.NewNumberProperty(2),
			})},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "ls":cty.MapVal(map[string]cty.Value{"one":cty.NumberIntVal(1), "two":cty.NumberIntVal(2)})})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "ls": {
        "map": {}
      }
    }
  }
}`),
		},
		{
			name: "asset",
			tfs: map[string]*schema.Schema{
				"x": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			ps: map[string]*info.Schema{
				"x": {
					Asset: &info.AssetTranslation{
						Kind: FileAsset,
					},
				},
			},
			inputs: (func() resource.PropertyMap {
				asset, err := asset.FromPathWithWD(filepath.Join("testdata", "asset.txt"), ".")
				require.NoError(t, err)
				return resource.PropertyMap{"x": resource.NewAssetProperty(asset)}
			})(),
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "x":cty.StringVal("${TMPDIR}/pulumi-asset-e6f48d2de0fb13762c32a37daeef1a225a4793cacb598826dbb269e2cbe5b7f2")})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "x": {
        "asset": {
          "kind": 0
        }
      }
    }
  }
}`),
		},
		{
			name: "object",
			tfs: map[string]*schema.Schema{
				"x": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"s1": {Type: schema.TypeString, Optional: true},
							"s2": {Type: schema.TypeString, Optional: true},
						},
					},
				},
			},
			ps: map[string]*info.Schema{
				"x": {
					Fields: map[string]*info.Schema{
						"s2": {Name: "renamedS2"},
					},
					Elem: &info.Schema{
						Fields: map[string]*info.Schema{
							"s2": {Name: "renamedS2"},
						},
					},
				},
			},
			inputs: resource.PropertyMap{"x": resource.NewObjectProperty(resource.PropertyMap{
				"s1":        resource.NewStringProperty("S1"),
				"renamedS2": resource.NewStringProperty("S2"),
				"ignoredS3": resource.NewStringProperty("S3"),
			})},
			//nolint:lll
			tfState: autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "x":cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"s1":cty.StringVal("S1"), "s2":cty.StringVal("S2")})})})`),
			infl: autogold.Expect(`{
  "obj": {
    "ignored": {
      "__meta": {}
    },
    "o": {
      "x": {
        "plu": {
          "i": {
            "obj": {
              "renamed": {
                "renamedS2": "s2"
              }
            }
          }
        }
      }
    }
  }
}`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.inputs == nil {
				t.Skip("tc.inputs is nil")
			}

			tok := "r1"
			supportsSecrets := true

			p := sdkv2.NewProvider(&schema.Provider{
				ResourcesMap: map[string]*schema.Resource{
					tok: {
						CreateContext: func(
							ctx context.Context,
							rd *schema.ResourceData,
							i interface{},
						) diag.Diagnostics {
							rd.SetId("id0")
							return nil
						},
						Schema: tc.tfs,
					},
				},
			})

			tfs := sdkv2.NewSchemaMap(tc.tfs)
			prov := &Provider{tf: p}

			resourceConfig, assets, err := MakeTerraformConfig(ctx, prov, tc.inputs, tfs, tc.ps)
			require.NoError(t, err)

			instanceDiff, err := p.Diff(ctx, tok, nil /*state*/, resourceConfig, shim.DiffOptions{})
			require.NoError(t, err)

			state, err := p.Apply(ctx, tok, nil, instanceDiff)
			require.NoError(t, err)

			stateWithValue, ok := state.(shim.InstanceStateWithCtyValue)
			require.Truef(t, ok, "shim.InstanceStateWithCtyValue cast failed")
			stateValue := stateWithValue.Value()

			t.Logf("stateValue: %v", stateValue.GoString())

			tc.tfState.Equal(t, replaceTempdir(stateValue.GoString()))

			outMap, err := MakeTerraformResult(ctx, p, state, tfs, tc.ps, assets, supportsSecrets)
			require.NoError(t, err)

			ih := &inflectHelper{
				schemaMap:   tfs,
				schemaInfos: tc.ps,
			}

			pv := resource.NewObjectProperty(outMap)

			infl, err := ih.inflections(pv, stateValue)
			require.NoError(t, err)

			inflEnc, err := rawStateEncodeInflections(infl)
			require.NoError(t, err)

			inflEncJ, err := json.MarshalIndent(inflEnc, "", "  ")
			require.NoError(t, err)

			tc.infl.Equal(t, string(inflEncJ))

			err = rawStateTurnaroundCheck(stateValue, pv, infl)
			require.NoError(t, err)
		})
	}
}

func replaceTempdir(s string) string {
	sep := string(os.PathSeparator)
	tmp := strings.TrimSuffix(os.TempDir(), sep)
	pattern := regexp.MustCompile(fmt.Sprintf("%s(%s)*", regexp.QuoteMeta(tmp), regexp.QuoteMeta(sep)))
	return pattern.ReplaceAllLiteralString(s, "${TMPDIR}"+sep)
}

func Test_replaceTempdir(t *testing.T) {
	t.Parallel()
	//nolint:lll
	x := `cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "x":cty.StringVal("/var/folders/gd/3ncjb1lj5ljgk8xl5ssn_gvc0000gn/T/com.apple.shortcuts.mac-helper/pulumi-asset-e6f48d2de0fb13762c32a37daeef1a225a4793cacb598826dbb269e2cbe5b7f2")})`
	//nolint:lll
	autogold.Expect(`cty.ObjectVal(map[string]cty.Value{"id":cty.StringVal("id0"), "x":cty.StringVal("${TMPDIR}/pulumi-asset-e6f48d2de0fb13762c32a37daeef1a225a4793cacb598826dbb269e2cbe5b7f2")})`).Equal(t, replaceTempdir(x))
}
