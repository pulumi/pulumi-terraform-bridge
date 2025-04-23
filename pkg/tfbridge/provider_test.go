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

package tfbridge

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	schemav2 "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pkg/errors"
	testutils "github.com/pulumi/providertest/replay"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	hostclient "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	pdiag "github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/reservedkeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestConvertStringToPropertyValue(t *testing.T) {
	t.Parallel()
	type testcase struct {
		str      string
		typ      shim.ValueType
		expected interface{}
	}

	cases := []testcase{
		{
			typ:      shim.TypeBool,
			expected: false,
		},
		{
			str:      "false",
			typ:      shim.TypeBool,
			expected: false,
		},
		{
			str:      "true",
			typ:      shim.TypeBool,
			expected: true,
		},
		{
			str: "root",
			typ: shim.TypeBool,
		},

		{
			typ:      shim.TypeString,
			expected: "",
		},
		{
			str:      "stringP",
			typ:      shim.TypeString,
			expected: "stringP",
		},

		{
			typ:      shim.TypeInt,
			expected: 0,
		},
		{
			str:      "42",
			typ:      shim.TypeInt,
			expected: 42,
		},
		{
			str: "root",
			typ: shim.TypeInt,
		},

		{
			typ:      shim.TypeFloat,
			expected: 0,
		},
		{
			str:      "42",
			typ:      shim.TypeFloat,
			expected: 42,
		},
		{
			str: "root",
			typ: shim.TypeFloat,
		},

		{
			typ:      shim.TypeList,
			expected: []interface{}{},
		},
		{
			str:      "[ \"foo\", \"bar\" ]",
			typ:      shim.TypeList,
			expected: []interface{}{"foo", "bar"},
		},

		{
			typ:      shim.TypeSet,
			expected: []interface{}{},
		},
		{
			str:      "[ \"foo\", \"bar\" ]",
			typ:      shim.TypeSet,
			expected: []interface{}{"foo", "bar"},
		},

		{
			typ:      shim.TypeMap,
			expected: map[string]interface{}{},
		},
		{
			str: "{ \"foo\": { \"bar\": 42 }, \"baz\": [ true ] }",
			typ: shim.TypeMap,
			expected: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": 42,
				},
				"baz": []interface{}{
					true,
				},
			},
		},
	}

	for _, c := range cases {
		enc := &ConfigEncoding{}
		v, err := enc.convertStringToPropertyValue(c.str, c.typ)
		assert.Equal(t, resource.NewPropertyValue(c.expected), v)
		if c.expected == nil {
			assert.Error(t, err)
		}
	}
}

func TestCamelPascalPulumiName(t *testing.T) {
	t.Parallel()
	p := Provider{
		info: ProviderInfo{
			Name:           "name",
			ResourcePrefix: "resource_prefix",
		},
	}

	t.Run("Produces correct names", func(t *testing.T) {
		camel, pascal := p.camelPascalPulumiName("resource_prefix_some_resource")

		assert.Equal(t, "someResource", camel)
		assert.Equal(t, "SomeResource", pascal)
	})

	t.Run("Panics if the prefix is incorrect", func(t *testing.T) {
		assert.Panics(t, func() {
			p.camelPascalPulumiName("not_resource_prefix_some_resource")
		})
	})
}

func TestDiffConfig(t *testing.T) {
	t.Parallel()
	yes := true
	tfProvider := shimv2.NewProvider(&schema.Provider{
		Schema: map[string]*schema.Schema{
			"access_key": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"region": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	})
	provider := &Provider{
		tf:     tfProvider,
		config: tfProvider.Schema(),
		info: ProviderInfo{
			Config: map[string]*SchemaInfo{
				"region": {
					ForcesProviderReplace: &yes,
				},
			},
		},
	}

	t.Run("no changes", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/DiffConfig",
		  "request": {
		    "urn": "urn:pulumi:dev2::bridge-244::pulumi:providers:aws::name1",
		    "olds": {
		      "region": "us-east-1",
		      "version": "6.22.0"
		    },
		    "news": {
		      "region": "us-east-1",
		      "version": "6.22.0"
		    },
		    "oldInputs": {
		      "region": "us-east-1",
		      "version": "6.22.0"
		    }
		  },
		  "response": {}
		}`)
	})

	t.Run("changing access key results in an in-place update", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/DiffConfig",
		  "request": {
		    "urn": "urn:pulumi:dev2::bridge-244::pulumi:providers:aws::name1",
		    "olds": {
		      "version": "6.22.0"
		    },
		    "news": {
		      "accessKey": "ak1",
		      "version": "6.22.0"
		    },
		    "oldInputs": {
		      "version": "6.22.0"
		    }
		  },
		  "response": {
                    "diffs": ["accessKey"],
		    "changes": "DIFF_SOME",
                    "detailedDiff": {
                      "accessKey": {"inputDiff": true}
                    }
		  }
		}`)
	})

	t.Run("changing access key can be ignored", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/DiffConfig",
		  "request": {
		    "urn": "urn:pulumi:dev2::bridge-244::pulumi:providers:aws::name1",
		    "olds": {
		      "version": "6.22.0"
		    },
		    "news": {
		      "accessKey": "ak1",
		      "version": "6.22.0"
		    },
		    "oldInputs": {
		      "version": "6.22.0"
		    },
                    "ignoreChanges": ["accessKey"]
		  },
		  "response": {}
		}`)
	})

	t.Run("changing region forces a cascading replace", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/DiffConfig",
		  "request": {
		    "urn": "urn:pulumi:dev2::bridge-244::pulumi:providers:aws::name1",
		    "olds": {
		      "region": "us-east-1",
		      "version": "6.22.0"
		    },
		    "news": {
		      "region": "us-west-1",
		      "version": "6.22.0"
		    },
		    "oldInputs": {
		      "region": "us-east-1",
		      "version": "6.22.0"
		    }
		  },
		  "response": {
                    "diffs": ["region"],
		    "replaces": ["region"],
		    "changes": "DIFF_SOME",
                    "detailedDiff": {
                      "region": {"inputDiff": true, "kind": "UPDATE_REPLACE"}
                    }
		  }
		}`)
	})

	t.Run("changing region from empty does not result in a cascading replace", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/DiffConfig",
		  "request": {
		    "urn": "urn:pulumi:dev2::bridge-244::pulumi:providers:aws::name1",
		    "olds": {
		      "version": "6.22.0"
		    },
		    "news": {
		      "region": "us-west-1",
		      "version": "6.22.0"
		    },
		    "oldInputs": {
		      "version": "6.22.0"
		    }
		  },
		  "response": {
                    "diffs": ["region"],
		    "changes": "DIFF_SOME",
                    "detailedDiff": {
                      "region": {"inputDiff": true}
                    }
		  }
		}`)
	})

	t.Run("changing region can be ignored", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/DiffConfig",
		  "request": {
		    "urn": "urn:pulumi:dev2::bridge-244::pulumi:providers:aws::name1",
                    "ignoreChanges": ["region"],
		    "olds": {
		      "region": "us-east-1",
		      "version": "6.22.0"
		    },
		    "news": {
		      "region": "us-west-1",
		      "version": "6.22.0"
		    },
		    "oldInputs": {
		      "region": "us-east-1",
		      "version": "6.22.0"
		    }
		  },
		  "response": {}
		}`)
	})

	// It is sub-optimal that this shows as no-change where it actually might be a change,
	// but Pulumi CLI emits this warning, so it is not so bad:
	//
	// The provider for this resource has inputs that are not known during preview.
	// This preview may not correctly represent the changes that will be applied during an update.
	//
	// This seems to be a better trade-off than indicating that a replace is needed, where it
	// actually will not be needed if the unknown resolves to the same region as the one prior.
	t.Run("unknown region ignored in planning", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/DiffConfig",
		  "request": {
		    "urn": "urn:pulumi:dev2::bridge-244::pulumi:providers:aws::name1",
		    "olds": {
		      "region": "us-east-1",
		      "version": "6.22.0"
		    },
		    "news": {
		      "region": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
		      "version": "6.22.0"
		    },
		    "oldInputs": {
		      "region": "us-east-1",
		      "version": "6.22.0"
		    }
		  },
		  "response": {}
		}`)
	})
}

func TestBuildConfig(t *testing.T) {
	t.Parallel()
	provider := &Provider{
		tf:     shimv1.NewProvider(testTFProvider),
		config: shimv1.NewSchemaMap(testTFProvider.Schema),
	}

	configIn := resource.PropertyMap{
		"configValue": resource.NewStringProperty("foo"),
		"version":     resource.NewStringProperty("0.0.1"),
	}
	ctx := context.Background()
	configOut, err := buildTerraformConfig(ctx, provider, configIn)
	assert.NoError(t, err)

	expected := provider.tf.NewResourceConfig(ctx, map[string]interface{}{
		"config_value": "foo",
	})
	assert.Equal(t, expected, configOut)
}

func testIgnoreChanges(t *testing.T, provider *Provider) {
	urn := resource.NewURN("stack", "project", "", "ExampleResource", "name")

	// Step 1: create and check an input bag.
	pulumiIns, err := plugin.MarshalProperties(resource.PropertyMap{
		"stringPropertyValue": resource.NewStringProperty("foo"),
		"setPropertyValues":   resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	checkResp, err := provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)

	// Step 2a: preview the creation of a resource using the checked input bag.
	createResp, err := provider.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: checkResp.GetInputs(),
		Preview:    true,
	})
	assert.NoError(t, err)

	outs, err := plugin.UnmarshalProperties(createResp.GetProperties(), plugin.MarshalOptions{
		KeepUnknowns: true,
		SkipNulls:    true,
	})
	assert.NoError(t, err)

	assert.Equal(t, outs["stringPropertyValue"], resource.NewStringProperty("foo"))

	expectSetPV := resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")})
	assert.Equal(t, outs["setPropertyValues"], expectSetPV)

	// Step 2b: actually create the resource.
	pulumiIns, err = plugin.MarshalProperties(resource.NewPropertyMapFromMap(map[string]interface{}{
		"stringPropertyValue": "foo",
		"setPropertyValues":   []interface{}{"foo"},
	}), plugin.MarshalOptions{})
	assert.NoError(t, err)
	checkResp, err = provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)
	createResp, err = provider.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	// Step 3: preview an update to the resource we just created.
	pulumiIns, err = plugin.MarshalProperties(resource.PropertyMap{
		"stringPropertyValue": resource.NewStringProperty("bar"),
		"setPropertyValues": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("foo"),
			resource.NewStringProperty("bar"),
		}),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	checkResp, err = provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
		Olds: createResp.GetProperties(),
	})
	assert.NoError(t, err)

	updateResp, err := provider.Update(context.Background(), &pulumirpc.UpdateRequest{
		Id:            "MyID",
		Urn:           string(urn),
		Olds:          createResp.GetProperties(),
		News:          checkResp.GetInputs(),
		IgnoreChanges: []string{"setPropertyValues"},
		Preview:       true,
	})
	assert.NoError(t, err)

	outs, err = plugin.UnmarshalProperties(updateResp.GetProperties(), plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty("bar"), outs["stringPropertyValue"])
	assert.True(t, resource.NewArrayProperty([]resource.PropertyValue{
		resource.NewStringProperty("foo"),
	}).DeepEquals(outs["setPropertyValues"]))
}

func TestIgnoreChanges(t *testing.T) {
	t.Parallel()
	provider := &Provider{
		tf:     shimv1.NewProvider(testTFProvider),
		config: shimv1.NewSchemaMap(testTFProvider.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"ExampleResource": {
			TF:     shimv1.NewResource(testTFProvider.ResourcesMap["example_resource"]),
			TFName: "example_resource",
			Schema: &ResourceInfo{Tok: "ExampleResource"},
		},
	}
	testIgnoreChanges(t, provider)
}

func TestIgnoreChangesV2(t *testing.T) {
	t.Parallel()
	testIgnoreChangesV2(t, shimv2.NewProvider(testTFProviderV2))
}

func testIgnoreChangesV2(t *testing.T, prov shim.Provider) {
	provider := &Provider{
		tf:     prov,
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
		info: ProviderInfo{
			ResourcePrefix: "example",
			Resources: map[string]*ResourceInfo{
				"example_resource":       {Tok: "ExampleResource"},
				"second_resource":        {Tok: "SecondResource"},
				"nested_secret_resource": {Tok: "NestedSecretResource"},
			},
		},
	}
	provider.initResourceMaps()
	testIgnoreChanges(t, provider)
}

func TestProviderPreview(t *testing.T) {
	t.Parallel()
	provider := &Provider{
		tf:     shimv1.NewProvider(testTFProvider),
		config: shimv1.NewSchemaMap(testTFProvider.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"ExampleResource": {
			TF:     shimv1.NewResource(testTFProvider.ResourcesMap["example_resource"]),
			TFName: "example_resource",
			Schema: &ResourceInfo{Tok: "ExampleResource"},
		},
	}
	urn := resource.NewURN("stack", "project", "", "ExampleResource", "name")

	unknown := resource.MakeComputed(resource.NewStringProperty(""))

	// Step 1: create and check an input bag.
	pulumiIns, err := plugin.MarshalProperties(resource.PropertyMap{
		"stringPropertyValue": resource.NewStringProperty("foo"),
		"setPropertyValues":   resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
		"nestedResources": resource.NewObjectProperty(resource.PropertyMap{
			"kind": unknown,
			"configuration": resource.NewObjectProperty(resource.PropertyMap{
				"name": resource.NewStringProperty("foo"),
			}),
		}),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	checkResp, err := provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)

	// Step 2a: preview the creation of a resource using the checked input bag.
	createResp, err := provider.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: checkResp.GetInputs(),
		Preview:    true,
	})
	assert.NoError(t, err)

	outs, err := plugin.UnmarshalProperties(createResp.GetProperties(), plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	assert.True(t, resource.PropertyMap{
		"id":                  resource.NewStringProperty(""),
		"stringPropertyValue": resource.NewStringProperty("foo"),
		"setPropertyValues":   resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
		"nestedResources": resource.NewObjectProperty(resource.PropertyMap{
			"kind": unknown,
			"configuration": resource.NewObjectProperty(resource.PropertyMap{
				"name": resource.NewStringProperty("foo"),
			}),
			"optBool": resource.NewBoolProperty(false),
		}),
	}.DeepEquals(outs))

	// Step 2b: actually create the resource.
	pulumiIns, err = plugin.MarshalProperties(resource.NewPropertyMapFromMap(map[string]interface{}{
		"stringPropertyValue": "foo",
		"setPropertyValues":   []interface{}{"foo"},
		"nestedResources": map[string]interface{}{
			"kind": "foo",
			"configuration": map[string]interface{}{
				"name": "foo",
			},
		},
	}), plugin.MarshalOptions{})
	assert.NoError(t, err)
	checkResp, err = provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)
	createResp, err = provider.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	// Step 3: preview an update to the resource we just created.
	pulumiIns, err = plugin.MarshalProperties(resource.PropertyMap{
		"stringPropertyValue": resource.NewStringProperty("bar"),
		"setPropertyValues":   resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
		"nestedResources": resource.NewObjectProperty(resource.PropertyMap{
			"kind": unknown,
			"configuration": resource.NewObjectProperty(resource.PropertyMap{
				"name": resource.NewStringProperty("foo"),
			}),
		}),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	checkResp, err = provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
		Olds: createResp.GetProperties(),
	})
	assert.NoError(t, err)

	updateResp, err := provider.Update(context.Background(), &pulumirpc.UpdateRequest{
		Id:      "MyID",
		Urn:     string(urn),
		Olds:    createResp.GetProperties(),
		News:    checkResp.GetInputs(),
		Preview: true,
	})
	assert.NoError(t, err)

	outs, err = plugin.UnmarshalProperties(updateResp.GetProperties(), plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty("bar"), outs["stringPropertyValue"])
	assert.True(t, resource.NewObjectProperty(resource.PropertyMap{
		"kind": unknown,
		"configuration": resource.NewObjectProperty(resource.PropertyMap{
			"name": resource.NewStringProperty("foo"),
		}),
		"optBool": resource.NewBoolProperty(false),
	}).DeepEquals(outs["nestedResources"]))
}

func TestProviderPreviewV2(t *testing.T) {
	t.Parallel()
	shimProvider := shimv2.NewProvider(testTFProviderV2)
	provider := &Provider{
		tf:     shimProvider,
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"ExampleResource": {
			TF:     shimProvider.ResourcesMap().Get("example_resource"),
			TFName: "example_resource",
			Schema: &ResourceInfo{Tok: "ExampleResource"},
		},
	}
	urn := resource.NewURN("stack", "project", "", "ExampleResource", "name")

	unknown := resource.MakeComputed(resource.NewStringProperty(""))

	// Step 1: create and check an input bag.
	pulumiIns, err := plugin.MarshalProperties(resource.PropertyMap{
		"stringPropertyValue": resource.NewStringProperty("foo"),
		"setPropertyValues":   resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
		"nestedResources": resource.NewObjectProperty(resource.PropertyMap{
			"kind": unknown,
			"configuration": resource.NewObjectProperty(resource.PropertyMap{
				"name": resource.NewStringProperty("foo"),
			}),
		}),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	checkResp, err := provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)

	// Step 2a: preview the creation of a resource using the checked input bag.
	createResp, err := provider.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: checkResp.GetInputs(),
		Preview:    true,
	})
	assert.NoError(t, err)

	outs, err := plugin.UnmarshalProperties(createResp.GetProperties(), plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	//nolint:lll
	autogold.Expect(resource.PropertyMap{
		resource.PropertyKey(reservedkeys.Meta): resource.PropertyValue{
			V: `{"_new_extra_shim":{},"e2bfb730-ecaa-11e6-8f88-34363bc7c4c0":{"create":120000000000}}`,
		},
		resource.PropertyKey("arrayPropertyValues"): resource.PropertyValue{},
		resource.PropertyKey("boolPropertyValue"):   resource.PropertyValue{},
		resource.PropertyKey("floatPropertyValue"):  resource.PropertyValue{},
		resource.PropertyKey("id"): resource.PropertyValue{V: resource.Computed{Element: resource.PropertyValue{
			V: "",
		}}},
		resource.PropertyKey("nestedResources"): resource.PropertyValue{V: resource.PropertyMap{
			resource.PropertyKey("configuration"): resource.PropertyValue{V: resource.PropertyMap{resource.PropertyKey("name"): resource.PropertyValue{
				V: "foo",
			}}},
			resource.PropertyKey("kind"):    resource.PropertyValue{V: resource.Computed{Element: resource.PropertyValue{V: ""}}},
			resource.PropertyKey("optBool"): resource.PropertyValue{},
		}},
		resource.PropertyKey("nilPropertyValue"):           resource.PropertyValue{},
		resource.PropertyKey("numberPropertyValue"):        resource.PropertyValue{},
		resource.PropertyKey("objectPropertyValue"):        resource.PropertyValue{},
		resource.PropertyKey("setPropertyValues"):          resource.PropertyValue{V: []resource.PropertyValue{{V: "foo"}}},
		resource.PropertyKey("stringPropertyValue"):        resource.PropertyValue{V: "foo"},
		resource.PropertyKey("stringWithBadInterpolation"): resource.PropertyValue{},
	}).Equal(t, outs)

	// Step 2b: actually create the resource.
	pulumiIns, err = plugin.MarshalProperties(resource.NewPropertyMapFromMap(map[string]interface{}{
		"stringPropertyValue": "foo",
		"setPropertyValues":   []interface{}{"foo"},
		"nestedResources": map[string]interface{}{
			"kind": "foo",
			"configuration": map[string]interface{}{
				"name": "foo",
			},
		},
	}), plugin.MarshalOptions{})
	assert.NoError(t, err)
	checkResp, err = provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)
	createResp, err = provider.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: checkResp.GetInputs(),
	})
	assert.NoError(t, err)

	// Step 3: preview an update to the resource we just created.
	pulumiIns, err = plugin.MarshalProperties(resource.PropertyMap{
		"stringPropertyValue": resource.NewStringProperty("bar"),
		"setPropertyValues":   resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
		"nestedResources": resource.NewObjectProperty(resource.PropertyMap{
			"kind": unknown,
			"configuration": resource.NewObjectProperty(resource.PropertyMap{
				"name": resource.NewStringProperty("foo"),
			}),
		}),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	checkResp, err = provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
		Olds: createResp.GetProperties(),
	})
	assert.NoError(t, err)

	updateResp, err := provider.Update(context.Background(), &pulumirpc.UpdateRequest{
		Id:      "MyID",
		Urn:     string(urn),
		Olds:    createResp.GetProperties(),
		News:    checkResp.GetInputs(),
		Preview: true,
	})
	assert.NoError(t, err)

	outs, err = plugin.UnmarshalProperties(updateResp.GetProperties(), plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	assert.Equal(t, resource.NewStringProperty("bar"), outs["stringPropertyValue"])
	assert.True(t, resource.NewObjectProperty(resource.PropertyMap{
		"kind": unknown,
		"configuration": resource.NewObjectProperty(resource.PropertyMap{
			"name": resource.NewStringProperty("foo"),
		}),
		"optBool": resource.NewBoolProperty(false),
	}).DeepEquals(outs["nestedResources"]))
}

func TestProviderCheckWithAutonaming(t *testing.T) {
	t.Parallel()
	provider := &Provider{
		tf:     shimv2.NewProvider(testTFProviderV2),
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"ExampleResource": {
			TF:     shimv1.NewResource(testTFProvider.ResourcesMap["example_resource"]),
			TFName: "example_resource",
			Schema: &ResourceInfo{
				Tok: "ExampleResource",
				Fields: map[string]*SchemaInfo{
					"string_property_value": AutoNameWithCustomOptions("string_property_value", AutoNameOptions{
						Separator: "-",
						Maxlen:    50,
						Randlen:   8,
					}),
				},
			},
		},
	}
	urn := resource.NewURN("stack", "project", "", "ExampleResource", "name")

	pulumiIns, err := plugin.MarshalProperties(resource.PropertyMap{
		"arrayPropertyValues": resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	checkResp, err := provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
		Autonaming: &pulumirpc.CheckRequest_AutonamingOptions{
			ProposedName: "this-name-please",
			Mode:         pulumirpc.CheckRequest_AutonamingOptions_ENFORCE,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, checkResp)
	require.Empty(t, checkResp.Failures)
	ins, err := plugin.UnmarshalProperties(checkResp.GetInputs(), plugin.MarshalOptions{})
	require.NoError(t, err)
	name := ins["string_property_value"]
	require.True(t, name.IsString())
	require.Equal(t, "this-name-please", name.StringValue())
	_ = name
}

func testCheckFailures(t *testing.T, provider *Provider, typeName tokens.Type) []*pulumirpc.CheckFailure {
	urn := resource.NewURN("stack", "project", "", typeName, "name")
	unknown := resource.MakeComputed(resource.NewStringProperty(""))

	pulumiIns, err := plugin.MarshalProperties(resource.PropertyMap{
		"stringPropertyValue": resource.NewStringProperty("foo"),
		"setPropertyValues":   resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
		"nestedResources": resource.NewObjectProperty(resource.PropertyMap{
			"kind": unknown,
			"configuration": resource.NewObjectProperty(resource.PropertyMap{
				"name": resource.NewStringProperty("foo"),
			}),
		}),
		"conflictingProperty":  resource.NewStringProperty("foo"),
		"conflictingProperty2": resource.NewStringProperty("foo"),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	checkResp, err := provider.Check(context.Background(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		News: pulumiIns,
	})
	assert.NoError(t, err)
	return checkResp.Failures
}

func testCheckFailuresV1(t *testing.T, failures []*pulumirpc.CheckFailure) {
	assert.Equal(t, "\"conflicting_property\": conflicts with conflicting_property2."+
		" Examine values at 'name.conflictingProperty'.", failures[0].Reason)
	assert.Equal(t, "", failures[0].Property)
	assert.Equal(t, "\"conflicting_property2\": conflicts with conflicting_property."+
		" Examine values at 'name.conflictingProperty2'.", failures[1].Reason)
	assert.Equal(t, "", failures[1].Property)
	assert.Equal(t, "Missing required property 'arrayPropertyValues'", failures[2].Reason)
	assert.Equal(t, "", failures[2].Property)
}

func testCheckFailuresV2(t *testing.T, failures []*pulumirpc.CheckFailure) {
	assert.Equal(t, "Conflicting configuration arguments. \"conflicting_property\": conflicts with "+
		"conflicting_property2. Examine values at 'name.conflictingProperty'.", failures[0].Reason)
	assert.Equal(t, "", failures[0].Property)
	assert.Equal(t, "Conflicting configuration arguments. \"conflicting_property2\": conflicts with "+
		"conflicting_property. Examine values at 'name.conflictingProperty2'.", failures[1].Reason)
	assert.Equal(t, "", failures[1].Property)
	assert.Equal(t, "Missing required argument. The argument \"array_property_value\" is required"+
		", but no definition was found.. Examine values at 'name.arrayPropertyValues'.", failures[2].Reason)
	assert.Equal(t, "", failures[2].Property)
}

func TestProviderCheck(t *testing.T) {
	t.Parallel()
	testFailures := map[string]func(*testing.T, []*pulumirpc.CheckFailure){
		"v1": testCheckFailuresV1,
		"v2": testCheckFailuresV2,
	}
	for _, f := range factories {
		provider := &Provider{
			tf:     f.NewTestProvider(),
			config: f.NewTestProvider().Schema(),
		}

		provider.resources = map[tokens.Type]Resource{
			"SecondResource": {
				TF:     provider.tf.ResourcesMap().Get("second_resource"),
				TFName: "second_resource",
				Schema: &ResourceInfo{Tok: "SecondResource"},
			},
		}

		t.Run(f.SDKVersion(), func(t *testing.T) {
			t.Run("failures", func(t *testing.T) {
				failures := testCheckFailures(t, provider, "SecondResource")
				assert.Len(t, failures, 3)
				sort.SliceStable(failures, func(i, j int) bool {
					return failures[i].Reason < failures[j].Reason
				})
				testFailures[f.SDKVersion()](t, failures)
			})
		})
	}
}

func TestCheckCallback(t *testing.T) {
	t.Parallel()
	test := func(t *testing.T, p *Provider) {
		testutils.ReplaySequence(t, p, `
			[
			  {
			    "method": "/pulumirpc.ResourceProvider/Configure",
			    "request": {
			      "args": {
				"prop": "global"
			      },
			      "variables": {
				"prop": "global"
			      }
			    },
			    "response": {
			      "supportsPreview": true,
			      "supportsAutonamingConfiguration": true
			    }
			  },
			  {
			    "method": "/pulumirpc.ResourceProvider/Check",
			    "request": {
			      "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
			      "olds": {},
			      "news": {},
			      "randomSeed": "wqZZaHWVfsS1ozo3bdauTfZmjslvWcZpUjn7BzpS79c="
			    },
			    "response": {
			      "inputs": {
				"__defaults": [],
				"arrayPropertyValues": ["global"]
			      }
			    }
			  }
			]`)
	}

	callback := func(
		ctx context.Context, config, meta resource.PropertyMap,
	) (resource.PropertyMap, error) {
		// We test that we have access to the logger in this callback.
		GetLogger(ctx).Status().Info("Did not panic")

		urn := GetUrn(ctx)
		assert.Equal(t, urn, resource.URN("urn:pulumi:st::pg::testprovider:index/res:Res::r"))

		config["arrayPropertyValues"] = resource.NewArrayProperty(
			[]resource.PropertyValue{meta["prop"]},
		)
		return config, nil
	}

	for _, f := range factories {
		f := f
		t.Run(f.SDKVersion(), func(t *testing.T) {
			t.Parallel()
			p := &Provider{
				tf:     f.NewTestProvider(),
				config: f.NewTestProvider().Schema(),
			}
			p.resources = map[tokens.Type]Resource{
				"testprovider:index/res:Res": {
					TF:     p.tf.ResourcesMap().Get("example_resource"),
					TFName: "example_resource",
					Schema: &ResourceInfo{
						Tok:              "testprovider:index/res:Res",
						PreCheckCallback: callback,
					},
				},
			}
			test(t, p)
		})
	}
}

func testProviderRead(t *testing.T, provider *Provider, typeName tokens.Type, checkRawConfig bool) {
	urn := resource.NewURN("stack", "project", "", typeName, "name")
	props, err := structpb.NewStruct(map[string]interface{}{
		"rawConfigValue": "fromRawConfig",
	})
	require.NoError(t, err)
	readResp, err := provider.Read(context.Background(), &pulumirpc.ReadRequest{
		Id:         string("resource-id"),
		Urn:        string(urn),
		Properties: nil,
		Inputs:     props,
	})
	require.NoError(t, err)

	assert.NotNil(t, readResp.GetInputs())
	assert.NotNil(t, readResp.GetProperties())

	ins, err := plugin.UnmarshalProperties(readResp.GetInputs(), plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	// Check all the expected inputs were read
	assert.NotContains(t, ins, "boolPropertyValue") // This was "false" from Read, but it's default is false
	assert.Equal(t, resource.NewNumberProperty(42), ins["numberPropertyValue"])
	assert.Equal(t, resource.NewNumberProperty(99.6767932), ins["floatPropertyValue"])
	assert.Equal(t, resource.NewStringProperty("ognirts"), ins["stringPropertyValue"])
	assert.Equal(t, resource.NewArrayProperty(
		[]resource.PropertyValue{resource.NewStringProperty("an array")}), ins["arrayPropertyValues"])
	assert.Equal(t, resource.NewObjectProperty(resource.PropertyMap{
		"__defaults": resource.NewArrayProperty([]resource.PropertyValue{}),
		"property_a": resource.NewStringProperty("a"),
		"property_b": resource.NewStringProperty("true"),
		"property.c": resource.NewStringProperty("some.value"),
	}), ins["objectPropertyValue"])
	assert.Equal(t, resource.NewObjectProperty(resource.PropertyMap{
		"__defaults": resource.NewArrayProperty([]resource.PropertyValue{}),
		"configuration": resource.NewObjectProperty(resource.PropertyMap{
			"__defaults":         resource.NewArrayProperty([]resource.PropertyValue{}),
			"configurationValue": resource.NewStringProperty("true"),
		}),
	}), ins["nestedResources"])
	assert.Len(t, ins["setPropertyValues"].ArrayValue(), 2)
	assert.Contains(t, ins["setPropertyValues"].ArrayValue(), resource.NewStringProperty("set member 1"))
	assert.Contains(t, ins["setPropertyValues"].ArrayValue(), resource.NewStringProperty("set member 2"))
	assert.Equal(t, resource.NewStringProperty("some ${interpolated:value} with syntax errors"),
		ins["stringWithBadInterpolation"])

	if checkRawConfig {
		readResp, err := provider.Read(context.Background(), &pulumirpc.ReadRequest{
			Id:     string("set-raw-config-id"),
			Urn:    string(urn),
			Inputs: props,
		})
		require.NoError(t, err)
		outs, err := plugin.UnmarshalProperties(readResp.GetProperties(),
			plugin.MarshalOptions{KeepUnknowns: true})
		require.NoError(t, err)
		assert.Equal(t, "fromRawConfig", outs["stringPropertyValue"].StringValue())
	}

	// Read again with the ID that results in all the optinal fields not being set
	readResp, err = provider.Read(context.Background(), &pulumirpc.ReadRequest{
		Id:         string("empty-resource-id"),
		Urn:        string(urn),
		Properties: nil,
	})
	assert.NoError(t, err)

	assert.NotNil(t, readResp.GetInputs())
	assert.NotNil(t, readResp.GetProperties())

	ins, err = plugin.UnmarshalProperties(readResp.GetInputs(), plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	// Check all the expected inputs were read
	assert.NotContains(t, ins, "boolPropertyValue")
	assert.NotContains(t, ins, "numberPropertyValue")
	assert.NotContains(t, ins, "floatPropertyValue")
	assert.NotContains(t, ins, "stringPropertyValue")
	assert.Equal(t, resource.NewArrayProperty(
		[]resource.PropertyValue{resource.NewStringProperty("an array")}), ins["arrayPropertyValues"])
	assert.NotContains(t, ins, "objectPropertyValue")
	assert.NotContains(t, ins, "nestedResources")
	assert.NotContains(t, ins, "setPropertyValues")
	assert.NotContains(t, ins, "stringWithBadInterpolation")
}

func TestProviderReadV1(t *testing.T) {
	t.Parallel()
	provider := &Provider{
		tf:     shimv1.NewProvider(testTFProvider),
		config: shimv1.NewSchemaMap(testTFProvider.Schema),
	}

	provider.resources = map[tokens.Type]Resource{
		"ExampleResource": {
			TF:     shimv1.NewResource(testTFProvider.ResourcesMap["example_resource"]),
			TFName: "example_resource",
			Schema: &ResourceInfo{Tok: "ExampleResource"},
		},
	}

	testProviderRead(t, provider, "ExampleResource", false /* CheckRawConfig */)
}

func TestProviderReadV2(t *testing.T) {
	t.Parallel()
	shimProvider := shimv2.NewProvider(testTFProviderV2)
	provider := &Provider{
		tf:     shimProvider,
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"ExampleResource": {
			TF:     shimProvider.ResourcesMap().Get("example_resource"),
			TFName: "example_resource",
			Schema: &ResourceInfo{Tok: "ExampleResource"},
		},
	}

	// TODO[pulumi/pulumi-terraform-bridge#1977] currently un-schematized fields do not propagate to RawConfig which
	// causes the test to panic as written.
	checkRawConfig := false

	testProviderRead(t, provider, "ExampleResource", checkRawConfig)
}

func testProviderReadNestedSecret(t *testing.T, provider *Provider, typeName tokens.Type) {
	urn := resource.NewURN("stack", "project", "", typeName, "name")

	// Configure that we support secrets
	_, _ = provider.Configure(context.Background(), &pulumirpc.ConfigureRequest{
		AcceptSecrets:   true,
		AcceptResources: true,
	})

	// Check that if we create the resource the secret property comes back as a secret
	createResp, err := provider.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: nil,
	})
	assert.NoError(t, err)

	assert.NotNil(t, createResp.GetProperties())
	props, err := plugin.UnmarshalProperties(createResp.GetProperties(),
		plugin.MarshalOptions{KeepUnknowns: true, KeepSecrets: true})
	assert.NoError(t, err)

	assert.Equal(t, resource.NewObjectProperty(resource.PropertyMap{
		"aSecret": resource.MakeSecret(resource.NewStringProperty("password")),
	}), props["nested"])

	// Check that read is also a secret
	readResp, err := provider.Read(context.Background(), &pulumirpc.ReadRequest{
		Id:         string("0"),
		Urn:        string(urn),
		Properties: nil,
	})
	assert.NoError(t, err)

	assert.NotNil(t, readResp.GetProperties())
	props, err = plugin.UnmarshalProperties(readResp.GetProperties(),
		plugin.MarshalOptions{KeepUnknowns: true, KeepSecrets: true})
	assert.NoError(t, err)

	assert.Equal(t, resource.NewObjectProperty(resource.PropertyMap{
		"aSecret": resource.MakeSecret(resource.NewStringProperty("password")),
	}), props["nested"])
}

func TestProviderReadNestedSecretV1(t *testing.T) {
	t.Parallel()
	provider := &Provider{
		tf:     shimv1.NewProvider(testTFProvider),
		config: shimv1.NewSchemaMap(testTFProvider.Schema),
	}

	provider.resources = map[tokens.Type]Resource{
		"NestedSecretResource": {
			TF:     shimv1.NewResource(testTFProvider.ResourcesMap["nested_secret_resource"]),
			TFName: "nested_secret_resource",
			Schema: &ResourceInfo{Tok: "NestedSecretResource"},
		},
	}

	testProviderReadNestedSecret(t, provider, "NestedSecretResource")
}

func TestProviderReadNestedSecretV2(t *testing.T) {
	t.Parallel()
	shimProvider := shimv2.NewProvider(testTFProviderV2)
	provider := &Provider{
		tf:     shimProvider,
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"NestedSecretResource": {
			TF:     shimProvider.ResourcesMap().Get("nested_secret_resource"),
			TFName: "nested_secret_resource",
			Schema: &ResourceInfo{Tok: "NestedSecretResource"},
		},
	}

	testProviderReadNestedSecret(t, provider, "NestedSecretResource")
}

func TestCheck(t *testing.T) {
	t.Parallel()
	t.Run("Default application can consult prior state in Check", func(t *testing.T) {
		shimProvider := shimv2.NewProvider(testTFProviderV2)
		provider := &Provider{
			tf:     shimProvider,
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
		}
		computeStringDefault := func(ctx context.Context, opts ComputeDefaultOptions) (interface{}, error) {
			// We check that we have access to the logger when computing a default value.
			GetLogger(ctx).Status().Info("Did not panic")

			if v, ok := opts.PriorState["stringPropertyValue"]; ok {
				require.Equal(t, resource.NewStringProperty("oldString"), opts.PriorValue)
				return v.StringValue() + "!", nil
			}
			return nil, nil
		}
		provider.resources = map[tokens.Type]Resource{
			"ExampleResource": {
				TF:     shimProvider.ResourcesMap().Get("example_resource"),
				TFName: "example_resource",
				Schema: &ResourceInfo{
					Tok: "ExampleResource",
					Fields: map[string]*SchemaInfo{
						"string_property_value": {
							Default: &DefaultInfo{
								ComputeDefault: computeStringDefault,
							},
						},
					},
				},
			},
		}
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Check",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
		    "randomSeed": "ZCiVOcvG/CT5jx4XriguWgj2iMpQEb8P3ZLqU/AS2yg=",
		    "olds": {
                      "__defaults": [],
		      "stringPropertyValue": "oldString"
		    },
		    "news": {
		      "arrayPropertyValues": []
		    }
		  },
		  "response": {
		    "inputs": {
                      "__defaults": ["stringPropertyValue"],
		      "arrayPropertyValues": [],
		      "stringPropertyValue": "oldString!"
		    }
		  }
		}
                `)
		// If old value is missing it is ignored.
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Check",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
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
	})

	t.Run("respect schema secrets", func(t *testing.T) {
		p2 := testprovider.ProviderV2()
		p2.ResourcesMap["example_resource"].Schema["string_property_value"].Sensitive = true

		shimProvider := shimv2.NewProvider(p2)
		provider := &Provider{
			tf:     shimProvider,
			config: shimv2.NewSchemaMap(p2.Schema),
		}

		provider.resources = map[tokens.Type]Resource{
			"ExampleResource": {
				TF:     shimProvider.ResourcesMap().Get("example_resource"),
				TFName: "example_resource",
				Schema: &ResourceInfo{
					Tok: "ExampleResource",
				},
			},
		}

		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Check",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
		    "randomSeed": "ZCiVOcvG/CT5jx4XriguWgj2iMpQEb8P3ZLqU/AS2yg=",
		    "olds": {
		      "stringPropertyValue": "oldString"
		    },
		    "news": {
		      "arrayPropertyValues": [],
		      "stringPropertyValue": "newString"
		    }
		  },
		  "response": {
		    "inputs": {
                      "__defaults": [],
		      "arrayPropertyValues": [],
		      "stringPropertyValue": {
                        "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                        "value": "newString"
                      }
		    }
		  }
		}
                `)
	})
}

func TestCheckWarnings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var logs bytes.Buffer
	ctx = logging.InitLogging(ctx, logging.LogOptions{
		LogSink: &testWarnLogSink{&logs},
	})
	p := &schemav2.Provider{
		Schema: map[string]*schemav2.Schema{},
		ResourcesMap: map[string]*schemav2.Resource{
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
	}

	// we need the schema for type checking
	pulumiSchemaSpec := &pschema.PackageSpec{
		Resources: map[string]pschema.ResourceSpec{
			"ExampleResource": {
				StateInputs: &pschema.ObjectTypeSpec{
					Properties: map[string]pschema.PropertySpec{
						"networkConfiguration": {
							TypeSpec: pschema.TypeSpec{
								Ref: "#/types/testprov:ExampleResourceNetworkConfiguration",
							},
						},
					},
				},
				InputProperties: map[string]pschema.PropertySpec{
					"networkConfiguration": {
						TypeSpec: pschema.TypeSpec{
							Ref: "#/types/testprov:ExampleResourceNetworkConfiguration",
						},
					},
				},
				ObjectTypeSpec: pschema.ObjectTypeSpec{
					Properties: map[string]pschema.PropertySpec{
						"networkConfiguration": {
							TypeSpec: pschema.TypeSpec{
								Ref: "#/types/testprov:ExampleResourceNetworkConfiguration",
							},
						},
					},
				},
			},
		},
		Types: map[string]pschema.ComplexTypeSpec{
			"testprov:ExampleResourceNetworkConfiguration": {
				ObjectTypeSpec: pschema.ObjectTypeSpec{
					Properties: map[string]pschema.PropertySpec{
						"securityGroups": {
							TypeSpec: pschema.TypeSpec{
								Type: "array",
								Items: &pschema.TypeSpec{
									Type: "string",
								},
							},
						},
						"subnets": {
							TypeSpec: pschema.TypeSpec{
								Type: "array",
								Items: &pschema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
				},
			},
		},
	}
	shimProvider := shimv2.NewProvider(p)

	provider := &Provider{
		tf:               shimProvider,
		module:           "testprov",
		config:           shimv2.NewSchemaMap(p.Schema),
		pulumiSchema:     []byte("hello"), // we only check whether this is nil in type checking
		pulumiSchemaSpec: pulumiSchemaSpec,
		hasTypeErrors:    make(map[resource.URN]struct{}),
		resources: map[tokens.Type]Resource{
			"ExampleResource": {
				TF:     shimProvider.ResourcesMap().Get("example_resource"),
				TFName: "example_resource",
				Schema: &ResourceInfo{
					Tok: "ExampleResource",
				},
			},
		},
	}

	args, err := structpb.NewStruct(map[string]interface{}{
		"networkConfiguration": []interface{}{
			map[string]interface{}{
				"securityGroups": []interface{}{
					"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				},
				"subnets": "[\"first\",\"second\"]", // this is a type error
			},
		},
	})
	require.NoError(t, err)
	_, err = provider.Check(ctx, &pulumirpc.CheckRequest{
		Urn:  "urn:pulumi:dev::teststack::ExampleResource::exres",
		Olds: &structpb.Struct{},
		News: args,
	})
	require.NoError(t, err)

	// run 'go test  -run=TestCheckWarnings -v ./pkg/tfbridge/ -update' to update
	autogold.Expect(`warning: Type checking failed:
warning: Unexpected type at field "networkConfiguration":
           expected object type, got {[{map[securityGroups:{[out... of type []
warning: Type checking is still experimental. If you believe that a warning is incorrect,
please let us know by creating an issue at https://github.com/pulumi/pulumi-terraform-bridge/issues.
This will become a hard error in the future.
`).Equal(t, logs.String())
}

func TestCheckConfig(t *testing.T) {
	t.Parallel()
	t.Run("minimal", func(t *testing.T) {
		// Ensure the method is minimally implemented. Pulumi will be passing a provider version. Make sure it
		// is mirrored back.
		provider := &Provider{
			tf:     shimv2.NewProvider(testTFProviderV2),
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
		}
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("config_value", func(t *testing.T) {
		provider := &Provider{
			tf:     shimv2.NewProvider(testTFProviderV2),
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
		}
		// Ensure Pulumi can configure config_value in the testprovider.
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
                      "config_value": "foo",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
                      "config_value": "foo",
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("unknown_config_value", func(t *testing.T) {
		// Currently if a top-level config property is a Computed value, or it's a composite value with any
		// Computed values inside, the engine sends a sentinel string. Ensure that CheckConfig propagates the
		// same sentinel string back to the engine.

		p := testprovider.ProviderV2()

		p.Schema["scopes"] = &schema.Schema{
			Type:     schema.TypeList,
			Required: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		}

		provider := &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
		}

		assert.Equal(t, "04da6b54-80e4-46f7-96ec-b56ff0331ba9", plugin.UnknownStringValue)

		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
                      "configValue": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
                      "scopes": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
                      "configValue": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
                      "scopes": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("config_changed", func(t *testing.T) {
		provider := &Provider{
			tf:     shimv2.NewProvider(testTFProviderV2),
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
		}
		// In this scenario Pulumi plans an update plan when a config has changed on an existing stack.
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {
                      "config_value": "foo",
		      "version": "6.54.0"
                    },
		    "news": {
                      "config_value": "bar",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
                      "config_value": "bar",
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("invalid_config_value", func(t *testing.T) {
		p := testprovider.ProviderV2()
		p.Schema["assume_role"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"role_arn": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		}
		provider := &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
			module: "cloudflare",
		}
		ctx := context.Background()
		args, err := structpb.NewStruct(map[string]interface{}{
			"requiredprop": "baz",
		})
		require.NoError(t, err)
		// Default provider.
		resp, err := provider.CheckConfig(ctx, &pulumirpc.CheckRequest{
			Urn:  "urn:pulumi:r::cloudflare-record-ts::pulumi:providers:cloudflare::default_5_2_1",
			News: args,
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(resp.Failures))
		//nolint:lll
		autogold.Expect("`cloudflare:requiredprop` is not a valid configuration key for the cloudflare provider. If the referenced key is not intended for the provider, please choose a different namespace from `cloudflare:`.").Equal(t, resp.Failures[0].Reason)
		// Default provider nested config property case.
		deepArgs, err := structpb.NewStruct(
			map[string]interface{}{
				"assumeRole": map[string]interface{}{
					"roleAnr": "someRoleARN",
				},
			},
		)
		require.NoError(t, err)
		resp, err = provider.CheckConfig(ctx, &pulumirpc.CheckRequest{
			Urn:  "urn:pulumi:r::cloudflare-record-ts::pulumi:providers:aws::default_5_2_1",
			News: deepArgs,
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(resp.Failures))
		//nolint:lll
		autogold.Expect("`cloudflare:assumeRole.roleAnr` is not a valid configuration key for the cloudflare provider. If the referenced key is not intended for the provider, please choose a different namespace from `cloudflare:`.").Equal(t, resp.Failures[0].Reason)
		// Explicit provider.
		resp, err = provider.CheckConfig(ctx, &pulumirpc.CheckRequest{
			Urn:  "urn:pulumi:r::cloudflare-record-ts::pulumi:providers:cloudflare::explicitprovider",
			News: args,
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(resp.Failures))
		require.Equal(t, "could not validate provider configuration: Invalid or unknown key. "+
			"Examine values at 'explicitprovider.requiredprop'.",
			resp.Failures[0].Reason)
	})

	t.Run("levenshtein_correction", func(t *testing.T) {
		p := testprovider.ProviderV2()
		provider := &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
			module: "testprovider",
		}
		ctx := context.Background()
		args, err := structpb.NewStruct(map[string]interface{}{
			"cofnigValue": "baz",
		})
		require.NoError(t, err)
		resp, err := provider.CheckConfig(ctx, &pulumirpc.CheckRequest{
			Urn:  "urn:pulumi:r::cloudflare-record-ts::pulumi:providers:cloudflare::default_5_2_1",
			News: args,
		})
		require.NoError(t, err)
		require.NoError(t, err)
		require.Equal(t, 1, len(resp.Failures))
		//nolint:lll
		autogold.Expect("`testprovider:cofnigValue` is not a valid configuration key for the testprovider provider. Did you mean `testprovider:configValue`? If the referenced key is not intended for the provider, please choose a different namespace from `testprovider:`.").Equal(t, resp.Failures[0].Reason)
	})

	t.Run("missing_required_config_value_explicit_provider", func(t *testing.T) {
		p := testprovider.ProviderV2()
		p.Schema["req_prop"] = &schema.Schema{
			Type:        schema.TypeString,
			Required:    true,
			Description: "A very important required attribute",
		}
		provider := &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
			module: "testprovider",
		}
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:test1::example::pulumi:providers:prov::explicitprovider",
		    "olds": {},
		    "news": {
		      "version": "6.54.0"
		    }
		  },
		  "response": {
	            "failures": [{
	               "reason": "Missing required property 'reqProp': A very important required attribute"
	            }]
	          }
		}`)
	})

	t.Run("missing_required_config_value_default_provider", func(t *testing.T) {
		p := testprovider.ProviderV2()
		p.Schema["req_prop"] = &schema.Schema{
			Type:        schema.TypeString,
			Required:    true,
			Description: "A very important required attribute",
		}
		provider := &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
			module: "testprovider",
		}
		testutils.Replay(t, provider, fmt.Sprintf(`
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:test1::example::pulumi:providers:prov::default_1_1_42",
		    "olds": {},
		    "news": {
		      "version": "6.54.0"
		    }
		  },
		  "response": {
	            "failures": [{
                       "reason": "Provider is missing a required configuration key, try %s: %s"
	            }]
	          }
		}`, "`pulumi config set testprovider:reqProp`",
			"A very important required attribute"))
	})

	t.Run("flattened_compound_values", func(t *testing.T) {
		// Providers may have nested objects or arrays in their configuration space. As of Pulumi v3.63.0 these
		// may be coming over the wire under a flattened JSON-in-protobuf encoding. This test makes sure they
		// are recognized correctly.

		p := testprovider.ProviderV2()

		// Examples here are taken from pulumi-gcp, scopes is a list and batching is a nested object.
		p.Schema["scopes"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		}

		p.Schema["batching"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"send_after": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"enable_batching": {
						Type:     schema.TypeBool,
						Optional: true,
					},
				},
			},
		}

		provider := &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
		}

		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::testcfg::pulumi:providers:gcp::test",
		    "olds": {},
		    "news": {
		      "batching": "{\"enableBatching\":true,\"sendAfter\":\"1s\"}",
		      "scopes": "[\"a\",\"b\"]",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
                    "inputs": {
		      "batching": "{\"enableBatching\":true,\"sendAfter\":\"1s\"}",
		      "scopes": "[\"a\",\"b\"]",
		      "version": "6.54.0"
                    }
                  }
		}`)
	})

	t.Run("enforce_schema_secrets", func(t *testing.T) {
		// If the schema marks a config property as sensitive, enforce the secret bit on that property.
		p := testprovider.ProviderV2()

		p.Schema["mysecret"] = &schema.Schema{
			Type:      schema.TypeString,
			Optional:  true,
			Sensitive: true,
		}

		provider := &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
		}

		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
                      "mysecret": "foo",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
                      "mysecret": {
			"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                        "value": "foo"
                      },
		      "version": "6.54.0"
		    }
		  }
		}`)
	})

	t.Run("enforce_schema_nested_secrets", func(t *testing.T) {
		// Flattened compound values may encode that some nested properties are sensitive. There is currently no
		// way to preserve the secret-ness accurately in the JSON-in-proto encoding. Instead of this, bridged
		// providers approximate and mark the entire property as secret when any of the components are
		// sensitive.
		p := testprovider.ProviderV2()

		p.Schema["scopes"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		}

		p.Schema["batching"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"send_after": {
						Type:      schema.TypeString,
						Sensitive: true,
						Optional:  true,
					},
					"enable_batching": {
						Type:     schema.TypeBool,
						Optional: true,
					},
				},
			},
		}

		provider := &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
		}

		testutils.Replay(t, provider, `
                {
                  "method": "/pulumirpc.ResourceProvider/CheckConfig",
                  "request": {
                    "urn": "urn:pulumi:dev::testcfg::pulumi:providers:gcp::test",
                    "olds": {},
                    "news": {
                      "batching": "{\"enableBatching\":true,\"sendAfter\":\"1s\"}",
                      "scopes": "[\"a\",\"b\"]",
                      "version": "6.54.0"
                    }
                  },
                  "response": {
                    "inputs": {
                      "batching": {
                        "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                        "value": "{\"enableBatching\":true,\"sendAfter\":\"1s\"}"
                      },
                      "scopes": "[\"a\",\"b\"]",
                      "version": "6.54.0"
                    }
                  }
                }`)
	})
}

func TestConfigure(t *testing.T) {
	t.Parallel()
	t.Run("handle_secret_nested_objects", func(t *testing.T) {
		p := testprovider.ProviderV2()

		p.ConfigureContextFunc = func(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
			batching, ok := d.GetOk("batching")
			require.Truef(t, ok, "Configure expected to receive batching data but did not")
			batchingList, ok := batching.([]any)
			require.Truef(t, ok, "Configure expected to receive batching as a slice but got #%T", batching)
			assert.Equalf(t, 1, len(batchingList), "len(batchingList)==1")
			b0 := batchingList[0]
			bm, ok := b0.(map[string]any)
			require.Truef(t, ok, "Configure expected to receive batching slice elements as maps but got #%T", bm)
			assert.Equal(t, true, bm["enable_batching"])
			assert.Equal(t, "5s", bm["send_after"])
			return nil, nil
		}

		p.Schema["scopes"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		}

		p.Schema["batching"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"send_after": {
						Type:      schema.TypeString,
						Sensitive: true,
						Optional:  true,
					},
					"enable_batching": {
						Type:     schema.TypeBool,
						Optional: true,
					},
				},
			},
		}

		provider := &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
		}

		testutils.Replay(t, provider, `
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {
			    "variables": {
			      "gcp:config:batching": "{\"enableBatching\":true,\"sendAfter\":\"5s\"}"
			    },
			    "args": {
				"batching": {
				  "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
				  "value": "{\"enableBatching\":true,\"sendAfter\":\"5s\"}"
				}
			    },
			    "acceptSecrets": true,
			    "acceptResources": true
			  },
			  "response": {
			    "supportsPreview": true,
			    "supportsAutonamingConfiguration": true
			  }
			}`)
	})
}

func TestConfigureErrorReplacement(t *testing.T) {
	t.Parallel()
	t.Run("replace_config_properties", func(t *testing.T) {
		p := testprovider.ProviderV2()
		p.ConfigureContextFunc = func(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
			return nil, diag.Errorf(`some error with "config_property" and "config" but not config`)
		}
		p.Schema["config_property"] = &schema.Schema{Type: schema.TypeString}
		p.Schema["config"] = &schema.Schema{Type: schema.TypeString}

		shimProv := shimv2.NewProvider(p)
		provider := &Provider{
			tf:     shimProv,
			config: shimv2.NewSchemaMap(p.Schema),
			info: ProviderInfo{
				P: shimProv,
				Config: map[string]*SchemaInfo{
					"config_property": {Name: "configProperty"},
					"config":          {Name: "CONFIG!"},
				},
			},
		}

		testutils.Replay(t, provider, `
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {"acceptResources": true},
			  "errors": ["1 error occurred:\n\t* some error with \"configProperty\" and \"CONFIG!\" but not config\n\n"]
			}`)
	})
}

// Legacy providers like gcp hold onto the Context object passed to Configure to use it in various
// clients. To mitigate this, Configure in terraform-plugin-sdk injects a ctxHack context, as can be
// seen under "see TODO: remove global stop context hack":
//
// https://github.com/hashicorp/terraform-plugin-sdk/blob/main/helper/schema/grpc_provider.go#L602
//
// This test tries to make sure such providers work when bridged.
func TestConfigureContextCapture(t *testing.T) {
	t.Parallel()
	var clientContext context.Context

	configure := func(ctx context.Context, rd *schema.ResourceData) (interface{}, diag.Diagnostics) {
		// StopContext is deprecated but still used in GCP for example:
		// https://github.com/hashicorp/terraform-provider-google-beta/blob/master/google-beta/provider/provider.go#L2258
		stopCtx, ok := schema.StopContext(ctx) //nolint
		if !ok {
			stopCtx = ctx
		}
		clientContext = stopCtx
		return nil, nil
	}

	createR1 := func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
		fail := false
		go func() {
			<-clientContext.Done()
			fail = true
		}()
		time.Sleep(100 * time.Millisecond)
		assert.Falsef(t, fail, "clientContext is Done() during Create")
		rd.SetId("0")
		return nil
	}

	sProvider := &schema.Provider{
		Schema:               map[string]*schema.Schema{},
		ConfigureContextFunc: configure,
		ResourcesMap: map[string]*schema.Resource{
			"p_r1": {CreateContext: createR1},
		},
	}

	provider := &Provider{
		tf:     shimv2.NewProvider(sProvider),
		config: shimv2.NewSchemaMap(sProvider.Schema),
		info: ProviderInfo{
			Resources: map[string]*ResourceInfo{
				"p_r1": {Tok: "prov:index:ExampleResource"},
			},
		},
	}
	provider.initResourceMaps()

	ctx, cancel := context.WithCancel(context.Background())
	_, err := provider.Configure(ctx, &pulumirpc.ConfigureRequest{
		Variables: map[string]string{},
	})
	require.NoError(t, err)
	cancel()

	_, err = provider.Create(context.Background(), &pulumirpc.CreateRequest{
		Urn: "urn:pulumi:dev::teststack::prov:index:ExampleResource::exres",
	})
	require.NoError(t, err)
}

func TestPreConfigureCallback(t *testing.T) {
	t.Parallel()
	t.Run("PreConfigureCallback called by CheckConfig", func(t *testing.T) {
		callCounter := 0
		provider := &Provider{
			tf:     shimv2.NewProvider(testTFProviderV2),
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
			info: ProviderInfo{
				PreConfigureCallback: func(vars resource.PropertyMap, config shim.ResourceConfig) error {
					require.Equal(t, "bar", vars["config_value"].StringValue())
					require.Truef(t, config.IsSet("config_value"), "config_value should be set")
					require.Falsef(t, config.IsSet("unknown_prop"), "unknown_prop should not be set")
					callCounter++
					return nil
				},
			},
		}
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
                      "config_value": "bar",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
                      "config_value": "bar",
		      "version": "6.54.0"
		    }
		  }
		}`)
		require.Equalf(t, 1, callCounter, "PreConfigureCallback should be called once")
	})
	t.Run("PreConfigureCallbackWithLoggger called by CheckConfig", func(t *testing.T) {
		callCounter := 0
		provider := &Provider{
			tf:     shimv2.NewProvider(testTFProviderV2),
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
			info: ProviderInfo{
				PreConfigureCallbackWithLogger: func(
					ctx context.Context,
					host *hostclient.HostClient,
					vars resource.PropertyMap,
					config shim.ResourceConfig,
				) error {
					require.Equal(t, "bar", vars["config_value"].StringValue())
					require.Truef(t, config.IsSet("config_value"), "config_value should be set")
					require.Falsef(t, config.IsSet("unknown_prop"), "unknown_prop should not be set")
					callCounter++
					return nil
				},
			},
		}
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
                      "config_value": "bar",
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
                      "config_value": "bar",
		      "version": "6.54.0"
		    }
		  }
		}`)
		require.Equalf(t, 1, callCounter, "PreConfigureCallbackWithLogger should be called once")
	})
	t.Run("PreConfigureCallback can modify config values", func(t *testing.T) {
		provider := &Provider{
			tf:     shimv2.NewProvider(testTFProviderV2),
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
			info: ProviderInfo{
				PreConfigureCallback: func(vars resource.PropertyMap, config shim.ResourceConfig) error {
					vars["config_value"] = resource.NewStringProperty("updated")
					return nil
				},
			},
		}
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
                      "config_value": "updated",
		      "version": "6.54.0"
		    }
		  }
		}`)
	})
	t.Run("PreConfigureCallbackWithLogger can modify config values", func(t *testing.T) {
		provider := &Provider{
			tf:     shimv2.NewProvider(testTFProviderV2),
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
			info: ProviderInfo{
				PreConfigureCallbackWithLogger: func(
					ctx context.Context,
					host *hostclient.HostClient,
					vars resource.PropertyMap,
					config shim.ResourceConfig,
				) error {
					vars["config_value"] = resource.NewStringProperty("updated")
					return nil
				},
			},
		}
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
		      "version": "6.54.0"
		    }
		  },
		  "response": {
		    "inputs": {
                      "config_value": "updated",
		      "version": "6.54.0"
		    }
		  }
		}`)
	})
	t.Run("PreConfigureCallback not called at preview with unknown values", func(t *testing.T) {
		provider := &Provider{
			tf:     shimv2.NewProvider(testTFProviderV2),
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
			info: ProviderInfo{
				PreConfigureCallbackWithLogger: func(
					ctx context.Context,
					host *hostclient.HostClient,
					vars resource.PropertyMap,
					config shim.ResourceConfig,
				) error {
					// We check that we have access to the logger in this callback.
					GetLogger(ctx).Status().Info("Did not panic")

					if cv, ok := vars["configValue"]; ok {
						// This used to panic when cv is a resource.Computed.
						cv.StringValue()
					}
					// PreConfigureCallback should not even be called.
					t.FailNow()
					return nil
				},
			},
		}
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {
		      "version": "6.54.0",
                      "configValue": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
		    }
		  },
		  "response": {
		    "inputs": {
		      "version": "6.54.0",
                      "configValue": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
		    }
		  }
		}`)
	})
}

func TestInvoke(t *testing.T) {
	t.Parallel()
	t.Run("preserve_program_secrets", func(t *testing.T) {
		// Currently the provider is unable to preserve secret-ness of values marked as secrets. Returning
		// secrets makes SDKs unable to consume the provider. Therefore currently the secrets are stripped.
		//
		// See also https://github.com/pulumi/pulumi/issues/12710

		p := testprovider.ProviderV2()

		dsName := "example_resource"
		ds := p.DataSourcesMap[dsName]

		prop := ds.Schema["string_property_value"]
		prop.Sensitive = true
		prop.Computed = true
		prop.Optional = true

		shimProvider := shimv2.NewProvider(p)
		provider := &Provider{
			tf:     shimProvider,
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),

			dataSources: map[tokens.ModuleMember]DataSource{
				"tprov:index/ExampleFn:ExampleFn": {
					TF:     shimProvider.DataSourcesMap().Get(dsName),
					TFName: dsName,
					Schema: &DataSourceInfo{
						Tok: "tprov:index/ExampleFn:ExampleFn",
					},
				},
			},
		}

		// Note that Invoke receives a secret "foo" but returns an un-secret "foo".
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Invoke",
		  "request": {
		    "tok": "tprov:index/ExampleFn:ExampleFn",
		    "args": {
                      "string_property_value": {
			"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
                        "value": "foo"
                      },
		      "array_property_value": []
		    }
		  },
		  "response": {
		    "return": {
		      "stringPropertyValue": "foo",
		      "__meta": "*",
		      "arrayPropertyValues": "*",
		      "boolPropertyValue": "*",
		      "floatPropertyValue": "*",
		      "id": "*",
		      "nestedResources": "*",
		      "numberPropertyValue": "*",
		      "objectPropertyValue": "*",
		      "setPropertyValues": "*",
		      "stringWithBadInterpolation": "*"
		    }
		  }
		}`)
	})
}

func TestTransformOutputs(t *testing.T) {
	t.Parallel()
	shimProvider := shimv2.NewProvider(testTFProviderV2)
	provider := &Provider{
		tf:     shimProvider,
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
		resources: map[tokens.Type]Resource{
			"ExampleResource": {
				TF:     shimProvider.ResourcesMap().Get("example_resource"),
				TFName: "example_resource",
				Schema: &ResourceInfo{
					Tok: "ExampleResource",
					TransformOutputs: func(
						ctx context.Context,
						pm resource.PropertyMap,
					) (resource.PropertyMap, error) {
						p := pm.Copy()
						p["stringPropertyValue"] = resource.NewStringProperty("TRANSFORMED")
						return p, nil
					},
				},
			},
		},
	}

	t.Run("Create preview", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Create",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
		    "properties": {
		      "__defaults": [],
		      "stringPropertyValue": "SOME"
		    },
		    "preview": true
		  },
		  "response": {
		    "properties": {
		      "id": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
		      "stringPropertyValue": "TRANSFORMED",
			  "__meta": "*",
			  "arrayPropertyValues": "*",
			  "boolPropertyValue": "*",
			  "floatPropertyValue": "*",
			  "nestedResources": "*",
			  "nilPropertyValue": "*",
			  "numberPropertyValue": "*",
			  "objectPropertyValue": "*",
			  "setPropertyValues": "*",
			  "stringWithBadInterpolation": "*"
		    }
		  }
		}`)
	})

	t.Run("Create", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Create",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
		    "properties": {
		      "__defaults": [],
		      "boolPropertyValue": true
		    }
		  },
		  "response": {
		    "id": "*",
		    "properties": {
		      "id": "*",
                      "stringPropertyValue": "TRANSFORMED",
		      "boolPropertyValue": "*",
		      "__meta": "*",
		      "objectPropertyValue": "*",
		      "floatPropertyValue": "*",
		      "stringPropertyValue": "*",
		      "arrayPropertyValues": "*",
		      "nestedResources": "*",
		      "numberPropertyValue": "*",
		      "setPropertyValues": "*",
		      "stringWithBadInterpolation": "*",
			  "nilPropertyValue": "*",
			  "boolPropertyValue": "*",
			  "floatPropertyValue": "*"
		    }
		  }
		}`)
	})

	t.Run("Update preview", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Update",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
		    "olds": {
		      "stringPropertyValue": "OLD"
		    },
		    "news": {
		      "stringPropertyValue": "NEW"
		    },
                    "preview": true
		  },
		  "response": {
		    "properties": {
		      "id": "*",
                      "stringPropertyValue": "TRANSFORMED",
		      "__meta": "*",
		      "objectPropertyValue": "*",
		      "floatPropertyValue": "*",
		      "stringPropertyValue": "*",
		      "arrayPropertyValues": "*",
		      "nestedResources": "*",
		      "numberPropertyValue": "*",
		      "setPropertyValues": "*",
		      "stringWithBadInterpolation": "*",
			  "nilPropertyValue": "*",
			  "boolPropertyValue": "*",
			  "floatPropertyValue": "*"
		    }
		  }
		}`)
	})

	t.Run("Update", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Update",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
		    "olds": {
		      "stringPropertyValue": "OLD"
		    },
		    "news": {
		      "stringPropertyValue": "NEW"
		    }
		  },
		  "response": {
		    "properties": {
		      "id": "*",
		      "stringPropertyValue": "TRANSFORMED",
		      "boolPropertyValue": "*",
		      "__meta": "*",
		      "objectPropertyValue": "*",
		      "floatPropertyValue": "*",
		      "arrayPropertyValues": "*",
		      "nestedResources": "*",
		      "numberPropertyValue": "*",
		      "setPropertyValues": "*",
		      "stringWithBadInterpolation": "*",
			  "nilPropertyValue": "*"
		    }
		  }
		}`)
	})

	t.Run("Read to import", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Read",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
                    "properties": {}
		  },
		  "response": {
                    "id": "0",
                    "inputs": "*",
		    "properties": {
			"id": "*",
			"stringPropertyValue": "TRANSFORMED",
			"boolPropertyValue": "*",
			"__meta": "*",
			"objectPropertyValue": "*",
			"floatPropertyValue": "*",
			"arrayPropertyValues": "*",
			"nestedResources": "*",
			"numberPropertyValue": "*",
			"setPropertyValues": "*",
			"stringWithBadInterpolation": "*",
			  "nilPropertyValue": "*"
		    }
		  }
		}`)
	})
}

func TestSkipDetailedDiff(t *testing.T) {
	t.Parallel()
	provider := func() *Provider {
		p := testprovider.CustomizedDiffProvider(func(data *schema.ResourceData) {})
		shimProvider := shimv2.NewProvider(p)
		return &Provider{
			tf:     shimProvider,
			config: shimv2.NewSchemaMap(p.Schema),
			resources: map[tokens.Type]Resource{
				"Resource": {
					TF:     shimProvider.ResourcesMap().Get("test_resource"),
					TFName: "test_resource",
					Schema: &ResourceInfo{Tok: "Resource"},
				},
				"Replace": {
					TF:     shimProvider.ResourcesMap().Get("test_replace"),
					TFName: "test_replace",
					Schema: &ResourceInfo{Tok: "Replace"},
				},
			},
		}
	}
	t.Run("Diff", func(t *testing.T) {
		testutils.Replay(t, provider(), `
                {
		  "method": "/pulumirpc.ResourceProvider/Diff",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::Resource::exres",
		    "olds": {},
		    "news": {}
		  },
		  "response": {
		    "changes": "DIFF_SOME",
		    "hasDetailedDiff": true,
			"detailedDiff": {
				"labels": {}
			},
			"diffs": ["labels"]
		  }
		}`)
	})

	t.Run("EmptyDiffWithReplace", func(t *testing.T) {
		testutils.Replay(t, provider(), `
                {
		  "method": "/pulumirpc.ResourceProvider/Diff",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::Replace::exres",
		    "olds": {},
		    "news": {}
		  },
		  "response": {
		    "changes": "DIFF_SOME",
		    "replaces": ["__meta"],
		    "hasDetailedDiff": true,
			"detailedDiff": {
				"__meta": {"kind": "UPDATE_REPLACE"},
				"labels": {}
			},
			"diffs": ["__meta", "labels"]
		  }
		}`)
	})

	t.Run("FullDiffWithReplace", func(t *testing.T) {
		testutils.Replay(t, provider(), `
                {
		  "method": "/pulumirpc.ResourceProvider/Diff",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::Replace::exres",
		    "olds": {"labels": "0"},
		    "news": {"labels": "1"}
		  },
		  "response": {
		    "changes": "DIFF_SOME",
		    "replaces": ["__meta"],
		    "hasDetailedDiff": true,
		    "detailedDiff": {
				"__meta": {"kind": "UPDATE_REPLACE"},
				"labels": {"kind": "UPDATE"}
			},
		    "diffs": ["__meta", "labels"]
		  }
		}`)
	})
}

func TestTransformFromState(t *testing.T) {
	t.Parallel()
	provider := func(t *testing.T) *Provider {
		p := testprovider.AssertProvider(func(data *schema.ResourceData) {
			// GetRawState is not available during deletes.
			if raw := data.GetRawState(); !raw.IsNull() {
				assert.Equal(t, "TRANSFORMED", raw.AsValueMap()["string_property_value"].AsString())
			}
			testprovider.MustSet(data, "string_property_value", "SET")
		})
		var called bool
		t.Cleanup(func() { assert.True(t, called, "Transform was not called") })
		shimProvider := shimv2.NewProvider(p)
		return &Provider{
			tf:     shimProvider,
			config: shimv2.NewSchemaMap(p.Schema),
			resources: map[tokens.Type]Resource{
				"Echo": {
					TF:     shimProvider.ResourcesMap().Get("echo"),
					TFName: "echo",
					Schema: &ResourceInfo{
						Tok: "Echo",
						TransformFromState: func(
							ctx context.Context,
							pm resource.PropertyMap,
						) (resource.PropertyMap, error) {
							p := pm.Copy()
							assert.Equal(t, "OLD", p["stringPropertyValue"].StringValue())
							p["stringPropertyValue"] = resource.NewStringProperty("TRANSFORMED")
							called = true
							return p, nil
						},
					},
				},
			},
		}
	}

	t.Run("Check", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
		{
		  "method": "/pulumirpc.ResourceProvider/Check",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::Echo::exres",
		    "olds": {
		      "stringPropertyValue": "OLD"
		    },
		    "news": {
		      "stringPropertyValue": "NEW"
                    }
		  },
		  "response": {
		    "inputs": {
                      "__defaults": [],
                      "stringPropertyValue": "NEW"
                    }
		  }
		}`)
	})

	t.Run("Update preview", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
		{
		  "method": "/pulumirpc.ResourceProvider/Update",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::Echo::exres",
		    "olds": {
		      "stringPropertyValue": "OLD"
		    },
		    "news": {
		      "stringPropertyValue": "NEW"
                    },
                    "preview": true
		  },
		  "response": {
		    "properties": {
		      "id": "*",
                      "stringPropertyValue": "NEW",
		      "__meta": "*"
		    }
		  }
		}`)
	})

	t.Run("Update", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
		{
		  "method": "/pulumirpc.ResourceProvider/Update",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::Echo::exres",
		    "olds": {
		      "stringPropertyValue": "OLD"
		    },
		    "news": {
		      "stringPropertyValue": "NEW"
		    }
		  },
		  "response": {
		    "properties": {
		      "id": "*",
		      "stringPropertyValue": "SET",
		      "__meta": "*"
		    }
		  }
		}`)
	})

	t.Run("Diff", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
                {
		  "method": "/pulumirpc.ResourceProvider/Diff",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::Echo::exres",
		    "olds": {
		      "stringPropertyValue": "OLD"
		    },
		    "news": {
		      "stringPropertyValue": "TRANSFORMED"
		    }
		  },
		  "response": {
		    "changes": "DIFF_NONE",
		    "hasDetailedDiff": true
		  }
               }`)
	})

	t.Run("Delete", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
                {
		  "method": "/pulumirpc.ResourceProvider/Delete",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::Echo::exres",
		    "properties": {
		      "stringPropertyValue": "OLD"
		    }
		  },
		  "response": {}
               }`)
	})

	t.Run("Read (Refresh)", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
		{
		  "method": "/pulumirpc.ResourceProvider/Read",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::Echo::exres",
	            "properties": {
                   	"stringPropertyValue": "OLD"
                    }
		  },
		  "response": {
	            "id": "0",
	            "inputs": "*",
		    "properties": {
			"id": "*",
			"stringPropertyValue": "SET",
			"__meta": "*"
		    }
		  }
		}`)
	})
}

// This emulates the situation where we migrate from a state without maxItemsOne
// which would make the property a list
// into a state with maxItemsOne, which would flatten the type.
// https://github.com/pulumi/pulumi-aws/issues/3092
func TestMaxItemOneWrongStateDiff(t *testing.T) {
	t.Parallel()
	p := testprovider.MaxItemsOneProvider()
	shimProvider := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProvider,
		config: shimv2.NewSchemaMap(p.Schema),
		resources: map[tokens.Type]Resource{
			"NestedStrRes": {
				TF:     shimProvider.ResourcesMap().Get("nested_str_res"),
				TFName: "nested_str_res",
				Schema: &ResourceInfo{
					Tok:    "NestedStrRes",
					Fields: map[string]*SchemaInfo{},
				},
			},
		},
	}
	t.Run("DiffListAndVal", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Diff",
			"request": {
				"urn": "urn:pulumi:dev::teststack::NestedStrRes::exres",
				"id": "0",
				"olds": {
					"nested_str": [""]
				},
				"news": {
					"nested_str": ""
				}
			},
			"response": {
				"changes": "DIFF_SOME",
				"hasDetailedDiff": true
			}
		}`)
	})
	t.Run("DiffNilListAndVal", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Diff",
			"request": {
				"urn": "urn:pulumi:dev::teststack::NestedStrRes::exres",
				"id": "0",
				"olds": {
					"nested_str": []
				},
				"news": {
					"nested_str": ""
				}
			},
			"response": {
				"changes": "DIFF_SOME",
				"hasDetailedDiff": true,
				"detailedDiff": {
					"nestedStr": {
					}
				},
				"diffs": ["nestedStr"]
			}
		}`)
	})
	t.Run("DiffListAndValNonEmpty", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Diff",
			"request": {
				"urn": "urn:pulumi:dev::teststack::NestedStrRes::exres",
				"id": "0",
				"olds": {
					"nested_str": ["val"]
				},
				"news": {
					"nested_str": "val"
				}
			},
			"response": {
				"changes": "DIFF_SOME",
				"hasDetailedDiff": true
			}
		}`)
	})

	// Also check that we don't produce spurious diffs when not necessary.
	t.Run("DiffValAndValEmpty", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Diff",
			"request": {
				"urn": "urn:pulumi:dev::teststack::NestedStrRes::exres",
				"id": "0",
				"olds": {
					"nested_str": ""
				},
				"news": {
					"nested_str": ""
				}
			},
			"response": {
				"changes": "DIFF_NONE",
				"hasDetailedDiff": true
			}
		}`)
	})
	t.Run("DiffValAndValNonempty", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Diff",
			"request": {
				"urn": "urn:pulumi:dev::teststack::NestedStrRes::exres",
				"id": "0",
				"olds": {
					"nested_str": "val"
				},
				"news": {
					"nested_str": "val"
				}
			},
			"response": {
				"changes": "DIFF_NONE",
				"hasDetailedDiff": true
			}
		}`)
	})
}

// These should test that we validate resources before applying TF defaults,
// since this is what TF does.
// https://github.com/pulumi/pulumi-terraform-bridge/issues/1546
func TestDefaultsAndConflictsWithValidationInteraction(t *testing.T) {
	t.Parallel()
	p := testprovider.ConflictsWithValidationProvider()
	shimProvider := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProvider,
		config: shimv2.NewSchemaMap(p.Schema),
		resources: map[tokens.Type]Resource{
			"DefaultValueRes": {
				TF:     shimProvider.ResourcesMap().Get("default_value_res"),
				TFName: "default_value_res",
				Schema: &ResourceInfo{},
			},
		},
	}

	t.Run("CheckMissingRequiredProp", func(t *testing.T) {
		//nolint:lll
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::DefaultValueRes::exres",
				"olds": {},
				"news": {},
				"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
			},
			"response": {
				"inputs": {
					"__defaults": []
				},
				"failures": [
					{ "reason": "Missing required argument. The argument \"conflicting_required_property\" is required, but no definition was found.. Examine values at 'exres.conflictingRequiredProperty'."
					}
				]
			}
		}`)
	})

	t.Run("CheckRequiredDoesNotConflict", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::DefaultValueRes::exres",
				"olds": {},
				"news": {
					"conflicting_required_property": "required"
				},
				"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"conflictingRequiredProperty": "required"
				}
			}
		}`)
	})
}

// https://github.com/pulumi/pulumi-terraform-bridge/issues/1546
func TestDefaultsAndExactlyOneOfValidationInteraction(t *testing.T) {
	t.Parallel()
	p := testprovider.ExactlyOneOfValidationProvider()
	shimProvider := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProvider,
		config: shimv2.NewSchemaMap(p.Schema),
		resources: map[tokens.Type]Resource{
			"DefaultValueRes": {
				TF:     shimProvider.ResourcesMap().Get("default_value_res"),
				TFName: "default_value_res",
				Schema: &ResourceInfo{},
			},
		},
	}
	t.Run("CheckFailsWhenExactlyOneOfNotSpecified", func(t *testing.T) {
		//nolint:lll
		testutils.Replay(t, provider, strings.ReplaceAll(`
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::DefaultValueRes::exres",
				"olds": {},
				"news": {
				},
				"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
			},
			"response": {
				"inputs": {
					"__defaults": []
				},
				"failures": [
					{"reason": "Invalid combination of arguments. \"exactly_one_of_nonrequired_property2\": one of $exactly_one_of_nonrequired_property2,exactly_one_of_required_property$ must be specified. Examine values at 'exres.exactlyOneOfNonrequiredProperty2'."},
					{"reason": "Invalid combination of arguments. \"exactly_one_of_property\": one of $exactly_one_of_property,exactly_one_of_property2$ must be specified. Examine values at 'exres.exactlyOneOfProperty'."},
					{"reason": "Invalid combination of arguments. \"exactly_one_of_property2\": one of $exactly_one_of_property,exactly_one_of_property2$ must be specified. Examine values at 'exres.exactlyOneOfProperty2'."},
					{"reason": "Invalid combination of arguments. \"exactly_one_of_required_property\": one of $exactly_one_of_nonrequired_property2,exactly_one_of_required_property$ must be specified. Examine values at 'exres.exactlyOneOfRequiredProperty'."}
				]
			}
		}`, "$", "`"))
	})

	t.Run("Check", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::DefaultValueRes::exres",
				"olds": {},
				"news": {
					"exactlyOneOfProperty": "exactly_one_value",
					"exactlyOneOfRequiredProperty": "exactly_one_req_value"
				},
				"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"exactlyOneOfProperty": "exactly_one_value",
					"exactlyOneOfRequiredProperty": "exactly_one_req_value"
				}
			}
		}`)
	})
}

// https://github.com/pulumi/pulumi-terraform-bridge/issues/1546
func TestDefaultsAndRequiredWithValidationInteraction(t *testing.T) {
	t.Parallel()
	p := testprovider.RequiredWithValidationProvider()
	shimProvider := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProvider,
		config: shimv2.NewSchemaMap(p.Schema),
		resources: map[tokens.Type]Resource{
			"DefaultValueRes": {
				TF:     shimProvider.ResourcesMap().Get("default_value_res"),
				TFName: "default_value_res",
				Schema: &ResourceInfo{},
			},
		},
	}

	t.Run("CheckMissingRequiredPropErrors", func(t *testing.T) {
		//nolint:lll
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::DefaultValueRes::exres",
				"olds": {},
				"news": {},
				"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
			},
			"response": {
				"inputs": {
					"__defaults": [
						"requiredWithNonrequiredProperty",
						"requiredWithProperty",
						"requiredWithProperty2",
						"requiredWithRequiredProperty",
						"requiredWithRequiredProperty2",
						"requiredWithRequiredProperty3"
					],
					"requiredWithNonrequiredProperty": "",
					"requiredWithProperty": "",
					"requiredWithProperty2": "",
					"requiredWithRequiredProperty": "",
					"requiredWithRequiredProperty2": "",
					"requiredWithRequiredProperty3": ""
				},
				"failures": [
					{"reason": "Missing required argument. The argument \"required_with_required_property\" is required, but no definition was found.. Examine values at 'exres.requiredWithRequiredProperty'."},
					{"reason": "Missing required argument. The argument \"required_with_required_property2\" is required, but no definition was found.. Examine values at 'exres.requiredWithRequiredProperty2'."},
					{"reason": "Missing required argument. The argument \"required_with_required_property3\" is required, but no definition was found.. Examine values at 'exres.requiredWithRequiredProperty3'."}
				]
			}
		}`)
	})

	t.Run("CheckHappyPath", func(t *testing.T) {
		testutils.Replay(t, provider, `
			{
				"method": "/pulumirpc.ResourceProvider/Check",
				"request": {
					"urn": "urn:pulumi:dev::teststack::DefaultValueRes::exres",
					"olds": {},
					"news": {
						"requiredWithNonrequiredProperty": "foo",
						"requiredWithProperty": "foo",
						"requiredWithProperty2": "foo",
						"requiredWithRequiredProperty": "foo",
						"requiredWithRequiredProperty2": "foo",
						"requiredWithRequiredProperty3": "foo"
					},
					"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
				},
				"response": {
					"inputs": {
						"__defaults": [],
						"requiredWithNonrequiredProperty": "foo",
						"requiredWithProperty": "foo",
						"requiredWithProperty2": "foo",
						"requiredWithRequiredProperty": "foo",
						"requiredWithRequiredProperty2": "foo",
						"requiredWithRequiredProperty3": "foo"
					}
				}
			}`)
	})

	t.Run("CheckMissingRequiredWith", func(t *testing.T) {
		//nolint:lll
		testutils.Replay(t, provider, strings.ReplaceAll(`
			{
				"method": "/pulumirpc.ResourceProvider/Check",
				"request": {
					"urn": "urn:pulumi:dev::teststack::DefaultValueRes::exres",
					"olds": {},
					"news": {
						"requiredWithProperty": "foo",
						"requiredWithRequiredProperty": "foo",
						"requiredWithRequiredProperty2": "foo"
					},
					"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
				},
				"response": {
					"inputs": {
						"__defaults": [
							"requiredWithNonrequiredProperty",
							"requiredWithProperty2",
							"requiredWithRequiredProperty3"
						],
						"requiredWithNonrequiredProperty": "",
						"requiredWithProperty": "foo",
						"requiredWithProperty2": "",
						"requiredWithRequiredProperty": "foo",
						"requiredWithRequiredProperty2": "foo",
						"requiredWithRequiredProperty3": ""
					},
					"failures": [
						{"reason": "Missing required argument. \"required_with_property\": all of $required_with_property,required_with_property2$ must be specified. Examine values at 'exres.requiredWithProperty'."},
						{"reason": "Missing required argument. \"required_with_required_property\": all of $required_with_nonrequired_property,required_with_required_property$ must be specified. Examine values at 'exres.requiredWithRequiredProperty'."},
						{"reason": "Missing required argument. \"required_with_required_property2\": all of $required_with_required_property2,required_with_required_property3$ must be specified. Examine values at 'exres.requiredWithRequiredProperty2'."},
						{"reason": "Missing required argument. The argument \"required_with_required_property3\" is required, but no definition was found.. Examine values at 'exres.requiredWithRequiredProperty3'."}
					]
				}
			}`, "$", "`"))
	})
}

func TestSetAutoNaming(t *testing.T) {
	t.Parallel()
	tok := func(name string) tokens.Type { return MakeResource("auto", "index", name) }
	prov := &ProviderInfo{
		Name: "auto",
		P:    shimv2.NewProvider(testprovider.AutonamingProvider()),
		Resources: map[string]*ResourceInfo{
			"auto_v1": {Tok: tok("v1")},
			"auto_v2": {Tok: tok("v2")},
		},
	}

	prov.SetAutonaming(10, "-")

	assert.True(t, prov.Resources["auto_v1"].Fields["name"].Default.AutoNamed) // Of type string - so applied
	assert.Nil(t, prov.Resources["auto_v2"].Fields["name"])                    // Of type list - so not applied
}

func TestPreConfigureCallbackEmitsFailures(t *testing.T) {
	t.Parallel()
	t.Run("can_emit_failure", func(t *testing.T) {
		p := testprovider.ProviderV2()
		shimProv := shimv2.NewProvider(p)
		provider := &Provider{
			tf:     shimProv,
			config: shimv2.NewSchemaMap(p.Schema),
			info: ProviderInfo{
				P: shimProv,
				PreConfigureCallbackWithLogger: func(
					ctx context.Context,
					host *provider.HostClient, vars resource.PropertyMap,
					config shim.ResourceConfig,
				) error {
					return CheckFailureError{
						[]CheckFailureErrorElement{
							{
								Reason:   "failure reason",
								Property: "",
							},
						},
					}
				},
			},
		}

		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/CheckConfig",
			"request": {
				"urn": "urn:pulumi:dev::aws_no_creds::pulumi:providers:aws::default_6_18_2",
				"olds": {},
				"news": { "version": "6.18.2" }
			},
			"response": {
				"failures": [
					{
						"reason": "failure reason"
					}
				]
			}
		}`)
	})

	t.Run("can_emit_multiple_failures", func(t *testing.T) {
		p := testprovider.ProviderV2()
		shimProv := shimv2.NewProvider(p)
		provider := &Provider{
			tf:     shimProv,
			config: shimv2.NewSchemaMap(p.Schema),
			info: ProviderInfo{
				P: shimProv,
				PreConfigureCallbackWithLogger: func(
					ctx context.Context,
					host *provider.HostClient, vars resource.PropertyMap,
					config shim.ResourceConfig,
				) error {
					return CheckFailureError{
						[]CheckFailureErrorElement{
							{
								Reason:   "failure reason",
								Property: "",
							},
							{
								Reason:   "failure reason 2",
								Property: "",
							},
						},
					}
				},
			},
		}

		// NOTE: failure reasons are sorted as an artifact of testutils.Replay trying not to
		// care about ordering for CheckFailures, and are actually returned as-is.
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/CheckConfig",
			"request": {
				"urn": "urn:pulumi:dev::aws_no_creds::pulumi:providers:aws::default_6_18_2",
				"olds": {},
				"news": { "version": "6.18.2" }
			},
			"response": {
				"failures": [
					{
						"reason": "failure reason"
					},
					{
						"reason": "failure reason 2"
					}
				]
			}
		}`)
	})

	t.Run("can_error", func(t *testing.T) {
		p := testprovider.ProviderV2()
		shimProv := shimv2.NewProvider(p)
		provider := &Provider{
			tf:     shimProv,
			config: shimv2.NewSchemaMap(p.Schema),
			info: ProviderInfo{
				P: shimProv,
				PreConfigureCallbackWithLogger: func(
					ctx context.Context,
					host *provider.HostClient, vars resource.PropertyMap,
					config shim.ResourceConfig,
				) error {
					return fmt.Errorf("error")
				},
			},
		}

		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/CheckConfig",
			"request": {
				"urn": "urn:pulumi:dev::aws_no_creds::pulumi:providers:aws::default_6_18_2",
				"olds": {},
				"news": { "version": "6.18.2" }
			},
			"errors": ["error"]
		}`)
	})
}

func TestImport(t *testing.T) {
	t.Parallel()

	testImport(t, func(p *schema.Provider) shim.Provider {
		return shimv2.NewProvider(p)
	})
}

func testImport(t *testing.T, newProvider func(*schema.Provider) shim.Provider) {
	init := func(rcf schema.ReadContextFunc) *Provider {
		p := testprovider.ProviderV2()
		er := p.ResourcesMap["example_resource"]
		er.Read = nil //nolint
		er.ReadContext = rcf
		er.Importer = &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		}
		er.Schema = map[string]*schema.Schema{
			"string_property_value": {Type: schema.TypeString, Optional: true},
		}
		shimProv := newProvider(p)
		provider := &Provider{
			tf:     shimProv,
			config: shimv2.NewSchemaMap(p.Schema),
			info: ProviderInfo{
				P:              shimProv,
				ResourcePrefix: "example",
				Resources: map[string]*ResourceInfo{
					"example_resource":       {Tok: "ExampleResource"},
					"second_resource":        {Tok: "SecondResource"},
					"nested_secret_resource": {Tok: "NestedSecretResource"},
				},
			},
		}
		provider.initResourceMaps()
		return provider
	}

	t.Run("import", func(t *testing.T) {
		provider := init(func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			require.NoError(t, rd.Set("string_property_value", "imported"))
			return diag.Diagnostics{}
		})

		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Read",
		  "request": {
		    "id": "res1",
		    "urn": "urn:pulumi:dev::mystack::ExampleResource::res1name",
		    "properties": {}
		  },
		  "response": {
		    "inputs": {
		      "__defaults": [],
		      "stringPropertyValue": "imported"
		    },
		    "properties": {
		      "id": "res1",
		      "stringPropertyValue": "imported",
		      "__meta": "{\"e2bfb730-ecaa-11e6-8f88-34363bc7c4c0\":{\"create\":120000000000},\"schema_version\":\"1\"}"
		    },
		    "id": "res1"
		  }
		}`)
	})

	t.Run("import-not-found", func(t *testing.T) {
		provider := init(func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			rd.SetId("") // emulate not found
			return diag.Diagnostics{}
		})
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Read",
		  "request": {
		    "id": "res1",
		    "urn": "urn:pulumi:dev::mystack::ExampleResource::res1name",
		    "properties": {}
		  },
		  "response": {}
		}`)
	})
}

func TestRefresh(t *testing.T) {
	t.Parallel()
	testRefresh(t, func(p *schema.Provider) shim.Provider {
		return shimv2.NewProvider(p)
	})
}

func testRefresh(t *testing.T, newProvider func(*schema.Provider) shim.Provider) {
	init := func(rcf schema.ReadContextFunc) *Provider {
		p := testprovider.ProviderV2()
		er := p.ResourcesMap["example_resource"]
		er.Read = nil //nolint
		er.ReadContext = rcf
		er.Schema = map[string]*schema.Schema{
			"string_property_value": {Type: schema.TypeString, Optional: true},
		}
		shimProv := newProvider(p)
		provider := &Provider{
			tf:     shimProv,
			config: shimv2.NewSchemaMap(p.Schema),
			info: ProviderInfo{
				P:              shimProv,
				ResourcePrefix: "example",
				Resources: map[string]*ResourceInfo{
					"example_resource":       {Tok: "ExampleResource"},
					"second_resource":        {Tok: "SecondResource"},
					"nested_secret_resource": {Tok: "NestedSecretResource"},
				},
			},
		}
		provider.initResourceMaps()
		return provider
	}

	t.Run("refresh", func(t *testing.T) {
		provider := init(func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			require.NoError(t, rd.Set("string_property_value", "imported"))
			return diag.Diagnostics{}
		})

		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Read",
		  "request": {
		    "id": "res1",
		    "urn": "urn:pulumi:dev::mystack::ExampleResource::res1name",
		    "properties": {"stringPropertyValue": "old"},
                    "inputs": {"stringPropertyValue": "old"}
		  },
		  "response": {
		    "inputs": {
		      "stringPropertyValue": "imported"
		    },
		    "properties": {
		      "id": "res1",
		      "stringPropertyValue": "imported",
		      "__meta": "{\"schema_version\":\"1\"}"
		    },
		    "id": "res1"
		  }
		}`)
	})

	t.Run("refresh-not-found", func(t *testing.T) {
		provider := init(func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			rd.SetId("") // emulate not found
			return diag.Diagnostics{}
		})
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Read",
		  "request": {
		    "id": "res1",
		    "urn": "urn:pulumi:dev::mystack::ExampleResource::res1name",
		    "properties": {"stringPropertyValue": "old"},
                    "inputs": {"stringPropertyValue": "old"}
		  },
		  "response": {}
		}`)
	})

	t.Run("refresh-read-unchanged-archive", func(t *testing.T) {
		provider := init(func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			return diag.Diagnostics{}
		})
		provider.info.Resources["example_resource"].Fields = map[string]*SchemaInfo{
			"string_property_value": {
				Asset: &AssetTranslation{
					Kind: FileAsset,
				},
			},
		}

		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Read",
		  "request": {
		    "id": "someres",
		    "urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
		    "properties": {
		      "stringPropertyValue": {
			"4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
			"hash": "a72e573d8c91ec1c6bb0dfdf641bc2de1e2417c0d980ecfcdf039c2a9bcbbf67"
		      }
		    },
		    "inputs": {
		      "stringPropertyValue": {
			"4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
			"hash": "a72e573d8c91ec1c6bb0dfdf641bc2de1e2417c0d980ecfcdf039c2a9bcbbf67"
		      }
		    }
		  },
		  "response": {
		    "id": "someres",
		    "properties": {
                      "id": "someres",
                      "__meta": "*",
		      "stringPropertyValue": {
			"4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
			"hash": "a72e573d8c91ec1c6bb0dfdf641bc2de1e2417c0d980ecfcdf039c2a9bcbbf67"
		      }
		    },
		    "inputs": {
		      "stringPropertyValue": {
			"4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
			"hash": "a72e573d8c91ec1c6bb0dfdf641bc2de1e2417c0d980ecfcdf039c2a9bcbbf67"
		      }
		    }
		  }
		}`)
	})
}

func TestDestroy(t *testing.T) {
	t.Parallel()

	testDestroy(t, func(p *schema.Provider) shim.Provider {
		return shimv2.NewProvider(p)
	})
}

func testDestroy(t *testing.T, newProvider func(*schema.Provider) shim.Provider) {
	init := func(dcf schema.DeleteContextFunc) *Provider {
		p := testprovider.ProviderV2()
		er := p.ResourcesMap["example_resource"]
		er.Schema = map[string]*schema.Schema{
			"string_property_value": {Type: schema.TypeString, Optional: true},
		}
		er.Delete = nil //nolint
		er.DeleteContext = dcf
		shimProv := newProvider(p)
		provider := &Provider{
			tf:     shimProv,
			config: shimv2.NewSchemaMap(p.Schema),
			info: ProviderInfo{
				P:              shimProv,
				ResourcePrefix: "example",
				Resources: map[string]*ResourceInfo{
					"example_resource":       {Tok: "ExampleResource"},
					"second_resource":        {Tok: "SecondResource"},
					"nested_secret_resource": {Tok: "NestedSecretResource"},
				},
			},
		}
		provider.initResourceMaps()
		return provider
	}

	t.Run("destroy", func(t *testing.T) {
		called := 0
		provider := init(func(
			ctx context.Context, rd *schema.ResourceData, i interface{},
		) diag.Diagnostics {
			called++
			return diag.Diagnostics{}
		})

		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Delete",
		  "request": {
		    "id": "res1",
		    "urn": "urn:pulumi:dev::mystack::ExampleResource::res1name",
		    "properties": {"stringPropertyValue": "old"}
		  },
		  "response": {}
		}`)
		require.Equal(t, 1, called)
	})
}

func TestSchemaFuncsNotCalledDuringRuntime(t *testing.T) {
	t.Parallel()
	p := testprovider.SchemaFuncPanicsProvider()
	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProv,
		config: shimv2.NewSchemaMap(p.Schema),
		info: ProviderInfo{
			P: shimProv,
		},
	}

	t.Run("Schema func not called if validate disabled", func(t *testing.T) {
		schema.RunProviderInternalValidation = false
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/CheckConfig",
			"request": {
				"urn": "urn:pulumi:dev::aws_no_creds::pulumi:providers:aws::default_6_18_2",
				"olds": {},
				"news": { "version": "6.18.2" }
			},
			"response": {
				"inputs": {
					"version": "6.18.2"
				}
			}
		}`)
	})

	t.Run("Schema func panic if validate enabled", func(t *testing.T) {
		schema.RunProviderInternalValidation = true
		defer func() {
			r := recover()
			if r.(string) != "schema func panic" {
				t.Errorf("Wrong panic: %v", r)
			}
		}()
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/CheckConfig",
			"request": {
				"urn": "urn:pulumi:dev::aws_no_creds::pulumi:providers:aws::default_6_18_2",
				"olds": {},
				"news": { "version": "6.18.2" }
			},
			"response": {
				"inputs": {
					"version": "6.18.2"
				}
			}
		}`)
		t.Errorf("The code did not panic!")
	})
}

func TestMaxItemsOneConflictsWith(t *testing.T) {
	t.Parallel()
	p := &schemav2.Provider{
		Schema: map[string]*schemav2.Schema{},
		ResourcesMap: map[string]*schemav2.Resource{
			"res": {
				Schema: map[string]*schemav2.Schema{
					"max_items_one_prop": {
						Type:          schemav2.TypeList,
						MaxItems:      1,
						Elem:          &schemav2.Schema{Type: schemav2.TypeString},
						Optional:      true,
						ConflictsWith: []string{"other_prop"},
					},
					"other_prop": {
						Type:          schemav2.TypeString,
						Optional:      true,
						ConflictsWith: []string{"max_items_one_prop"},
					},
				},
			},
		},
	}
	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProv,
		config: shimv2.NewSchemaMap(p.Schema),
		info: ProviderInfo{
			P: shimProv,
		},
		resources: map[tokens.Type]Resource{
			"Res": {
				TF:     shimProv.ResourcesMap().Get("res"),
				TFName: "res",
				Schema: &ResourceInfo{},
			},
		},
	}

	t.Run("No conflict when other specified", func(t *testing.T) {
		testutils.ReplaySequence(t, provider, `[
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {
				"args": {},
				"variables": {}
			  },
			  "response": {
				"supportsPreview": true,
				"supportsAutonamingConfiguration": true
			  }
			},
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {},
				"news": {
				  "other_prop": "other"
				},
				"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
			  },
			  "response": {
				"inputs": {
				  "__defaults": [],
				  "otherProp": "other"
				}
			  }
			}
		  ]
		  `)
	})

	t.Run("No conflict when no props specified", func(t *testing.T) {
		testutils.ReplaySequence(t, provider, `[
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {
				"args": {},
				"variables": {}
			  },
			  "response": {
				"supportsPreview": true,
				"supportsAutonamingConfiguration": true
			  }
			},
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {},
				"news": {
				},
				"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
			  },
			  "response": {
				"inputs": {
				  "__defaults": []
				}
			  }
			}
		  ]
		  `)
	})
}

func TestMinMaxItemsOneOptional(t *testing.T) {
	t.Parallel()
	p := &schemav2.Provider{
		Schema: map[string]*schemav2.Schema{},
		ResourcesMap: map[string]*schemav2.Resource{
			"res": {
				Schema: map[string]*schemav2.Schema{
					"max_items_one_prop": &schema.Schema{
						Type:     schema.TypeSet,
						Optional: true,
						MaxItems: 1,
						MinItems: 1,
						Elem:     &schemav2.Schema{Type: schemav2.TypeString},
					},
				},
			},
		},
	}
	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProv,
		config: shimv2.NewSchemaMap(p.Schema),
		info: ProviderInfo{
			P: shimProv,
		},
		resources: map[tokens.Type]Resource{
			"Res": {
				TF:     shimProv.ResourcesMap().Get("res"),
				TFName: "res",
				Schema: &ResourceInfo{},
			},
		},
	}

	t.Run("No error when not specified", func(t *testing.T) {
		testutils.ReplaySequence(t, provider, `[
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {
				"args": {},
				"variables": {}
			  },
			  "response": {
				"supportsPreview": true,
				"supportsAutonamingConfiguration": true
			  }
			},
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {},
				"news": {
				},
				"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
			  },
			  "response": {
				"inputs": {
				  "__defaults": []
				}
			  }
			}
		  ]
		  `)
	})

	t.Run("No error when specified", func(t *testing.T) {
		testutils.ReplaySequence(t, provider, `[
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {
				"args": {},
				"variables": {}
			  },
			  "response": {
				"supportsPreview": true,
				"supportsAutonamingConfiguration": true
			  }
			},
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {},
				"news": {
					"max_items_one_prop": ["prop"]
				},
				"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
			  },
			  "response": {
				"inputs": {
				  "__defaults": [],
				  "maxItemsOneProp": "prop"
				}
			  }
			}
		  ]
		  `)
	})
}

func TestComputedMaxItemsOneNotSpecified(t *testing.T) {
	t.Parallel()
	p := &schemav2.Provider{
		Schema: map[string]*schemav2.Schema{},
		ResourcesMap: map[string]*schemav2.Resource{
			"res": {
				Schema: map[string]*schemav2.Schema{
					"specs": {
						Computed: true,
						MaxItems: 1,
						Type:     schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"disk": {
									Type:     schema.TypeInt,
									Computed: true,
								},
							},
						},
					},
				},
			},
		},
	}
	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProv,
		config: shimv2.NewSchemaMap(p.Schema),
		info: ProviderInfo{
			P: shimProv,
		},
		resources: map[tokens.Type]Resource{
			"Res": {
				TF:     shimProv.ResourcesMap().Get("res"),
				TFName: "res",
				Schema: &ResourceInfo{},
			},
		},
	}

	t.Run("Computed property not specified", func(t *testing.T) {
		testutils.ReplaySequence(t, provider, `[
			{
			  "method": "/pulumirpc.ResourceProvider/Configure",
			  "request": {
				"args": {},
				"variables": {}
			  },
			  "response": {
				"supportsPreview": true,
				"supportsAutonamingConfiguration": true
			  }
			},
			{
			  "method": "/pulumirpc.ResourceProvider/Check",
			  "request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {},
				"news": {
				},
				"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
			  },
			  "response": {
				"inputs": {
				  "__defaults": []
				}
			  }
			}
		  ]
		  `)
	})
}

func TestProviderConfigMinMaxItemsOne(t *testing.T) {
	t.Parallel()
	p := &schemav2.Provider{
		Schema: map[string]*schemav2.Schema{
			"max_items_one_config": {
				Type:     schemav2.TypeList,
				Elem:     &schemav2.Schema{Type: schemav2.TypeString},
				MaxItems: 1,
				MinItems: 1,
				Optional: true,
			},
		},
	}
	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:        shimProv,
		config:    shimv2.NewSchemaMap(p.Schema),
		info:      ProviderInfo{P: shimProv},
		resources: map[tokens.Type]Resource{},
	}

	t.Run("No error when config not specified", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {}
		  },
		  "response": {
		    "inputs": {}
		  }
		}`)
	})
}

func TestProviderCheckConfigRequiredDefaultEnvConfig(t *testing.T) {
	t.Setenv("REQUIRED_CONFIG", "required")
	// Note that this config should be invalid.
	//
	// From the Required docs: Required cannot be used with Computed Default, DefaultFunc
	// attributes in a Provider schema
	//
	// From the DefaultFunc docs: For legacy reasons, DefaultFunc can be used with Required
	//
	// This is needed right now since some providers (e.g. Azure) depend on this.
	p := &schemav2.Provider{
		Schema: map[string]*schemav2.Schema{
			"required_env": {
				Type:        schemav2.TypeString,
				Required:    true,
				DefaultFunc: schemav2.EnvDefaultFunc("REQUIRED_CONFIG", nil),
			},
			// This is actually invalid!
			// "required": {
			// 	Type:     schemav2.TypeString,
			// 	Required: true,
			// 	Default:  "default",
			// },
		},
	}
	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:        shimProv,
		config:    shimv2.NewSchemaMap(p.Schema),
		info:      ProviderInfo{P: shimProv},
		resources: map[tokens.Type]Resource{},
	}

	t.Run("No error with env config", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/CheckConfig",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::pulumi:providers:testprovider::test",
		    "olds": {},
		    "news": {}
		  },
		  "response": {
		    "inputs": {}
		  }
		}`)
	})
}

func TestMaxItemsOnePropCheckResponseNoNulls(t *testing.T) {
	t.Parallel()
	p := &schemav2.Provider{
		Schema: map[string]*schemav2.Schema{},
		ResourcesMap: map[string]*schemav2.Resource{
			"res": {
				Schema: map[string]*schemav2.Schema{
					"networkRulesets": {
						Computed: true,
						Optional: true,
						MaxItems: 1,
						Type:     schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"defaultAction": {
									Type:     schema.TypeString,
									Required: true,
								},
							},
						},
					},
				},
			},
		},
	}
	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProv,
		config: shimv2.NewSchemaMap(p.Schema),
		info:   ProviderInfo{P: shimProv},
		resources: map[tokens.Type]Resource{
			"Res": {
				TF:     shimProv.ResourcesMap().Get("res"),
				TFName: "res",
				Schema: &ResourceInfo{},
			},
		},
	}

	t.Run("Check includes no nulls in response for unspecified props", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {
					"__defaults": [],
					"networkRulesets": null
				},
				"news": {},
				"randomSeed": "zjSL8IMF68r5aLLepOpsIT53uBTbkDryYFDnHQHkjko="
			},
			"response": {
				"inputs": {
					"__defaults": []
				}
			}
		}`)
	})
}

// TODO[pulumi/pulumi#15636] if/when Pulumi supports customizing Read timeouts these could be added here.
func TestCustomTimeouts(t *testing.T) {
	t.Parallel()
	// TODO[pulumi/pulumi-terraform-bridge#2386]
	t.Skipf("Skipping test until pulumi/pulumi-terraform-bridge#2386 is resolved")
	t.Parallel()

	type testCase struct {
		name                 string
		cud                  string // Create, Update, or Delete
		schemaTimeout        *time.Duration
		userSpecifiedTimeout *time.Duration
	}

	testCases := []testCase{}

	sec1 := 1 * time.Second
	sec2 := 2 * time.Second
	timeouts := []*time.Duration{nil, &sec1, &sec2}
	cuds := []string{"Create", "Update", "Delete"}

	for _, schemaTimeout := range timeouts {
		for _, userSpecifiedTimeout := range timeouts {
			// It seems that schema timeout must be non-nil to permit customizing user timeout; omit these
			// for now as the current behavior is falling back to 20m default.
			if userSpecifiedTimeout != nil && schemaTimeout == nil {
				continue
			}
			for _, cud := range cuds {
				n := fmt.Sprintf("%s-schema-%v-user-%v", cud, schemaTimeout, userSpecifiedTimeout)
				testCases = append(testCases, testCase{
					cud:                  cud,
					name:                 n,
					schemaTimeout:        schemaTimeout,
					userSpecifiedTimeout: userSpecifiedTimeout,
				})
			}
		}
	}

	seconds := func(d *time.Duration) float64 {
		if d == nil {
			return 0
		}
		return d.Seconds()
	}

	actualTimeout := func(tc testCase) *time.Duration {
		var capturedTimeout *time.Duration

		tok := "testprov:index:TestRes"
		urn := fmt.Sprintf("urn:pulumi:dev::teststack::%s::testresource", tok)
		id := "r1"

		upstreamProvider := &schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"testprov_testres": {
					Schema: map[string]*schema.Schema{
						"x": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
					Timeouts: &schema.ResourceTimeout{
						Default: tc.schemaTimeout,
						Create:  tc.schemaTimeout,
						Update:  tc.schemaTimeout,
						Delete:  tc.schemaTimeout,
					},
					CreateContext: func(
						ctx context.Context, rd *schema.ResourceData, i interface{},
					) diag.Diagnostics {
						t := rd.Timeout(schema.TimeoutCreate)
						capturedTimeout = &t
						rd.SetId(id)
						return diag.Diagnostics{}
					},
					UpdateContext: func(
						ctx context.Context, rd *schema.ResourceData, i interface{},
					) diag.Diagnostics {
						t := rd.Timeout(schema.TimeoutUpdate)
						capturedTimeout = &t
						return diag.Diagnostics{}
					},
					DeleteContext: func(
						ctx context.Context, rd *schema.ResourceData, i interface{},
					) diag.Diagnostics {
						t := rd.Timeout(schema.TimeoutDelete)
						capturedTimeout = &t
						return diag.Diagnostics{}
					},
				},
			},
		}

		providerInfo := ProviderInfo{
			Name: "testprov",
			Resources: map[string]*ResourceInfo{
				"testprov_testres": {
					Tok: tokens.Type(tok),
				},
			},
		}

		shimmedProvider := shimv2.NewProvider(upstreamProvider)

		bridgedProvider := &Provider{
			tf:   shimmedProvider,
			info: providerInfo,
		}

		bridgedProvider.initResourceMaps()

		switch tc.cud {
		case "Create":
			_, err := bridgedProvider.Create(context.Background(), &pulumirpc.CreateRequest{
				Urn:        urn,
				Properties: &structpb.Struct{},
				Timeout:    seconds(tc.userSpecifiedTimeout),
			})
			require.NoError(t, err)
		case "Update":
			// When testing the Update case it is required that olds != news because otherwise the
			// implementation assigns prior state to proposed state to produce a no-op diff and ignores
			// custom timeouts.
			olds, err := structpb.NewStruct(map[string]interface{}{
				"x": "x1",
			})
			require.NoError(t, err)
			news, err := structpb.NewStruct(map[string]interface{}{
				"x": "x2",
			})
			require.NoError(t, err)
			_, err = bridgedProvider.Update(context.Background(), &pulumirpc.UpdateRequest{
				Id:      id,
				Urn:     urn,
				Olds:    olds,
				News:    news,
				Timeout: seconds(tc.userSpecifiedTimeout),
			})
			require.NoError(t, err)
		case "Delete":
			_, err := bridgedProvider.Delete(context.Background(), &pulumirpc.DeleteRequest{
				Id:         id,
				Urn:        urn,
				Properties: &structpb.Struct{},
				Timeout:    seconds(tc.userSpecifiedTimeout),
			})
			require.NoError(t, err)
		}

		return capturedTimeout
	}

	expectedTimeout := func(tc testCase) time.Duration {
		if tc.schemaTimeout == nil && tc.userSpecifiedTimeout == nil {
			return 20 * time.Minute
		}
		if tc.userSpecifiedTimeout != nil {
			return *tc.userSpecifiedTimeout
		}
		return *tc.schemaTimeout
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			a := actualTimeout(tc)
			require.NotNil(t, a)
			assert.Equal(t, expectedTimeout(tc), *a)
		})
	}
}

func TestIgnoreMappings(t *testing.T) {
	t.Parallel()

	provider := func(
		ignoredMappings []string,
		resources map[string]*ResourceInfo,
		datasources map[string]*DataSourceInfo,
	) *Provider {
		p := &Provider{
			info: ProviderInfo{
				ResourcePrefix: "test",
				Resources:      resources,
				DataSources:    datasources,
				IgnoreMappings: ignoredMappings,
			},
			tf: shimv2.NewProvider(&schemav2.Provider{
				ResourcesMap: map[string]*schemav2.Resource{
					"test_r1":     {DeprecationMessage: "r1"},
					"test_r2":     {DeprecationMessage: "r2"},
					"alt_ignored": {DeprecationMessage: "r-alt"},
				},
				DataSourcesMap: map[string]*schemav2.Resource{
					"test_r1":     {DeprecationMessage: "d1"},
					"test_r2":     {DeprecationMessage: "d2"},
					"alt_ignored": {DeprecationMessage: "d-alt"},
				},
			}),
		}

		p.initResourceMaps()
		return p
	}

	// This panic isn't necessary to lock in. It's not "desired behavior", but we
	// should be thoughtful if and when we change it. We should surface an error our
	// users when this happens.
	t.Run("panics on alt_ignored not mapped", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() { provider(nil, nil, nil) })
	})

	t.Run("no panic on ignored mappings", func(t *testing.T) {
		t.Parallel()
		p := provider([]string{"alt_ignored"}, nil, nil)
		for _, r := range p.resources {
			assert.NotEqual(t, r.TF.DeprecationMessage(), "r-alt")
		}
	})

	t.Run("can override alt mappings", func(t *testing.T) {
		t.Parallel()
		p := provider(nil, map[string]*ResourceInfo{
			"alt_ignored": {Tok: "test:AltRes"},
		}, map[string]*DataSourceInfo{
			"alt_ignored": {Tok: "test:AltDs"},
		})

		// Check resource
		r, ok := p.resources["test:AltRes"]
		if assert.True(t, ok) {
			assert.Equal(t, r.TF.DeprecationMessage(), "r-alt")
		}

		// Check datasource
		d, ok := p.dataSources["test:AltDs"]
		if assert.True(t, ok) {
			assert.Equal(t, d.TF.DeprecationMessage(), "d-alt")
		}
	})

	t.Run("partial override of ignored mappings", func(t *testing.T) {
		t.Parallel()
		p := provider([]string{"alt_ignored", "test_r1"},
			map[string]*ResourceInfo{
				"test_r1": {Tok: "test:index:R1"},
			}, nil)

		// Check resource
		r, ok := p.resources["test:index:R1"]
		if assert.True(t, ok) {
			assert.Equal(t, r.TF.DeprecationMessage(), "r1")
		}

		for _, d := range p.dataSources {
			assert.NotEqual(t, d.TF.DeprecationMessage(), "d-r1")
		}
	})
}

// ProviderMeta is an old experimental TF feature which does not seem to be used.
// We want to make sure it doesn't break anything.
func TestProviderMetaPlanResourceChangeNoError(t *testing.T) {
	t.Parallel()
	type otherMetaType struct {
		val string
	}

	p := testprovider.ProviderV2()
	er := p.ResourcesMap["example_resource"]
	er.Schema = map[string]*schemav2.Schema{
		"string_property_value": {Type: schema.TypeString, Optional: true},
	}
	er.Create = nil //nolint:all
	er.CreateContext = func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
		rd.SetId("r1")
		return diag.Diagnostics{}
	}
	// In GCP the meta we receive is not even close to the schema.
	// We should make sure this does not cause issues.
	p.ProviderMetaSchema = map[string]*schemav2.Schema{
		"module_name": {
			Type:     schemav2.TypeString,
			Optional: true,
		},
	}
	p.SetMeta(otherMetaType{val: "foo"})

	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProv,
		config: shimv2.NewSchemaMap(p.Schema),
		info: ProviderInfo{
			P:              shimProv,
			ResourcePrefix: "example",
			Resources: map[string]*ResourceInfo{
				"example_resource":       {Tok: "ExampleResource"},
				"second_resource":        {Tok: "SecondResource"},
				"nested_secret_resource": {Tok: "NestedSecretResource"},
			},
		},
	}
	provider.initResourceMaps()

	t.Run("Create", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Create",
			"request": {
			  "urn": "urn:pulumi:dev::teststack::ExampleResource::example",
				"properties": {
		        "__defaults": []
			  },
			  "preview": false
			},
			"response": {
				"id": "*",
				"properties": "*"
			}
		  }`)
	})
}

func TestStringValForOtherProperty(t *testing.T) {
	t.Parallel()
	const largeNumber int64 = 1<<62 + 1

	p := &schemav2.Provider{
		Schema: map[string]*schemav2.Schema{},
		ResourcesMap: map[string]*schemav2.Resource{
			"res": {
				Schema: map[string]*schemav2.Schema{
					"int_prop": {
						Optional: true,
						Type:     schema.TypeInt,
					},
					"float_prop": {
						Optional: true,
						Type:     schema.TypeFloat,
					},
					"bool_prop": {
						Optional: true,
						Type:     schema.TypeBool,
					},
					"nested_int": {
						Optional: true,
						Type:     schema.TypeList,
						Elem: &schemav2.Schema{
							Type: schema.TypeInt,
						},
					},
					"nested_float": {
						Optional: true,
						Type:     schema.TypeList,
						Elem: &schemav2.Schema{
							Type: schema.TypeFloat,
						},
					},
					"nested_bool": {
						Optional: true,
						Type:     schema.TypeList,
						Elem: &schemav2.Schema{
							Type: schema.TypeBool,
						},
					},
				},
			},
		},
	}
	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProv,
		config: shimv2.NewSchemaMap(p.Schema),
		info:   ProviderInfo{P: shimProv},
		resources: map[tokens.Type]Resource{
			"Res": {
				TF:     shimProv.ResourcesMap().Get("res"),
				TFName: "res",
				Schema: &ResourceInfo{},
			},
		},
	}

	t.Run("String value for int property", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {
				},
				"news": {
					"__defaults": [],
					"intProp": "80"
				},
				"randomSeed": "zjSL8IMF68r5aLLepOpsIT53uBTbkDryYFDnHQHkjko="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"intProp": 80
				}
			}
		}`)
	})

	t.Run("String value for large int property", func(t *testing.T) {
		testutils.Replay(t, provider, fmt.Sprintf(`
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {
				},
				"news": {
					"__defaults": [],
					"intProp": "%d"
				},
				"randomSeed": "zjSL8IMF68r5aLLepOpsIT53uBTbkDryYFDnHQHkjko="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"intProp": %d
				}
			}
		}`, largeNumber, largeNumber))
	})

	t.Run("String value for bool property", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {
				},
				"news": {
					"__defaults": [],
					"boolProp": "true"
				},
				"randomSeed": "zjSL8IMF68r5aLLepOpsIT53uBTbkDryYFDnHQHkjko="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"boolProp": true
				}
			}
		}`)
	})

	t.Run("String num value for bool property", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {
				},
				"news": {
					"__defaults": [],
					"boolProp": "1"
				},
				"randomSeed": "zjSL8IMF68r5aLLepOpsIT53uBTbkDryYFDnHQHkjko="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"boolProp": true
				}
			}
		}`)
	})

	t.Run("String value for float property", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {
				},
				"news": {
					"__defaults": [],
					"floatProp": "8.2"
				},
				"randomSeed": "zjSL8IMF68r5aLLepOpsIT53uBTbkDryYFDnHQHkjko="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"floatProp": 8.2
				}
			}
		}`)
	})

	t.Run("String value for nested int property", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {
				},
				"news": {
					"__defaults": [],
					"nestedInts": ["80"]
				},
				"randomSeed": "zjSL8IMF68r5aLLepOpsIT53uBTbkDryYFDnHQHkjko="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"nestedInts": [80]
				}
			}
		}`)
	})

	t.Run("String value for nested float property", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {
				},
				"news": {
					"__defaults": [],
					"nestedFloats": ["8.2"]
				},
				"randomSeed": "zjSL8IMF68r5aLLepOpsIT53uBTbkDryYFDnHQHkjko="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"nestedFloats": [8.2]
				}
			}
		}`)
	})

	t.Run("String value for nested bool property", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {
				},
				"news": {
					"__defaults": [],
					"nestedBools": ["true"]
				},
				"randomSeed": "zjSL8IMF68r5aLLepOpsIT53uBTbkDryYFDnHQHkjko="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"nestedBools": [true]
				}
			}
		}`)
	})

	t.Run("String num value for nested bool property", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
			"method": "/pulumirpc.ResourceProvider/Check",
			"request": {
				"urn": "urn:pulumi:dev::teststack::Res::exres",
				"olds": {
				},
				"news": {
					"__defaults": [],
					"nestedBools": ["1"]
				},
				"randomSeed": "zjSL8IMF68r5aLLepOpsIT53uBTbkDryYFDnHQHkjko="
			},
			"response": {
				"inputs": {
					"__defaults": [],
					"nestedBools": [true]
				}
			}
		}`)
	})
}

type testWarnLogSink struct {
	buf *bytes.Buffer
}

var _ logging.Sink = &testWarnLogSink{}

func (s *testWarnLogSink) Log(context context.Context, sev pdiag.Severity, urn resource.URN, msg string) error {
	fmt.Fprintf(s.buf, "%v: %s\n", sev, msg)
	return nil
}

func (s *testWarnLogSink) LogStatus(context context.Context, sev pdiag.Severity, urn resource.URN, msg string) error {
	fmt.Fprintf(s.buf, "[status] [%v] [%v] %s\n", sev, urn, msg)
	return nil
}

func UnknownsSchema() map[string]*schemav2.Resource {
	return map[string]*schemav2.Resource{
		"example_resource": {
			Schema: map[string]*schemav2.Schema{
				"set_prop": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem:     &schemav2.Schema{Type: schemav2.TypeString},
				},
				"set_block_prop": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schemav2.Resource{
						Schema: map[string]*schemav2.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
				"string_prop": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"list_prop": {
					Type:     schema.TypeList,
					Optional: true,
					Elem:     &schemav2.Schema{Type: schemav2.TypeString},
				},
				"list_block_prop": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schemav2.Resource{
						Schema: map[string]*schemav2.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
				"nested_list_prop": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schemav2.Schema{
						Type:     schema.TypeList,
						Optional: true,
						Elem:     &schemav2.Schema{Type: schemav2.TypeString},
					},
				},
				"nested_list_block_prop": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schemav2.Resource{
						Schema: map[string]*schemav2.Schema{
							"nested_prop": {
								Type:     schema.TypeList,
								Optional: true,
								Elem: &schemav2.Resource{
									Schema: map[string]*schemav2.Schema{
										"prop": {
											Type:     schema.TypeString,
											Optional: true,
										},
									},
								},
							},
						},
					},
				},
				"max_items_one_prop": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem:     &schemav2.Schema{Type: schemav2.TypeString},
				},
				"max_items_one_block_prop": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schemav2.Resource{
						Schema: map[string]*schemav2.Schema{
							"prop": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
		},
	}
}

func TestUnknowns(t *testing.T) {
	t.Parallel()
	// Related to [pulumi/pulumi-terraform-bridge#1885]
	// This test is to ensure that we can handle unknowns in the schema.
	// Note that the behaviour here might not match TF and can NOT match TF completely
	// as HCL has no way of expressing unknown blocks.
	// We currently have a workaround in makeTerraformInputs where we convert unknown blocks
	// to blocks of unknown.
	//
	// The structure is that for each property we inject an unknown at every level.
	// For the block tests:
	// _subprop is an unknown for the subproperty in the block object
	// _prop is an unknown for the whole block
	// _collection is an unknown for the whole collection
	// The nested match the above convention but also iterate over the nested object.

	p := &schemav2.Provider{
		Schema:       map[string]*schemav2.Schema{},
		ResourcesMap: UnknownsSchema(),
	}
	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProv,
		config: shimv2.NewSchemaMap(p.Schema),
		info: ProviderInfo{
			P:              shimProv,
			ResourcePrefix: "example",
			Resources: map[string]*ResourceInfo{
				"example_resource": {Tok: "ExampleResource"},
			},
		},
	}
	provider.initResourceMaps()

	t.Run("unknown for string prop", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"string_prop":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for set prop", func(t *testing.T) {
		// The unknownness gets promoted one level up. This seems to be TF behaviour, independent of PRC.
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"setProps":["04da6b54-80e4-46f7-96ec-b56ff0331ba9"]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for set block prop subprop", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"setBlockProps":[{"prop":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"}]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[{
				  "prop": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
				}],
				"listBlockProps":[],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for set block prop", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"setBlockProps":["04da6b54-80e4-46f7-96ec-b56ff0331ba9"]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"listBlockProps":[],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for set block prop collection", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"setBlockProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"listBlockProps":[],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for list prop", func(t *testing.T) {
		// The unknownness gets promoted one level up. This seems to be TF behaviour, independent of PRC.
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"listProps":["04da6b54-80e4-46f7-96ec-b56ff0331ba9"]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"maxItemsOneProp":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for list block prop subprop", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"listBlockProps":[{"prop":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"}]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[{
					"prop": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
				  }],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for list block prop", func(t *testing.T) {
		// The unknownness gets promoted one level up. This seems to be TF behaviour, independent of PRC.
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"listBlockProps":["04da6b54-80e4-46f7-96ec-b56ff0331ba9"]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for list block prop collection", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"listBlockProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for nested list prop", func(t *testing.T) {
		// The unknownness gets promoted one level up. This seems to be TF behaviour, independent of PRC.
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"nestedListProps":[["04da6b54-80e4-46f7-96ec-b56ff0331ba9"]]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"maxItemsOneProp":null,
				"nestedListProps":["04da6b54-80e4-46f7-96ec-b56ff0331ba9"],
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for nested list block prop nested subprop", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"nestedListBlockProps":[{"nestedProps":[{"prop":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"}]}]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":[{
					"nestedProps": [
						{"prop":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"}
					]
				  }],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for nested list block prop nested prop", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"nestedListBlockProps":[{"nestedProps":["04da6b54-80e4-46f7-96ec-b56ff0331ba9"]}]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":[{
					"nestedProps": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
				  }],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for nested list block prop nested collection", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"nestedListBlockProps":[{"nestedProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"}]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":[{
					"nestedProps": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
				  }],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for nested list block prop", func(t *testing.T) {
		// The unknownness gets promoted one level up. This seems to be TF behaviour, independent of PRC.
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"nestedListBlockProps":["04da6b54-80e4-46f7-96ec-b56ff0331ba9"]
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for nested list block collection", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"nestedListBlockProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for max items one prop", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"maxItemsOneProp":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":null
			}
		}
	}`)
	})

	t.Run("unknown for max items one block subprop", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"maxItemsOneBlockProp":{"prop":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"}
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":{"prop":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"}
			}
		}
	}`)
	})

	t.Run("unknown for max items one block prop", func(t *testing.T) {
		testutils.Replay(t, provider, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
			"properties":{
				"__defaults":[],
				"maxItemsOneBlockProp":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
			},
			"preview":true
		},
		"response": {
			"properties":{
				"id":"04da6b54-80e4-46f7-96ec-b56ff0331ba9",
				"stringProp":null,
				"setProps":null,
				"listProps":null,
				"nestedListProps":null,
				"maxItemsOneProp":null,
				"setBlockProps":[],
				"listBlockProps":[],
				"nestedListBlockProps":[],
				"maxItemsOneBlockProp":"04da6b54-80e4-46f7-96ec-b56ff0331ba9"
			}
		}
	}`)
	})
}

func TestSetDuplicatedDiffEntries(t *testing.T) {
	t.Parallel()
	// Duplicated diff entries cause the engine to display the wrong detailed diff.
	// We have a workaround in place to deduplicate the entries.
	// [pulumi/pulumi#16466]
	p := &schemav2.Provider{
		Schema: map[string]*schemav2.Schema{},
		ResourcesMap: map[string]*schemav2.Resource{
			"example_resource": {
				Schema: map[string]*schemav2.Schema{
					"privileges": &schema.Schema{
						Type:     schema.TypeSet,
						Optional: true,
						Elem:     &schemav2.Schema{Type: schemav2.TypeString},
					},
				},
			},
		},
	}
	shimProv := shimv2.NewProvider(p)
	provider := &Provider{
		tf:     shimProv,
		config: shimv2.NewSchemaMap(p.Schema),
		info: ProviderInfo{
			P:              shimProv,
			ResourcePrefix: "example",
			Resources: map[string]*ResourceInfo{
				"example_resource": {Tok: "ExampleResource"},
			},
		},
	}
	provider.initResourceMaps()

	testutils.Replay(t, provider, `
{
    "method": "/pulumirpc.ResourceProvider/Diff",
    "request": {
        "id": "id",
		"urn": "urn:pulumi:dev::teststack::ExampleResource::exres",
        "olds": {
            "id": "id",
            "privileges": [
                "CREATE EXTERNAL TABLE",
                "CREATE TABLE",
                "CREATE VIEW",
                "CREATE TEMPORARY TABLE",
                "USAGE"
            ]
        },
        "news": {
            "__defaults": [],
            "privileges": [
                "USAGE"
            ]
        },
        "oldInputs": {
            "__defaults": [],
            "privileges": [
                "CREATE EXTERNAL TABLE",
                "CREATE TABLE",
                "CREATE VIEW",
                "CREATE TEMPORARY TABLE",
                "USAGE"
            ]
        }
    },
    "response": {
        "changes": "DIFF_SOME",
        "diffs": [
            "privileges"
        ],
        "detailedDiff": {
            "privileges[0]": {
                "kind": "DELETE"
            },
            "privileges[1]": {
                "kind": "DELETE"
            },
            "privileges[2]": {
                "kind": "DELETE"
            },
            "privileges[3]": {
                "kind": "DELETE"
            }
        },
        "hasDetailedDiff": true
    },
    "metadata": {
        "kind": "resource",
        "mode": "client",
        "name": "snowflake"
    }
}`)
}

func TestProcessImportValidationErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		schema        schemav2.Schema
		cloudVal      interface{}
		expectedProps resource.PropertyMap
		expectFailure bool
	}{
		{
			name:     "TypeString no validate",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
				Optional: true,
			},
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			}),
		},
		{
			name: "Secret value",
			schema: schema.Schema{
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
				Computed:  true,
			},
			cloudVal: "ABC",
			expectedProps: resource.PropertyMap{
				// input not dropped
				"collectionProp": resource.NewSecretProperty(
					&resource.Secret{Element: resource.NewPropertyValue("ABC")},
				),
			},
		},
		{
			name:     "TypeString ValidateFunc does not error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					return []string{}, []error{}
				},
			},
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			}),
		},
		{
			name:     "TypeString ValidateDiagFunc does not error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return nil
				},
			},
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			}),
		},
		{
			name:     "TypeString ValidateDiagFunc returns error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
			},
			// input dropped
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{}),
		},
		{
			name: "TypeMap ValidateDiagFunc returns error",
			cloudVal: map[string]string{
				"nestedProp":      "ABC",
				"nestedOtherProp": "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Computed: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			// input dropped
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{}),
		},
		{
			name: "Non-Computed TypeMap ValidateDiagFunc does not drop",
			cloudVal: map[string]string{
				"nestedProp":      "ABC",
				"nestedOtherProp": "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Computed: false,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			// input not dropped
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{
				"collectionProp": map[string]interface{}{
					"nestedProp":      "ABC",
					"nestedOtherProp": "value",
				},
			}),
			// we don't drop computed: false attributes, so they will
			// still fail
			expectFailure: true,
		},
		{
			name: "Required TypeMap ValidateDiagFunc does not drop",
			cloudVal: map[string]string{
				"nestedProp":      "ABC",
				"nestedOtherProp": "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeMap,
				Required: true,
				ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
					return diag.Errorf("Error")
				},
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{
				"collectionProp": map[string]interface{}{
					"nestedProp":      "ABC",
					"nestedOtherProp": "value",
				},
			}),
			expectFailure: true,
		},
		{
			name:     "TypeString ValidateFunc returns error",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					return []string{}, []error{errors.New("Error")}
				},
			},
			// input dropped
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{}),
		},
		{
			name:     "TypeString ValidateFunc does not drop required fields",
			cloudVal: "ABC",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: func(i interface{}, s string) ([]string, []error) {
					return []string{}, []error{errors.New("Error")}
				},
			},
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{
				// input not dropped
				"collectionProp": "ABC",
			}),
			expectFailure: true,
		},
		{
			name: "TypeSet ValidateDiagFunc returns error",
			cloudVal: []interface{}{
				"ABC", "value",
			},
			schema: schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
						if val, ok := i.(string); ok && val != "ABC" {
							return diag.Errorf("Error")
						}
						return nil
					},
				},
			},
			// if one element of the list fails validation
			// the entire list is removed. Terraform does not return
			// list indexes as part of the diagnostic attribute path
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{}),
		},

		// ValidateDiagFunc & ValidateFunc are not supported for TypeList &
		// TypeSet, but they are supported on the nested elements. For now we are
		// not processing the results of those with `schema.Resource` elements
		// since it can get complicated. Nothing will get dropped and the
		// validation error will pass through
		{
			name: "TypeList do not validate nested fields",
			cloudVal: []interface{}{
				map[string]interface{}{
					"nestedProp":      "ABC",
					"nestedOtherProp": "ABC",
				},
			},
			schema: schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Optional: true,
							ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
								return diag.Errorf("Error")
							},
						},
						"nested_other_prop": {
							Type:     schema.TypeString,
							Optional: true,
							ValidateDiagFunc: func(i interface{}, p cty.Path) diag.Diagnostics {
								return nil
							},
						},
					},
				},
			},
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{
				"collectionProp": []map[string]interface{}{
					{
						"nestedOtherProp": "ABC",
						"nestedProp":      "ABC",
					},
				},
			}),
			expectFailure: true,
		},
		{
			name: "TypeSet Do not validate nested fields",
			cloudVal: []interface{}{
				map[string]interface{}{
					"nestedProp": "ABC",
				},
			},
			schema: schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: func(i interface{}, s string) ([]string, []error) {
								return []string{}, []error{errors.New("Error")}
							},
						},
					},
				},
			},
			expectedProps: resource.NewPropertyMapFromMap(map[string]interface{}{
				"collectionProps": []interface{}{
					map[string]interface{}{
						"nestedProp": "ABC",
					},
				},
			}),
			expectFailure: true,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			urn, err := resource.ParseURN("urn:pulumi:test::test::prov:index/test:Test::mainRes")
			assert.NoError(t, err)
			tfName := "prov_test"
			sch := map[string]*schemav2.Schema{
				"collection_prop": &tc.schema,
				"other_prop": {
					Type:     schema.TypeString,
					Optional: true,
				},
			}
			schemaMap := shimv2.NewSchemaMap(sch)
			tfProvider := shimv2.NewProvider(&schema.Provider{
				Schema: map[string]*schema.Schema{},
				ResourcesMap: map[string]*schemav2.Resource{
					"prov_test": {
						Schema: sch,
					},
				},
			})
			var inputs resource.PropertyValue
			if tc.schema.Sensitive {
				inputs = resource.NewSecretProperty(&resource.Secret{Element: resource.NewPropertyValue(tc.cloudVal)})
			} else {
				inputs = resource.NewPropertyValue(tc.cloudVal)
			}
			plural := ""
			if (tc.schema.Type == schema.TypeList || tc.schema.Type == schema.TypeSet) && tc.schema.MaxItems != 1 {
				plural = "s"
			}
			inputsMap := resource.PropertyMap{
				resource.PropertyKey("collectionProp" + plural): inputs,
			}

			schemaInfos := map[string]*info.Schema{}
			p := Provider{
				configValues: resource.PropertyMap{},
				tf:           tfProvider,
			}
			ctx := context.Background()
			ctx = p.loggingContext(ctx, urn)

			p.processImportValidationErrors(ctx, urn, tfName, inputsMap, schemaMap, schemaInfos)
			assert.Equal(t, tc.expectedProps, inputsMap)
		})
	}
}

func TestProviderCallTerraformConfig(t *testing.T) {
	t.Parallel()
	// Setup: Give our test provider a more interesting schema
	nestedConfigSchema := map[string]*schema.Schema{
		"region": {
			Type:     schema.TypeString,
			Optional: true,
		},
		"ignore_tags": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"key_prefixes": {
						Type:     schema.TypeList,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
					},
				},
			},
		},
	}

	callTestProvider := testprovider.ProviderV2()
	callTestProvider.Schema = nestedConfigSchema
	testProvider := shimv2.NewProvider(callTestProvider)

	provider := &Provider{
		tf:     testProvider,
		config: shimv2.NewSchemaMap(nestedConfigSchema),
	}

	// Setup: Configure the test provider to populate with provider configValues
	pulumiConfigs, err := plugin.MarshalProperties(resource.PropertyMap{
		"region": resource.NewStringProperty("us-west-space-odyssey-2000"),
		"ignoreTags": resource.NewObjectProperty(resource.PropertyMap{
			"keyPrefixes": resource.NewArrayProperty([]resource.PropertyValue{
				resource.NewStringProperty("dev"),
				resource.NewStringProperty("staging"),
			}),
		}),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	require.NoError(t, err)

	configureResp, err := provider.Configure(context.Background(), &pulumirpc.ConfigureRequest{
		Args: pulumiConfigs,
	})
	require.NoError(t, err)
	require.NotNil(t, configureResp)

	// Actually test Call
	callReq := &pulumirpc.CallRequest{
		Tok: "pulumi:providers:testprovider/terraformConfig",
	}

	callResp, err := provider.Call(context.Background(), callReq)

	require.NoError(t, err)
	require.NotNil(t, callResp)

	// Assert our return object is as expected, with terraform_cased keys
	callReturnProperties, err := plugin.UnmarshalProperties(callResp.GetReturn(), plugin.MarshalOptions{KeepSecrets: true})
	require.NoError(t, err)
	autogold.Expect(resource.PropertyMap{resource.PropertyKey("result"): resource.PropertyValue{
		V: resource.PropertyMap{
			resource.PropertyKey("ignore_tags"): resource.PropertyValue{
				V: &resource.Secret{Element: resource.PropertyValue{
					V: []resource.PropertyValue{{
						V: resource.PropertyMap{resource.PropertyKey("key_prefixes"): resource.PropertyValue{
							V: []resource.PropertyValue{
								{
									V: "dev",
								},
								{V: "staging"},
							},
						}},
					}},
				}},
			},
			resource.PropertyKey("region"): resource.PropertyValue{V: &resource.Secret{Element: resource.PropertyValue{V: "us-west-space-odyssey-2000"}}},
		},
	}}).Equal(t, callReturnProperties)

	// Assert invalid method token results in error
	reqInvalidToken := &pulumirpc.CallRequest{
		Tok: "pulumi:providers:testprovider/unknownMethod",
	}

	invalidCallResp, err := provider.Call(context.Background(), reqInvalidToken)

	require.Error(t, err)
	require.ErrorContains(t, err, "unknown method token for Call pulumi:providers:testprovider/unknownMethod")
	require.Nil(t, invalidCallResp)
}
