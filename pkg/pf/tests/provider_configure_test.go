// Copyright 2016-2023, Pulumi Corporation.
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
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
	tfpf "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func TestPFConfigure(t *testing.T) {
	t.Parallel()

	t.Run("string", testConfigurePrimitive{
		zeroValue:    cty.StringVal(""),
		nonZeroValue: cty.StringVal("a string"),
		attrOptional: schema.StringAttribute{Optional: true},
		attrRequired: schema.StringAttribute{Required: true},
	}.run)

	t.Run("bool", testConfigurePrimitive{
		zeroValue:    cty.BoolVal(false),
		nonZeroValue: cty.BoolVal(true),
		attrOptional: schema.BoolAttribute{Optional: true},
		attrRequired: schema.BoolAttribute{Required: true},
	}.run)

	t.Run("int", testConfigurePrimitive{
		zeroValue:    cty.NumberIntVal(0),
		nonZeroValue: cty.NumberIntVal(123),
		attrOptional: schema.Int64Attribute{Optional: true},
		attrRequired: schema.Int64Attribute{Required: true},
	}.run)

	t.Run("float", testConfigurePrimitive{
		zeroValue:    cty.NumberFloatVal(0),
		nonZeroValue: cty.NumberFloatVal(123.5),
		attrOptional: schema.Float64Attribute{Optional: true},
		attrRequired: schema.Float64Attribute{Required: true},
	}.run)

	t.Run("list-attribute", func(t *testing.T) {
		testConfigureCollection{
			attrOptional: func(a attr.Type) schema.Attribute {
				return schema.ListAttribute{Optional: true, ElementType: a}
			},
			makeCollection: func(v []cty.Value, elem cty.Type) cty.Value {
				if len(v) == 0 {
					return cty.ListValEmpty(elem)
				}
				return cty.ListVal(v)
			},
		}.run(t)

		t.Run("null element", func(t *testing.T) {
			t.Skip("TODO[pulumi/pulumi-terraform-bridge#2555]: Pulumi behavior does not match TF when passing a null to an array")
			crosstests.Configure(t,
				schema.Schema{Attributes: map[string]schema.Attribute{
					"k": schema.ListAttribute{Optional: true, ElementType: types.StringType},
				}},
				map[string]cty.Value{
					"k": cty.ListVal([]cty.Value{
						cty.NullVal(cty.String),
						cty.StringVal("another-value"),
					}),
				},
			)
		})
	})

	t.Run("set-attribute", testConfigureCollection{
		attrOptional: func(a attr.Type) schema.Attribute {
			return schema.SetAttribute{Optional: true, ElementType: a}
		},
		makeCollection: func(v []cty.Value, elem cty.Type) cty.Value {
			if len(v) == 0 {
				return cty.SetValEmpty(elem)
			}
			return cty.SetVal(v)
		},
	}.run)

	t.Run("map-attribute", testConfigureCollection{
		attrOptional: func(a attr.Type) schema.Attribute {
			return schema.MapAttribute{Optional: true, ElementType: a}
		},
		makeCollection: func(v []cty.Value, elem cty.Type) cty.Value {
			if len(v) == 0 {
				return cty.MapValEmpty(elem)
			}
			m := make(map[string]cty.Value, len(v))
			for i, e := range v {
				m[strconv.Itoa(i)] = e
			}
			return cty.MapVal(m)
		},
	}.run)

	t.Run("single-nested-attribute", func(t *testing.T) {
		t.Parallel()

		optionalAttrs := schema.Schema{Attributes: map[string]schema.Attribute{
			"o": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"n1": schema.StringAttribute{Optional: true},
					"n2": schema.BoolAttribute{Optional: true},
				},
				Optional: true,
			},
		}}

		t.Run("missing", crosstests.MakeConfigure(optionalAttrs,
			map[string]cty.Value{},
		))

		t.Run("empty", crosstests.MakeConfigure(optionalAttrs,
			map[string]cty.Value{"o": cty.EmptyObjectVal},
		))

		t.Run("null", crosstests.MakeConfigure(optionalAttrs,
			map[string]cty.Value{
				"o": cty.NullVal(cty.Object(map[string]cty.Type{"n1": cty.String, "n2": cty.Bool})),
			},
		))

		t.Run("full", crosstests.MakeConfigure(optionalAttrs,
			map[string]cty.Value{
				"o": cty.ObjectVal(map[string]cty.Value{
					"n1": cty.StringVal("123"),
					"n2": cty.BoolVal(false),
				}),
			},
		))

		t.Run("partial", crosstests.MakeConfigure(optionalAttrs,
			map[string]cty.Value{
				"o": cty.ObjectVal(map[string]cty.Value{
					"n1": cty.StringVal("123"),
				}),
			},
		))
	})

	t.Run("single-nested-block", func(t *testing.T) {
		t.Parallel()

		optionalAttrs := schema.Schema{Blocks: map[string]schema.Block{
			"o": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"n1": schema.StringAttribute{Optional: true},
					"n2": schema.BoolAttribute{Optional: true},
				},
			},
		}}

		t.Run("missing", crosstests.MakeConfigure(optionalAttrs,
			map[string]cty.Value{},
		))

		t.Run("empty", crosstests.MakeConfigure(optionalAttrs,
			map[string]cty.Value{"o": cty.EmptyObjectVal},
		))

		t.Run("null", func(t *testing.T) {
			t.Skip("TODO[pulumi/pulumi-terraform-bridge#2564] Null blocks don't match")
			t.Parallel()
			crosstests.Configure(t,
				optionalAttrs,
				map[string]cty.Value{
					"o": cty.NullVal(cty.Object(map[string]cty.Type{"n1": cty.String, "n2": cty.Bool})),
				},
			)
		})

		t.Run("full", crosstests.MakeConfigure(optionalAttrs,
			map[string]cty.Value{
				"o": cty.ObjectVal(map[string]cty.Value{
					"n1": cty.StringVal("123"),
					"n2": cty.BoolVal(false),
				}),
			},
		))

		t.Run("partial", crosstests.MakeConfigure(optionalAttrs,
			map[string]cty.Value{
				"o": cty.ObjectVal(map[string]cty.Value{
					"n1": cty.StringVal("123"),
				}),
			},
		))
	})
}

func TestConfigureNameOverrides(t *testing.T) {
	t.Parallel()

	t.Run("top-level", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"tf_name": schema.StringAttribute{Optional: true},
		}},
		map[string]cty.Value{
			"tf_name": cty.StringVal("my-value"),
		},
		crosstests.ConfigureProviderInfo(map[string]*info.Schema{
			"tf_name": {Name: "puName"},
		}),
	))

	t.Run("single-nested-attribute", func(t *testing.T) {
		t.Skip("TODO[pulumi/pulumi-terraform-bridge#2560]: SingleNestedAttribute nested type overrides don't line up for PF")
		t.Parallel()
		crosstests.Configure(t,
			schema.Schema{
				Attributes: map[string]schema.Attribute{
					"a": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"as": schema.Int64Attribute{Optional: true},
						},
					},
				},
			},
			map[string]cty.Value{
				"a": cty.ObjectVal(map[string]cty.Value{
					"as": cty.NumberIntVal(123),
				}),
			},
			crosstests.ConfigureProviderInfo(map[string]*info.Schema{
				"a": {
					Name: "puAttr",
					Elem: &info.Schema{Fields: map[string]*info.Schema{
						"as": {Name: "puNestedAttrField"},
					}},
				},
			}),
		)
	})

	t.Run("single-nested-block", func(t *testing.T) {
		t.Skip("TODO[pulumi/pulumi-terraform-bridge#2560]: SingleNestedBlock type overrides don't line up for PF")
		t.Parallel()
		crosstests.Configure(t,
			schema.Schema{
				Blocks: map[string]schema.Block{
					"a": schema.SingleNestedBlock{
						Attributes: map[string]schema.Attribute{
							"as": schema.Int64Attribute{Optional: true},
						},
					},
				},
			},
			map[string]cty.Value{
				"a": cty.ObjectVal(map[string]cty.Value{
					"as": cty.NumberIntVal(123),
				}),
			},
			crosstests.ConfigureProviderInfo(map[string]*info.Schema{
				"a": {
					Name: "puAttr",
					Elem: &info.Schema{Fields: map[string]*info.Schema{
						"as": {Name: "puNestedAttrField"},
					}},
				},
			}),
		)
	})

	t.Run("list-nested-block", crosstests.MakeConfigure(
		schema.Schema{
			Blocks: map[string]schema.Block{
				"b": schema.ListNestedBlock{NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"bs": schema.Float64Attribute{Optional: true},
					},
				}},
			},
		},
		map[string]cty.Value{
			"b": cty.ListVal([]cty.Value{
				cty.ObjectVal(map[string]cty.Value{
					"bs": cty.NumberFloatVal(0.5),
				}),
				cty.ObjectVal(map[string]cty.Value{
					"bs": cty.NumberFloatVal(1.5),
				}),
			}),
		},
		crosstests.ConfigureProviderInfo(map[string]*info.Schema{
			"b": {
				Name: "puBlock",
				Elem: &info.Schema{Fields: map[string]*info.Schema{
					"bs": {Name: "puListNestedBlockField"},
				}},
			},
		}),
	))
}

func TestConfigureSecrets(t *testing.T) {
	t.Parallel()
	t.Run("secret-string", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"k": schema.StringAttribute{Optional: true},
		}},
		map[string]cty.Value{"k": cty.StringVal("foo")},
		crosstests.ConfigurePulumiConfig(resource.PropertyMap{"k": resource.MakeSecret(resource.NewProperty("foo"))}),
	))
}

type testConfigureCollection struct {
	attrOptional   func(attr.Type) schema.Attribute
	makeCollection func([]cty.Value, cty.Type) cty.Value
}

func (tc testConfigureCollection) run(t *testing.T) {
	t.Parallel()
	t.Helper()

	t.Run("missing", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"k": tc.attrOptional(types.StringType),
		}},
		map[string]cty.Value{},
	))

	t.Run("empty", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"k": tc.attrOptional(types.StringType),
		}},
		map[string]cty.Value{
			"k": tc.makeCollection(nil, cty.String),
		},
	))

	t.Run("1 element", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"k": tc.attrOptional(types.StringType),
		}},
		map[string]cty.Value{
			"k": tc.makeCollection([]cty.Value{
				cty.StringVal("some-value"),
			}, cty.String),
		},
	))

	t.Run("n element", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"k": tc.attrOptional(types.StringType),
		}},
		map[string]cty.Value{
			"k": tc.makeCollection([]cty.Value{
				cty.StringVal("some-value"),
				cty.StringVal("another-value"),
			}, cty.String),
		},
	))
}

type testConfigurePrimitive struct {
	zeroValue, nonZeroValue    cty.Value
	attrOptional, attrRequired schema.Attribute
}

func (tc testConfigurePrimitive) run(t *testing.T) {
	t.Parallel()
	t.Helper()

	t.Run("zero value - optional", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"v": tc.attrOptional,
		}},
		map[string]cty.Value{"v": tc.zeroValue},
	))

	t.Run("zero value - required", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"v": tc.attrRequired,
		}},
		map[string]cty.Value{"v": tc.zeroValue},
	))
	t.Run("value - optional", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"v": tc.attrOptional,
		}},
		map[string]cty.Value{"v": tc.nonZeroValue},
	))

	t.Run("value - required", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"v": tc.attrRequired,
		}},
		map[string]cty.Value{"v": tc.nonZeroValue},
	))
}

// TestConfigureInvalidTypes tests configure for inputs that are not type-safe but that we
// expect to work.
func TestConfigureInvalidTypes(t *testing.T) {
	t.Setenv("PULUMI_DEBUG_YAML_DISABLE_TYPE_CHECKING", "true")

	t.Run("string-as-bool", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"b": schema.BoolAttribute{Optional: true},
		}},
		map[string]cty.Value{"b": cty.BoolVal(false)},
		crosstests.ConfigurePulumiConfig(resource.PropertyMap{"b": resource.NewProperty("false")}),
	))

	t.Run("string-as-int", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"b": schema.Int64Attribute{Optional: true},
		}},
		map[string]cty.Value{"b": cty.NumberIntVal(1234)},
		crosstests.ConfigurePulumiConfig(resource.PropertyMap{"b": resource.NewProperty("1234")}),
	))

	t.Run("string-as-float", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"b": schema.Float64Attribute{Optional: true},
		}},
		map[string]cty.Value{"b": cty.NumberFloatVal(1234.5)},
		crosstests.ConfigurePulumiConfig(resource.PropertyMap{"b": resource.NewProperty("1234.5")}),
	))

	t.Run("bool-as-string", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"b": schema.StringAttribute{Optional: true},
		}},
		map[string]cty.Value{"b": cty.StringVal("false")},
		crosstests.ConfigurePulumiConfig(resource.PropertyMap{"b": resource.NewProperty(false)}),
	))

	t.Run("int-as-string", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"b": schema.StringAttribute{Optional: true},
		}},
		map[string]cty.Value{"b": cty.StringVal("1234")},
		crosstests.ConfigurePulumiConfig(resource.PropertyMap{"b": resource.NewProperty(1234.0)}),
	))

	t.Run("float-as-string", crosstests.MakeConfigure(
		schema.Schema{Attributes: map[string]schema.Attribute{
			"b": schema.StringAttribute{Optional: true},
		}},
		map[string]cty.Value{"b": cty.StringVal("1234.5")},
		crosstests.ConfigurePulumiConfig(resource.PropertyMap{"b": resource.NewProperty(1234.5)}),
	))
}

// Test interaction of Configure and Create.
//
// The resource TestConfigRes will read stringConfigProp information the provider receives via Configure.
func TestConfigureToCreate(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	replay.ReplaySequence(t, server, `
	[
	  {
	    "method": "/pulumirpc.ResourceProvider/Configure",
	    "request": {
	      "args": {
		"stringConfigProp": "example"
	      }
	    },
	    "response": {
	      "supportsPreview": true,
	      "acceptResources": true,
	      "supportsAutonamingConfiguration": true
	    }
	  },
	  {
	    "method": "/pulumirpc.ResourceProvider/Create",
	    "request": {
	      "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/testres:TestConfigRes::r1",
	      "preview": false
	    },
	    "response": {
	      "id": "id-1",
	      "properties": {
		"configCopy": "example",
		"id": "id-1",
                "*": "*"
	      }
	    }
	  }
	]`)
}

func TestConfigureBooleans(t *testing.T) {
	t.Parallel()
	// Non-string properties caused trouble at some point, test booleans.
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)

	replay.Replay(t, server, `
	{
	  "method": "/pulumirpc.ResourceProvider/Configure",
	  "request": {
	    "args": {
	      "boolConfigProp": "true"
	    }
	  },
	  "response": {
	    "supportsPreview": true,
	    "acceptResources": true,
	    "supportsAutonamingConfiguration": true
	  }
	}`)
}

func TestPFConfigureErrorReplacement(t *testing.T) {
	t.Parallel()
	t.Run("replace_config_properties", func(t *testing.T) {
		errString := `some error with "config_property" and "config" but not config`
		prov := &testprovider.ConfigTestProvider{
			ConfigErr: diag.NewErrorDiagnostic(errString, errString),
			ProviderSchema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"config":          schema.StringAttribute{},
					"config_property": schema.StringAttribute{},
				},
			},
		}

		providerInfo := testprovider.SyntheticTestBridgeProvider()
		providerInfo.P = tfpf.ShimProvider(prov)
		providerInfo.Config["config_property"] = &info.Schema{Name: "configProperty"}
		providerInfo.Config["config"] = &info.Schema{Name: "CONFIG!"}

		server, err := newProviderServer(t, providerInfo)
		require.NoError(t, err)

		replay.Replay(t, server, `
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {"acceptResources": true},
			  "errors": ["some error with \"configProperty\" and \"CONFIG!\" but not config"]
			}`)
	})

	t.Run("different_error_detail_and_summary_not_dropped", func(t *testing.T) {
		errSummary := `problem with "config_property" and "config"`
		errString := `some error with "config_property" and "config" but not config`
		prov := &testprovider.ConfigTestProvider{
			ConfigErr: diag.NewErrorDiagnostic(errSummary, errString),
			ProviderSchema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"config":          schema.StringAttribute{},
					"config_property": schema.StringAttribute{},
				},
			},
		}

		providerInfo := testprovider.SyntheticTestBridgeProvider()
		providerInfo.P = tfpf.ShimProvider(prov)
		providerInfo.Config["config_property"] = &info.Schema{Name: "configProperty"}
		providerInfo.Config["config"] = &info.Schema{Name: "CONFIG!"}

		server, err := newProviderServer(t, providerInfo)
		require.NoError(t, err)

		replay.Replay(t, server, `
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {"acceptResources": true},
			  "errors": ["problem with \"configProperty\" and \"CONFIG!\": some error with \"configProperty\" and \"CONFIG!\" but not config"]
			}`)
	})
}

func TestJSONNestedConfigure(t *testing.T) {
	t.Parallel()
	p := testprovider.SyntheticTestBridgeProvider()
	server, err := newProviderServer(t, p)
	require.NoError(t, err)
	replay.Replay(t, server, `{
		  "method": "/pulumirpc.ResourceProvider/Configure",
		  "request": {
		    "args": {
                      "validateNested": "true",
                      "mapNestedProp": "{\"k1\":1,\"k2\":2}",
                      "listNestedProps": "[true,false]",
                      "singleNested": "{\"stringProp\":\"foo\",\"boolProp\":true,\"mapNestedProp\":{\"v1\":1234},\"listNestedProps\":[true,false]}",
                      "listNesteds": "[{\"stringProp\":\"foo\",\"boolProp\":true,\"mapNestedProp\":{\"v1\":1234},\"listNestedProps\":[true,false]}]"
		    }
		  },
		  "response": {
		    "supportsPreview": true,
		    "acceptResources": true,
		    "supportsAutonamingConfiguration": true
		  }
		}`)
}

func TestJSONNestedConfigureWithSecrets(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	replay.ReplaySequence(t, server, `
[
  {
    "method": "/pulumirpc.ResourceProvider/Configure",
    "request": {
      "args": {
        "stringConfigProp": {
          "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
          "value": "secret-example"
        },
        "mapNestedProp": "{\"k1\":{\"4dabf18193072939515e22adb298388d\":\"1b47061264138c4ac30d75fd1eb44270\",\"value\":1},\"k2\":2}",
        "listNestedProps": "[{\"4dabf18193072939515e22adb298388d\":\"1b47061264138c4ac30d75fd1eb44270\",\"value\":true},false]"
      }
    },
    "response": {
      "supportsPreview": true,
      "acceptResources": true,
      "supportsAutonamingConfiguration": true
    }
  },
  {
    "method": "/pulumirpc.ResourceProvider/Create",
    "request": {
      "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/testres:TestConfigRes::r1",
      "preview": false
    },
    "response": {
      "id": "id-1",
      "properties": {
        "configCopy": "secret-example",
        "id": "id-1",
        "*": "*"
      }
    }
  }
]`)
}

func TestConfigureWithSecrets(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	replay.ReplaySequence(t, server, `
[
  {
    "method": "/pulumirpc.ResourceProvider/Configure",
    "request": {
      "args": {
        "stringConfigProp": {
          "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
          "value": "secret-example"
        },
        "mapNestedProp": {
          "k1": {
            "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
            "value": 1
          },
          "k2": 2
        },
        "listNestedProps": [
          {
            "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
            "value": true
          },
          false
        ]
      }
    },
    "response": {
      "supportsPreview": true,
      "acceptResources": true,
      "supportsAutonamingConfiguration": true
    }
  },
  {
    "method": "/pulumirpc.ResourceProvider/Create",
    "request": {
      "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/testres:TestConfigRes::r1",
      "preview": false
    },
    "response": {
      "id": "id-1",
      "properties": {
        "configCopy": "secret-example",
        "id": "id-1",
        "*": "*"
      }
    }
  }
]`)
}
