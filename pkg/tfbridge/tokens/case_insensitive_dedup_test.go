package tokens

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseStandardToken(t *testing.T) {
	t.Parallel()

	tok := "pkg:module/lower:Name"
	res, ok := parseStandardToken(tok)
	require.True(t, ok)
	require.Equal(t, tokenParts{
		Package:    "pkg",
		Module:     "module",
		PascalName: "Name",
	}, res)
}
