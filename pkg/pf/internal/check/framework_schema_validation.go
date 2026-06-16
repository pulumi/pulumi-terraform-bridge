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
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type frameworkProviderShim interface {
	// FrameworkProvider returns the original Terraform Plugin Framework provider
	// behind a schema-only shim. check uses this narrow internal hook for
	// build-time Framework validation without adding Framework-specific methods
	// to public shim interfaces.
	FrameworkProvider() provider.Provider
}

// validateFrameworkSchemas runs Terraform Plugin Framework implementation
// validation for the generated PF surface before Pulumi schema generation.
//
// Runtime startup no longer calls full GetProviderSchema for static providers,
// so this is the build-time replacement for Framework provider-wide validation.
// It unwraps muxed providers, validates each PF sub-provider, and validates
// only resources, data sources, and list resources that are generated into the
// Pulumi schema.
func validateFrameworkSchemas(
	ctx context.Context,
	sink diag.Sink,
	info tfbridge.ProviderInfo,
	isPFResource func(tfToken string) bool,
	isPFDataSource func(tfToken string) bool,
) error {
	var errs []error
	for _, frameworkProvider := range frameworkProviders(info.P) {
		errs = append(errs, validateFrameworkProvider(
			ctx,
			sink,
			frameworkProvider,
			generatedPFResources(info, isPFResource),
			generatedPFDataSources(info, isPFDataSource),
			generatedListResources(info),
		)...)
	}
	return errors.Join(errs...)
}

// generatedPFResources returns the Terraform resource type names that belong
// to PF and are included in the generated Pulumi schema. SDKv2 resources in a
// muxed provider and nil mapping entries are excluded so validation follows the
// same ownership boundary as generation.
func generatedPFResources(info tfbridge.ProviderInfo, isPFResource func(tfToken string) bool) map[string]bool {
	generated := map[string]bool{}
	for name, resInfo := range info.Resources {
		if resInfo == nil || !isPFResource(name) {
			continue
		}
		generated[name] = true
	}
	return generated
}

// generatedPFDataSources returns the Terraform data source type names that
// belong to PF and are included in the generated Pulumi schema. SDKv2 data
// sources in a muxed provider and nil mapping entries are excluded so
// validation does not reject provider entries the bridge is not generating.
func generatedPFDataSources(info tfbridge.ProviderInfo, isPFDataSource func(tfToken string) bool) map[string]bool {
	generated := map[string]bool{}
	for name, dsInfo := range info.DataSources {
		if dsInfo == nil || !isPFDataSource(name) {
			continue
		}
		generated[name] = true
	}
	return generated
}

// generatedListResources returns Terraform resource type names that have
// generated Pulumi resources and may therefore need PF list-resource validation.
// List resources are keyed by the same Terraform type name as the corresponding
// CRUD resource.
func generatedListResources(info tfbridge.ProviderInfo) map[string]bool {
	generated := map[string]bool{}
	for name, resInfo := range info.Resources {
		if resInfo == nil {
			continue
		}
		generated[name] = true
	}
	return generated
}

// frameworkProviders returns the concrete Framework providers hidden behind a
// direct schema-only shim or behind the PF side of a muxed provider. Providers
// that do not expose the internal FrameworkProvider hook are ignored because
// they cannot be Framework-validated here.
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

// frameworkProvider unwraps one shim provider into its original Framework
// provider when the shim was built by the PF schema-only path.
func frameworkProvider(p shim.Provider) provider.Provider {
	if p, ok := p.(frameworkProviderShim); ok {
		return p.FrameworkProvider()
	}
	return nil
}

// validateFrameworkProvider validates one concrete Framework provider and all
// generated PF entity schemas reachable from it. It returns every validation
// error instead of stopping at the first entity so provider upgrades can report
// all invalid Framework schemas in one tfgen run.
func validateFrameworkProvider(
	ctx context.Context,
	sink diag.Sink,
	p provider.Provider,
	generatedResources map[string]bool,
	generatedDataSources map[string]bool,
	generatedListResources map[string]bool,
) []error {
	if p == nil {
		return nil
	}

	var errs []error
	providerTypeName := frameworkProviderTypeName(ctx, p)
	errs = append(errs, validateFrameworkProviderSchema(ctx, sink, p, providerTypeName))
	errs = append(errs, validateFrameworkResourceSchemas(ctx, sink, p, providerTypeName, generatedResources)...)
	errs = append(errs, validateFrameworkDataSourceSchemas(ctx, sink, p, providerTypeName, generatedDataSources)...)
	errs = append(errs, validateFrameworkListResourceSchemas(ctx, sink, p, providerTypeName, generatedListResources)...)
	return errs
}

// frameworkProviderTypeName asks the Framework provider for its type name so
// resource, data source, and list resource Metadata calls compute the same
// Terraform type names they would use at runtime.
func frameworkProviderTypeName(ctx context.Context, p provider.Provider) string {
	resp := &provider.MetadataResponse{}
	p.Metadata(ctx, provider.MetadataRequest{}, resp)
	return resp.TypeName
}

// validateFrameworkProviderSchema checks provider.Schema diagnostics and then
// runs Framework ValidateImplementation on the provider config schema. Both
// failures are reported through the Pulumi diagnostics sink before returning.
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

// validateFrameworkResourceSchemas validates every generated PF resource schema
// for a provider. Resource schemas that are present upstream but not generated
// by this bridge mapping are skipped so optional or unsupported upstream
// resources do not block schema generation.
func validateFrameworkResourceSchemas(
	ctx context.Context,
	sink diag.Sink,
	p provider.Provider,
	providerTypeName string,
	generatedResources map[string]bool,
) []error {
	var errs []error
	for _, makeResource := range p.Resources(ctx) {
		r := makeResource()
		meta := &resource.MetadataResponse{}
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: providerTypeName}, meta)
		if !generatedResources[meta.TypeName] {
			continue
		}

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

// validateFrameworkDataSourceSchemas validates every generated PF data source
// schema for a provider. Data sources that are present upstream but not
// generated by this bridge mapping are skipped for the same reason as resources.
func validateFrameworkDataSourceSchemas(
	ctx context.Context,
	sink diag.Sink,
	p provider.Provider,
	providerTypeName string,
	generatedDataSources map[string]bool,
) []error {
	var errs []error
	for _, makeDataSource := range p.DataSources(ctx) {
		ds := makeDataSource()
		meta := &datasource.MetadataResponse{}
		ds.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: providerTypeName}, meta)
		if !generatedDataSources[meta.TypeName] {
			continue
		}

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

// validateFrameworkListResourceSchemas validates PF list query schemas for
// generated resources. Framework list resources are optional, so providers that
// do not implement ProviderWithListResources have no list-resource validation
// work to do.
func validateFrameworkListResourceSchemas(
	ctx context.Context,
	sink diag.Sink,
	p provider.Provider,
	providerTypeName string,
	generatedListResources map[string]bool,
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
		if !generatedListResources[meta.TypeName] {
			continue
		}

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

// frameworkDiagnosticsError converts Framework diagnostics into one bridge
// error that identifies the entity kind, Terraform type name, and Framework
// operation that failed. Non-error diagnostics do not block generation.
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

// reportFrameworkValidationError mirrors validation errors into the configured
// Pulumi diagnostics sink while still allowing callers to aggregate and return
// the underlying errors.
func reportFrameworkValidationError(sink diag.Sink, err error) {
	if sink != nil {
		sink.Errorf(&diag.Diag{Message: err.Error()})
	}
}
