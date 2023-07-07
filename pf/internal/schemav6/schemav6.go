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

package schemav6

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const (
	resourceName = "xres"
	dsName       = "xds"
)

func ProviderSchema(schema pschema.Schema) (*tfprotov6.Schema, error) {
	psrv := providerserver.NewProtocol6(&schemaExtractingProvider{
		providerSchema: &schema,
	})

	resp, err := psrv().GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		return nil, err
	}

	if err := renderDiagnostics(resp.Diagnostics); err != nil {
		return nil, err
	}

	contract.Assertf(resp.Provider != nil, "schemav6.ProviderSchema: nil Provider in GetProviderSchema response")
	return resp.Provider, nil
}

func ResourceSchema(schema rschema.Schema) (*tfprotov6.Schema, error) {
	psrv := providerserver.NewProtocol6(&schemaExtractingProvider{
		resourceSchemas: map[string]rschema.Schema{
			resourceName: schema,
		}})

	resp, err := psrv().GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		return nil, err
	}

	if err := renderDiagnostics(resp.Diagnostics); err != nil {
		return nil, err
	}

	res, ok := resp.ResourceSchemas[resourceName]
	contract.Assertf(ok, "schemav6.ResourceSchema: no %q resource in GetProviderSchema response", resourceName)

	return res, nil
}

func DataSourceSchema(schema dschema.Schema) (*tfprotov6.Schema, error) {
	psrv := providerserver.NewProtocol6(&schemaExtractingProvider{
		dsSchemas: map[string]dschema.Schema{
			dsName: schema,
		}})

	resp, err := psrv().GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		return nil, err
	}

	if err := renderDiagnostics(resp.Diagnostics); err != nil {
		return nil, err
	}

	res, ok := resp.DataSourceSchemas[dsName]
	contract.Assertf(ok, "schemav6.DataSourceSchema: no %q datasource in GetProviderSchema response", dsName)

	return res, nil
}

type schemaExtractingProvider struct {
	providerSchema  *pschema.Schema
	resourceSchemas map[string]rschema.Schema
	dsSchemas       map[string]dschema.Schema
}

func (*schemaExtractingProvider) Metadata(context.Context, provider.MetadataRequest, *provider.MetadataResponse) {
}

func (p *schemaExtractingProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	if p.providerSchema != nil {
		resp.Schema = *p.providerSchema
	}
}

func (*schemaExtractingProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

func (p *schemaExtractingProvider) DataSources(context.Context) []func() datasource.DataSource {
	keys := mapKeys(p.dsSchemas)
	ret := []func() datasource.DataSource{}
	for _, k := range keys {
		k := k
		ret = append(ret, func() datasource.DataSource {
			return &schemaExtractingDS{p.dsSchemas[k]}
		})
	}
	return ret
}

func (p *schemaExtractingProvider) Resources(context.Context) []func() resource.Resource {
	keys := mapKeys(p.resourceSchemas)
	ret := []func() resource.Resource{}
	for _, k := range keys {
		k := k
		ret = append(ret, func() resource.Resource {
			return &schemaExtractingResource{p.resourceSchemas[k]}
		})
	}
	return ret
}

var _ provider.Provider = (*schemaExtractingProvider)(nil)

type schemaExtractingResource struct {
	rschema rschema.Schema
}

func (r *schemaExtractingResource) Metadata(
	_ context.Context,
	_ resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = resourceName
}

func (r *schemaExtractingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.rschema
}

func (r *schemaExtractingResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {
}

func (r *schemaExtractingResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

func (r *schemaExtractingResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (r *schemaExtractingResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

var _ resource.Resource = (*schemaExtractingResource)(nil)

type schemaExtractingDS struct {
	dschema dschema.Schema
}

func (d *schemaExtractingDS) Metadata(
	_ context.Context,
	_ datasource.MetadataRequest,
	resp *datasource.MetadataResponse) {
	resp.TypeName = dsName
}

func (d *schemaExtractingDS) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = d.dschema
}

func (d *schemaExtractingDS) Read(context.Context, datasource.ReadRequest, *datasource.ReadResponse) {
}

var _ datasource.DataSource = (*schemaExtractingDS)(nil)

func mapKeys[T any](m map[string]T) []string {
	r := []string{}
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func formatDiagnostic(w io.Writer, d *tfprotov6.Diagnostic) {
	fmt.Fprintf(w, "%v", d.Severity)
	if d.Attribute != nil {
		fmt.Fprintf(w, " at %v", d.Attribute.String())
	}
	fmt.Fprintf(w, ". %s", d.Summary)
	if d.Detail != "" {
		fmt.Fprintf(w, ": %s", d.Detail)
	}
}

func formatDiagnostics(w io.Writer, diags []*tfprotov6.Diagnostic) {
	fmt.Fprintf(w, "%d unexpected diagnostic(s):", len(diags))
	fmt.Fprintln(w)
	for _, d := range diags {
		fmt.Fprintf(w, "- ")
		formatDiagnostic(w, d)
		fmt.Fprintln(w)
	}
}

func renderDiagnostics(diags []*tfprotov6.Diagnostic) error {
	if len(diags) > 0 {
		var buf bytes.Buffer
		formatDiagnostics(&buf, diags)
		return fmt.Errorf("%s", buf.String())
	}
	return nil
}
