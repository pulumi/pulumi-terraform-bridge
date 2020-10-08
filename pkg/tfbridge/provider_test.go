package tfbridge

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
	"github.com/stretchr/testify/assert"

	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim/sdk-v1"
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
