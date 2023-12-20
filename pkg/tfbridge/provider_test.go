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
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	hostclient "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestConvertStringToPropertyValue(t *testing.T) {
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
	t.Skip("Temporarily skipped")
	provider := &Provider{
		tf:     shimv1.NewProvider(testTFProvider),
		config: shimv1.NewSchemaMap(testTFProvider.Schema),
	}

	oldConfig := resource.PropertyMap{"configValue": resource.NewStringProperty("foo")}
	newConfig := resource.PropertyMap{"configValue": resource.NewStringProperty("bar")}

	olds, err := plugin.MarshalProperties(oldConfig, plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	news, err := plugin.MarshalProperties(newConfig, plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)

	req := &pulumirpc.DiffRequest{
		Id:   "provider",
		Urn:  "provider",
		Olds: olds,
		News: news,
	}

	resp, err := provider.DiffConfig(context.Background(), req)
	assert.NoError(t, err)
	assert.True(t, resp.HasDetailedDiff)
	assert.Len(t, resp.DetailedDiff, 1)
}

func TestBuildConfig(t *testing.T) {
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

	expected := provider.tf.NewResourceConfig(map[string]interface{}{
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

	outs, err := plugin.UnmarshalProperties(createResp.GetProperties(), plugin.MarshalOptions{KeepUnknowns: true})
	assert.NoError(t, err)
	assert.True(t, resource.PropertyMap{
		"id":                  resource.NewStringProperty(""),
		"stringPropertyValue": resource.NewStringProperty("foo"),
		"setPropertyValues":   resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("foo")}),
	}.DeepEquals(outs))

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
	provider := &Provider{
		tf:     shimv2.NewProvider(testTFProviderV2),
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"ExampleResource": {
			TF:     shimv2.NewResource(testTFProviderV2.ResourcesMap["example_resource"]),
			TFName: "example_resource",
			Schema: &ResourceInfo{Tok: "ExampleResource"},
		},
	}
	testIgnoreChanges(t, provider)
}

func testProviderPreview(t *testing.T, provider *Provider) {
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

func TestProviderPreview(t *testing.T) {
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
	testProviderPreview(t, provider)
}

func TestProviderPreviewV2(t *testing.T) {
	provider := &Provider{
		tf:     shimv2.NewProvider(testTFProviderV2),
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"ExampleResource": {
			TF:     shimv2.NewResource(testTFProviderV2.ResourcesMap["example_resource"]),
			TFName: "example_resource",
			Schema: &ResourceInfo{Tok: "ExampleResource"},
		},
	}
	testProviderPreview(t, provider)
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
	assert.Equal(t, "Conflicting configuration arguments: \"conflicting_property\": conflicts with "+
		"conflicting_property2. Examine values at 'name.conflictingProperty'.", failures[0].Reason)
	assert.Equal(t, "", failures[0].Property)
	assert.Equal(t, "Conflicting configuration arguments: \"conflicting_property2\": conflicts with "+
		"conflicting_property. Examine values at 'name.conflictingProperty2'.", failures[1].Reason)
	assert.Equal(t, "", failures[1].Property)
	assert.Equal(t, "Missing required argument. The argument \"array_property_value\" is required"+
		", but no definition was found.. Examine values at 'name.arrayPropertyValues'.", failures[2].Reason)
	assert.Equal(t, "", failures[2].Property)
}

func TestProviderCheck(t *testing.T) {
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
			      "supportsPreview": true
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
	assert.Equal(t, resource.NewArrayProperty(
		[]resource.PropertyValue{
			resource.NewStringProperty("set member 2"),
			resource.NewStringProperty("set member 1"),
		}), ins["setPropertyValues"])
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
	provider := &Provider{
		tf:     shimv2.NewProvider(testTFProviderV2),
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"ExampleResource": {
			TF:     shimv2.NewResource(testTFProviderV2.ResourcesMap["example_resource"]),
			TFName: "example_resource",
			Schema: &ResourceInfo{Tok: "ExampleResource"},
		},
	}

	testProviderRead(t, provider, "ExampleResource", true /* CheckRawConfig */)
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
	provider := &Provider{
		tf:     shimv2.NewProvider(testTFProviderV2),
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"NestedSecretResource": {
			TF:     shimv2.NewResource(testTFProviderV2.ResourcesMap["nested_secret_resource"]),
			TFName: "nested_secret_resource",
			Schema: &ResourceInfo{Tok: "NestedSecretResource"},
		},
	}

	testProviderReadNestedSecret(t, provider, "NestedSecretResource")
}

func TestCheck(t *testing.T) {
	t.Run("Default application can consult prior state in Check", func(t *testing.T) {
		provider := &Provider{
			tf:     shimv2.NewProvider(testTFProviderV2),
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
				TF:     shimv2.NewResource(testTFProviderV2.ResourcesMap["example_resource"]),
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

		provider := &Provider{
			tf:     shimv2.NewProvider(p2),
			config: shimv2.NewSchemaMap(p2.Schema),
		}

		provider.resources = map[tokens.Type]Resource{
			"ExampleResource": {
				TF:     shimv2.NewResource(p2.ResourcesMap["example_resource"]),
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

func TestCheckConfig(t *testing.T) {
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
		require.Equal(t, "could not validate provider configuration: "+
			"Invalid or unknown key. Check `pulumi config get cloudflare:requiredprop`.",
			resp.Failures[0].Reason)
		// Explicit provider.
		resp, err = provider.CheckConfig(ctx, &pulumirpc.CheckRequest{
			Urn:  "urn:pulumi:r::cloudflare-record-ts::pulumi:providers:cloudflare::explicitprovider",
			News: args,
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(resp.Failures))
		require.Equal(t, "could not validate provider configuration: "+
			"Invalid or unknown key. Examine values at 'explicitprovider.requiredprop'.",
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
		require.Equal(t, "could not validate provider configuration: "+
			"Invalid or unknown key. Check `pulumi config get testprovider:cofnigValue`. "+
			"Did you mean `testprovider:configValue`?",
			resp.Failures[0].Reason)
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
			    "supportsPreview": true
			  }
			}`)
	})
}

func TestConfigureErrorReplacement(t *testing.T) {
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
			  "errors": "1 error occurred:\n\t* some error with \"configProperty\" and \"CONFIG!\" but not config\n\n"
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

		provider := &Provider{
			tf:     shimv2.NewProvider(testTFProviderV2),
			config: shimv2.NewSchemaMap(testTFProviderV2.Schema),

			dataSources: map[tokens.ModuleMember]DataSource{
				"tprov:index/ExampleFn:ExampleFn": {
					TF:     shimv2.NewResource(ds),
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
	provider := &Provider{
		tf:     shimv2.NewProvider(testTFProviderV2),
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
		resources: map[tokens.Type]Resource{
			"ExampleResource": {
				TF:     shimv2.NewResource(testTFProviderV2.ResourcesMap["example_resource"]),
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
		      "id": "",
		      "stringPropertyValue": "TRANSFORMED"
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
		      "stringWithBadInterpolation": "*"
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
		      "arrayPropertyValues": "*",
		      "nestedResources": "*",
		      "setPropertyValues": "*"
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
		      "stringWithBadInterpolation": "*"
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
			"stringWithBadInterpolation": "*"
		    }
		  }
		}`)
	})
}

func TestSkipDetailedDiff(t *testing.T) {
	provider := func(t *testing.T, skipDetailedDiffForChanges bool) *Provider {
		p := testprovider.CustomizedDiffProvider(func(data *schema.ResourceData) {})
		return &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
			resources: map[tokens.Type]Resource{
				"Resource": {
					TF:     shimv2.NewResource(p.ResourcesMap["test_resource"]),
					TFName: "test_resource",
					Schema: &ResourceInfo{Tok: "Resource"},
				},
				"Replace": {
					TF:     shimv2.NewResource(p.ResourcesMap["test_replace"]),
					TFName: "test_replace",
					Schema: &ResourceInfo{Tok: "Replace"},
				},
			},
			info: ProviderInfo{
				XSkipDetailedDiffForChanges: skipDetailedDiffForChanges,
			},
		}
	}
	t.Run("Diff", func(t *testing.T) {
		testutils.Replay(t, provider(t, true), `
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
		    "hasDetailedDiff": true
		  }
		}`)
	})

	// This test checks that we will flag some meta field (`__meta`) as a replace if
	// the upstream diff indicates a replace but there is no field associated with the
	// replace.
	//
	// We do this since Pulumi's gRPC protocol doesn't have direct support for
	// declaring a replace on a resource without an associated property.
	t.Run("EmptyDiffWithReplace", func(t *testing.T) {
		test := func(skipDetailedDiffForChanges bool) func(t *testing.T) {
			return func(t *testing.T) {
				testutils.Replay(t, provider(t, skipDetailedDiffForChanges), `
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
		    "hasDetailedDiff": true
		  }
		}`)
			}
		}
		t.Run("withDetailedDiff", test(false))
		t.Run("skipDetailedDiff", test(true))
	})

	// This test checks that we don't insert extraneous replaces when there is an
	// existing field that holds a replace.
	t.Run("FullDiffWithReplace", func(t *testing.T) {
		test := func(skipDetailedDiffForChanges bool) func(t *testing.T) {
			return func(t *testing.T) {
				testutils.Replay(t, provider(t, skipDetailedDiffForChanges), `
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
		    "replaces": ["labels"],
		    "hasDetailedDiff": true,
		    "detailedDiff": { "labels": { "kind": "UPDATE_REPLACE" } },
		    "diffs": ["labels"]
		  }
		}`)
			}
		}
		t.Run("withDetailedDiff", test(false))
		t.Run("skipDetailedDiff", test(true))
	})
}

func TestTransformFromState(t *testing.T) {
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
		return &Provider{
			tf:     shimv2.NewProvider(p),
			config: shimv2.NewSchemaMap(p.Schema),
			resources: map[tokens.Type]Resource{
				"Echo": {
					TF:     shimv2.NewResource(p.ResourcesMap["echo"]),
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
	p := testprovider.MaxItemsOneProvider()
	provider := &Provider{
		tf:     shimv2.NewProvider(p),
		config: shimv2.NewSchemaMap(p.Schema),
		resources: map[tokens.Type]Resource{
			"NestedStrRes": {
				TF:     shimv2.NewResource(p.ResourcesMap["nested_str_res"]),
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
					"nested_str": []
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

// These should test that we validate resources before applying TF defaults.
// This is what TF does!
// https://github.com/pulumi/pulumi-terraform-bridge/issues/1546
func TestDefaultsAndConflictsWithValidationInteraction(t *testing.T) {
	p := testprovider.ConflictsWithValidationProvider()
	provider := &Provider{
		tf:     shimv2.NewProvider(p),
		config: shimv2.NewSchemaMap(p.Schema),
		resources: map[tokens.Type]Resource{
			"DefaultValueRes": {
				TF:     shimv2.NewResource(p.ResourcesMap["default_value_res"]),
				TFName: "default_value_res",
				Schema: &ResourceInfo{},
			},
		},
	}

	t.Run("CheckMissingRequiredProp", func(t *testing.T) {
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
	p := testprovider.ExactlyOneOfValidationProvider()
	provider := &Provider{
		tf:     shimv2.NewProvider(p),
		config: shimv2.NewSchemaMap(p.Schema),
		resources: map[tokens.Type]Resource{
			"DefaultValueRes": {
				TF:     shimv2.NewResource(p.ResourcesMap["default_value_res"]),
				TFName: "default_value_res",
				Schema: &ResourceInfo{},
			},
		},
	}
	t.Run("CheckFailsWhenExactlyOneOfNotSpecified", func(t *testing.T) {
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
					{"reason": "Invalid combination of arguments. \"exactly_one_of_property2\": one of $exactly_one_of_property,exactly_one_of_property2$ must be specified. Examine values at 'exres.exactlyOneOfProperty2'."},
					{"reason": "Invalid combination of arguments. \"exactly_one_of_property\": one of $exactly_one_of_property,exactly_one_of_property2$ must be specified. Examine values at 'exres.exactlyOneOfProperty'."},
					{"reason": "Invalid combination of arguments. \"exactly_one_of_nonrequired_property2\": one of $exactly_one_of_nonrequired_property2,exactly_one_of_required_property$ must be specified. Examine values at 'exres.exactlyOneOfNonrequiredProperty2'."},
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
	p := testprovider.RequiredWithValidationProvider()
	provider := &Provider{
		tf:     shimv2.NewProvider(p),
		config: shimv2.NewSchemaMap(p.Schema),
		resources: map[tokens.Type]Resource{
			"DefaultValueRes": {
				TF:     shimv2.NewResource(p.ResourcesMap["default_value_res"]),
				TFName: "default_value_res",
				Schema: &ResourceInfo{},
			},
		},
	}

	t.Run("CheckMissingRequiredPropErrors", func(t *testing.T) {
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
						"requiredWithProperty",
						"requiredWithProperty2",
						"requiredWithRequiredProperty",
						"requiredWithNonrequiredProperty",
						"requiredWithRequiredProperty2",
						"requiredWithRequiredProperty3"
					],
					"requiredWithProperty": "",
					"requiredWithProperty2": "",
					"requiredWithRequiredProperty": "",
					"requiredWithNonrequiredProperty": "",
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
						"requiredWithProperty": "foo",
						"requiredWithProperty2": "foo",
						"requiredWithRequiredProperty": "foo",
						"requiredWithNonrequiredProperty": "foo",
						"requiredWithRequiredProperty2": "foo",
						"requiredWithRequiredProperty3": "foo"
					},
					"randomSeed": "iYRxB6/8Mm7pwKIs+yK6IyMDmW9JSSTM6klzRUgZhRk="
				},
				"response": {
					"inputs": {
						"__defaults": [],
						"requiredWithProperty": "foo",
						"requiredWithProperty2": "foo",
						"requiredWithRequiredProperty": "foo",
						"requiredWithNonrequiredProperty": "foo",
						"requiredWithRequiredProperty2": "foo",
						"requiredWithRequiredProperty3": "foo"
					}
				}
			}`)
	})

	t.Run("CheckMissingRequiredWith", func(t *testing.T) {
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
							"requiredWithProperty2",
							"requiredWithNonrequiredProperty",
							"requiredWithRequiredProperty3"
						],
						"requiredWithProperty": "foo",
						"requiredWithProperty2": "",
						"requiredWithRequiredProperty": "foo",
						"requiredWithNonrequiredProperty": "",
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
