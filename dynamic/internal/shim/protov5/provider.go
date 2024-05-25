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
	panic("UNIMPLIMENTED")
}

func (p provider) ConfigureProvider(ctx context.Context, req *tfprotov5.ConfigureProviderRequest) (*tfprotov5.ConfigureProviderResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) StopProvider(ctx context.Context, req *tfprotov5.StopProviderRequest) (*tfprotov5.StopProviderResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) ImportResourceState(ctx context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) MoveResourceState(ctx context.Context, req *tfprotov5.MoveResourceStateRequest) (*tfprotov5.MoveResourceStateResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) ValidateDataSourceConfig(ctx context.Context, req *tfprotov5.ValidateDataSourceConfigRequest) (*tfprotov5.ValidateDataSourceConfigResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) ReadDataSource(ctx context.Context, req *tfprotov5.ReadDataSourceRequest) (*tfprotov5.ReadDataSourceResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) CallFunction(ctx context.Context, req *tfprotov5.CallFunctionRequest) (*tfprotov5.CallFunctionResponse, error) {
	panic("UNIMPLIMENTED")
}

func (p provider) GetFunctions(ctx context.Context, req *tfprotov5.GetFunctionsRequest) (*tfprotov5.GetFunctionsResponse, error) {
	panic("UNIMPLIMENTED")
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
