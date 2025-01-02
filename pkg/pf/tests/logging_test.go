package tfbridgetests

import (
	"context"
	"log"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func TestRegress2699(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var didLog bool

	info := testprovider.MuxedRandomProvider()
	info.PreConfigureCallbackWithLogger = func(
		ctx context.Context,
		host *provider.HostClient, vars presource.PropertyMap,
		config shim.ResourceConfig,
	) error {
		log.Println("[DEBUG] Test")
		didLog = true
		return nil
	}
	info.UpstreamRepoPath = "."

	server, err := pfbridge.MakeMuxedServer(ctx, "muxedrandom", info, genSDKSchema(t, info))(nil)
	require.NoError(t, err)

	_, err = server.CheckConfig(ctx, &pulumirpc.CheckRequest{})
	require.NoError(t, err)
	require.True(t, didLog)
}

func TestLogCaputure(t *testing.T) {
	t.Setenv("TF_LOG", "WARN")

	provider := info.Provider{
		Name:    "test",
		Version: "0.0.1",
		P: pfbridge.ShimProvider(providerbuilder.NewProvider(providerbuilder.NewProviderArgs{
			TypeName: "test",
			AllResources: []providerbuilder.Resource{{
				Name: "res",
				ResourceSchema: schema.Schema{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
					},
				},
				CreateFunc: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
					require.Empty(t, resp.State.SetAttribute(ctx, path.Root("id"), "1234"))
					log.Println("[INFO] This is info")
					log.Println("[WARN] You have been warned")
				},
			}},
		})),
		MetadataInfo: info.NewProviderMetadata(nil),
	}

	provider.MustComputeTokens(tokens.SingleModule("test", "index", tokens.MakeStandard("test")))

	pt, err := pulcheck.PulCheck(t, provider, `
name: test
runtime: yaml
resources:
  mainRes:
    type: test:Res
`)

	require.NoError(t, err)
	result := pt.Up(t)

	assert.NotContains(t, result.StdOut, "This is info")
	assert.Contains(t, result.StdOut, "You have been warned")
}
