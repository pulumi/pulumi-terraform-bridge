package sdkv2

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider1UpgradeResourceState(t *testing.T) {
	t.Parallel()

	type tc struct {
		name   string
		schema *schema.Resource
		input  func() *terraform.InstanceState
		expect func(t *testing.T, actual *terraform.InstanceState, tc tc)
	}

	tests := []tc{
		{
			name: "roundtrip int64",
			schema: &schema.Resource{
				UseJSONNumber: true,
				Schema: map[string]*schema.Schema{
					"x": {Type: schema.TypeInt, Optional: true},
				},
			},
			input: func() *terraform.InstanceState {
				n, err := cty.ParseNumberVal("641577219598130723")
				require.NoError(t, err)
				v := cty.ObjectVal(map[string]cty.Value{"x": n})
				s := terraform.NewInstanceStateShimmedFromValue(v, 0)
				s.Meta["schema_version"] = "0"
				s.ID = "id"
				s.RawState = v
				s.Attributes["id"] = s.ID
				return s
			},
			expect: func(t *testing.T, actual *terraform.InstanceState, tc tc) {
				assert.Equal(t, tc.input().Attributes, actual.Attributes)
			},
		},
		{
			name: "type change",
			schema: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"x1": {Type: schema.TypeInt, Optional: true},
				},
				SchemaVersion: 1,
				StateUpgraders: []schema.StateUpgrader{{
					Version: 0,
					Type: cty.Object(map[string]cty.Type{
						"id": cty.String,
						"x0": cty.String,
					}),
					Upgrade: func(_ context.Context, rawState map[string]any, _ interface{}) (map[string]any, error) {
						return map[string]any{
							"id": rawState["id"],
							"x1": len(rawState["x0"].(string)),
						}, nil
					},
				}},
			},
			input: func() *terraform.InstanceState {
				s := terraform.NewInstanceStateShimmedFromValue(cty.ObjectVal(map[string]cty.Value{
					"x0": cty.StringVal("123"),
				}), 0)
				s.Meta["schema_version"] = "0"
				s.ID = "id"
				return s
			},
			expect: func(t *testing.T, actual *terraform.InstanceState, tc tc) {
				t.Logf("Actual = %#v", actual)
				assert.Equal(t, map[string]string{
					"id": "id",
					"x1": "3",
				}, actual.Attributes)
			},
		},
	}

	const tfToken = "test_token"

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			require.NoError(t, tt.schema.InternalValidate(tt.schema.Schema, true))

			p := &schema.Provider{ResourcesMap: map[string]*schema.Resource{tfToken: tt.schema}}

			actual, err := upgradeResourceState(ctx, tfToken, p, tt.schema, tt.input())
			require.NoError(t, err)

			tt.expect(t, actual, tt)
		})
	}
}

//nolint:lll
func TestMarshalProviderShim(t *testing.T) {
	prov := NewProvider(&schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"test_resource": {
				Schema: map[string]*schema.Schema{
					"foo": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"bar": {
						Type:     schema.TypeInt,
						Optional: true,
					},
					"nested_prop": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"nested_foo": {
									Type:     schema.TypeString,
									Optional: true,
								},
							},
						},
					},
					"max_item_one_prop": {
						Type:     schema.TypeList,
						Required: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"nested_foo": {
									Type:     schema.TypeString,
									Optional: true,
								},
							},
						},
						MaxItems: 1,
					},
					"map_prop": {
						Type:     schema.TypeMap,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
					},
					"config_mode_prop": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"prop": {
									Type:     schema.TypeString,
									Optional: true,
								},
							},
						},
						ConfigMode: schema.SchemaConfigModeAttr,
					},
					"default_prop": {
						Type:     schema.TypeString,
						Default:  "default",
						Optional: true,
					},
					"conflicting_prop": {
						Type:          schema.TypeString,
						ExactlyOneOf:  []string{"foo", "conflicting_prop"},
						ConflictsWith: []string{"bar"},
						RequiredWith:  []string{"map_prop"},
						AtLeastOneOf:  []string{"nested_prop", "conflicting_prop"},
						Optional:      true,
					},
				},
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"test_data_source": {
				Schema: map[string]*schema.Schema{
					"foo": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"bar": {
						Type:     schema.TypeInt,
						Optional: true,
					},
				},
			},
		},
		Schema: map[string]*schema.Schema{
			"test_schema": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	})

	err := prov.InternalValidate()
	require.NoError(t, err)

	marshallableProv := info.MarshalProviderShim(prov)

	jsonArr, err := json.Marshal(marshallableProv)
	require.NoError(t, err)
	var out bytes.Buffer
	err = json.Indent(&out, jsonArr, "", "    ")
	require.NoError(t, err)

	autogold.Expect(`{
    "schema": {
        "test_schema": {
            "type": "String",
            "optional": true
        }
    },
    "resources": {
        "test_resource": {
            "schema": {
                "bar": {
                    "type": "Int",
                    "optional": true
                },
                "config_mode_prop": {
                    "type": "List",
                    "optional": true,
                    "element": {
                        "resource": {
                            "schema": {
                                "prop": {
                                    "type": "String",
                                    "optional": true
                                }
                            }
                        }
                    },
                    "configMode": "attr"
                },
                "conflicting_prop": {
                    "type": "String",
                    "optional": true,
                    "conflictsWith": [
                        "bar"
                    ],
                    "exactlyOneOf": [
                        "foo",
                        "conflicting_prop"
                    ],
                    "atLeastOneOf": [
                        "nested_prop",
                        "conflicting_prop"
                    ],
                    "requiredWith": [
                        "map_prop"
                    ]
                },
                "default_prop": {
                    "type": "String",
                    "optional": true,
                    "default": "default"
                },
                "foo": {
                    "type": "String",
                    "optional": true
                },
                "map_prop": {
                    "type": "Map",
                    "optional": true,
                    "element": {
                        "schema": {
                            "type": "String"
                        }
                    }
                },
                "max_item_one_prop": {
                    "type": "List",
                    "required": true,
                    "element": {
                        "resource": {
                            "schema": {
                                "nested_foo": {
                                    "type": "String",
                                    "optional": true
                                }
                            }
                        }
                    },
                    "maxItems": 1
                },
                "nested_prop": {
                    "type": "List",
                    "optional": true,
                    "element": {
                        "resource": {
                            "schema": {
                                "nested_foo": {
                                    "type": "String",
                                    "optional": true
                                }
                            }
                        }
                    }
                }
            }
        }
    },
    "dataSources": {
        "test_data_source": {
            "schema": {
                "bar": {
                    "type": "Int",
                    "optional": true
                },
                "foo": {
                    "type": "String",
                    "optional": true
                }
            }
        }
    }
}`).Equal(t, out.String())

	tokenMapping := map[string]string{
		"test_resource":    "testResource",
		"test_data_source": "testDataSource",
	}

	resBuf := &bytes.Buffer{}
	schBuf := &bytes.Buffer{}

	err = marshallableProv.GetCSVSchema("myProvider", "0.0.1", tokenMapping, schBuf, resBuf)
	require.NoError(t, err)

	autogold.Expect(`provider,version,path,type,optional,required,computed,forceNew,maxItems,minItems,deprecated,default,configMode,conflictsWith,exactlyOneOf,atLeastOneOf,requiredWith
myProvider,0.0.1,myProvider.test_schema,String,true,false,false,false,0,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_resource.bar,Int,true,false,false,false,0,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_resource.nested_prop.Elem.nested_foo,String,true,false,false,false,0,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_resource.nested_prop,List,true,false,false,false,0,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_resource.max_item_one_prop.Elem.nested_foo,String,true,false,false,false,0,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_resource.max_item_one_prop,List,false,true,false,false,1,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_resource.map_prop.Elem,String,false,false,false,false,0,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_resource.map_prop,Map,true,false,false,false,0,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_resource.config_mode_prop.Elem.prop,String,true,false,false,false,0,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_resource.config_mode_prop,List,true,false,false,false,0,0,,<nil>,attr,,,,
myProvider,0.0.1,myProvider.test_resource.default_prop,String,true,false,false,false,0,0,,default,auto,,,,
myProvider,0.0.1,myProvider.test_resource.conflicting_prop,String,true,false,false,false,0,0,,<nil>,auto,bar,"foo,conflicting_prop","nested_prop,conflicting_prop",map_prop
myProvider,0.0.1,myProvider.test_resource.foo,String,true,false,false,false,0,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_data_source.bar,Int,true,false,false,false,0,0,,<nil>,auto,,,,
myProvider,0.0.1,myProvider.test_data_source.foo,String,true,false,false,false,0,0,,<nil>,auto,,,,
`).Equal(t, schBuf.String())

	autogold.Expect(`provider,version,path,schemaVersion,token
myProvider,0.0.1,myProvider.test_resource.nested_prop.Elem,0,nested,testResource
myProvider,0.0.1,myProvider.test_resource.max_item_one_prop.Elem,0,nested,testResource
myProvider,0.0.1,myProvider.test_resource.config_mode_prop.Elem,0,nested,testResource
myProvider,0.0.1,myProvider.test_resource,0,resource,testResource
myProvider,0.0.1,myProvider.test_data_source,0,dataSource,testDataSource
`).Equal(t, resBuf.String())
}
