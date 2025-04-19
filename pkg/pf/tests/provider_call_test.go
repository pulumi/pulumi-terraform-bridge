package tfbridgetests

import (
	testutils "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCallWithTerraformConfig(t *testing.T) {
	server, err := newProviderServer(t, testprovider.TLSProvider())
	require.NoError(t, err)
	testutils.ReplayFile(t, server, "testdata/terraform-config-logs.json")
}
