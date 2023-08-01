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
	"testing"

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
	configOut, err := buildTerraformConfig(provider, configIn)
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
	assert.Len(t, checkResp.Failures, 3)
	return checkResp.Failures
}

func TestProviderCheck(t *testing.T) {
	provider := &Provider{
		tf:     shimv1.NewProvider(testTFProvider),
		config: shimv1.NewSchemaMap(testTFProvider.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"SecondResource": {
			TF:     shimv1.NewResource(testTFProvider.ResourcesMap["second_resource"]),
			TFName: "second_resource",
			Schema: &ResourceInfo{Tok: "SecondResource"},
		},
	}

	failures := testCheckFailures(t, provider, "SecondResource")
	sort.SliceStable(failures, func(i, j int) bool { return failures[i].Reason < failures[j].Reason })
	assert.Equal(t, "\"conflicting_property\": conflicts with conflicting_property2."+
		" Examine values at 'name.conflictingProperty'.", failures[0].Reason)
	assert.Equal(t, "", failures[0].Property)
	assert.Equal(t, "\"conflicting_property2\": conflicts with conflicting_property."+
		" Examine values at 'name.conflictingProperty2'.", failures[1].Reason)
	assert.Equal(t, "", failures[1].Property)
	assert.Equal(t, "Missing required property 'arrayPropertyValues'", failures[2].Reason)
	assert.Equal(t, "", failures[2].Property)
}

func TestProviderCheckV2(t *testing.T) {
	provider := &Provider{
		tf:     shimv2.NewProvider(testTFProviderV2),
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
	}
	provider.resources = map[tokens.Type]Resource{
		"SecondResource": {
			TF:     shimv2.NewResource(testTFProviderV2.ResourcesMap["second_resource"]),
			TFName: "second_resource",
			Schema: &ResourceInfo{Tok: "SecondResource"},
		},
	}

	failures := testCheckFailures(t, provider, "SecondResource")
	sort.SliceStable(failures, func(i, j int) bool { return failures[i].Reason < failures[j].Reason })
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

func testProviderRead(t *testing.T, provider *Provider, typeName tokens.Type) {
	urn := resource.NewURN("stack", "project", "", typeName, "name")
	readResp, err := provider.Read(context.Background(), &pulumirpc.ReadRequest{
		Id:         string("resource-id"),
		Urn:        string(urn),
		Properties: nil,
	})
	assert.NoError(t, err)

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

	testProviderRead(t, provider, "ExampleResource")
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

	testProviderRead(t, provider, "ExampleResource")
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
		computeStringDefault := func(opts ComputeDefaultOptions) (interface{}, error) {
			require.Equal(t, resource.PropertyPath{"stringPropertyValue"}, opts.PropertyPath)
			if v, ok := opts.PriorState["stringPropertyValue"]; ok {
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
