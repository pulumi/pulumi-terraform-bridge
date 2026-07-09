// Copyright 2016-2026, Pulumi Corporation.
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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/function"
	sdk2schema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	testutils "github.com/pulumi/providertest/replay"
	"github.com/stretchr/testify/require"

	pfmuxer "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/muxer"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

// shoutFunction upper-cases its input; used to exercise provider-defined functions
// through a muxed (SDKv2 + PF) provider.
type shoutFunction struct{}

func (shoutFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "shout"
}

func (shoutFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Parameters: []function.Parameter{function.StringParameter{Name: "input"}},
		Return:     function.StringReturn{},
	}
}

func (shoutFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var input string
	resp.Error = req.Arguments.Get(ctx, &input)
	if resp.Error != nil {
		return
	}
	resp.Error = resp.Result.Set(ctx, strings.ToUpper(input)+"!")
}

// A muxed provider dispatches function invokes to its Plugin Framework half; the SDKv2
// half cannot define functions.
func TestMuxedProviderFunctionInvoke(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pfProvider := pb.NewProvider(pb.NewProviderArgs{
		TypeName: "muxedfn",
		AllFunctions: []func() function.Function{
			func() function.Function { return shoutFunction{} },
		},
	})
	sdkProvider := &sdk2schema.Provider{
		ResourcesMap: map[string]*sdk2schema.Resource{
			"muxedfn_res": {
				Schema: map[string]*sdk2schema.Schema{
					"x": {Type: sdk2schema.TypeString, Optional: true},
				},
			},
		},
	}

	prov := tfbridge0.ProviderInfo{
		Name:         "muxedfn",
		Version:      "0.0.1",
		P:            pfbridge.MuxShimWithPF(ctx, sdkv2.NewProvider(sdkProvider), pfProvider),
		MetadataInfo: tfbridge0.NewProviderMetadata(nil),
	}
	prov.MustComputeTokens(tokens.SingleModule("muxedfn_", "index", tokens.MakeStandard("muxedfn")))
	require.Equal(t, map[string]*info.Function{
		"shout": {Tok: "muxedfn:index/shout:shout"},
	}, prov.Functions)

	schema := genSDKSchema(t, prov)
	dispatch, err := prov.P.(*pfmuxer.ProviderShim).ResolveDispatch(&prov)
	require.NoError(t, err)
	require.NoError(t, metadata.Set(prov.GetMetadata(), "mux", dispatch))

	server, err := pfbridge.MakeMuxedServer(ctx, "muxedfn", prov, schema)(nil)
	require.NoError(t, err)

	testutils.Replay(t, server, `
	{
	  "method": "/pulumirpc.ResourceProvider/Invoke",
	  "request": {
	    "tok": "muxedfn:index/shout:shout",
	    "args": {
	      "input": "hi"
	    }
	  },
	  "response": {
	    "return": {
	      "result": "HI!"
	    }
	  }
	}`)
}
