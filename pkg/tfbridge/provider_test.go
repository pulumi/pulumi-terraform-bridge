package tfbridge

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"

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
		v, err := convertStringToPropertyValue(c.str, c.typ)
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
	assert.Equal(t, "\"conflicting_property\": conflicts with conflicting_property2", failures[0].Reason)
	assert.Equal(t, "", failures[0].Property)
	assert.Equal(t, "\"conflicting_property2\": conflicts with conflicting_property", failures[1].Reason)
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
		"conflicting_property2. Examine values at 'SecondResource.ConflictingProperty'.", failures[0].Reason)
	assert.Equal(t, "", failures[0].Property)
	assert.Equal(t, "Conflicting configuration arguments: \"conflicting_property2\": conflicts with "+
		"conflicting_property. Examine values at 'SecondResource.ConflictingProperty2'.", failures[1].Reason)
	assert.Equal(t, "", failures[1].Property)
	assert.Equal(t, "Missing required argument: The argument \"array_property_value\" is required, but no "+
		"definition was found.. Examine values at 'SecondResource.ArrayPropertyValues'.", failures[2].Reason)
	assert.Equal(t, "", failures[2].Property)
}

func testProviderPreConfigureCallback(t *testing.T, provider *Provider) {
	expectedErr := errors.New("failedToPreConfigure")
	provider.info = ProviderInfo{
		PreConfigureCallback: func(vars resource.PropertyMap, config shim.ResourceConfig) error {
			return expectedErr
		},
	}
	_, err := provider.Configure(context.Background(), &pulumirpc.ConfigureRequest{
		Variables: map[string]string{
			"foo:config:bar": "invalid",
		},
	})
	assert.Equal(t, expectedErr, err)
}

func TestProviderPreConfigureCallbackV1(t *testing.T) {
	provider := &Provider{
		tf:     shimv1.NewProvider(testTFProvider),
		config: shimv1.NewSchemaMap(testTFProvider.Schema),
	}
	testProviderPreConfigureCallback(t, provider)

}

func TestProviderPreConfigureCallbackV2(t *testing.T) {
	provider := &Provider{
		tf:     shimv2.NewProvider(testTFProviderV2),
		config: shimv2.NewSchemaMap(testTFProviderV2.Schema),
	}
	testProviderPreConfigureCallback(t, provider)
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
		// Uncomment the line below to make the test pass:
		// "kind": resource.NewStringProperty(""),
		// But this is not really expected! The Read function for ExampleResource in internal/testprovider/schema.go does not set "kind"!
	}), ins["nestedResources"])
	assert.Equal(t, resource.NewArrayProperty(
		[]resource.PropertyValue{
			resource.NewStringProperty("set member 2"),
			resource.NewStringProperty("set member 1"),
		}), ins["setPropertyValues"])
	assert.Equal(t, resource.NewStringProperty("some ${interpolated:value} with syntax errors"), ins["stringWithBadInterpolation"])
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
