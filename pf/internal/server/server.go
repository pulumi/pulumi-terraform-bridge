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

package server

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func NewProviderServer(provider plugin.Provider) pulumirpc.ResourceProviderServer {
	return &providerServer{
		ResourceProviderServer: plugin.NewProviderServer(provider),
		provider:               provider,
	}
}

type providerServer struct {
	pulumirpc.ResourceProviderServer
	provider plugin.Provider
}

var _ pulumirpc.ResourceProviderServer = &providerServer{}

// Override Read from pulumirpc.ResourceProviderServer to actually propagate ID returned from inner provider.Read.
func (p *providerServer) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {

	urn, id := resource.URN(req.GetUrn()), resource.ID(req.GetId())

	state, err := plugin.UnmarshalProperties(req.GetProperties(), p.unmarshalOptions("state"))
	if err != nil {
		return nil, err
	}

	inputs, err := plugin.UnmarshalProperties(req.GetInputs(), p.unmarshalOptions("inputs"))
	if err != nil {
		return nil, err
	}

	result, _, err := p.provider.Read(urn, id, inputs, state)
	if err != nil {
		return nil, err
	}

	rpcState, err := plugin.MarshalProperties(result.Outputs, p.marshalOptions("newState"))
	if err != nil {
		return nil, err
	}

	rpcInputs, err := plugin.MarshalProperties(result.Inputs, p.marshalOptions("newInputs"))
	if err != nil {
		return nil, err
	}

	return &pulumirpc.ReadResponse{
		Id:         string(result.ID),
		Properties: rpcState,
		Inputs:     rpcInputs,
	}, nil
}

func (p *providerServer) unmarshalOptions(label string) plugin.MarshalOptions {
	return plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	}
}

func (p *providerServer) marshalOptions(label string) plugin.MarshalOptions {
	return plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	}
}
