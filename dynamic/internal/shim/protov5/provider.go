package protov5

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/opentofu/opentofu/internal/tfplugin5"
	otshim "github.com/opentofu/opentofu/shim/tfplugin5"
	"google.golang.org/grpc"
)

var _ tfprotov5.ProviderServer = (*provider)(nil)

func New(server tfplugin5.ProviderClient) tfprotov5.ProviderServer {
	return provider{server}
}

type provider struct{ remote tfplugin5.ProviderClient }

func (p provider) GetMetadata(ctx context.Context, req *tfprotov5.GetMetadataRequest) (*tfprotov5.GetMetadataResponse, error) {
	return translateGRPC(ctx, p.remote.GetMetadata, otshim.GetMetadataRequest(req), otshim.GetMetadataResponse)
}

func (p provider) GetProviderSchema(ctx context.Context, req *tfprotov5.GetProviderSchemaRequest) (*tfprotov5.GetProviderSchemaResponse, error) {
	return translateGRPC(ctx, p.remote.GetSchema, otshim.GetProviderSchemaRequest(req), otshim.GetProviderSchemaResponse)
}

func (p provider) PrepareProviderConfig(ctx context.Context, req *tfprotov5.PrepareProviderConfigRequest) (*tfprotov5.PrepareProviderConfigResponse, error) {
	return translateGRPC(ctx, p.remote.PrepareProviderConfig, otshim.PrepareProviderConfigRequest(req), otshim.PrepareProviderConfigResponse)
}

func (p provider) ConfigureProvider(ctx context.Context, req *tfprotov5.ConfigureProviderRequest) (*tfprotov5.ConfigureProviderResponse, error) {
	return translateGRPC(ctx, p.remote.Configure, otshim.ConfigureProviderRequest(req), otshim.ConfigureProviderResponse)
}

func (p provider) StopProvider(ctx context.Context, req *tfprotov5.StopProviderRequest) (*tfprotov5.StopProviderResponse, error) {
	return translateGRPC(ctx, p.remote.Stop, otshim.StopProviderRequest(req), otshim.StopProviderResponse)
}

func (p provider) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	return translateGRPC(ctx, p.remote.ValidateResourceTypeConfig, otshim.ValidateResourceTypeConfigRequest(req), otshim.ValidateResourceTypeConfigResponse)
}

func (p provider) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	return translateGRPC(ctx, p.remote.UpgradeResourceState, otshim.UpgradeResourceStateRequest(req), otshim.UpgradeResourceStateResponse)
}

func (p provider) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	return translateGRPC(ctx, p.remote.ReadResource, otshim.ReadResourceRequest(req), otshim.ReadResourceResponse)
}

func (p provider) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	return translateGRPC(ctx, p.remote.PlanResourceChange, otshim.PlanResourceChangeRequest(req), otshim.PlanResourceChangeResponse)
}

func (p provider) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	return translateGRPC(ctx, p.remote.ApplyResourceChange, otshim.ApplyResourceChangeRequest(req), otshim.ApplyResourceChangeResponse)
}

func (p provider) ImportResourceState(ctx context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	return translateGRPC(ctx, p.remote.ImportResourceState, otshim.ImportResourceStateRequest(req), otshim.ImportResourceStateResponse)
}

func (p provider) MoveResourceState(ctx context.Context, req *tfprotov5.MoveResourceStateRequest) (*tfprotov5.MoveResourceStateResponse, error) {
	return translateGRPC(ctx, p.remote.MoveResourceState, otshim.MoveResourceStateRequest(req), otshim.MoveResourceStateResponse)
}

func (p provider) ValidateDataSourceConfig(ctx context.Context, req *tfprotov5.ValidateDataSourceConfigRequest) (*tfprotov5.ValidateDataSourceConfigResponse, error) {
	return translateGRPC(ctx, p.remote.ValidateDataSourceConfig, otshim.ValidateDataSourceConfigRequest(req), otshim.ValidateDataSourceConfigResponse)
}

func (p provider) ReadDataSource(ctx context.Context, req *tfprotov5.ReadDataSourceRequest) (*tfprotov5.ReadDataSourceResponse, error) {
	return translateGRPC(ctx, p.remote.ReadDataSource, otshim.ReadDataSourceRequest(req), otshim.ReadDataSourceResponse)
}

func (p provider) CallFunction(ctx context.Context, req *tfprotov5.CallFunctionRequest) (*tfprotov5.CallFunctionResponse, error) {
	return translateGRPC(ctx, p.remote.CallFunction, otshim.CallFunctionRequest(req), otshim.CallFunctionResponse)
}

func (p provider) GetFunctions(ctx context.Context, req *tfprotov5.GetFunctionsRequest) (*tfprotov5.GetFunctionsResponse, error) {
	return translateGRPC(ctx, p.remote.GetFunctions, otshim.GetFunctionsRequest(req), otshim.GetFunctionsResponse)
}

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
