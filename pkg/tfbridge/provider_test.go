package tfbridge

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
	"github.com/stretchr/testify/assert"
)

func TestConvertStringToPropertyValue(t *testing.T) {
	type testcase struct {
		str      string
		typ      schema.ValueType
		expected interface{}
	}

	cases := []testcase{
		{
			typ:      schema.TypeBool,
			expected: false,
		},
		{
			str:      "false",
			typ:      schema.TypeBool,
			expected: false,
		},
		{
			str:      "true",
			typ:      schema.TypeBool,
			expected: true,
		},
		{
			str: "root",
			typ: schema.TypeBool,
		},

		{
			typ:      schema.TypeString,
			expected: "",
		},
		{
			str:      "stringP",
			typ:      schema.TypeString,
			expected: "stringP",
		},

		{
			typ:      schema.TypeInt,
			expected: 0,
		},
		{
			str:      "42",
			typ:      schema.TypeInt,
			expected: 42,
		},
		{
			str: "root",
			typ: schema.TypeInt,
		},

		{
			typ:      schema.TypeFloat,
			expected: 0,
		},
		{
			str:      "42",
			typ:      schema.TypeFloat,
			expected: 42,
		},
		{
			str: "root",
			typ: schema.TypeFloat,
		},

		{
			typ:      schema.TypeList,
			expected: []interface{}{},
		},
		{
			str:      "[ \"foo\", \"bar\" ]",
			typ:      schema.TypeList,
			expected: []interface{}{"foo", "bar"},
		},

		{
			typ:      schema.TypeSet,
			expected: []interface{}{},
		},
		{
			str:      "[ \"foo\", \"bar\" ]",
			typ:      schema.TypeSet,
			expected: []interface{}{"foo", "bar"},
		},

		{
			typ:      schema.TypeMap,
			expected: map[string]interface{}{},
		},
		{
			str: "{ \"foo\": { \"bar\": 42 }, \"baz\": [ true ] }",
			typ: schema.TypeMap,
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
	provider := &Provider{
		tf:     testTFProvider,
		config: testTFProvider.Schema,
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
