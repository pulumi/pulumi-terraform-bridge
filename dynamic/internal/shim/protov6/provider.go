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

// protov6 providers a translation layer between [otshim.ProviderClient] and
// [tfprotov6.ProviderServer]. It relies on
// [github.com/opentofu/opentofu/internal/protov6/translate] for type conversions.
//
// This package is intentionally simple to allow for easy validation.
package protov6

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/grpcutil"
	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/protov6/translate"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6"
)

var _ tfprotov6.ProviderServer = (*shimProvider)(nil)

func New(p tfplugin6.ProviderClient) tfprotov6.ProviderServer {
	return shimProvider{remote: p}
}

type shimProvider struct {
	UnimplementedEphemeralResourceServer
	remote tfplugin6.ProviderClient
}

func (p shimProvider) GetMetadata(
	ctx context.Context, req *tfprotov6.GetMetadataRequest,
) (*tfprotov6.GetMetadataResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.GetMetadata,
		translate.GetMetadataRequest(req),
		translate.GetMetadataResult)
}

func (p shimProvider) GetProviderSchema(
	ctx context.Context, req *tfprotov6.GetProviderSchemaRequest,
) (*tfprotov6.GetProviderSchemaResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.GetProviderSchema,
		translate.GetProviderSchemaRequest(req),
		translate.GetProviderSchemaResponse)
}

func (p shimProvider) GetResourceIdentitySchemas(
	ctx context.Context,
	_ *tfprotov6.GetResourceIdentitySchemasRequest,
) (*tfprotov6.GetResourceIdentitySchemasResponse, error) {
	// TODO: implement this properly via the remote provider.
	// https://github.com/pulumi/pulumi-terraform-bridge/issues/3106
	return &tfprotov6.GetResourceIdentitySchemasResponse{
		IdentitySchemas: map[string]*tfprotov6.ResourceIdentitySchema{},
		Diagnostics:     nil,
	}, nil
}

func (p shimProvider) UpgradeResourceIdentity(
	ctx context.Context,
	req *tfprotov6.UpgradeResourceIdentityRequest,
) (*tfprotov6.UpgradeResourceIdentityResponse, error) {
	// TODO: implement this properly via the remote provider.
	// https://github.com/pulumi/pulumi-terraform-bridge/issues/3106
	return &tfprotov6.UpgradeResourceIdentityResponse{
		UpgradedIdentity: nil,
		Diagnostics:      nil,
	}, nil
}

func (p shimProvider) ValidateProviderConfig(
	ctx context.Context, req *tfprotov6.ValidateProviderConfigRequest,
) (*tfprotov6.ValidateProviderConfigResponse, error) {
	resp, err := grpcutil.Translate(ctx,
		p.remote.ValidateProviderConfig,
		translate.ValidateProviderConfigRequest(req),
		translate.ValidateProviderConfigResponse)
	if err != nil {
		return nil, err
	}

	// From the docs on PreparedConfig:
	//
	// This RPC call exists because early versions of the Terraform Plugin
	// SDK allowed providers to set defaults for provider configurations in
	// such a way that Terraform couldn't validate the provider config
	// without retrieving the default values first. As providers using
	// terraform-plugin-go directly and new frameworks built on top of it
	// have no such requirement, it is safe and recommended to simply set
	// PreparedConfig to the value of the PrepareProviderConfigRequest's
	// Config property, indicating that no changes are needed to the
	// configuration.
	resp.PreparedConfig = req.Config

	return resp, nil
}

func (p shimProvider) ConfigureProvider(
	ctx context.Context, req *tfprotov6.ConfigureProviderRequest,
) (*tfprotov6.ConfigureProviderResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ConfigureProvider,
		translate.ConfigureProviderRequest(req),
		translate.ConfigureProviderResponse)
}

func (p shimProvider) StopProvider(
	ctx context.Context, req *tfprotov6.StopProviderRequest,
) (*tfprotov6.StopProviderResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.StopProvider,
		translate.StopProviderRequest(req),
		translate.StopProviderResponse)
}

func (p shimProvider) ValidateResourceConfig(
	ctx context.Context, req *tfprotov6.ValidateResourceConfigRequest,
) (*tfprotov6.ValidateResourceConfigResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ValidateResourceConfig,
		translate.ValidateResourceConfigRequest(req),
		translate.ValidateResourceConfigResponse)
}

func (p shimProvider) UpgradeResourceState(
	ctx context.Context, req *tfprotov6.UpgradeResourceStateRequest,
) (*tfprotov6.UpgradeResourceStateResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.UpgradeResourceState,
		translate.UpgradeResourceStateRequest(req),
		translate.UpgradeResourceStateResponse)
}

func (p shimProvider) ReadResource(
	ctx context.Context, req *tfprotov6.ReadResourceRequest,
) (*tfprotov6.ReadResourceResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ReadResource,
		translate.ReadResourceRequest(req),
		translate.ReadResourceResponse)
}

func (p shimProvider) PlanResourceChange(
	ctx context.Context, req *tfprotov6.PlanResourceChangeRequest,
) (*tfprotov6.PlanResourceChangeResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.PlanResourceChange,
		translate.PlanResourceChangeRequest(req),
		translate.PlanResourceChangeResponse)
}

func (p shimProvider) ApplyResourceChange(
	ctx context.Context, req *tfprotov6.ApplyResourceChangeRequest,
) (*tfprotov6.ApplyResourceChangeResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ApplyResourceChange,
		translate.ApplyResourceChangeRequest(req),
		translate.ApplyResourceChangeResponse)
}

func (p shimProvider) ImportResourceState(
	ctx context.Context, req *tfprotov6.ImportResourceStateRequest,
) (*tfprotov6.ImportResourceStateResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ImportResourceState,
		translate.ImportResourceStateRequest(req),
		translate.ImportResourceStateResponse)
}

func (p shimProvider) ValidateDataResourceConfig(
	ctx context.Context, req *tfprotov6.ValidateDataResourceConfigRequest,
) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ValidateDataResourceConfig,
		translate.ValidateDataResourceConfigRequest(req),
		translate.ValidateDataResourceConfigResponse)
}

func (p shimProvider) MoveResourceState(
	ctx context.Context, req *tfprotov6.MoveResourceStateRequest,
) (*tfprotov6.MoveResourceStateResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.MoveResourceState,
		translate.MoveResourceStateRequest(req),
		translate.MoveResourceStateResponse)
}

func (p shimProvider) ReadDataSource(
	ctx context.Context, req *tfprotov6.ReadDataSourceRequest,
) (*tfprotov6.ReadDataSourceResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ReadDataSource,
		translate.ReadDataSourceRequest(req),
		translate.ReadDataSourceResponse)
}

func (p shimProvider) CallFunction(
	ctx context.Context, req *tfprotov6.CallFunctionRequest,
) (*tfprotov6.CallFunctionResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.CallFunction,
		translate.CallFunctionRequest(req),
		translate.CallFunctionResponse)
}

func (p shimProvider) GetFunctions(
	ctx context.Context, req *tfprotov6.GetFunctionsRequest,
) (*tfprotov6.GetFunctionsResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.GetFunctions,
		translate.GetFunctionsRequest(req),
		translate.GetFunctionsResponse)
}
