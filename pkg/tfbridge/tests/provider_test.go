package tfbridgetests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

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

func TestValidateInputsPanic(t *testing.T) {
	ctx := context.Background()
	p := newTestProvider(ctx, tfbridge.ProviderInfo{
		P: shimv2.NewProvider(&schema.Provider{
			Schema: map[string]*schema.Schema{},
			ResourcesMap: map[string]*schema.Resource{
				"example_resource": {
					Schema: map[string]*schema.Schema{
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
									"security_groups": {
										Type:     schema.TypeSet,
										Optional: true,
										Elem:     &schema.Schema{Type: schema.TypeString},
									},
									"subnets": {
										Type:     schema.TypeSet,
										Required: true,
										Elem:     &schema.Schema{Type: schema.TypeString},
									},
								},
							},
						},
					},
				},
			},
		}, shimv2.WithDiffStrategy(shimv2.PlanState)),
		Name:           "testprov",
		ResourcePrefix: "example",
		Resources: map[string]*tfbridge.ResourceInfo{
			"example_resource": {Tok: "testprov:index:ExampleResource"},
		},
	}, newTestProviderOptions{})

	t.Run("diff_panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Expected diff to panic")
			}
		}()
		replay.ReplaySequence(t, p, `
	[
		{
			"method": "/pulumirpc.ResourceProvider/Diff",
			"request": {
				"urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres",
				"olds": {
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				},
				"news": {
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				}
			},
			"response": {
			}
		}
	]
	`)
	})

	t.Run("diff_no_panic", func(t *testing.T) {
		replay.ReplaySequence(t, p, `
	[
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres",
				"randomSeed": "ZCiVOcvG/CT5jx4XriguWgj2iMpQEb8P3ZLqU/AS2yg=",
				"olds": {
					"__defaults": []
				},
				"news": {
					"networkConfiguration": {
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				}
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				}
			}
		},
		{
			"method": "/pulumirpc.ResourceProvider/Diff",
			"request": {
				"urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres",
				"olds": {
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				},
				"news": {
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				}
			},
			"response": {
			},
			"errors": [
			"diffing urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres: panicked: \"value has no attribute of that name\""

			]
		}
	]
	`)
	})

	t.Run("update_panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Expected update to panic")
			}
		}()
		replay.ReplaySequence(t, p, `
	[
		{
			"method": "/pulumirpc.ResourceProvider/Update",
			"request": {
				"urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres1",
				"olds": {
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				},
				"news": {
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				},
				"preview": true
			},
			"response": {
			}
		}
	]
	`)

	})

	t.Run("update_no_panic", func(t *testing.T) {
		replay.ReplaySequence(t, p, `
	[
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres2",
				"randomSeed": "ZCiVOcvG/CT5jx4XriguWgj2iMpQEb8P3ZLqU/AS2yg=",
				"olds": {
					"__defaults": []
				},
				"news": {
					"networkConfiguration": {
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				}
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				}
			}
		},
		{
			"method": "/pulumirpc.ResourceProvider/Update",
			"request": {
				"urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres2",
				"olds": {
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				},
				"news": {
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				},
				"preview": true
			},
			"response": {
			},
			"errors": [
			"diffing urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres2: panicked: \"value has no attribute of that name\""
			]
		}
	]
	`)

	})

	t.Run("create_no_panic", func(t *testing.T) {
		replay.ReplaySequence(t, p, `
	[
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres3",
				"randomSeed": "ZCiVOcvG/CT5jx4XriguWgj2iMpQEb8P3ZLqU/AS2yg=",
				"olds": {
					"__defaults": []
				},
				"news": {
					"networkConfiguration": {
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				}
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				}
			}
		},
		{
			"method": "/pulumirpc.ResourceProvider/Create",
			"request": {
				"urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres3",
				"properties": {
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				},
				"preview": true
			},
			"response": {
			},
			"errors": [
			"diffing urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres3: panicked: \"value has no attribute of that name\""
			]
		}
	]
	`)
	})

	t.Run("create_panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Expected create to panic")
			}
		}()
		replay.ReplaySequence(t, p, `
	[
		{
			"method": "/pulumirpc.ResourceProvider/Create",
			"request": {
				"urn": "urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres4",
				"properties": {
					"networkConfiguration": {
						"__defaults": [
						"assignPublicIp"
						],
						"assignPublicIp": false,
						"securityGroups": [
						"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
						],
						"subnets": "[\"first\",\"second\"]"
					}
				},
				"preview": true
			},
			"response": {
			}
		}
	]
	`)
	})

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
