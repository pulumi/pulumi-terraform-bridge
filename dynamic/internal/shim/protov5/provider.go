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

// protov5 providers a translation layer between [tfplugin5.ProviderClient] and
// [tfprotov5.ProviderServer]. It relies on
// [github.com/opentofu/opentofu/shim/protov5/translate] for type conversions.
//
// This package is intentionally simple to allow for easy validation.
package protov5

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"

	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/grpcutil"
	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/protov5/translate"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin5"
)

var _ tfprotov5.ProviderServer = (*provider)(nil)

func New(server tfplugin5.ProviderClient) tfprotov5.ProviderServer {
	return provider{server}
}

type provider struct{ remote tfplugin5.ProviderClient }

func (p provider) GetMetadata(
	ctx context.Context, req *tfprotov5.GetMetadataRequest,
) (*tfprotov5.GetMetadataResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.GetMetadata,
		translate.GetMetadataRequest(req),
		translate.GetMetadataResponse)
}

func (p provider) GetProviderSchema(
	ctx context.Context, req *tfprotov5.GetProviderSchemaRequest,
) (*tfprotov5.GetProviderSchemaResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.GetSchema,
		translate.GetProviderSchemaRequest(req),
		translate.GetProviderSchemaResponse)
}

func (p provider) PrepareProviderConfig(
	ctx context.Context, req *tfprotov5.PrepareProviderConfigRequest,
) (*tfprotov5.PrepareProviderConfigResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.PrepareProviderConfig,
		translate.PrepareProviderConfigRequest(req),
		translate.PrepareProviderConfigResponse)
}

func (p provider) ConfigureProvider(
	ctx context.Context, req *tfprotov5.ConfigureProviderRequest,
) (*tfprotov5.ConfigureProviderResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.Configure,
		translate.ConfigureProviderRequest(req),
		translate.ConfigureProviderResponse)
}

func (p provider) StopProvider(
	ctx context.Context, req *tfprotov5.StopProviderRequest,
) (*tfprotov5.StopProviderResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.Stop,
		translate.StopProviderRequest(req),
		translate.StopProviderResponse)
}

func (p provider) ValidateResourceTypeConfig(
	ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest,
) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ValidateResourceTypeConfig,
		translate.ValidateResourceTypeConfigRequest(req),
		translate.ValidateResourceTypeConfigResponse)
}

func (p provider) UpgradeResourceState(
	ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest,
) (*tfprotov5.UpgradeResourceStateResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.UpgradeResourceState,
		translate.UpgradeResourceStateRequest(req),
		translate.UpgradeResourceStateResponse)
}

func (p provider) ReadResource(
	ctx context.Context, req *tfprotov5.ReadResourceRequest,
) (*tfprotov5.ReadResourceResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ReadResource,
		translate.ReadResourceRequest(req),
		translate.ReadResourceResponse)
}

func (p provider) PlanResourceChange(
	ctx context.Context, req *tfprotov5.PlanResourceChangeRequest,
) (*tfprotov5.PlanResourceChangeResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.PlanResourceChange,
		translate.PlanResourceChangeRequest(req),
		translate.PlanResourceChangeResponse)
}

func (p provider) ApplyResourceChange(
	ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest,
) (*tfprotov5.ApplyResourceChangeResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ApplyResourceChange,
		translate.ApplyResourceChangeRequest(req),
		translate.ApplyResourceChangeResponse)
}

func (p provider) ImportResourceState(
	ctx context.Context, req *tfprotov5.ImportResourceStateRequest,
) (*tfprotov5.ImportResourceStateResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ImportResourceState,
		translate.ImportResourceStateRequest(req),
		translate.ImportResourceStateResponse)
}

func (p provider) MoveResourceState(
	ctx context.Context, req *tfprotov5.MoveResourceStateRequest,
) (*tfprotov5.MoveResourceStateResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.MoveResourceState,
		translate.MoveResourceStateRequest(req),
		translate.MoveResourceStateResponse)
}

func (p provider) ValidateDataSourceConfig(
	ctx context.Context, req *tfprotov5.ValidateDataSourceConfigRequest,
) (*tfprotov5.ValidateDataSourceConfigResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ValidateDataSourceConfig,
		translate.ValidateDataSourceConfigRequest(req),
		translate.ValidateDataSourceConfigResponse)
}

func (p provider) ReadDataSource(
	ctx context.Context, req *tfprotov5.ReadDataSourceRequest,
) (*tfprotov5.ReadDataSourceResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.ReadDataSource,
		translate.ReadDataSourceRequest(req),
		translate.ReadDataSourceResponse)
}

func (p provider) CallFunction(
	ctx context.Context, req *tfprotov5.CallFunctionRequest,
) (*tfprotov5.CallFunctionResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.CallFunction,
		translate.CallFunctionRequest(req),
		translate.CallFunctionResponse)
}

func (p provider) GetFunctions(
	ctx context.Context, req *tfprotov5.GetFunctionsRequest,
) (*tfprotov5.GetFunctionsResponse, error) {
	return grpcutil.Translate(ctx,
		p.remote.GetFunctions,
		translate.GetFunctionsRequest(req),
		translate.GetFunctionsResponse)
}
