package tfbridge

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestReplaceSubstringsRegex(t *testing.T) {
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
	}

	for _, tc := range cases {
		result, err := replaceSubstringsRegex(tc.s, tc.replacements)
		assert.NilError(t, err)
		assert.Equal(t, result, tc.expected)
	}
}
