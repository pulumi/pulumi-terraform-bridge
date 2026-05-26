// Reproduces the failure observed when running aws_cloudwatch_log_group
// through the dynamic (terraform-provider) bridge: a resource that has
// `name` and `name_prefix` with ConflictsWith fails Create when only
// `namePrefix` is supplied. Tofu accepts this configuration (it sees only
// name_prefix); the bridge should as well.
package main

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	helper "github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/testing"
)

var conflictsProviderPath = helper.BuildOnce(&globalTempDir,
	"test/conflictsprovider", "terraform-provider-conflictsprovider")

func TestConflictsWithNamePrefixOnly(t *testing.T) {
	t.Parallel()
	helper.Integration(t)
	skipWindows(t)

	server := parameterizedTestServer(t, conflictsProviderPath)

	const typ = "conflictsprovider:index/logGroup:LogGroup"
	urn := string(resource.NewURN("test", "test", "", typ, "lg"))

	// User specifies only namePrefix; name is intentionally omitted (the
	// equivalent of HCL `name_prefix = "x"` with `name` unset).
	news := marshal(resource.PropertyMap{
		"namePrefix": resource.NewProperty("example-"),
	})

	t.Run("check", func(t *testing.T) {
		resp, err := server.Check(t.Context(), &pulumirpc.CheckRequest{Urn: urn, News: news})
		require.NoError(t, err)
		assert.Empty(t, resp.Failures, "Check should not fail validation")
	})

	t.Run("create", func(t *testing.T) {
		resp, err := server.Create(t.Context(), &pulumirpc.CreateRequest{
			Urn: urn, Properties: news,
		})
		require.NoErrorf(t, err,
			"Create with only namePrefix should succeed (tofu accepts it); got: %v", err)
		require.NotNil(t, resp)
	})
}
