package diagnostics

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/stretchr/testify/require"
)

func TestValidationError(t *testing.T) {
	type testCase struct {
		e   ValidationError
		exp string
	}
	testCases := []testCase{
		{
			ValidationError{Summary: "Bad input"},
			"Bad input",
		},
		{
			ValidationError{Summary: "Bad input", Detail: "the input is very bad"},
			"Bad input: the input is very bad",
		},
		{
			ValidationError{Summary: "Bad input", AttributePath: cty.GetAttrPath("foo").IndexInt(0)},
			"[foo[0]] Bad input",
		},
		{
			ValidationError{Summary: "Bad input", Detail: "the input is very bad"},
			"Bad input: the input is very bad",
		},
		{
			ValidationError{
				Summary:       "Bad input",
				Detail:        "the input is very bad",
				AttributePath: cty.GetAttrPath("foo").IndexString("bar").GetAttr("baz"),
			},
			"[foo[\"bar\"].baz] Bad input: the input is very bad",
		},
	}
	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			require.Equal(t, tc.exp, tc.e.Error())
		})
	}
}
