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
	"io"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	p "github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// A version of Provider interface that is enhanced by giving access to the request Context.
type ProviderWithContext interface {
	io.Closer

	PkgWithContext(ctx context.Context) tokens.Package

	GetSchemaWithContext(ctx context.Context, version int) ([]byte, error)

	CheckConfigWithContext(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool) (resource.PropertyMap, []p.CheckFailure, error)

	DiffConfigWithContext(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool, ignoreChanges []string) (p.DiffResult, error)

	ConfigureWithContext(ctx context.Context, inputs resource.PropertyMap) error

	CheckWithContext(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool, randomSeed []byte) (resource.PropertyMap, []p.CheckFailure, error)

	DiffWithContext(ctx context.Context, urn resource.URN, id resource.ID, olds resource.PropertyMap,
		news resource.PropertyMap, allowUnknowns bool, ignoreChanges []string) (p.DiffResult, error)

	CreateWithContext(ctx context.Context, urn resource.URN, news resource.PropertyMap, timeout float64,
		preview bool) (resource.ID, resource.PropertyMap, resource.Status, error)

	ReadWithContext(ctx context.Context, urn resource.URN, id resource.ID,
		inputs, state resource.PropertyMap) (p.ReadResult, resource.Status, error)

	UpdateWithContext(ctx context.Context, urn resource.URN, id resource.ID,
		olds resource.PropertyMap, news resource.PropertyMap, timeout float64,
		ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error)

	DeleteWithContext(ctx context.Context, urn resource.URN, id resource.ID,
		inputs, outputs resource.PropertyMap, timeout float64) (resource.Status, error)

	ConstructWithContext(ctx context.Context, info p.ConstructInfo, typ tokens.Type, name tokens.QName,
		parent resource.URN, inputs resource.PropertyMap,
		options p.ConstructOptions) (p.ConstructResult, error)

	InvokeWithContext(ctx context.Context, tok tokens.ModuleMember,
		args resource.PropertyMap) (resource.PropertyMap, []p.CheckFailure, error)

	StreamInvokeWithContext(
		ctx context.Context,
		tok tokens.ModuleMember,
		args resource.PropertyMap,
		onNext func(resource.PropertyMap) error) ([]p.CheckFailure, error)

	CallWithContext(ctx context.Context, tok tokens.ModuleMember, args resource.PropertyMap, info p.CallInfo,
		options p.CallOptions) (p.CallResult, error)

	GetPluginInfoWithContext(ctx context.Context) (workspace.PluginInfo, error)

	SignalCancellationWithContext(ctx context.Context) error

	GetMappingWithContext(ctx context.Context, key, provider string) ([]byte, string, error)

	GetMappingsWithContext(ctx context.Context, key string) ([]string, error)
}

func NewProvider(ctx context.Context, p ProviderWithContext) plugin.Provider {
	return &provider{ctx: ctx, ProviderWithContext: p}
}

type provider struct {
	ctx context.Context
	ProviderWithContext
}

var _ plugin.Provider = (*provider)(nil)

func (prov *provider) Pkg() tokens.Package { return prov.PkgWithContext(prov.ctx) }

func (prov *provider) GetSchema(version int) ([]byte, error) {
	return prov.ProviderWithContext.GetSchemaWithContext(prov.ctx, version)
}

func (prov *provider) CheckConfig(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return prov.ProviderWithContext.CheckConfigWithContext(prov.ctx, urn, olds, news, allowUnknowns)
}

func (prov *provider) DiffConfig(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {
	return prov.ProviderWithContext.DiffConfigWithContext(
		prov.ctx, urn, oldOutputs, newInputs, allowUnknowns, ignoreChanges)
}

func (prov *provider) Configure(inputs resource.PropertyMap) error {
	return prov.ProviderWithContext.ConfigureWithContext(prov.ctx, inputs)
}

func (prov *provider) Check(urn resource.URN, olds, news resource.PropertyMap, allowUnknowns bool,
	randomSeed []byte) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return prov.ProviderWithContext.CheckWithContext(prov.ctx, urn, olds, news, allowUnknowns, randomSeed)
}

func (prov *provider) Diff(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {
	return prov.ProviderWithContext.DiffWithContext(prov.ctx, urn, id, oldOutputs, newInputs, allowUnknowns, ignoreChanges)
}

func (prov *provider) Create(urn resource.URN, news resource.PropertyMap, timeout float64, preview bool) (resource.ID,
	resource.PropertyMap, resource.Status, error) {
	return prov.ProviderWithContext.CreateWithContext(prov.ctx, urn, news, timeout, preview)
}

func (prov *provider) Read(urn resource.URN, id resource.ID, inputs, state resource.PropertyMap) (plugin.ReadResult,
	resource.Status, error) {
	return prov.ProviderWithContext.ReadWithContext(prov.ctx, urn, id, inputs, state)
}

func (prov *provider) Update(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	timeout float64, ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {
	return prov.ProviderWithContext.UpdateWithContext(prov.ctx, urn, id, oldOutputs, newInputs, timeout, ignoreChanges,
		preview)
}

func (prov *provider) Delete(
	urn resource.URN,
	id resource.ID,
	inputs, outputs resource.PropertyMap,
	timeout float64,
) (resource.Status, error) {
	return prov.ProviderWithContext.DeleteWithContext(prov.ctx, urn, id, inputs, outputs,
		timeout)
}

func (prov *provider) Construct(info plugin.ConstructInfo, typ tokens.Type, name tokens.QName, parent resource.URN,
	inputs resource.PropertyMap, options plugin.ConstructOptions) (plugin.ConstructResult, error) {
	return prov.ProviderWithContext.ConstructWithContext(prov.ctx, info, typ, name, parent, inputs, options)
}

func (prov *provider) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return prov.ProviderWithContext.InvokeWithContext(prov.ctx, tok, args)
}

func (prov *provider) StreamInvoke(tok tokens.ModuleMember, args resource.PropertyMap,
	onNext func(resource.PropertyMap) error) ([]plugin.CheckFailure, error) {
	return prov.ProviderWithContext.StreamInvokeWithContext(prov.ctx, tok, args, onNext)
}

func (prov *provider) Call(tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions) (plugin.CallResult, error) {
	return prov.ProviderWithContext.CallWithContext(prov.ctx, tok, args, info, options)
}

func (prov *provider) GetPluginInfo() (workspace.PluginInfo, error) {
	return prov.ProviderWithContext.GetPluginInfoWithContext(prov.ctx)
}

func (prov *provider) SignalCancellation() error {
	return prov.ProviderWithContext.SignalCancellationWithContext(prov.ctx)
}

func (prov *provider) GetMapping(key, provider string) ([]byte, string, error) {
	return prov.ProviderWithContext.GetMappingWithContext(prov.ctx, key, provider)
}

func (prov *provider) GetMappings(key string) ([]string, error) {
	return prov.ProviderWithContext.GetMappingsWithContext(prov.ctx, key)
}
