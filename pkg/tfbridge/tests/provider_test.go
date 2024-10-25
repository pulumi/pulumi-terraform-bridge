// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and

package tfbridgetests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestIntToStringOverride(t *testing.T) {
	t.Parallel()

	const largeInt = math.MaxInt64 / 2

	crosstests.Create(t,
		map[string]*schema.Schema{
			"f0": {Required: true, Type: schema.TypeInt},
		},
		cty.ObjectVal(map[string]cty.Value{
			"f0": cty.NumberIntVal(largeInt),
		}),
		resource.PropertyMap{
			"f0": resource.NewProperty(strconv.FormatInt(largeInt, 10)),
		},
		crosstests.CreateResourceInfo(info.Resource{Fields: map[string]*info.Schema{
			"f0": {Type: "string"},
		}}),
	)
}

// Regression test for [pulumi/pulumi-terraform-bridge#1762]
func TestInputsConfigModeEqual(t *testing.T) {
	t.Parallel()

	emptyConfig := cty.ObjectVal(map[string]cty.Value{})

	emptyListConfig := cty.ObjectVal(map[string]cty.Value{
		"f0": cty.ListValEmpty(cty.Object(map[string]cty.Type{
			"x": cty.String,
		})),
	})

	nonEmptyConfig := cty.ObjectVal(map[string]cty.Value{
		"f0": cty.ListVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"x": cty.StringVal("val"),
			}),
		}),
	})

	for _, tc := range []struct {
		name       string
		config     cty.Value
		maxItems   int
		configMode schema.SchemaConfigMode
	}{
		{"MaxItems: 0, ConfigMode: Auto, Empty", emptyConfig, 0, schema.SchemaConfigModeAuto},
		{"MaxItems: 0, ConfigMode: Auto, EmptyList", emptyListConfig, 0, schema.SchemaConfigModeAuto},
		{"MaxItems: 0, ConfigMode: Auto, NonEmpty", nonEmptyConfig, 0, schema.SchemaConfigModeAuto},
		{"MaxItems: 0, ConfigMode: Block, Empty", emptyConfig, 0, schema.SchemaConfigModeBlock},
		{"MaxItems: 0, ConfigMode: Block, EmptyList", emptyListConfig, 0, schema.SchemaConfigModeBlock},
		{"MaxItems: 0, ConfigMode: Block, NonEmpty", nonEmptyConfig, 0, schema.SchemaConfigModeBlock},
		{"MaxItems: 0, ConfigMode: Attr, Empty", emptyConfig, 0, schema.SchemaConfigModeAttr},
		{"MaxItems: 0, ConfigMode: Attr, EmptyList", emptyListConfig, 0, schema.SchemaConfigModeAttr},
		{"MaxItems: 0, ConfigMode: Attr, NonEmpty", nonEmptyConfig, 0, schema.SchemaConfigModeAttr},
		{"MaxItems: 1, ConfigMode: Auto, Empty", emptyConfig, 1, schema.SchemaConfigModeAuto},
		{"MaxItems: 1, ConfigMode: Auto, EmptyList", emptyListConfig, 1, schema.SchemaConfigModeAuto},
		{"MaxItems: 1, ConfigMode: Auto, NonEmpty", nonEmptyConfig, 1, schema.SchemaConfigModeAuto},
		{"MaxItems: 1, ConfigMode: Block, Empty", emptyConfig, 1, schema.SchemaConfigModeBlock},
		{"MaxItems: 1, ConfigMode: Block, EmptyList", emptyListConfig, 1, schema.SchemaConfigModeBlock},
		{"MaxItems: 1, ConfigMode: Block, NonEmpty", nonEmptyConfig, 1, schema.SchemaConfigModeBlock},
		{"MaxItems: 1, ConfigMode: Attr, Empty", emptyConfig, 1, schema.SchemaConfigModeAttr},
		// TODO[pulumi/pulumi-terraform-bridge#2025]
		// This is not expressible in pulumi after the ConfigModeOne flattening.
		// {"MaxItems: 1, ConfigMode: Attr, EmptyList", emptyListConfig, 1, schema.SchemaConfigModeAttr},
		{"MaxItems: 1, ConfigMode: Attr, NonEmpty", nonEmptyConfig, 1, schema.SchemaConfigModeAttr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			crosstests.Create(t,
				map[string]*schema.Schema{
					"f0": {
						Optional:   true,
						Type:       schema.TypeList,
						MaxItems:   tc.maxItems,
						ConfigMode: tc.configMode,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"x": {Optional: true, Type: schema.TypeString},
							},
						},
					},
				},
				tc.config,
				crosstests.InferPulumiValue(),
			)
		})
	}
}

// TestStateFunc ensures that resources with a StateFunc set on their schema are correctly
// handled. This includes ensuring that the PlannedPrivate blob is passed from
// PlanResourceChange to ApplyResourceChange. If this is passed correctly, the provider
// will see the original value of the field, rather than the value that was produced by
// the StateFunc.
func TestStateFunc(t *testing.T) {
	t.Parallel()
	crosstests.Create(t,
		map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				StateFunc: func(v interface{}) string {
					return v.(string) + " world"
				},
			},
		},
		cty.ObjectVal(map[string]cty.Value{
			"test": cty.StringVal("hello"),
		}),
		crosstests.InferPulumiValue(),
	)
}

// Regression test for [pulumi/pulumi-terraform-bridge#1767]
func TestInputsUnspecifiedMaxItemsOne(t *testing.T) {
	t.Parallel()
	crosstests.Create(t,
		map[string]*schema.Schema{
			"f0": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"x": {Optional: true, Type: schema.TypeString},
					},
				},
			},
		},
		cty.ObjectVal(map[string]cty.Value{}),
		crosstests.InferPulumiValue(),
	)
}

// Regression test for [pulumi/pulumi-terraform-bridge#1970] and [pulumi/pulumi-terraform-bridge#1964]
func TestOptionalSetNotSpecified(t *testing.T) {
	t.Parallel()
	crosstests.Create(t,
		map[string]*schema.Schema{
			"f0": {
				Optional: true,
				Type:     schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"x": {Optional: true, Type: schema.TypeString},
					},
				},
			},
		},
		cty.ObjectVal(map[string]cty.Value{}),
		crosstests.InferPulumiValue(),
	)
}

// Regression test for [pulumi/pulumi-terraform-bridge#1915]
func TestInputsEqualEmptyList(t *testing.T) {
	t.Parallel()
	for _, maxItems := range []int{0, 1} {
		t.Run(fmt.Sprintf("MaxItems: %v", maxItems), func(t *testing.T) {
			crosstests.Create(t,
				map[string]*schema.Schema{
					"f0": {
						Optional: true,
						Type:     schema.TypeList,
						MaxItems: maxItems,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"x": {Optional: true, Type: schema.TypeString},
							},
						},
					},
				},
				cty.ObjectVal(map[string]cty.Value{
					"f0": cty.ListValEmpty(cty.String),
				}),
				crosstests.InferPulumiValue(),
			)
		})
	}
}

// TestAccCloudWatch failed with PlanResourceChange to do a simple Create preview because the state upgrade was
// unexpectedly called with nil state. Emulate this here to test it does not fail.
func TestCreateDoesNotPanicWithStateUpgraders(t *testing.T) {
	t.Parallel()
	resourceRuleV0 := func() *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"event_bus_name": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"is_enabled": {
					Type:     schema.TypeBool,
					Optional: true,
				},
			},
		}
	}

	resourceRuleUpgradeV0 := func(ctx context.Context, rawState map[string]any, meta any) (map[string]any, error) {
		if rawState == nil {
			rawState = map[string]any{}
		}

		if rawState["is_enabled"].(bool) { // used to panic here
			t.Logf("enabled")
		} else {
			t.Logf("disabled")
		}

		return rawState, nil
	}

	crosstests.Create(t,
		resourceRuleV0().Schema,
		cty.ObjectVal(map[string]cty.Value{
			"event_bus_name": cty.StringVal("default"),
		}),
		crosstests.InferPulumiValue(),
		crosstests.CreateStateUpgrader(1, []schema.StateUpgrader{
			{
				Type:    resourceRuleV0().CoreConfigSchema().ImpliedType(),
				Upgrade: resourceRuleUpgradeV0,
				Version: 0,
			},
		}),
	)
}

func TestTimeouts(t *testing.T) {
	t.Parallel()
	crosstests.Create(t,
		map[string]*schema.Schema{
			"tags": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Optional: true,
					Type:     schema.TypeString,
				},
			},
		},
		cty.EmptyObjectVal,
		crosstests.InferPulumiValue(),
		crosstests.CreateTimeout(&schema.ResourceTimeout{
			Create: schema.DefaultTimeout(time.Duration(120)),
		}),
	)
}

func TestMap(t *testing.T) {
	t.Parallel()

	crosstests.Create(t,
		map[string]*schema.Schema{
			"tags": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Optional: true,
					Type:     schema.TypeString,
				},
			},
		},
		cty.ObjectVal(map[string]cty.Value{
			"tags": cty.MapVal(map[string]cty.Value{
				"key":  cty.StringVal("val"),
				"key2": cty.StringVal("val2"),
			}),
		}),
		crosstests.InferPulumiValue(),
	)
}

func TestEmptySetOfEmptyObjects(t *testing.T) {
	t.Parallel()

	crosstests.Create(t,
		map[string]*schema.Schema{
			"d3f0": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Resource{Schema: map[string]*schema.Schema{}},
			},
		},
		cty.ObjectVal(map[string]cty.Value{
			"d3f0": cty.SetValEmpty(cty.EmptyObject),
		}),
		crosstests.InferPulumiValue(),
	)
}

func TestInputsEmptyString(t *testing.T) {
	t.Parallel()

	crosstests.Create(t,
		map[string]*schema.Schema{
			"f0": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
		cty.ObjectVal(map[string]cty.Value{
			"f0": cty.StringVal(""),
		}),
		crosstests.InferPulumiValue(),
	)
}

func TestInputsNesqtedBlocksEmpty(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		typ1   schema.ValueType
		typ2   schema.ValueType
		config cty.Value
	}{
		{"empty list list block", schema.TypeList, schema.TypeList, cty.EmptyObjectVal},
		{"empty set set block", schema.TypeSet, schema.TypeSet, cty.EmptyObjectVal},
		{"empty list set block", schema.TypeList, schema.TypeSet, cty.EmptyObjectVal},
		{"non empty list list block", schema.TypeList, schema.TypeList, cty.ObjectVal(map[string]cty.Value{
			"f0": cty.ListValEmpty(cty.List(cty.Object(map[string]cty.Type{"f2": cty.String}))),
		})},
		{"nested non empty list list block", schema.TypeList, schema.TypeList, cty.ObjectVal(map[string]cty.Value{
			"f0": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"f2": cty.StringVal("val")})}),
		})},
		{"nested non empty set set block", schema.TypeSet, schema.TypeSet, cty.ObjectVal(map[string]cty.Value{
			"f0": cty.SetVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"f2": cty.StringVal("val")})}),
		})},
	} {
		t.Run(tc.name, crosstests.MakeCreate(
			map[string]*schema.Schema{
				"f0": {
					Type:     tc.typ1,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"f1": {
								Type:     tc.typ2,
								Optional: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"x": {Optional: true, Type: schema.TypeString},
									},
								},
							},
							// This allows us to specify non-empty f0s with an empty f1
							"f2": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
			tc.config,
			crosstests.InferPulumiValue(),
		))
	}
}

func TestExplicitNilList(t *testing.T) {
	t.Parallel()

	// This is an explicit null on the tf side:
	// resource "crossprovider_testres" "example" {
	//     f0 = null
	// }
	crosstests.Create(t,
		map[string]*schema.Schema{
			"f0": {
				Optional: true,
				Type:     schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
					Elem: &schema.Schema{
						Type: schema.TypeInt,
					},
				},
			},
		},
		cty.ObjectVal(map[string]cty.Value{"f0": cty.NullVal(cty.List(cty.Map(cty.Number)))}),
		crosstests.InferPulumiValue(),
	)
}

func TestInputsEmptyCollections(t *testing.T) {
	t.Parallel()

	// signifies a block
	resourceElem := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"x": {Optional: true, Type: schema.TypeString},
		},
	}

	// signifies an attribute
	schemaElem := &schema.Schema{
		Type: schema.TypeMap,
		Elem: &schema.Schema{Type: schema.TypeString},
	}

	for _, tc := range []struct {
		name       string
		maxItems   int
		typ        schema.ValueType
		elem       any
		configMode schema.SchemaConfigMode
	}{
		{"list block", 0, schema.TypeList, resourceElem, schema.SchemaConfigModeAuto},
		{"set block", 0, schema.TypeSet, resourceElem, schema.SchemaConfigModeAuto},
		// TypeMap with Elem *Resource not supported
		// {"map block", 0, schema.TypeMap, resourceElem, schema.SchemaConfigModeAuto},
		{"list max items one block", 1, schema.TypeList, resourceElem, schema.SchemaConfigModeAuto},
		{"set max items one block", 1, schema.TypeSet, resourceElem, schema.SchemaConfigModeAuto},
		// MaxItems is only valid on lists and sets
		// {"map max items one block", 1, schema.TypeMap, resourceElem, schema.SchemaConfigModeAuto},
		{"list attr", 0, schema.TypeList, schemaElem, schema.SchemaConfigModeAuto},
		{"set attr", 0, schema.TypeSet, schemaElem, schema.SchemaConfigModeAuto},
		{"map attr", 0, schema.TypeMap, schemaElem, schema.SchemaConfigModeAuto},
		{"list max items one attr", 1, schema.TypeList, schemaElem, schema.SchemaConfigModeAuto},
		{"set max items one attr", 1, schema.TypeSet, schemaElem, schema.SchemaConfigModeAuto},
		// MaxItems is only valid on lists and sets
		// {"map max items one attr", 1, schema.TypeMap, schemaElem, schema.SchemaConfigModeAuto},
		{"list config mode attr", 0, schema.TypeList, resourceElem, schema.SchemaConfigModeAttr},
		{"set config mode attr", 0, schema.TypeSet, resourceElem, schema.SchemaConfigModeAttr},
	} {
		t.Run(tc.name, crosstests.MakeCreate(
			map[string]*schema.Schema{
				"f0": {
					Type:       tc.typ,
					MaxItems:   tc.maxItems,
					Elem:       tc.elem,
					ConfigMode: tc.configMode,
					Optional:   true,
				},
			},
			cty.EmptyObjectVal,
			crosstests.InferPulumiValue(),
		))
	}
}

// Demonstrating the use of the newTestProvider helper.
func TestWithNewTestProvider(t *testing.T) {
	ctx := context.Background()
	p := newTestProvider(ctx, tfbridge.ProviderInfo{
		P: shimv2.NewProvider(&schema.Provider{
			Schema: map[string]*schema.Schema{},
			ResourcesMap: map[string]*schema.Resource{
				"example_resource": {
					Schema: map[string]*schema.Schema{
						"array_property_values": {
							Type: schema.TypeList,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
							Optional: true,
						},
					},
				},
			},
		}),
		Name:           "testprov",
		ResourcePrefix: "example",
		Resources: map[string]*tfbridge.ResourceInfo{
			"example_resource": {Tok: "testprov:index:ExampleResource"},
		},
	}, newTestProviderOptions{})

	replay.Replay(t, p, `
	{
	  "method": "/pulumirpc.ResourceProvider/Check",
	  "request": {
	    "urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres",
	    "randomSeed": "ZCiVOcvG/CT5jx4XriguWgj2iMpQEb8P3ZLqU/AS2yg=",
	    "olds": {
	      "__defaults": []
	    },
	    "news": {
	      "arrayPropertyValues": []
	    }
	  },
	  "response": {
	    "inputs": {
	      "__defaults": [],
	      "arrayPropertyValues": []
	    }
	  }
	}
	`)
}

// TestRegress1932 tests that we can have a list with different types (string & unknown)
func TestRegress1932(t *testing.T) {
	ctx := context.Background()
	p := newTestProvider(ctx, tfbridge.ProviderInfo{
		P: shimv2.NewProvider(&schema.Provider{
			Schema: map[string]*schema.Schema{},
			ResourcesMap: map[string]*schema.Resource{
				"aws_launch_template": {
					Schema: map[string]*schema.Schema{
						"tag_specifications": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"tags": {
										Type:     schema.TypeMap,
										Optional: true,
										Elem:     &schema.Schema{Type: schema.TypeString},
									},
								},
							},
						},
					},
				},
			},
		}, shimv2.WithPlanResourceChange(func(s string) bool {
			return true
		})),
		Name:           "aws",
		ResourcePrefix: "example",
		Resources: map[string]*tfbridge.ResourceInfo{
			"aws_launch_template": {Tok: "aws:ec2/launchTemplate:LaunchTemplate"},
		},
	}, newTestProviderOptions{})

	replay.Replay(t, p, `
	{
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::pulumi-go-app::aws:ec2/launchTemplate:LaunchTemplate::launch-template",
	    "properties": {
	      "__defaults": [ ],
	      "tagSpecifications": [
	        {
	          "__defaults": [],
	          "tags": {
	            "Name": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
	          }
	        },
	        {
	          "__defaults": [],
	          "tags": {
	            "Name": "Bucket Arn"
	          }
	        }
	      ]
	    },
	    "preview": true
	  },
	"response": {
		"properties": {
		"id": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
		"tagSpecifications": [
			{
			"tags": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
			},
			{
			"tags": {
				"Name": "Bucket Arn"
			}
			}
		]
		}
	},
	  "metadata": {
	    "kind": "resource",
	    "mode": "client",
	    "name": "aws"
	  }
	}

		`)
}

func TestReproMinimalDiffCycle(t *testing.T) {
	customResponseSchema := func() *schema.Schema {
		return &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"custom_response_body_key": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		}
	}
	blockConfigSchema := func() *schema.Schema {
		return &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"custom_response": customResponseSchema(),
				},
			},
		}
	}
	ruleElement := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"action": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"block": blockConfigSchema(),
					},
				},
			},
		},
	}

	resource := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"rule": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     ruleElement,
			},
		},
	}

	// Here i may receive maps or slices over base types and *schema.Set which is not friendly to diffing.
	resource.Schema["rule"].Set = func(i interface{}) int {
		actual := schema.HashResource(resource.Schema["rule"].Elem.(*schema.Resource))(i)
		fmt.Printf("hashing %#v as %d\n", i, actual)
		return actual
	}
	ctx := context.Background()
	p := newTestProvider(ctx, tfbridge.ProviderInfo{
		P: shimv2.NewProvider(&schema.Provider{
			Schema: map[string]*schema.Schema{},
			ResourcesMap: map[string]*schema.Resource{
				"example_resource": resource,
			},
		}, shimv2.WithPlanResourceChange(func(tfResourceType string) bool {
			return true
		})),
		Name:           "testprov",
		ResourcePrefix: "example",
		Resources: map[string]*tfbridge.ResourceInfo{
			"example_resource": {Tok: "testprov:index:ExampleResource"},
		},
	}, newTestProviderOptions{})

	replay.Replay(t, p, `
	{
	  "method": "/pulumirpc.ResourceProvider/Diff",
	  "request": {
	    "id": "newid",
	    "urn": "urn:pulumi:test::project::testprov:index:ExampleResource::example",
	    "olds": {
	      "id": "newid",
	      "rules": [
		{
		  "action": {
		    "block": {
		      "customResponse": null
		    }
		  }
		}
	      ]
	    },
	    "news": {
	      "__defaults": [],
	      "rules": [
		{
		  "__defaults": [],
		  "action": {
		    "__defaults": [],
		    "block": {
		      "__defaults": []
		    }
		  }
		}
	      ]
	    },
	    "oldInputs": {
	      "__defaults": [],
	      "rules": [
		{
		  "__defaults": [],
		  "action": {
		    "__defaults": [],
		    "block": {
		      "__defaults": []
		    }
		  }
		}
	      ]
	    }
	  },
	  "response": {
	    "changes": "DIFF_NONE",
	    "hasDetailedDiff": true
	  }
	}`)
}

func TestValidateConfig(t *testing.T) {
	ctx := context.Background()
	p := newTestProvider(ctx, tfbridge.ProviderInfo{
		P: shimv2.NewProvider(&schema.Provider{
			Schema: map[string]*schema.Schema{
				"endpoints": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"abcd": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
		}, shimv2.WithDiffStrategy(shimv2.PlanState)),
		Name:           "testprov",
		ResourcePrefix: "example",
	}, newTestProviderOptions{})

	t.Run("type_check_error", func(t *testing.T) {
		t.Setenv("PULUMI_ERROR_CONFIG_TYPE_CHECKER", "true")
		replay.ReplaySequence(t, p, `
	[
	{
		"method": "/pulumirpc.ResourceProvider/CheckConfig",
		"request": {
			"urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres",
			"olds": { },
			"news": {
				"endpoints": "[{\"wxyz\":\"http://localhost:4566\"}]",
				"version": "6.35.0"
			}
		},
		"response": {
			"failures": [
			{
				"reason": "an unexpected argument \"wxyz\" was provided. Examine values at 'exres.endpoints[0]'."
			}
			]
		},
		"metadata": {
			"kind": "resource",
			"mode": "client",
			"name": "aws"
		}
	}
	]
	`)
	})
}

// Assert that passing strings into fields of boolean type triggers a type-checking error message, even if some of the
// resource inputs are unknown. See also: https://github.com/pulumi/pulumi-aws/issues/4342
func TestTypeCheckingMistypedBooleansWithUnknowns(t *testing.T) {
	t.Setenv("PULUMI_ERROR_TYPE_CHECKER", "true")
	ctx := context.Background()
	resourceID := "aws_ecs_service"
	resourceSchema := map[string]*schema.Schema{
		"network_configuration": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"assign_public_ip": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  false,
					},
					"subnets": {
						Type:     schema.TypeSet,
						Required: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
				},
			},
		},
	}

	res := &schema.Resource{
		Schema: resourceSchema,
	}

	resMap := map[string]*schema.Resource{resourceID: res}

	schemaProvider := &schema.Provider{ResourcesMap: resMap}

	p := newTestProvider(ctx, tfbridge.ProviderInfo{
		P:              shimv2.NewProvider(schemaProvider),
		Name:           "aws",
		ResourcePrefix: "aws",
		Resources: map[string]*info.Resource{
			resourceID: {
				Tok: "aws:ecs/service:Service",
			},
		},
	}, newTestProviderOptions{})

	reason := `expected boolean type, got \"DISABLED\" of type string. ` +
		"Examine values at 'my-ecs-service.networkConfiguration.assignPublicIp'."

	// networkConfiguration.assignPublicIp has the wrong type intentionally.
	replay.ReplaySequence(t, p, fmt.Sprintf(`
	[
	  {
	    "method": "/pulumirpc.ResourceProvider/Check",
	    "request": {
	      "urn": "urn:pulumi:dev::aws-4342::aws:ecs/service:Service::my-ecs-service",
	      "olds": {},
	      "news": {
		"networkConfiguration": {
		  "assignPublicIp": "DISABLED",
		  "subnets": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
		}
	      },
	      "randomSeed": "7WaseITzLnMm7TGBDCYIbSUvAatQKt0rkmDuHUXxR9U="
	    },
	    "response": {
              "inputs": "*",
              "failures": [{"reason": "%s"}]
	    }
	  }
	]`, reason))
}

func nilSink() diag.Sink {
	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	})
	return nilSink
}

// Variation of NewProvider to facilitate white-box testing.
func newTestProvider(
	ctx context.Context,
	info tfbridge.ProviderInfo,
	opts newTestProviderOptions,
) pulumirpc.ResourceProviderServer {
	if opts.version == "" {
		opts.version = "0.0.1"
	}
	if opts.module == "" {
		opts.module = "testprovier"
	}

	var schemaBytes []byte

	if !opts.noSchema {
		packageSpec, err := tfgen.GenerateSchema(info, nilSink())
		contract.AssertNoErrorf(err, "Failed to generate a schema for the test provider")
		bytes, err := json.Marshal(packageSpec)
		contract.AssertNoErrorf(err, "Failed to marshal a schema for the test provider")
		schemaBytes = bytes
	}

	return tfbridge.NewProvider(ctx, nil, opts.module, opts.version, info.P, info, schemaBytes)
}

type newTestProviderOptions struct {
	module   string
	version  string
	noSchema bool
}
