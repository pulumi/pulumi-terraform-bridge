package testprovider

import (
	"context"

	testproviderdata "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func ProviderMiniMuxedReplace() tfbridge.ProviderInfo {
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
		MuxWith: []tfbridge.MuxProvider{
			newMuxReplaceProvider(),
		},
	}
}

func newMuxReplaceProvider() tfbridge.MuxProvider {
	return &muxReplaceProvider{
		packageSchema: schema.PackageSpec{
			Name: "minimuxed",
			Resources: map[string]schema.ResourceSpec{
				"minimuxed:index/minimuxedInteger:MinimuxedInteger": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"max": {
								TypeSpec: schema.TypeSpec{
									Type: "integer",
								},
							},
							"min": {
								TypeSpec: schema.TypeSpec{
									Type: "integer",
								},
							},
							"result": {
								TypeSpec: schema.TypeSpec{
									Type: "integer",
								},
							},
						},
						Required: []string{
							"max",
							"min",
							"result",
						},
					},
					InputProperties: map[string]schema.PropertySpec{
						"max": {
							TypeSpec: schema.TypeSpec{
								Type: "integer",
							},
							WillReplaceOnChanges: true,
						},
						"min": {
							TypeSpec: schema.TypeSpec{
								Type: "integer",
							},
							WillReplaceOnChanges: true,
						},
					},
					RequiredInputs: []string{
						"max",
						"min",
					},
					StateInputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"max": {
								TypeSpec: schema.TypeSpec{
									Type: "integer",
								},
								WillReplaceOnChanges: true,
							},
							"min": {
								TypeSpec: schema.TypeSpec{
									Type: "integer",
								},
								WillReplaceOnChanges: true,
							},
							"result": {
								TypeSpec: schema.TypeSpec{
									Type: "integer",
								},
							},
						},
						Type: "object",
					},
				},
			},
		},
	}
}

type muxReplaceProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	packageSchema schema.PackageSpec
}

func (m *muxReplaceProvider) GetSpec(ctx context.Context, name, version string) (schema.PackageSpec, error) {
	return m.packageSchema, nil
}

func (m *muxReplaceProvider) GetInstance(ctx context.Context, name, version string, host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
	return m, nil
}
