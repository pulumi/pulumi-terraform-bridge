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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

// TestAttributeAsBlocksRequired asserts that when no required/optional information is present, then we fall back to
// assuming all fields are optional.
func TestAttributeAsBlocksRequired(t *testing.T) {
	t.Parallel()
	p := proto.New(t.Context(), providerServer{
		SchemaResponse: &tfprotov6.GetProviderSchemaResponse{
			ResourceSchemas: map[string]*tfprotov6.Schema{
				"testprov_my_res": {Block: &tfprotov6.SchemaBlock{
					Attributes: []*tfprotov6.SchemaAttribute{{
						Name:     "ingress",
						Optional: true,
						Type: tftypes.List{ElementType: tftypes.Object{
							AttributeTypes: map[string]tftypes.Type{
								"from_port":   tftypes.Number,
								"to_port":     tftypes.Number,
								"protocol":    tftypes.String,
								"description": tftypes.String,
								"cidr_blocks": tftypes.List{ElementType: tftypes.String},
							},
						}},
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

	typ := spec.Types["testprov:index/MyResIngress:MyResIngress"]
	autogold.Expect([]string{}).Equal(t, typ.Required)
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

// TestNestedAttributeSchema asserts that a list- or set-nested attribute
// carries its object as a [shim.Schema] element, which tells it apart from a
// list- or set-nested block, and that the Pulumi schema of both is the same.
func TestNestedAttributeSchema(t *testing.T) {
	t.Parallel()
	nested := func(nesting tfprotov6.SchemaObjectNestingMode) *tfprotov6.SchemaObject {
		return &tfprotov6.SchemaObject{
			Nesting: nesting,
			Attributes: []*tfprotov6.SchemaAttribute{{
				Name:     "name",
				Type:     tftypes.String,
				Optional: true,
			}},
		}
	}
	p := proto.New(t.Context(), providerServer{
		SchemaResponse: &tfprotov6.GetProviderSchemaResponse{
			ResourceSchemas: map[string]*tfprotov6.Schema{
				"testprov_my_res": {Block: &tfprotov6.SchemaBlock{
					Attributes: []*tfprotov6.SchemaAttribute{
						{Name: "list_attr", Optional: true, NestedType: nested(tfprotov6.SchemaObjectNestingModeList)},
						{Name: "set_attr", Optional: true, NestedType: nested(tfprotov6.SchemaObjectNestingModeSet)},
					},
					BlockTypes: []*tfprotov6.SchemaNestedBlock{{
						TypeName: "list_block",
						Nesting:  tfprotov6.SchemaNestedBlockNestingModeList,
						Block: &tfprotov6.SchemaBlock{
							Attributes: []*tfprotov6.SchemaAttribute{{
								Name:     "name",
								Type:     tftypes.String,
								Optional: true,
							}},
						},
					}},
				}},
			},
		},
	})

	autogold.Expect(`{
    "resources": {
        "testprov_my_res": {
            "list_attr": {
                "type": 5,
                "optional": true,
                "element": {
                    "schema": {
                        "type": 6,
                        "optional": true,
                        "element": {
                            "resource": {
                                "name": {
                                    "type": 4,
                                    "optional": true
                                }
                            }
                        }
                    }
                }
            },
            "list_block": {
                "type": 5,
                "optional": true,
                "element": {
                    "resource": {
                        "name": {
                            "type": 4,
                            "optional": true
                        }
                    }
                }
            },
            "set_attr": {
                "type": 5,
                "optional": true,
                "element": {
                    "schema": {
                        "type": 6,
                        "optional": true,
                        "element": {
                            "resource": {
                                "name": {
                                    "type": 4,
                                    "optional": true
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
`).Equal(t, string(marshalProviderShim(t, p)))

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
	specBytes, err := json.MarshalIndent(map[string]any{
		"inputProperties": spec.Resources["testprov:index:MyRes"].InputProperties,
		"types":           spec.Types,
	}, "", "  ")
	require.NoError(t, err)
	autogold.Expect(`{
  "inputProperties": {
    "listAttrs": {
      "type": "array",
      "items": {
        "$ref": "#/types/testprov:index/MyResListAttr:MyResListAttr"
      }
    },
    "listBlocks": {
      "type": "array",
      "items": {
        "$ref": "#/types/testprov:index/MyResListBlock:MyResListBlock"
      }
    },
    "setAttrs": {
      "type": "array",
      "items": {
        "$ref": "#/types/testprov:index/MyResSetAttr:MyResSetAttr"
      }
    }
  },
  "types": {
    "testprov:index/MyResListAttr:MyResListAttr": {
      "properties": {
        "name": {
          "type": "string"
        }
      },
      "type": "object"
    },
    "testprov:index/MyResListBlock:MyResListBlock": {
      "properties": {
        "name": {
          "type": "string"
        }
      },
      "type": "object"
    },
    "testprov:index/MyResSetAttr:MyResSetAttr": {
      "properties": {
        "name": {
          "type": "string"
        }
      },
      "type": "object"
    }
  }
}`).Equal(t, string(specBytes))
}
