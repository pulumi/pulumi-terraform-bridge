package main

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/hexops/autogold/v2"
	helper "github.com/pulumi/pulumi-terraform-bridge/dynamic/internal/testing"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func grpcTestServer(ctx context.Context, t *testing.T) pulumirpc.ResourceProviderServer {
	defaultInfo, metadata, close := initialSetup()
	t.Cleanup(func() { assert.NoError(t, close()) })
	s, err := pfbridge.NewProviderServer(ctx, nil, defaultInfo, metadata)
	require.NoError(t, err)
	return s
}

func TestSchemaGeneration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("autogold does not play nice with windows newlines")
	}

	testSchema := func(name, version string) {
		t.Run(strings.Join([]string{name, version}, "-"), func(t *testing.T) {
			helper.Integration(t)
			ctx := context.Background()

			server := grpcTestServer(ctx, t)

			result, err := server.Parameterize(ctx, &pulumirpc.ParameterizeRequest{
				Parameters: &pulumirpc.ParameterizeRequest_Args{
					Args: &pulumirpc.ParameterizeRequest_ParametersArgs{
						Args: []string{name, version},
					},
				},
			})
			require.NoError(t, err)

			assert.Equal(t, version, result.Version)

			schema, err := server.GetSchema(ctx, &pulumirpc.GetSchemaRequest{
				SubpackageName:    result.Name,
				SubpackageVersion: result.Version,
			})

			require.NoError(t, err)
			var fmtSchema bytes.Buffer
			require.NoError(t, json.Indent(&fmtSchema, []byte(schema.Schema), "", "    "))
			autogold.ExpectFile(t, autogold.Raw(fmtSchema.String()))
		})
	}

	testSchema("hashicorp/random", "3.3.0")
	testSchema("Azure/alz", "0.11.1")
	testSchema("Backblaze/b2", "0.8.9")
}
