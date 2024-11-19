// Copyright 2016-2024, Pulumi Corporation.
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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	sdkv2schema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/stretchr/testify/require"

	pf "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	sdkv2shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// Quick setup for integration-testing PF-based providers.
func newPulumiTest(t *testing.T, p provider.Provider, testProgramYAML string) *pulumitest.PulumiTest {
	ctx := context.Background()

	// Due to some historical limitations it is not yet possible to directly pass a PF-based provider to the main
	// [info.Provider.P] parameter, but the same effect can be achieved by passing it to [info.Provider.MuxWith] and
	// using a dummy empty provider for [info.Provider.P].
	dummyProvider := &sdkv2schema.Provider{}

	providerName := "testprovider"

	muxProviderInfo := info.Provider{
		Name:         providerName,
		P:            pf.ShimProvider(p),
		Version:      "0.0.1",
		MetadataInfo: info.NewProviderMetadata([]byte(`{}`)),
	}

	makeToken := func(module, name string) (string, error) {
		return tokens.MakeStandard(providerName)(module, name)
	}

	muxProviderInfo.MustComputeTokens(tokens.SingleModule(providerName, "index", makeToken))

	muxProvider, err := pf.NewMuxProvider(ctx, muxProviderInfo, nil)
	require.NoError(t, err)

	providerInfo := info.Provider{
		P:                              sdkv2shim.NewProvider(dummyProvider),
		Name:                           providerName,
		Version:                        "0.0.1",
		MetadataInfo:                   info.NewProviderMetadata([]byte(`{}`)),
		EnableZeroDefaultSchemaVersion: true,
		MuxWith:                        []info.MuxProvider{muxProvider},
	}

	providerInfo.MustComputeTokens(tokens.SingleModule(providerName, "index", makeToken))

	return pulcheck.PulCheck(t, providerInfo, testProgramYAML)
}
