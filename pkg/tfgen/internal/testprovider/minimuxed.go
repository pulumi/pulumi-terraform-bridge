package testprovider

import (
	testproviderdata "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func ProviderMiniMuxed() tfbridge.ProviderInfo {
	minimuxedPkg := "minimuxed"
	minimuxedMod := "index"

	return tfbridge.ProviderInfo{
		P:           shimv2.NewProvider(testproviderdata.ProviderMiniMuxed()),
		Name:        "minimuxed",
		Description: "A Pulumi package to safely use minimuxed resources in Pulumi programs.",
		Keywords:    []string{"pulumi", "minimuxed"},
		License:     "Apache-2.0",
		Homepage:    "https://pulumi.io",
		Repository:  "https://github.com/pulumi/pulumi-minimuxed",
		Resources: map[string]*tfbridge.ResourceInfo{
			"minimuxed_integer": {Tok: tfbridge.MakeResource(minimuxedPkg, minimuxedMod, "MinimuxedInteger")},
		},
		MuxWith: []muxer.Provider{
			newMuxProvider(),
		},
	}
}

func newMuxProvider() muxer.Provider {
	return &muxProvider{
		packageSchema: schema.PackageSpec{
			Name: "minimuxed",
			Types: map[string]schema.ComplexTypeSpec{
				"minimuxed:index/muxedFunctionResult:muxedFunctionResult": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type:        "object",
						Description: "A collection of values returned by muxedFunction.",
						Properties: map[string]schema.PropertySpec{
							"value": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
						},
						Required: []string{
							"value",
						},
					},
				},
			},
			Functions: map[string]schema.FunctionSpec{
				"minimuxed:index/muxedFunction:muxedFunction": {
					Inputs: &schema.ObjectTypeSpec{},
					ReturnType: &schema.ReturnTypeSpec{
						ObjectTypeSpec: &schema.ObjectTypeSpec{
							Type: "object",
							Properties: map[string]schema.PropertySpec{
								"values": {
									TypeSpec: schema.TypeSpec{
										Type: "array",
										Items: &schema.TypeSpec{
											Ref: "#/types/minimuxed:index/muxedFunctionResult:muxedFunctionResult",
										},
									},
								},
							},
							Required: []string{
								"values",
							},
						},
					},
				},
			},
		},
	}
}

type muxProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	packageSchema schema.PackageSpec
}

func (m *muxProvider) GetSpec() (schema.PackageSpec, error) {
	return m.packageSchema, nil
}

func (m *muxProvider) GetInstance(*provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
	return m, nil
}
