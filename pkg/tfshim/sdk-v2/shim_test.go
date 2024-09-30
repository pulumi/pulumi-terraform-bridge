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

package sdkv2

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Test how various SDKv2-based schemata translate to the shim.Schema layer.
func TestSchemaShimRepresentations(t *testing.T) {
	type testCase struct {
		name           string
		resourceSchema map[string]*schema.Schema
		expect         autogold.Value // expected prettified shim.Schema representation
		expectSchema   autogold.Value // expected corresponding Pulumi Package Schema extract
	}

	testCases := []testCase{
		{
			"string attribute",
			map[string]*schema.Schema{
				"field_attr": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			autogold.Expect(`{
  "resources": {
    "res": {
      "field_attr": {
        "optional": true,
        "type": 4
      }
    }
  }
}`),
			autogold.Expect(`{
  "resource": {
    "properties": {
      "fieldAttr": {
        "type": "string"
      }
    },
    "inputProperties": {
      "fieldAttr": {
        "type": "string"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering Res resources.\n",
      "properties": {
        "fieldAttr": {
          "type": "string"
        }
      },
      "type": "object"
    }
  },
  "types": {}
}`),
		},
		{
			"list attribute",
			map[string]*schema.Schema{
				"field_attr": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			autogold.Expect(`{
  "resources": {
    "res": {
      "field_attr": {
        "element": {
          "schema": {
            "type": 4
          }
        },
        "optional": true,
        "type": 5
      }
    }
  }
}`),
			autogold.Expect(`{
  "resource": {
    "properties": {
      "fieldAttrs": {
        "type": "array",
        "items": {
          "type": "string"
        }
      }
    },
    "inputProperties": {
      "fieldAttrs": {
        "type": "array",
        "items": {
          "type": "string"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering Res resources.\n",
      "properties": {
        "fieldAttrs": {
          "type": "array",
          "items": {
            "type": "string"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {}
}`),
		},
		{
			"list block",
			map[string]*schema.Schema{
				"block_field": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"field_attr": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
			autogold.Expect(`{
  "resources": {
    "res": {
      "block_field": {
        "element": {
          "resource": {
            "field_attr": {
              "optional": true,
              "type": 4
            }
          }
        },
        "optional": true,
        "type": 5
      }
    }
  }
}`),
			autogold.Expect(`{
  "resource": {
    "properties": {
      "blockFields": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/ResBlockField:ResBlockField"
        }
      }
    },
    "inputProperties": {
      "blockFields": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/ResBlockField:ResBlockField"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering Res resources.\n",
      "properties": {
        "blockFields": {
          "type": "array",
          "items": {
            "$ref": "#/types/testprov:index/ResBlockField:ResBlockField"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/ResBlockField:ResBlockField": {
      "properties": {
        "fieldAttr": {
          "type": "string"
        }
      },
      "type": "object"
    }
  }
}`),
		},
		{
			"list block nested",
			map[string]*schema.Schema{
				"block_field": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"nested_field": {
								Type:     schema.TypeList,
								Optional: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"field_attr": {
											Type:     schema.TypeString,
											Optional: true,
										},
									},
								},
							},
						},
					},
				},
			},
			autogold.Expect(`{
  "resources": {
    "res": {
      "block_field": {
        "element": {
          "resource": {
            "nested_field": {
              "element": {
                "resource": {
                  "field_attr": {
                    "optional": true,
                    "type": 4
                  }
                }
              },
              "optional": true,
              "type": 5
            }
          }
        },
        "optional": true,
        "type": 5
      }
    }
  }
}`),
			autogold.Expect(`{
  "resource": {
    "properties": {
      "blockFields": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/ResBlockField:ResBlockField"
        }
      }
    },
    "inputProperties": {
      "blockFields": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/ResBlockField:ResBlockField"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering Res resources.\n",
      "properties": {
        "blockFields": {
          "type": "array",
          "items": {
            "$ref": "#/types/testprov:index/ResBlockField:ResBlockField"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/ResBlockField:ResBlockField": {
      "properties": {
        "nestedFields": {
          "type": "array",
          "items": {
            "$ref": "#/types/testprov:index/ResBlockFieldNestedField:ResBlockFieldNestedField"
          }
        }
      },
      "type": "object"
    },
    "testprov:index/ResBlockFieldNestedField:ResBlockFieldNestedField": {
      "properties": {
        "fieldAttr": {
          "type": "string"
        }
      },
      "type": "object"
    }
  }
}`),
		},
		{
			"list attribute flattened",
			map[string]*schema.Schema{
				"field_attr": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
			autogold.Expect(`{
  "resources": {
    "res": {
      "field_attr": {
        "element": {
          "schema": {
            "type": 4
          }
        },
        "maxItems": 1,
        "optional": true,
        "type": 5
      }
    }
  }
}`),
			autogold.Expect(`{
  "resource": {
    "properties": {
      "fieldAttr": {
        "type": "string"
      }
    },
    "inputProperties": {
      "fieldAttr": {
        "type": "string"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering Res resources.\n",
      "properties": {
        "fieldAttr": {
          "type": "string"
        }
      },
      "type": "object"
    }
  },
  "types": {}
}`),
		},
		{
			"list block flattened",
			map[string]*schema.Schema{
				"block_field": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"field_attr": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
			autogold.Expect(`{
  "resources": {
    "res": {
      "block_field": {
        "element": {
          "resource": {
            "field_attr": {
              "optional": true,
              "type": 4
            }
          }
        },
        "maxItems": 1,
        "optional": true,
        "type": 5
      }
    }
  }
}`),
			autogold.Expect(`{
  "resource": {
    "properties": {
      "blockField": {
        "$ref": "#/types/testprov:index/ResBlockField:ResBlockField"
      }
    },
    "inputProperties": {
      "blockField": {
        "$ref": "#/types/testprov:index/ResBlockField:ResBlockField"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering Res resources.\n",
      "properties": {
        "blockField": {
          "$ref": "#/types/testprov:index/ResBlockField:ResBlockField"
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/ResBlockField:ResBlockField": {
      "properties": {
        "fieldAttr": {
          "type": "string"
        }
      },
      "type": "object"
    }
  }
}`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &schema.Provider{
				ResourcesMap: map[string]*schema.Resource{
					"res": {
						Schema: tc.resourceSchema,
					},
				},
			}
			shimmedProvider := NewProvider(provider)
			require.NoError(t, shimmedProvider.InternalValidate())

			m := tfbridge.MarshalProvider(shimmedProvider)
			bytes, err := json.Marshal(m)
			require.NoError(t, err)

			var pretty map[string]any
			err = json.Unmarshal(bytes, &pretty)
			require.NoError(t, err)

			prettyBytes, err := json.MarshalIndent(pretty, "", "  ")
			require.NoError(t, err)

			tc.expect.Equal(t, string(prettyBytes))

			rtok := "testprov:index:Res"

			info := info.Provider{
				Name: "testprov",
				P:    shimmedProvider,
				Resources: map[string]*info.Resource{
					"res": {
						Tok: tokens.Type(rtok),
					},
				},
			}

			nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
			pSpec, err := tfgen.GenerateSchema(info, nilSink)
			require.NoError(t, err)

			type miniSpec struct {
				Resource any `json:"resource"`
				Types    any `json:"types"`
			}

			ms := miniSpec{
				Resource: pSpec.Resources[rtok],
				Types:    pSpec.Types,
			}

			prettySpec, err := json.MarshalIndent(ms, "", "  ")
			require.NoError(t, err)

			tc.expectSchema.Equal(t, string(prettySpec))
		})
	}
}
