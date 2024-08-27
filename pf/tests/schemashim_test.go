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

package tfbridgetests

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	pb "github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/require"
)

// Test how various PF-based schemata translate to the shim.Schema layer. Excerpts of the resulting Pulumi Package
// Schema are included for reasoning convenience.
func TestSchemaShimRepresentations(t *testing.T) {

	type testCase struct {
		name         string
		provider     provider.Provider
		expect       autogold.Value // expected prettified shim.Schema representation
		expectSchema autogold.Value // expected corresponding Pulumi Package Schema extract
	}

	testCases := []testCase{
		//------------------------------------------------------------------------------------------------------
		{
			"single-nested-block",
			&pb.Provider{
				TypeName: "testprov",
				AllResources: []pb.Resource{{
					Name: "r1",
					ResourceSchema: schema.Schema{
						Blocks: map[string]schema.Block{
							"single_nested_block": schema.SingleNestedBlock{
								Attributes: map[string]schema.Attribute{
									"a1": schema.Float64Attribute{
										Optional: true,
									},
								},
							},
						},
					},
				}},
			},
			autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "single_nested_block": {
        "element": {
          "resource": {
            "a1": {
              "optional": true,
              "type": 3
            }
          }
        },
        "optional": true,
        "type": 6
      }
    }
  }
}`),
			autogold.Expect(`{
  "resource": {
    "properties": {
      "singleNestedBlock": {
        "$ref": "#/types/testprov:index/R1SingleNestedBlock:R1SingleNestedBlock"
      }
    },
    "inputProperties": {
      "singleNestedBlock": {
        "$ref": "#/types/testprov:index/R1SingleNestedBlock:R1SingleNestedBlock"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "singleNestedBlock": {
          "$ref": "#/types/testprov:index/R1SingleNestedBlock:R1SingleNestedBlock"
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1SingleNestedBlock:R1SingleNestedBlock": {
      "properties": {
        "a1": {
          "type": "number"
        }
      },
      "type": "object"
    }
  }
}`),
		},
		//------------------------------------------------------------------------------------------------------
		{
			"list-nested-block",
			&pb.Provider{
				TypeName: "testprov",
				AllResources: []pb.Resource{{
					Name: "r1",
					ResourceSchema: schema.Schema{
						Blocks: map[string]schema.Block{
							"list_nested_block": schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										"a1": schema.Float64Attribute{
											Optional: true,
										},
									},
								},
							},
						},
					},
				}},
			},
			autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "list_nested_block": {
        "element": {
          "resource": {
            "a1": {
              "optional": true,
              "type": 3
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
      "listNestedBlocks": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/R1ListNestedBlock:R1ListNestedBlock"
        }
      }
    },
    "inputProperties": {
      "listNestedBlocks": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/R1ListNestedBlock:R1ListNestedBlock"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "listNestedBlocks": {
          "type": "array",
          "items": {
            "$ref": "#/types/testprov:index/R1ListNestedBlock:R1ListNestedBlock"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1ListNestedBlock:R1ListNestedBlock": {
      "properties": {
        "a1": {
          "type": "number"
        }
      },
      "type": "object"
    }
  }
}`),
		},
		//------------------------------------------------------------------------------------------------------
		{
			"map-nested-attribute",
			&pb.Provider{
				TypeName: "testprov",
				AllResources: []pb.Resource{{
					Name: "r1",
					ResourceSchema: schema.Schema{
						Attributes: map[string]schema.Attribute{
							"map_nested_attribute": schema.MapNestedAttribute{
								Optional: true,
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"a1": schema.StringAttribute{
											Optional: true,
										},
									},
								},
							},
						},
					},
				}},
			},
			autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "map_nested_attribute": {
        "element": {
          "schema": {
            "element": {
              "resource": {
                "a1": {
                  "optional": true,
                  "type": 4
                }
              }
            },
            "type": 6
          }
        },
        "optional": true,
        "type": 6
      }
    }
  }
}`),
			autogold.Expect(`{
  "resource": {
    "properties": {
      "mapNestedAttribute": {
        "type": "object",
        "additionalProperties": {
          "$ref": "#/types/testprov:index/R1MapNestedAttribute:R1MapNestedAttribute"
        }
      }
    },
    "inputProperties": {
      "mapNestedAttribute": {
        "type": "object",
        "additionalProperties": {
          "$ref": "#/types/testprov:index/R1MapNestedAttribute:R1MapNestedAttribute"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "mapNestedAttribute": {
          "type": "object",
          "additionalProperties": {
            "$ref": "#/types/testprov:index/R1MapNestedAttribute:R1MapNestedAttribute"
          }
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1MapNestedAttribute:R1MapNestedAttribute": {
      "properties": {
        "a1": {
          "type": "string"
        }
      },
      "type": "object"
    }
  }
}`),
		},
		//------------------------------------------------------------------------------------------------------
		{
			"object-attribute",
			&pb.Provider{
				TypeName: "testprov",
				AllResources: []pb.Resource{{
					Name: "r1",
					ResourceSchema: schema.Schema{
						Attributes: map[string]schema.Attribute{
							"object_attribute": schema.ObjectAttribute{
								Optional: true,
								AttributeTypes: map[string]attr.Type{
									"a1": types.StringType,
								},
							},
						},
					},
				}},
			},
			autogold.Expect(`{
  "resources": {
    "testprov_r1": {
      "object_attribute": {
        "element": {
          "resource": {
            "a1": {
              "type": 4
            }
          }
        },
        "optional": true,
        "type": 6
      }
    }
  }
}`),
			autogold.Expect(`{
  "resource": {
    "properties": {
      "objectAttribute": {
        "$ref": "#/types/testprov:index/R1ObjectAttribute:R1ObjectAttribute"
      }
    },
    "inputProperties": {
      "objectAttribute": {
        "$ref": "#/types/testprov:index/R1ObjectAttribute:R1ObjectAttribute"
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering R1 resources.\n",
      "properties": {
        "objectAttribute": {
          "$ref": "#/types/testprov:index/R1ObjectAttribute:R1ObjectAttribute"
        }
      },
      "type": "object"
    }
  },
  "types": {
    "testprov:index/R1ObjectAttribute:R1ObjectAttribute": {
      "properties": {
        "a1": {
          "type": "string"
        }
      },
      "type": "object",
      "required": [
        "a1"
      ]
    }
  }
}`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shimmedProvider := schemashim.ShimSchemaOnlyProvider(context.Background(), tc.provider)

			m := tfbridge.MarshalProvider(shimmedProvider)
			bytes, err := json.Marshal(m)
			require.NoError(t, err)

			var pretty map[string]any
			err = json.Unmarshal(bytes, &pretty)
			require.NoError(t, err)

			prettyBytes, err := json.MarshalIndent(pretty, "", "  ")
			require.NoError(t, err)

			tc.expect.Equal(t, string(prettyBytes))

			token := "testprov:index:R1"

			info := info.Provider{
				Name: "testprov",
				P:    shimmedProvider,
				Resources: map[string]*info.Resource{
					"testprov_r1": {
						Tok: tokens.Type(token),
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
				Resource: pSpec.Resources[token],
				Types:    pSpec.Types,
			}

			prettySpec, err := json.MarshalIndent(ms, "", "  ")
			require.NoError(t, err)

			tc.expectSchema.Equal(t, string(prettySpec))
		})
	}
}
