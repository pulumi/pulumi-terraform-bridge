// Copyright 2016-2026, Pulumi Corporation.
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

package check

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwdiag "github.com/hashicorp/terraform-plugin-framework/diag"
	tflist "github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/muxer"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type frameworkProviderShim interface {
	FrameworkProvider() provider.Provider
}

func validateFrameworkSchemas(ctx context.Context, sink diag.Sink, p shim.Provider) error {
	var errs []error
	for _, frameworkProvider := range frameworkProviders(p) {
		errs = append(errs, validateFrameworkProvider(ctx, sink, frameworkProvider)...)
	}
	return errors.Join(errs...)
}

func frameworkProviders(p shim.Provider) []provider.Provider {
	if p, ok := p.(*muxer.ProviderShim); ok {
		var providers []provider.Provider
		for _, subProvider := range p.MuxedProviders {
			if provider := frameworkProvider(subProvider); provider != nil {
				providers = append(providers, provider)
			}
		}
		return providers
	}
	if frameworkProvider := frameworkProvider(p); frameworkProvider != nil {
		return []provider.Provider{frameworkProvider}
	}
	return nil
}

func frameworkProvider(p shim.Provider) provider.Provider {
	if p, ok := p.(frameworkProviderShim); ok {
		return p.FrameworkProvider()
	}
	return nil
}

func validateFrameworkProvider(ctx context.Context, sink diag.Sink, p provider.Provider) []error {
	if p == nil {
		return nil
	}

	var errs []error
	providerTypeName := frameworkProviderTypeName(ctx, p)
	errs = append(errs, validateFrameworkProviderSchema(ctx, sink, p, providerTypeName))
	errs = append(errs, validateFrameworkResourceSchemas(ctx, sink, p, providerTypeName)...)
	errs = append(errs, validateFrameworkDataSourceSchemas(ctx, sink, p, providerTypeName)...)
	errs = append(errs, validateFrameworkListResourceSchemas(ctx, sink, p, providerTypeName)...)
	return errs
}

func frameworkProviderTypeName(ctx context.Context, p provider.Provider) string {
	resp := &provider.MetadataResponse{}
	p.Metadata(ctx, provider.MetadataRequest{}, resp)
	return resp.TypeName
}

func validateFrameworkProviderSchema(
	ctx context.Context, sink diag.Sink, p provider.Provider, providerTypeName string,
) error {
	resp := &provider.SchemaResponse{}
	p.Schema(ctx, provider.SchemaRequest{}, resp)
	if err := frameworkDiagnosticsError("provider", providerTypeName, "Schema", resp.Diagnostics); err != nil {
		reportFrameworkValidationError(sink, err)
		return err
	}
	if err := frameworkDiagnosticsError(
		"provider", providerTypeName, "ValidateImplementation", resp.Schema.ValidateImplementation(ctx),
	); err != nil {
		reportFrameworkValidationError(sink, err)
		return err
	}
	return nil
}

func validateFrameworkResourceSchemas(
	ctx context.Context, sink diag.Sink, p provider.Provider, providerTypeName string,
) []error {
	var errs []error
	for _, makeResource := range p.Resources(ctx) {
		r := makeResource()
		meta := &resource.MetadataResponse{}
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: providerTypeName}, meta)

		resp := &resource.SchemaResponse{}
		r.Schema(ctx, resource.SchemaRequest{}, resp)
		if err := frameworkDiagnosticsError("resource", meta.TypeName, "Schema", resp.Diagnostics); err != nil {
			reportFrameworkValidationError(sink, err)
			errs = append(errs, err)
			continue
		}
		if err := frameworkDiagnosticsError(
			"resource", meta.TypeName, "ValidateImplementation", resp.Schema.ValidateImplementation(ctx),
		); err != nil {
			reportFrameworkValidationError(sink, err)
			errs = append(errs, err)
		}
	}
	return errs
}

func validateFrameworkDataSourceSchemas(
	ctx context.Context, sink diag.Sink, p provider.Provider, providerTypeName string,
) []error {
	var errs []error
	for _, makeDataSource := range p.DataSources(ctx) {
		ds := makeDataSource()
		meta := &datasource.MetadataResponse{}
		ds.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: providerTypeName}, meta)

		resp := &datasource.SchemaResponse{}
		ds.Schema(ctx, datasource.SchemaRequest{}, resp)
		if err := frameworkDiagnosticsError("data source", meta.TypeName, "Schema", resp.Diagnostics); err != nil {
			reportFrameworkValidationError(sink, err)
			errs = append(errs, err)
			continue
		}
		if err := frameworkDiagnosticsError(
			"data source", meta.TypeName, "ValidateImplementation", resp.Schema.ValidateImplementation(ctx),
		); err != nil {
			reportFrameworkValidationError(sink, err)
			errs = append(errs, err)
		}
	}
	return errs
}

func validateFrameworkListResourceSchemas(
	ctx context.Context, sink diag.Sink, p provider.Provider, providerTypeName string,
) []error {
	listProvider, ok := p.(provider.ProviderWithListResources)
	if !ok {
		return nil
	}

	var errs []error
	for _, makeListResource := range listProvider.ListResources(ctx) {
		listResource := makeListResource()
		meta := &resource.MetadataResponse{}
		listResource.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: providerTypeName}, meta)

		resp := &tflist.ListResourceSchemaResponse{}
		listResource.ListResourceConfigSchema(ctx, tflist.ListResourceSchemaRequest{}, resp)
		if err := frameworkDiagnosticsError("list resource", meta.TypeName, "Schema", resp.Diagnostics); err != nil {
			reportFrameworkValidationError(sink, err)
			errs = append(errs, err)
			continue
		}
		if err := frameworkDiagnosticsError(
			"list resource", meta.TypeName, "ValidateImplementation", resp.Schema.ValidateImplementation(ctx),
		); err != nil {
			reportFrameworkValidationError(sink, err)
			errs = append(errs, err)
		}
	}
	return errs
}

func frameworkDiagnosticsError(kind, name, op string, diags fwdiag.Diagnostics) error {
	if !diags.HasError() {
		return nil
	}

	parts := make([]string, 0, diags.ErrorsCount())
	for _, diagnostic := range diags.Errors() {
		part := diagnostic.Summary()
		if detail := diagnostic.Detail(); detail != "" {
			part += ": " + detail
		}
		parts = append(parts, part)
	}

	if name == "" {
		name = "<unknown>"
	}
	return fmt.Errorf("Plugin Framework %s %s %s failed: %s", kind, name, op, strings.Join(parts, "; "))
}

func reportFrameworkValidationError(sink diag.Sink, err error) {
	if sink != nil {
		sink.Errorf(&diag.Diag{Message: err.Error()})
	}
}
