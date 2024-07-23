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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	pb "github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/stretchr/testify/require"
)

// Test how various PF-based schemata translate to the shim.Schema layer.
func TestSchemaShimRepresentations(t *testing.T) {
	type testCase struct {
		name     string
		provider provider.Provider
		expect   autogold.Value
	}

	testCases := []testCase{
		{
			"simple-attribute",
			&pb.Provider{
				AllResources: []pb.Resource{{
					ResourceSchema: schema.Schema{
						Attributes: map[string]schema.Attribute{
							"simple_attribute": schema.StringAttribute{
								Optional: true,
							},
						},
					},
				}},
			},
			autogold.Expect(`{
  "resources": {
    "_": {
      "simple_attribute": {
        "optional": true,
        "type": 4
      }
    }
  }
}`),
		},
		{
			"object-attribute",
			&pb.Provider{
				AllResources: []pb.Resource{{
					ResourceSchema: schema.Schema{
						Attributes: map[string]schema.Attribute{
							"object_attribute": schema.ObjectAttribute{
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
    "_": {
      "object_attribute": {
        "element": {
          "resource": {
            "a1": {
              "type": 4
            }
          }
        },
        "type": 6
      }
    }
  }
}`),
		},
		{
			"list-attribute",
			&pb.Provider{
				AllResources: []pb.Resource{
					{
						ResourceSchema: schema.Schema{
							Attributes: map[string]schema.Attribute{
								"list_attribute": schema.ListAttribute{
									Optional:    true,
									ElementType: types.StringType,
								},
							},
						},
					},
				},
			},
			autogold.Expect(`{
  "resources": {
    "_": {
      "list_attribute": {
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
		},
		{
			"list-attribute-object-element",
			&pb.Provider{
				AllResources: []pb.Resource{
					{
						ResourceSchema: schema.Schema{
							Attributes: map[string]schema.Attribute{
								"list_attribute": schema.ListAttribute{
									Optional: true,
									ElementType: types.ObjectType{
										AttrTypes: map[string]attr.Type{
											"a1": types.StringType,
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
    "_": {
      "list_attribute": {
        "element": {
          "schema": {
            "element": {
              "resource": {
                "a1": {
                  "type": 4
                }
              }
            },
            "type": 6
          }
        },
        "optional": true,
        "type": 5
      }
    }
  }
}`),
		},
		{
			"list-nested-attribute",
			&pb.Provider{
				AllResources: []pb.Resource{
					{
						ResourceSchema: schema.Schema{
							Attributes: map[string]schema.Attribute{
								"list_nested_attribute": schema.ListNestedAttribute{
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
					},
				},
			},
			autogold.Expect(`{
  "resources": {
    "_": {
      "list_nested_attribute": {
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
        "type": 5
      }
    }
  }
}`),
		},
		{
			"map-nested-attribute",
			&pb.Provider{
				AllResources: []pb.Resource{{
					ResourceSchema: schema.Schema{
						Attributes: map[string]schema.Attribute{
							"map_nested_attribute": schema.MapNestedAttribute{
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
    "_": {
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
        "type": 6
      }
    }
  }
}`),
		},
		{
			"single-nested-attribute",
			&pb.Provider{
				AllResources: []pb.Resource{
					{
						ResourceSchema: schema.Schema{
							Attributes: map[string]schema.Attribute{
								"single_nested_attribute": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"a1": schema.StringAttribute{
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
    "_": {
      "single_nested_attribute": {
        "element": {
          "resource": {
            "a1": {
              "optional": true,
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
		},
		{
			"single-nested-block",
			&pb.Provider{
				AllResources: []pb.Resource{{
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
    "_": {
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
		},
		{
			"list-nested-block",
			&pb.Provider{
				AllResources: []pb.Resource{{
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
    "_": {
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
		})
	}
}
