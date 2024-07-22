// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tfbridgetests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
)

func newProviderServer(t *testing.T, info tfbridge0.ProviderInfo) pulumirpc.ResourceProviderServer {
	ctx := context.Background()
	meta := genMetadata(t, info)
	srv, err := tfbridge.NewProviderServer(ctx, nil, info, meta)
	require.NoError(t, err)
	return srv
}

func newMuxedProviderServer(t *testing.T, info tfbridge0.ProviderInfo) pulumirpc.ResourceProviderServer {
	ctx := context.Background()
	meta := genSDKSchema(t, info)
	p, err := tfbridge.MakeMuxedServer(ctx, info.Name, info, meta)(nil)
	require.NoError(t, err)
	return p
}

func ensureProviderValid(prov *providerbuilder.Provider) {
	for i := range prov.AllResources {
		r := &prov.AllResources[i]
		if r.ResourceSchema.Attributes["id"] == nil {
			r.ResourceSchema.Attributes["id"] = rschema.StringAttribute{Computed: true}
		}
		if r.CreateFunc == nil {
			r.CreateFunc = func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
				resp.State = tfsdk.State(req.Config)
				resp.State.SetAttribute(ctx, path.Root("id"), "test-id")
			}
		}
	}

}

func bridgedProvider(prov *providerbuilder.Provider) info.Provider {
	ensureProviderValid(prov)
	shimProvider := tfbridge.ShimProvider(prov)

	provider := tfbridge0.ProviderInfo{
		P:            shimProvider,
		Name:         prov.TypeName,
		Version:      "0.0.1",
		MetadataInfo: &tfbridge0.MetadataInfo{},
	}

	provider.MustComputeTokens(tokens.SingleModule(prov.TypeName, "index", tokens.MakeStandard(prov.TypeName)))

	return provider
}

func startPulumiProvider(t *testing.T, providerInfo tfbridge0.ProviderInfo) (*rpcutil.ServeHandle, error) {
	prov := newProviderServer(t, providerInfo)

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("rpcutil.ServeWithOptions failed: %w", err)
	}

	return &handle, nil
}

func skipUnlessLinux(t *testing.T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		// TODO[pulumi/pulumi-terraform-bridge#2221]
		t.Skip("Skipping on non-Linux platforms")
	}
}

func pulCheck(t *testing.T, bridgedProvider info.Provider, program string) *pulumitest.PulumiTest {
	skipUnlessLinux(t)
	puwd := t.TempDir()
	p := filepath.Join(puwd, "Pulumi.yaml")

	err := os.WriteFile(p, []byte(program), 0o600)
	require.NoError(t, err)

	opts := []opttest.Option{
		opttest.Env("DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true"),
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProvider(
			bridgedProvider.Name,
			func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
				handle, err := startPulumiProvider(t, bridgedProvider)
				require.NoError(t, err)
				return providers.Port(handle.Port), nil
			},
		),
	}

	return pulumitest.NewPulumiTest(t, puwd, opts...)
}
