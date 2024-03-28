package crosstests

import (
	"github.com/pulumi/providertest/pulumitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type T interface {
	Logf(string, ...any)
	TempDir() string
	require.TestingT
	assert.TestingT
	pulumitest.T
}
