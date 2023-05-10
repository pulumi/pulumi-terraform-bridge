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

package tfgen

import (
	"context"
	"testing"

	pulumiSchema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"encoding/json"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	pftfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/stretchr/testify/require"
)

// Regressing an issue with AWS provider not recognizing that assume_role config setting is singular via
// listvalidator.SizeAtMost(1).
func TestMaxItemsOne(t *testing.T) {
	ctx := context.Background()
	s := schema.Schema{
		Blocks: map[string]schema.Block{
			"assume_role": schema.ListNestedBlock{
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"external_id": schema.StringAttribute{
							Optional:    true,
							Description: "A unique identifier that might be required when you assume a role in another account.",
						},
					},
				},
			},
		},
	}
	res, err := GenerateSchema(ctx, GenerateSchemaOptions{
		ProviderInfo: pftfbridge.ProviderInfo{
			ProviderInfo: tfbridge.ProviderInfo{
				Name: "testprovider",
			},
			NewProvider: func() provider.Provider {
				return &schemaTestProvider{s}
			},
		},
	})
	require.NoError(t, err)

	var schema pulumiSchema.PackageSpec
	if err := json.Unmarshal(res.ProviderMetadata.PackageSchema, &schema); err != nil {
		t.Fatal(err)
	}

	require.Contains(t, schema.Config.Variables, "assumeRole")
	require.NotContains(t, schema.Config.Variables, "assumeRoles")
}

type schemaTestProvider struct {
	schema schema.Schema
}

func (*schemaTestProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test_"
}

func (p *schemaTestProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = p.schema
}

func (*schemaTestProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
	panic("NOT IMPLEMENTED")
}

func (*schemaTestProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

func (*schemaTestProvider) Resources(context.Context) []func() resource.Resource {
	return nil
}
