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

package protov6

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	otshim "github.com/opentofu/opentofu/internal/tfplugin6"
	"github.com/opentofu/opentofu/shim/tfplugin6"

	grpc "google.golang.org/grpc"
)

var _ tfprotov6.ProviderServer = (*shimProvider)(nil)

func New(p otshim.ProviderClient) tfprotov6.ProviderServer {
	return shimProvider{p}
}

type shimProvider struct{ remote otshim.ProviderClient }

func translateGRPC[
	In, Out, Final any,
	Call func(context.Context, In, ...grpc.CallOption) (Out, error),
	MapResult func(Out) Final,
](
	ctx context.Context,
	call Call,
	i In,
	m MapResult,
) (Final, error) {
	v, err := call(ctx, i)
	if err != nil {
		var tmp Final
		return tmp, err
	}
	return m(v), nil
}

// GetMetadata returns upfront information about server capabilities and
// supported resource types without requiring the server to instantiate all
// schema information, which may be memory intensive. This RPC is optional,
// where clients may receive an unimplemented RPC error. Clients should
// ignore the error and call the GetProviderSchema RPC as a fallback.
func (p shimProvider) GetMetadata(ctx context.Context, req *tfprotov6.GetMetadataRequest) (*tfprotov6.GetMetadataResponse, error) {
	return translateGRPC(ctx, p.remote.GetMetadata, tfplugin6.GetMetadataRequest(req), tfplugin6.GetMetadataResult)
}

// GetProviderSchema is called when Terraform needs to know what the
// provider's schema is, along with the schemas of all its resources
// and data sources.
func (p shimProvider) GetProviderSchema(ctx context.Context, req *tfprotov6.GetProviderSchemaRequest) (*tfprotov6.GetProviderSchemaResponse, error) {
	return translateGRPC(ctx, p.remote.GetProviderSchema, tfplugin6.GetProviderSchemaRequest(req), tfplugin6.GetProviderSchemaResponse)
}

// ValidateProviderConfig is called to give a provider a chance to
// validate the configuration the user specified.
func (p shimProvider) ValidateProviderConfig(ctx context.Context, req *tfprotov6.ValidateProviderConfigRequest) (*tfprotov6.ValidateProviderConfigResponse, error) {
	v, err := p.remote.ValidateProviderConfig(ctx, tfplugin6.ValidateProviderConfigRequest(req))
	if err != nil {
		return nil, err
	}
	resp := tfplugin6.ValidateProviderConfigResponse(v)

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

// ConfigureProvider is called to pass the user-specified provider
// configuration to the provider.
func (p shimProvider) ConfigureProvider(ctx context.Context, req *tfprotov6.ConfigureProviderRequest) (*tfprotov6.ConfigureProviderResponse, error) {
	return translateGRPC(ctx, p.remote.ConfigureProvider, tfplugin6.ConfigureProviderRequest(req), tfplugin6.ConfigureProviderResponse)
}

// StopProvider is called when Terraform would like providers to shut
// down as quickly as possible, and usually represents an interrupt.
func (p shimProvider) StopProvider(ctx context.Context, req *tfprotov6.StopProviderRequest) (*tfprotov6.StopProviderResponse, error) {
	return translateGRPC(ctx, p.remote.StopProvider, tfplugin6.StopProviderRequest(req), tfplugin6.StopProviderResponse)
}

// ValidateResourceConfig is called when Terraform is checking that
// a resource's configuration is valid. It is guaranteed to have types
// conforming to your schema. This is your opportunity to do custom or
// advanced validation prior to a plan being generated.
func (p shimProvider) ValidateResourceConfig(ctx context.Context, req *tfprotov6.ValidateResourceConfigRequest) (*tfprotov6.ValidateResourceConfigResponse, error) {
	return translateGRPC(ctx, p.remote.ValidateResourceConfig, tfplugin6.ValidateResourceConfigRequest(req), tfplugin6.ValidateResourceConfigResponse)
}

// UpgradeResourceState is called when Terraform has encountered a
// resource with a state in a schema that doesn't match the schema's
// current version. It is the provider's responsibility to modify the
// state to upgrade it to the latest state schema.
func (p shimProvider) UpgradeResourceState(ctx context.Context, req *tfprotov6.UpgradeResourceStateRequest) (*tfprotov6.UpgradeResourceStateResponse, error) {
	return translateGRPC(ctx, p.remote.UpgradeResourceState, tfplugin6.UpgradeResourceStateRequest(req), tfplugin6.UpgradeResourceStateResponse)
}

// ReadResource is called when Terraform is refreshing a resource's
// state.
func (p shimProvider) ReadResource(ctx context.Context, req *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	return translateGRPC(ctx, p.remote.ReadResource, tfplugin6.ReadResourceRequest(req), tfplugin6.ReadResourceResponse)
}

// PlanResourceChange is called when Terraform is attempting to
// calculate a plan for a resource. Terraform will suggest a proposed
// new state, which the provider can modify or return unmodified to
// influence Terraform's plan.
func (p shimProvider) PlanResourceChange(context.Context, *tfprotov6.PlanResourceChangeRequest) (*tfprotov6.PlanResourceChangeResponse, error) {
	panic("UNIMPLIMENTED")
}

// ApplyResourceChange is called when Terraform has detected a diff
// between the resource's state and the user's config, and the user has
// approved a planned change. The provider is to apply the changes
// contained in the plan, and return the resulting state.
func (p shimProvider) ApplyResourceChange(context.Context, *tfprotov6.ApplyResourceChangeRequest) (*tfprotov6.ApplyResourceChangeResponse, error) {
	panic("UNIMPLIMENTED")
}

// ImportResourceState is called when a user has requested Terraform
// import a resource. The provider should fetch the information
// specified by the passed ID and return it as one or more resource
// states for Terraform to assume control of.
func (p shimProvider) ImportResourceState(context.Context, *tfprotov6.ImportResourceStateRequest) (*tfprotov6.ImportResourceStateResponse, error) {
	panic("UNIMPLIMENTED")
}

// ValidateDataResourceConfig is called when Terraform is checking that a
// data source's configuration is valid. It is guaranteed to have types
// conforming to your schema, but it is not guaranteed that all values
// will be known. This is your opportunity to do custom or advanced
// validation prior to a plan being generated.
func (p shimProvider) ValidateDataResourceConfig(context.Context, *tfprotov6.ValidateDataResourceConfigRequest) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	panic("UNIMPLIMENTED")
}

// MoveResourceState is called when Terraform is asked to change a resource
// type for an existing resource. The provider must accept the change as
// valid by ensuring the source resource type, schema version, and provider
// address are compatible to convert the source state into the target
// resource type and latest state version.
//
// This functionality is only supported in Terraform 1.8 and later. The
// provider must have enabled the MoveResourceState server capability to
// enable these requests.
func (p shimProvider) MoveResourceState(context.Context, *tfprotov6.MoveResourceStateRequest) (*tfprotov6.MoveResourceStateResponse, error) {
	panic("UNIMPLIMENTED")
}

// ReadDataSource is called when Terraform is refreshing a data
// source's state.
func (p shimProvider) ReadDataSource(context.Context, *tfprotov6.ReadDataSourceRequest) (*tfprotov6.ReadDataSourceResponse, error) {
	panic("UNIMPLIMENTED")
}

// CallFunction is called when Terraform wants to execute the logic of a
// function referenced in the configuration.
func (p shimProvider) CallFunction(context.Context, *tfprotov6.CallFunctionRequest) (*tfprotov6.CallFunctionResponse, error) {
	panic("UNIMPLIMENTED")
}

// GetFunctions is called when Terraform wants to lookup which functions a
// provider supports when not calling GetProviderSchema.
func (p shimProvider) GetFunctions(context.Context, *tfprotov6.GetFunctionsRequest) (*tfprotov6.GetFunctionsResponse, error) {
	panic("UNIMPLIMENTED")
}
