package tfbridgetests

import (
	"context"
	"log"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
)

func TestLogCaputure(t *testing.T) {
	t.Setenv("TF_LOG", "WARN")

	provider := info.Provider{
		Name:    "test",
		Version: "0.0.1",
		P: tfbridge.ShimProvider(providerbuilder.NewProvider(providerbuilder.NewProviderArgs{
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
