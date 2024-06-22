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

	Pkg() tokens.Package

	GetSchemaWithContext(ctx context.Context, req plugin.GetSchemaRequest) ([]byte, error)

	CheckConfigWithContext(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool) (resource.PropertyMap, []p.CheckFailure, error)

	DiffConfigWithContext(ctx context.Context, urn resource.URN, oldInputs, olds, news resource.PropertyMap,
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

	ParameterizeWithContext(context.Context, plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error)
}

func NewProvider(p ProviderWithContext) plugin.Provider {
	return &provider{ProviderWithContext: p}
}

type provider struct {
	plugin.NotForwardCompatibleProvider
	ProviderWithContext
}

var _ plugin.Provider = (*provider)(nil)

func (prov *provider) Pkg() tokens.Package { return prov.ProviderWithContext.Pkg() }

func (prov *provider) Parameterize(
	ctx context.Context, req plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	return prov.ProviderWithContext.ParameterizeWithContext(ctx, req)
}

func (prov *provider) GetSchema(
	ctx context.Context, req plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	schema, err := prov.ProviderWithContext.GetSchemaWithContext(ctx, req)
	return plugin.GetSchemaResponse{Schema: schema}, err
}

func (prov *provider) CheckConfig(
	ctx context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	c, f, err := prov.ProviderWithContext.CheckConfigWithContext(
		ctx, req.URN, req.Olds, req.News, req.AllowUnknowns)
	return plugin.CheckConfigResponse{
		Properties: c,
		Failures:   f,
	}, err
}

func (prov *provider) DiffConfig(
	ctx context.Context, req plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return prov.ProviderWithContext.DiffConfigWithContext(
		ctx, req.URN, req.OldInputs, req.OldOutputs, req.NewInputs, req.AllowUnknowns, req.IgnoreChanges)
}

func (prov *provider) Configure(
	ctx context.Context, req plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, prov.ProviderWithContext.ConfigureWithContext(ctx, req.Inputs)
}

func (prov *provider) Check(
	ctx context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	c, f, err := prov.ProviderWithContext.CheckWithContext(
		ctx, req.URN, req.Olds, req.News, req.AllowUnknowns, req.RandomSeed)
	return plugin.CheckResponse{Properties: c, Failures: f}, err
}

func (prov *provider) Diff(
	ctx context.Context, req plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return prov.ProviderWithContext.DiffWithContext(ctx,
		req.URN, req.ID, req.OldOutputs, req.NewInputs, req.AllowUnknowns, req.IgnoreChanges)
}

func (prov *provider) Create(
	ctx context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id, p, s, err := prov.ProviderWithContext.CreateWithContext(ctx,
		req.URN, req.Properties, req.Timeout, req.Preview)
	return plugin.CreateResponse{ID: id, Properties: p, Status: s}, err
}

func (prov *provider) Read(
	ctx context.Context, req plugin.ReadRequest,
) (plugin.ReadResponse, error) {
	r, s, err := prov.ProviderWithContext.ReadWithContext(ctx, req.URN, req.ID, req.Inputs, req.State)
	return plugin.ReadResponse{ReadResult: r, Status: s}, err
}

func (prov *provider) Update(
	ctx context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	p, s, err := prov.ProviderWithContext.UpdateWithContext(ctx,
		req.URN, req.ID, req.OldOutputs, req.NewInputs, req.Timeout, req.IgnoreChanges, req.Preview)
	return plugin.UpdateResponse{Properties: p, Status: s}, err
}

func (prov *provider) Delete(
	ctx context.Context, req plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	s, err := prov.ProviderWithContext.DeleteWithContext(ctx,
		req.URN, req.ID, req.Inputs, req.Outputs, req.Timeout)
	return plugin.DeleteResponse{Status: s}, err
}

func (prov *provider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	return prov.ProviderWithContext.ConstructWithContext(ctx,
		req.Info, req.Type, tokens.QName(req.Name), req.Parent, req.Inputs, req.Options)
}

func (prov *provider) Invoke(
	ctx context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	p, f, err := prov.ProviderWithContext.InvokeWithContext(ctx, req.Tok, req.Args)
	return plugin.InvokeResponse{Properties: p, Failures: f}, err

}

func (prov *provider) StreamInvoke(
	ctx context.Context, req plugin.StreamInvokeRequest,
) (plugin.StreamInvokeResponse, error) {
	f, err := prov.ProviderWithContext.StreamInvokeWithContext(ctx, req.Tok, req.Args, req.OnNext)
	return plugin.StreamInvokeResponse{Failures: f}, err
}

func (prov *provider) Call(
	ctx context.Context, req plugin.CallRequest,
) (plugin.CallResponse, error) {
	return prov.ProviderWithContext.CallWithContext(ctx, req.Tok, req.Args, req.Info, req.Options)
}

func (prov *provider) GetPluginInfo(ctx context.Context) (workspace.PluginInfo, error) {
	return prov.ProviderWithContext.GetPluginInfoWithContext(ctx)
}

func (prov *provider) SignalCancellation(ctx context.Context) error {
	return prov.ProviderWithContext.SignalCancellationWithContext(ctx)
}

func (prov *provider) GetMapping(
	ctx context.Context, req plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	d, p, err := prov.ProviderWithContext.GetMappingWithContext(ctx, req.Key, req.Provider)
	return plugin.GetMappingResponse{Data: d, Provider: p}, err
}

func (prov *provider) GetMappings(
	ctx context.Context, req plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	k, err := prov.ProviderWithContext.GetMappingsWithContext(ctx, req.Key)
	return plugin.GetMappingsResponse{Keys: k}, err
}
