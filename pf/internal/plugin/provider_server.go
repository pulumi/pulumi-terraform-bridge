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

package plugin

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/configencoding"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func NewProviderServerWithContext(
	provider ProviderWithContext,
	configEncoding *tfbridge.ConfigEncoding,
) pulumirpc.ResourceProviderServer {
	p := &providerServer{
		ResourceProviderServer: plugin.NewProviderServer(
			providerThunk{
				configencoding.New(configEncoding, NewProvider(provider)),
			},
		),
	}

	// This is a hack to work around limitations in the Pulumi protocol handshake:
	//
	// We want to be able to return secret values from CheckConfig, but we don't know
	// if the engine accepts secrets until Configure is called (after
	// CheckConfig). [plugin.NewProviderServer] defaults to not sending secrets, which
	// is safe but breaks existing behavior.
	//
	// We send an artificial Configure call to tell the
	// [pulumirpc.ResourceProviderServer] that it should accept
	// secrets. [providerThunk] acts as a thunk, preventing the underlying
	// [ProviderWithContext] from witnessing the extra Configure call.
	_, err := p.ResourceProviderServer.Configure(
		context.WithValue(context.Background(), setupConfigureKey, setupConfigureKey),
		&pulumirpc.ConfigureRequest{
			AcceptSecrets: true,
		})
	contract.AssertNoErrorf(err, "False configure call should never read out of the module, so should not fail.")
	return p
}

type providerThunk struct {
	plugin.GrpcProvider
}

var setupConfigureKey = setupConfigure{}

type setupConfigure struct{}

func (p providerThunk) Configure(
	ctx context.Context, req plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	if ctx.Value(setupConfigureKey) != nil {
		return plugin.ConfigureResponse{}, nil
	}
	return p.GrpcProvider.Configure(ctx, req)
}

type providerServer struct {
	pulumirpc.ResourceProviderServer
}

func (p *providerServer) Configure(
	ctx context.Context, req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	resp, err := p.ResourceProviderServer.Configure(ctx, req)
	if resp != nil {
		// We don't accept secrets, indicating that the engine should apply a
		// default heuristic to secret outputs based on inputs. Because we can't
		// reason about data flow within the underlying provider (TF), we allow
		// the engine to apply its own heuristics.
		resp.AcceptSecrets = false

		resp.AcceptOutputs = false
	}
	return resp, err
}
