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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func ToProvider(prov ProviderWithContext) plugin.Provider {
	return &providerConverter{
		ProviderWithContext: prov,
		ctx:                 context.Background(),
	}
}

type providerConverter struct {
	ProviderWithContext
	ctx context.Context
}

func (p *providerConverter) Close() error {
	return p.ProviderWithContext.Close()
}

func (p *providerConverter) Pkg() tokens.Package {
	return p.ProviderWithContext.PkgWithContext(p.ctx)
}

func (p *providerConverter) GetSchema(version int) ([]byte, error) {
	return p.ProviderWithContext.GetSchemaWithContext(p.ctx, version)
}

func (p *providerConverter) CheckConfig(
	urn resource.URN,
	olds, news resource.PropertyMap,
	allowUnknowns bool,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return p.ProviderWithContext.CheckConfigWithContext(p.ctx, urn, olds, news, allowUnknowns)
}

func (p *providerConverter) DiffConfig(
	urn resource.URN,
	olds, news resource.PropertyMap,
	allowUnknowns bool,
	ignoreChanges []string,
) (plugin.DiffResult, error) {
	return p.ProviderWithContext.DiffConfigWithContext(p.ctx, urn, olds, news, allowUnknowns, ignoreChanges)
}

func (p *providerConverter) Configure(inputs resource.PropertyMap) error {
	return p.ProviderWithContext.ConfigureWithContext(p.ctx, inputs)
}

func (p *providerConverter) Check(
	urn resource.URN,
	olds, news resource.PropertyMap,
	allowUnknowns bool,
	randomSeed []byte,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return p.ProviderWithContext.CheckWithContext(p.ctx, urn, olds, news, allowUnknowns, randomSeed)
}

func (p *providerConverter) Diff(

	urn resource.URN,
	id resource.ID,
	olds, news resource.PropertyMap,
	allowUnknowns bool,
	ignoreChanges []string,
) (plugin.DiffResult, error) {
	return p.ProviderWithContext.DiffWithContext(p.ctx, urn, id, olds, news, allowUnknowns, ignoreChanges)
}

func (p *providerConverter) Create(

	urn resource.URN,
	news resource.PropertyMap,
	timeout float64,
	preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	return p.ProviderWithContext.CreateWithContext(p.ctx, urn, news, timeout, preview)
}

func (p *providerConverter) Read(

	urn resource.URN,
	id resource.ID,
	inputs, state resource.PropertyMap,
) (plugin.ReadResult, resource.Status, error) {
	return p.ProviderWithContext.ReadWithContext(p.ctx, urn, id, inputs, state)
}

func (p *providerConverter) Update(

	urn resource.URN,
	id resource.ID,
	olds resource.PropertyMap,
	news resource.PropertyMap,
	timeout float64,
	ignoreChanges []string,
	preview bool,
) (resource.PropertyMap, resource.Status, error) {
	return p.ProviderWithContext.UpdateWithContext(p.ctx, urn, id, olds, news, timeout, ignoreChanges, preview)
}

func (p *providerConverter) Delete(

	urn resource.URN,
	id resource.ID,
	props resource.PropertyMap,
	timeout float64,
) (resource.Status, error) {
	return p.ProviderWithContext.DeleteWithContext(p.ctx, urn, id, props, timeout)
}

func (p *providerConverter) Construct(

	info plugin.ConstructInfo,
	typ tokens.Type,
	name tokens.QName,
	parent resource.URN,
	inputs resource.PropertyMap,
	options plugin.ConstructOptions,
) (plugin.ConstructResult, error) {
	return p.ProviderWithContext.ConstructWithContext(p.ctx, info, typ, name, parent, inputs, options)
}

func (p *providerConverter) Invoke(

	tok tokens.ModuleMember,
	args resource.PropertyMap,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return p.ProviderWithContext.InvokeWithContext(p.ctx, tok, args)
}

func (p *providerConverter) StreamInvoke(

	tok tokens.ModuleMember,
	args resource.PropertyMap,
	onNext func(resource.PropertyMap) error,
) ([]plugin.CheckFailure, error) {
	return p.ProviderWithContext.StreamInvokeWithContext(p.ctx, tok, args, onNext)
}

func (p *providerConverter) Call(

	tok tokens.ModuleMember,
	args resource.PropertyMap,
	info plugin.CallInfo,
	options plugin.CallOptions,
) (plugin.CallResult, error) {
	return p.ProviderWithContext.CallWithContext(p.ctx, tok, args, info, options)
}

func (p *providerConverter) GetPluginInfo() (workspace.PluginInfo, error) {
	return p.ProviderWithContext.GetPluginInfoWithContext(p.ctx)
}

func (p *providerConverter) SignalCancellation() error {
	return p.ProviderWithContext.SignalCancellationWithContext(p.ctx)
}

func (p *providerConverter) GetMapping(key string) ([]byte, string, error) {
	return p.ProviderWithContext.GetMappingWithContext(p.ctx, key)
}
