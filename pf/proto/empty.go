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

package proto

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// Create a [shim.Provider] that wraps an empty [tfprotov6.ProviderServer].
//
// It is safe to use for schema operations (returning empty results). It has the same
// runtime restrictions as [New].
func Empty() shim.Provider { return New(context.Background(), emptyServer{}) }

type emptyServer struct{}

func (emptyServer) GetMetadata(
	context.Context, *tfprotov6.GetMetadataRequest,
) (*tfprotov6.GetMetadataResponse, error) {
	return &tfprotov6.GetMetadataResponse{}, nil
}

func (emptyServer) GetProviderSchema(
	context.Context, *tfprotov6.GetProviderSchemaRequest,
) (*tfprotov6.GetProviderSchemaResponse, error) {
	return &tfprotov6.GetProviderSchemaResponse{}, nil
}

func (emptyServer) ValidateProviderConfig(
	context.Context, *tfprotov6.ValidateProviderConfigRequest,
) (*tfprotov6.ValidateProviderConfigResponse, error) {
	return &tfprotov6.ValidateProviderConfigResponse{}, nil
}

func (emptyServer) ConfigureProvider(
	context.Context, *tfprotov6.ConfigureProviderRequest,
) (*tfprotov6.ConfigureProviderResponse, error) {
	return &tfprotov6.ConfigureProviderResponse{}, nil
}

func (emptyServer) StopProvider(
	context.Context, *tfprotov6.StopProviderRequest,
) (*tfprotov6.StopProviderResponse, error) {
	return &tfprotov6.StopProviderResponse{}, nil
}

func (emptyServer) ValidateResourceConfig(
	context.Context, *tfprotov6.ValidateResourceConfigRequest,
) (*tfprotov6.ValidateResourceConfigResponse, error) {
	return &tfprotov6.ValidateResourceConfigResponse{}, nil
}

func (emptyServer) UpgradeResourceState(
	context.Context, *tfprotov6.UpgradeResourceStateRequest,
) (*tfprotov6.UpgradeResourceStateResponse, error) {
	return &tfprotov6.UpgradeResourceStateResponse{}, nil
}

func (emptyServer) ReadResource(
	context.Context, *tfprotov6.ReadResourceRequest,
) (*tfprotov6.ReadResourceResponse, error) {
	return &tfprotov6.ReadResourceResponse{}, nil
}

func (emptyServer) PlanResourceChange(
	context.Context, *tfprotov6.PlanResourceChangeRequest,
) (*tfprotov6.PlanResourceChangeResponse, error) {
	return &tfprotov6.PlanResourceChangeResponse{}, nil
}

func (emptyServer) ApplyResourceChange(
	context.Context, *tfprotov6.ApplyResourceChangeRequest,
) (*tfprotov6.ApplyResourceChangeResponse, error) {
	return &tfprotov6.ApplyResourceChangeResponse{}, nil
}

func (emptyServer) ImportResourceState(
	context.Context, *tfprotov6.ImportResourceStateRequest,
) (*tfprotov6.ImportResourceStateResponse, error) {
	return &tfprotov6.ImportResourceStateResponse{}, nil
}

func (emptyServer) MoveResourceState(
	context.Context, *tfprotov6.MoveResourceStateRequest,
) (*tfprotov6.MoveResourceStateResponse, error) {
	return &tfprotov6.MoveResourceStateResponse{}, nil
}

func (emptyServer) ValidateDataResourceConfig(
	context.Context, *tfprotov6.ValidateDataResourceConfigRequest,
) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	return &tfprotov6.ValidateDataResourceConfigResponse{}, nil
}

func (emptyServer) ReadDataSource(
	context.Context, *tfprotov6.ReadDataSourceRequest,
) (*tfprotov6.ReadDataSourceResponse, error) {
	return &tfprotov6.ReadDataSourceResponse{}, nil
}

func (emptyServer) CallFunction(
	context.Context, *tfprotov6.CallFunctionRequest,
) (*tfprotov6.CallFunctionResponse, error) {
	return &tfprotov6.CallFunctionResponse{}, nil
}

func (emptyServer) GetFunctions(
	context.Context, *tfprotov6.GetFunctionsRequest,
) (*tfprotov6.GetFunctionsResponse, error) {
	return &tfprotov6.GetFunctionsResponse{}, nil
}
