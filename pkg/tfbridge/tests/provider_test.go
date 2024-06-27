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
	"github.com/stretchr/testify/assert"

	"bytes"
	"fmt"
	"github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"hash/crc32"
)

func TestRegressAws2095(t *testing.T) {
	diffCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Diff",
	  "request": {
	    "id": "ERDY5Q9QSIK29",
	    "urn": "urn:pulumi:aws-::cloudfront-diff::aws:cloudfront/distribution:Distribution::myDistribution",
	    "olds": {
	      "__meta": "{\"schema_version\":\"1\"}",
	      "arn": "arn:aws:cloudfront::616138583583:distribution/ERDY5Q9QSIK29",
	      "id": "ERDY5Q9QSIK29",
	      "origins": [
		{
		  "connectionAttempts": 3,
		  "connectionTimeout": 10,
		  "domainName": "mybucket.s3.amazonaws.com",
		  "originAccessControlId": "",
		  "originId": "myS3Origin",
		  "originPath": ""
		}
	      ]
	    },
	    "news": {
	      "__defaults": [],
	      "origins": [
		{
		  "__defaults": [
		    "originPath"
		  ],
		  "connectionAttempts": 2,
		  "connectionTimeout": 10,
		  "domainName": "mybucket.s3.amazonaws.com",
		  "originId": "myS3Origin",
		  "originPath": ""
		}
	      ]
	    },
	    "oldInputs": {
	      "__defaults": [],
	      "origins": [
		{
		  "__defaults": [
		    "originPath"
		  ],
		  "connectionAttempts": 3,
		  "connectionTimeout": 10,
		  "domainName": "mybucket.s3.amazonaws.com",
		  "originId": "myS3Origin",
		  "originPath": ""
		}
	      ]
	    }
	  },
	  "response": {
	    "changes": "DIFF_SOME",
	    "diffs": [
	      "origins",
	      "origins",
	      "origins",
	      "origins"
	    ],
	    "detailedDiff": {
	      "origins[0].connectionAttempts": {
		"kind": "UPDATE"
	      },
	      "origins[0].connectionTimeout": {
		"kind": "UPDATE"
	      },
	      "origins[0].domainName": {
		"kind": "UPDATE"
	      },
	      "origins[0].originId": {
		"kind": "UPDATE"
	      }
	    },
	    "hasDetailedDiff": true
	  }
	}`

	stringHashcode := func(s string) int {
		v := int(crc32.ChecksumIEEE([]byte(s)))
		if v >= 0 {
			return v
		}
		if -v >= 0 {
			return -v
		}
		// v == MinInt
		return 0
	}

	originHash := func(v interface{}) int {
		var buf bytes.Buffer
		m := v.(map[string]interface{})
		buf.WriteString(fmt.Sprintf("%s-", m["origin_id"].(string)))
		buf.WriteString(fmt.Sprintf("%s-", m["domain_name"].(string)))
		if v, ok := m["connection_attempts"]; ok {
			buf.WriteString(fmt.Sprintf("%d-", v.(int)))
		}
		if v, ok := m["connection_timeout"]; ok {
			buf.WriteString(fmt.Sprintf("%d-", v.(int)))
		}
		if v, ok := m["origin_access_control_id"]; ok {
			buf.WriteString(fmt.Sprintf("%s-", v.(string)))
		}
		if v, ok := m["origin_path"]; ok {
			buf.WriteString(fmt.Sprintf("%s-", v.(string)))
		}
		return stringHashcode(buf.String())
	}

	cloudFrontSchema := map[string]*schema.Schema{
		"origin": {
			Type:     schema.TypeSet,
			Required: true,
			Set:      originHash,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"connection_attempts": {
						Type:     schema.TypeInt,
						Optional: true,
						Default:  3,
						//ValidateFunc: validation.IntBetween(1, 3),
					},
					"connection_timeout": {
						Type:     schema.TypeInt,
						Optional: true,
						Default:  10,
						//ValidateFunc: validation.IntBetween(1, 10),
					},
					"domain_name": {
						Type:     schema.TypeString,
						Required: true,
						//ValidateFunc: validation.NoZeroValues,
					},
					"origin_access_control_id": {
						Type:     schema.TypeString,
						Optional: true,
						//ValidateFunc: validation.NoZeroValues,
					},
					"origin_id": {
						Type:     schema.TypeString,
						Required: true,
						//ValidateFunc: validation.NoZeroValues,
					},
					"origin_path": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "",
					},
				},
			},
		},
	}

	ctx := context.Background()
	p := newTestProvider(ctx, tfbridge.ProviderInfo{
		P: shimv2.NewProvider(&schema.Provider{
			Schema: map[string]*schema.Schema{},
			ResourcesMap: map[string]*schema.Resource{
				"aws_cloudfront_distribution": {
					Schema: cloudFrontSchema,
				},
			},
		}),
		Name:           "aws",
		ResourcePrefix: "aws",
		Resources: map[string]*tfbridge.ResourceInfo{
			"aws_cloudfront_distribution": {Tok: "aws:cloudfront/distribution:Distribution"},
		},
	}, newTestProviderOptions{})

	replay.Replay(t, p, diffCase)
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
		assert.Panics(t, func() {
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
	})

	t.Run("diff_no_panic", func(t *testing.T) {
		replay.ReplaySequence(t, p, fmt.Sprintf(`
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
			"%s"
			]
		}
		]
		`, "diffing urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres: "+
			`panicked: \"value has no attribute of that name\"`))
	})

	t.Run("update_panic", func(t *testing.T) {
		assert.Panics(t, func() {
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
	})

	t.Run("update_no_panic", func(t *testing.T) {
		replay.ReplaySequence(t, p, fmt.Sprintf(`
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
			"%s"
			]
		}
	]
	`, "diffing urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres2: "+
			`panicked: \"value has no attribute of that name\"`))

	})

	t.Run("create_no_panic", func(t *testing.T) {
		replay.ReplaySequence(t, p, fmt.Sprintf(`
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
			"%s"
			]
		}
	]
	`, "diffing urn:pulumi:dev::teststack::testprov:index:ExampleResource::exres3: "+
			`panicked: \"value has no attribute of that name\"`))
	})

	t.Run("create_panic", func(t *testing.T) {
		assert.Panics(t, func() {

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
	})
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
