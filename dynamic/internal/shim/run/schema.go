package run

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/protov6/translate"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6"
)

// A TFProviderSchema represents serializable, statically-available data extracted from a TF provider.
type TFProviderSchema struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`

	Metadata *tfplugin6.GetMetadata_Response       `json:"metadata,omitempty"`
	Schema   *tfplugin6.GetProviderSchema_Response `json:"schema,omitempty"`
}

// ExtractTFPRoviderSchema extracts static schema information from a TF provider.
func ExtractTFPRoviderSchema(ctx context.Context, p Provider) (*TFProviderSchema, error) {
	metadata, err := p.GetMetadata(ctx, &tfprotov6.GetMetadataRequest{})
	if err != nil {
		return nil, fmt.Errorf("getting metadata: %w", err)
	}

	schema, err := p.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		return nil, fmt.Errorf("getting schema: %w", err)
	}

	return &TFProviderSchema{
		Name:     p.Name(),
		Version:  p.Version(),
		URL:      p.URL(),
		Metadata: translate.GetMetadata_Response(metadata),
		Schema:   translate.GetProviderSchema_Response(schema),
	}, nil
}

var _ = tfprotov6.ProviderServer((*schemaClient)(nil))

type schemaClient struct {
	metadata *tfprotov6.GetMetadataResponse
	schema   *tfprotov6.GetProviderSchemaResponse
}

func newSchemaClient(s TFProviderSchema) *schemaClient {
	return &schemaClient{
		metadata: translate.GetMetadataResult(s.Metadata),
		schema:   translate.GetProviderSchemaResponse(s.Schema),
	}
}

func (s *schemaClient) ConfigureProvider(context.Context, *tfprotov6.ConfigureProviderRequest) (*tfprotov6.ConfigureProviderResponse, error) {
	return nil, nil
}

func (s *schemaClient) GetFunctions(context.Context, *tfprotov6.GetFunctionsRequest) (*tfprotov6.GetFunctionsResponse, error) {
	if s.schema == nil {
		return &tfprotov6.GetFunctionsResponse{}, nil
	}
	return &tfprotov6.GetFunctionsResponse{Functions: s.schema.Functions}, nil
}

func (s *schemaClient) GetMetadata(context.Context, *tfprotov6.GetMetadataRequest) (*tfprotov6.GetMetadataResponse, error) {
	return s.metadata, nil
}

func (s *schemaClient) GetProviderSchema(context.Context, *tfprotov6.GetProviderSchemaRequest) (*tfprotov6.GetProviderSchemaResponse, error) {
	return s.schema, nil
}

func (s *schemaClient) GetResourceIdentitySchemas(context.Context, *tfprotov6.GetResourceIdentitySchemasRequest) (*tfprotov6.GetResourceIdentitySchemasResponse, error) {
	return &tfprotov6.GetResourceIdentitySchemasResponse{}, nil
}

func (s *schemaClient) StopProvider(context.Context, *tfprotov6.StopProviderRequest) (*tfprotov6.StopProviderResponse, error) {
	return nil, nil
}

// All methods below this marker return an error.

func (s *schemaClient) ApplyResourceChange(context.Context, *tfprotov6.ApplyResourceChangeRequest) (*tfprotov6.ApplyResourceChangeResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) CallFunction(context.Context, *tfprotov6.CallFunctionRequest) (*tfprotov6.CallFunctionResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) CloseEphemeralResource(context.Context, *tfprotov6.CloseEphemeralResourceRequest) (*tfprotov6.CloseEphemeralResourceResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) ImportResourceState(context.Context, *tfprotov6.ImportResourceStateRequest) (*tfprotov6.ImportResourceStateResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) MoveResourceState(context.Context, *tfprotov6.MoveResourceStateRequest) (*tfprotov6.MoveResourceStateResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) OpenEphemeralResource(context.Context, *tfprotov6.OpenEphemeralResourceRequest) (*tfprotov6.OpenEphemeralResourceResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) PlanResourceChange(context.Context, *tfprotov6.PlanResourceChangeRequest) (*tfprotov6.PlanResourceChangeResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) ReadDataSource(context.Context, *tfprotov6.ReadDataSourceRequest) (*tfprotov6.ReadDataSourceResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) ReadResource(context.Context, *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) RenewEphemeralResource(context.Context, *tfprotov6.RenewEphemeralResourceRequest) (*tfprotov6.RenewEphemeralResourceResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) UpgradeResourceIdentity(context.Context, *tfprotov6.UpgradeResourceIdentityRequest) (*tfprotov6.UpgradeResourceIdentityResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) UpgradeResourceState(context.Context, *tfprotov6.UpgradeResourceStateRequest) (*tfprotov6.UpgradeResourceStateResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) ValidateDataResourceConfig(context.Context, *tfprotov6.ValidateDataResourceConfigRequest) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) ValidateEphemeralResourceConfig(context.Context, *tfprotov6.ValidateEphemeralResourceConfigRequest) (*tfprotov6.ValidateEphemeralResourceConfigResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) ValidateProviderConfig(context.Context, *tfprotov6.ValidateProviderConfigRequest) (*tfprotov6.ValidateProviderConfigResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}

func (s *schemaClient) ValidateResourceConfig(context.Context, *tfprotov6.ValidateResourceConfigRequest) (*tfprotov6.ValidateResourceConfigResponse, error) {
	return nil, fmt.Errorf("this provider is limited to schema-only operations")
}
