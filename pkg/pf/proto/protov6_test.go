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

package proto_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/proto"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func marshalProviderShim(t *testing.T, p shim.Provider) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	require.NoError(t, enc.Encode(info.MarshalProviderShim(p)))
	return b.Bytes()
}

func TestShimSchema(t *testing.T) {
	t.Parallel()
	autogold.Expect("{}\n").Equal(t, string(marshalProviderShim(t, proto.Empty())))
}

func TestDynamicType(t *testing.T) {
	t.Parallel()
	b := marshalProviderShim(t,
		proto.New(context.Background(), providerServer{
			SchemaResponse: &tfprotov6.GetProviderSchemaResponse{
				ResourceSchemas: map[string]*tfprotov6.Schema{
					"my_res": {Block: &tfprotov6.SchemaBlock{
						Attributes: []*tfprotov6.SchemaAttribute{{
							Name: "attr",
							Type: tftypes.DynamicPseudoType,
						}},
					}},
				},
			},
		}))
	autogold.Expect(`{
    "resources": {
        "my_res": {
            "attr": {
                "type": 8
            }
        }
    }
}
`).Equal(t, string(b))
}

func TestBlockSchemaGeneration(t *testing.T) {
	t.Parallel()
	p := proto.New(context.Background(), providerServer{
		SchemaResponse: &tfprotov6.GetProviderSchemaResponse{
			ResourceSchemas: map[string]*tfprotov6.Schema{
				"testprov_my_res": {Block: &tfprotov6.SchemaBlock{
					BlockTypes: []*tfprotov6.SchemaNestedBlock{
						{
							TypeName: "blk",
							Nesting:  tfprotov6.SchemaNestedBlockNestingModeList,
							Block: &tfprotov6.SchemaBlock{
								Attributes: []*tfprotov6.SchemaAttribute{{
									Name:     "bah",
									Type:     tftypes.Bool,
									Optional: true,
								}},
							},
						},
					},
				}},
			},
		},
	})
	providerInfo := info.Provider{
		P:    p,
		Name: "testprov",
		Resources: map[string]*info.Resource{
			"testprov_my_res": {
				Tok: "testprov:index:MyRes",
			},
		},
	}
	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	spec, err := tfgen.GenerateSchema(providerInfo, nilSink)
	require.NoError(t, err)
	specBytes, err := json.MarshalIndent(spec.Resources, "", "  ")
	require.NoError(t, err)
	autogold.Expect(`{
  "testprov:index:MyRes": {
    "properties": {
      "blks": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/MyResBlk:MyResBlk"
        }
      }
    },
    "inputProperties": {
      "blks": {
        "type": "array",
        "items": {
          "$ref": "#/types/testprov:index/MyResBlk:MyResBlk"
        }
      }
    },
    "stateInputs": {
      "description": "Input properties used for looking up and filtering MyRes resources.\n",
      "properties": {
        "blks": {
          "type": "array",
          "items": {
            "$ref": "#/types/testprov:index/MyResBlk:MyResBlk"
          }
        }
      },
      "type": "object"
    }
  }
}`).Equal(t, string(specBytes))
	assert.Containsf(t, spec.Resources["testprov:index:MyRes"].InputProperties, "blks",
		"Blocks should map to input properties")
}

func TestComputedPropagationForNestedAttributes(t *testing.T) {
	t.Parallel()
	p := proto.New(context.Background(), providerServer{
		SchemaResponse: &tfprotov6.GetProviderSchemaResponse{
			ResourceSchemas: map[string]*tfprotov6.Schema{
				"testprov_my_res": {Block: &tfprotov6.SchemaBlock{
					Attributes: []*tfprotov6.SchemaAttribute{{
						Name:     "metadata",
						Computed: true,
						Type: tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"value_input_details": tftypes.Object{
									AttributeTypes: map[string]tftypes.Type{
										"allowed_value_type": tftypes.String,
										"max_value":          tftypes.Number,
										"min_value":          tftypes.Number,
										"possible_values": tftypes.List{
											ElementType: tftypes.String,
										},
									},
								},
							},
						},
					}},
				}},
			},
		},
	})
	providerInfo := info.Provider{
		P:    p,
		Name: "testprov",
		Resources: map[string]*info.Resource{
			"testprov_my_res": {
				Tok: "testprov:index:MyRes",
			},
		},
	}
	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	spec, err := tfgen.GenerateSchema(providerInfo, nilSink)
	require.NoError(t, err)

	var found bool
	for _, typ := range spec.Types {
		if _, ok := typ.Properties["allowedValueType"]; !ok {
			continue
		}
		found = true
		assert.NotContains(t, typ.Required, "allowedValueType")
		assert.NotContains(t, typ.Required, "maxValue")
		assert.NotContains(t, typ.Required, "minValue")
		assert.NotContains(t, typ.Required, "possibleValues")

		var nodeInfo struct {
			RequiredOutputs []string `json:"requiredOutputs,omitempty"`
		}
		raw, ok := typ.Language["nodejs"]
		require.True(t, ok, "expected nodejs language info on nested object type")
		require.NoError(t, json.Unmarshal(raw, &nodeInfo))
		assert.ElementsMatch(t, []string{
			"allowedValueType",
			"maxValue",
			"minValue",
			"possibleValues",
		}, nodeInfo.RequiredOutputs)
		break
	}
	require.True(t, found, "expected to find nested object type with computed fields")
}

func TestComputedPropagationUpgradeFromSDKv2(t *testing.T) {
	t.Parallel()
	sdkProvider := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"testprov_my_res": {
				Schema: map[string]*schema.Schema{
					"config_items": {
						Optional: true,
						Type:     schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"metadata": {
									Optional: true,
									Type:     schema.TypeList,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"value_input_details": {
												Optional: true,
												Type:     schema.TypeList,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"allowed_value_type": {
															Computed: true,
															Type:     schema.TypeString,
														},
														"max_value": {
															Computed: true,
															Type:     schema.TypeFloat,
														},
														"min_value": {
															Computed: true,
															Type:     schema.TypeFloat,
														},
														"possible_values": {
															Computed: true,
															Type:     schema.TypeList,
															Elem: &schema.Schema{
																Type: schema.TypeString,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	v6server, err := tf5to6server.UpgradeServer(context.Background(), func() tfprotov5.ProviderServer {
		return sdkProvider.GRPCProvider()
	})
	require.NoError(t, err)

	p := proto.New(context.Background(), v6server)
	providerInfo := info.Provider{
		P:    p,
		Name: "testprov",
		Resources: map[string]*info.Resource{
			"testprov_my_res": {
				Tok: "testprov:index:MyRes",
			},
		},
	}
	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	spec, err := tfgen.GenerateSchema(providerInfo, nilSink)
	require.NoError(t, err)

	var found bool
	for _, typ := range spec.Types {
		if _, ok := typ.Properties["allowedValueType"]; !ok {
			continue
		}
		found = true
		assert.NotContains(t, typ.Required, "allowedValueType")
		assert.NotContains(t, typ.Required, "maxValue")
		assert.NotContains(t, typ.Required, "minValue")
		assert.NotContains(t, typ.Required, "possibleValues")

		var nodeInfo struct {
			RequiredOutputs []string `json:"requiredOutputs,omitempty"`
		}
		raw, ok := typ.Language["nodejs"]
		require.True(t, ok, "expected nodejs language info on nested object type")
		require.NoError(t, json.Unmarshal(raw, &nodeInfo))
		assert.ElementsMatch(t, []string{
			"allowedValueType",
			"maxValue",
			"minValue",
			"possibleValues",
		}, nodeInfo.RequiredOutputs)
		break
	}
	require.True(t, found, "expected to find nested object type with computed fields")
}

type providerServer struct {
	tfprotov6.ProviderServer // This will panic if un-overridden methods are called

	SchemaResponse *tfprotov6.GetProviderSchemaResponse
}

func (p providerServer) GetProviderSchema(
	context.Context, *tfprotov6.GetProviderSchemaRequest,
) (*tfprotov6.GetProviderSchemaResponse, error) {
	if p.SchemaResponse == nil {
		return nil, fmt.Errorf("unimplemented")
	}
	return p.SchemaResponse, nil
}
