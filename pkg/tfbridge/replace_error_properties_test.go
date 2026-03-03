package tfbridge

import (
	"testing"

	"gotest.tools/v3/assert"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestReplaceSubstringsRegex(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s            string
		replacements map[string]string
		expected     string
	}{
		{`"hello" world`, map[string]string{"hello": "hi"}, `"hi" world`},
		{`"hello" "world"`, map[string]string{"hello": "hi", "world": "earth"}, `"hi" "earth"`},
		{`"hello" "helloWorld"`, map[string]string{"hello": "hi", "helloWorld": "hiHi"}, `"hi" "hiHi"`},
		{`"helloWorld" "hello"`, map[string]string{"hello": "hi", "helloWorld": "hiHi"}, `"hiHi" "hi"`},
		{`"hello" "world" "hello" "world"`, map[string]string{"hello": "hi", "world": "earth"}, `"hi" "earth" "hi" "earth"`},
		// Deprecation warning pattern
		{
			`"resource_group_name": [DEPRECATED] use "new_field" instead`,
			map[string]string{"resource_group_name": "resourceGroupName", "new_field": "newField"},
			`"resourceGroupName": [DEPRECATED] use "newField" instead`,
		},
		// Empty replacements map returns input unchanged
		{`"hello" world`, map[string]string{}, `"hello" world`},
	}

	for _, tc := range cases {
		result, err := replaceSubstringsRegex(tc.s, tc.replacements)
		assert.NilError(t, err)
		assert.Equal(t, result, tc.expected)
	}
}

func TestRewriteWarningMessage(t *testing.T) {
	t.Parallel()

	schemaMap := schema.SchemaMap(map[string]shim.Schema{
		"resource_group_name": (&schema.Schema{Type: shim.TypeString}).Shim(),
		"new_field":           (&schema.Schema{Type: shim.TypeString}).Shim(),
	})

	msg := `"resource_group_name": [DEPRECATED] use "new_field" instead`
	result := rewriteWarningMessage(msg, nil, schemaMap)
	assert.Equal(t, result, `"resourceGroupName": [DEPRECATED] use "newField" instead`)
}

func TestRewriteWarningMessageNilSchema(t *testing.T) {
	t.Parallel()

	msg := `some warning without property names`
	result := rewriteWarningMessage(msg, nil, nil)
	assert.Equal(t, result, msg)
}
