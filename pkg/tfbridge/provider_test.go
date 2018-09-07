package tfbridge

import (
	"testing"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/pulumi/pulumi/pkg/resource"
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
