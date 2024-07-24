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

package pfutils

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// Assist converting Plugin framework schemata to proto schemata for a resource.
func convertResourceSchemaToProto(ctx context.Context, s *rschema.Schema) (*tfprotov6.Schema, error) {
	p := &singleResourceProvider{s}

	mk := providerserver.NewProtocol6WithError(p)
	srv, err := mk()
	if err != nil {
		return nil, err
	}
	resp, err := srv.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		return nil, err
	}
	var diagErrors []error
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			diagErrors = append(diagErrors, d.Attribute.NewErrorf("%s\n%s", d.Summary, d.Detail))
		}
	}
	if err := errors.Join(diagErrors...); err != nil {
		return nil, err
	}
	for _, r := range resp.ResourceSchemas {
		return r, nil
	}
	return nil, fmt.Errorf("GetProviderSchema did not return any resource schemas")
}

type singleResourceProvider struct {
	resourceSchema *rschema.Schema
}

var _ provider.Provider = &singleResourceProvider{}

func (srp singleResourceProvider) Metadata(
	ctx context.Context,
	req provider.MetadataRequest,
	resp *provider.MetadataResponse,
) {
	resp.TypeName = "p"
}

func (srp singleResourceProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {
}

func (srp singleResourceProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

func (srp singleResourceProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

func (srp singleResourceProvider) Resources(context.Context) []func() resource.Resource {
	mk := func() resource.Resource {
		return &schemaOnlyResource{srp.resourceSchema}
	}
	return []func() resource.Resource{mk}
}

type schemaOnlyResource struct {
	schema *rschema.Schema
}

var _ resource.Resource = &schemaOnlyResource{}

func (r *schemaOnlyResource) Metadata(
	ctx context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = "r"
}

func (r *schemaOnlyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	if r.schema != nil {
		resp.Schema = *r.schema
	}
}

func (r *schemaOnlyResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {
}

func (r *schemaOnlyResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

func (r *schemaOnlyResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (r *schemaOnlyResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
