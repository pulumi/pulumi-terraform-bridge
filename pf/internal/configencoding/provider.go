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

package configencoding

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

type provider struct {
	plugin.GrpcProvider

	encoding *tfbridge.ConfigEncoding
}

// New creates a new [plugin.Provider] that wraps prov.
//
// The returned provider will correctly handle JSON encoded property values according to
// the passed encoding.
func New(encoding *tfbridge.ConfigEncoding, prov plugin.GrpcProvider) plugin.GrpcProvider {
	return &provider{prov, encoding}
}

func (p *provider) CheckConfig(
	ctx context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	olds, err := p.encoding.UnfoldProperties(req.Olds)
	if err != nil {
		return plugin.CheckConfigResponse{}, err
	}
	req.Olds = olds

	news, err := p.encoding.UnfoldProperties(req.News)
	if err != nil {
		return plugin.CheckConfigResponse{}, err
	}
	req.News = news

	resp, err := p.GrpcProvider.CheckConfig(ctx, req)
	if err != nil {
		return plugin.CheckConfigResponse{}, err
	}
	props, err := p.encoding.FoldProperties(resp.Properties)
	if err != nil {
		return plugin.CheckConfigResponse{}, err
	}
	resp.Properties = props
	return resp, nil
}

func (p *provider) DiffConfig(
	ctx context.Context, req plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	oldInputs, err := p.encoding.UnfoldProperties(req.OldInputs)
	if err != nil {
		return plugin.DiffConfigResponse{}, err
	}
	req.OldInputs = oldInputs

	oldOutputs, err := p.encoding.UnfoldProperties(req.OldOutputs)
	if err != nil {
		return plugin.DiffConfigResponse{}, err
	}
	req.OldOutputs = oldOutputs

	newInputs, err := p.encoding.UnfoldProperties(req.NewInputs)
	if err != nil {
		return plugin.DiffConfigResponse{}, err
	}
	req.NewInputs = newInputs

	return p.GrpcProvider.DiffConfig(ctx, req)
}

func (p *provider) Configure(
	ctx context.Context, req plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	inputs, err := p.encoding.UnfoldProperties(req.Inputs)
	if err != nil {
		return plugin.ConfigureResponse{}, err
	}
	req.Inputs = inputs

	return p.GrpcProvider.Configure(ctx, req)
}
