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
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// Test how various SDKv2-based schemata translate to the shim.Schema layer.
func TestSchemaShimRepresentations(t *testing.T) {
	type testCase struct {
		name           string
		resourceSchema map[string]*schema.Schema
		expect         autogold.Value
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
		},
		{
			"list nested block",
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
		},
		{
			"list attribute max items one",
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
		},
		{
			"list block",
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
		})
	}
}
