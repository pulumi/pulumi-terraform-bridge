// Copyright 2016-2022, Pulumi Corporation.
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

package schemashim

import (
	"context"
	"fmt"

	pfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/pfutils"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/diagnostics"
)

var _ = pf.ShimProvider(&SchemaOnlyProvider{})

// SchemaOnlyProvider adapts a Terraform Plugin Framework provider to the bridge
// shim interfaces used by schema generation and PF runtime setup. It gathers
// cheap entity metadata up front and keeps resource, data source, and list
// resource schemas lazy so runtime startup can avoid a full Framework
// GetProviderSchema call.
type SchemaOnlyProvider struct {
	ctx             context.Context
	tf              pfprovider.Provider
	resourceMap     schemaOnlyResourceMap
	dataSourceMap   schemaOnlyDataSourceMap
	listResourceMap schemaOnlyListResourceMap
	functions       map[string]shim.Function
	internalinter.Internal
}

// FrameworkProvider exposes the original Framework provider to internal bridge
// packages that need concrete Framework APIs, such as build-time
// ValidateImplementation checks. It deliberately stays as an internal hook
// rather than broadening the public shim.Provider surface.
func (p *SchemaOnlyProvider) FrameworkProvider() pfprovider.Provider {
	return p.tf
}

// Server creates the Framework protocol server without calling full
// GetProviderSchema. Metadata and per-entity schema loading happen through the
// schema-only maps and Framework per-RPC paths instead.
func (p *SchemaOnlyProvider) Server(context.Context) (tfprotov6.ProviderServer, error) {
	newServer6 := providerserver.NewProtocol6(p.tf)
	return newServer6(), nil
}

func (p *SchemaOnlyProvider) Resources(ctx context.Context) (runtypes.Resources, error) {
	return p.resourceMap, nil
}

func (p *SchemaOnlyProvider) DataSources(ctx context.Context) (runtypes.DataSources, error) {
	return p.dataSourceMap, nil
}

func (p *SchemaOnlyProvider) Config(ctx context.Context) (tftypes.Object, error) {
	schemaResponse := &pfprovider.SchemaResponse{}
	p.tf.Schema(ctx, pfprovider.SchemaRequest{}, schemaResponse)
	schema, diags := schemaResponse.Schema, schemaResponse.Diagnostics
	if diags.HasError() {
		return tftypes.Object{}, fmt.Errorf("Schema() returned diagnostics with HasError")
	}

	return schema.Type().TerraformType(ctx).(tftypes.Object), nil
}

var _ shim.Provider = (*SchemaOnlyProvider)(nil)

func (p *SchemaOnlyProvider) Schema() shim.SchemaMap {
	ctx := p.ctx
	schemaResp := &pfprovider.SchemaResponse{}
	p.tf.Schema(ctx, pfprovider.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		panic("Schema() returned error diags")
	}
	return newSchemaMap(pfutils.FromProviderSchema(schemaResp.Schema))
}

func (p *SchemaOnlyProvider) ResourcesMap() shim.ResourceMap {
	return p.resourceMap
}

// ResourceSchemaFixupsMayBePrecomputed reports whether tfToken is a PF
// schema-only resource and therefore eligible to consume build-time
// default-fixup metadata at runtime. The caller still requires runtime metadata
// with an entry for the resource before it skips live schema inspection.
func (p *SchemaOnlyProvider) ResourceSchemaFixupsMayBePrecomputed(tfToken string) bool {
	_, ok := p.resourceMap.GetOk(tfToken)
	return ok
}

func (p *SchemaOnlyProvider) DataSourcesMap() shim.ResourceMap {
	return p.dataSourceMap
}

func (p *SchemaOnlyProvider) ListResourcesMap() shim.ResourceMap {
	return p.listResourceMap
}

func (p *SchemaOnlyProvider) Functions() map[string]shim.Function {
	return p.functions
}

func (p *SchemaOnlyProvider) InternalValidate() error {
	return nil
}

func (p *SchemaOnlyProvider) Validate(
	context.Context, shim.ResourceConfig,
) ([]diagnostics.ValidationWarning, []error) {
	panic("schemaOnlyProvider does not implement runtime operation Validate")
}

func (p *SchemaOnlyProvider) ValidateResource(
	context.Context, string, shim.ResourceConfig,
) ([]diagnostics.ValidationWarning, []error) {
	panic("schemaOnlyProvider does not implement runtime operation ValidateResource")
}

func (p *SchemaOnlyProvider) ValidateDataSource(
	context.Context, string, shim.ResourceConfig,
) ([]diagnostics.ValidationWarning, []error) {
	panic("schemaOnlyProvider does not implement runtime operation ValidateDataSource")
}

func (p *SchemaOnlyProvider) Configure(ctx context.Context, c shim.ResourceConfig) error {
	panic("schemaOnlyProvider does not implement runtime operation Configure")
}

func (p *SchemaOnlyProvider) Diff(
	context.Context, string, shim.InstanceState, shim.ResourceConfig, shim.DiffOptions,
) (shim.InstanceDiff, error) {
	panic("schemaOnlyProvider does not implement runtime operation Diff")
}

func (p *SchemaOnlyProvider) Apply(
	context.Context, string, shim.InstanceState, shim.InstanceDiff,
) (shim.InstanceState, error) {
	panic("schemaOnlyProvider does not implement runtime operation Apply")
}

func (p *SchemaOnlyProvider) Refresh(
	context.Context, string, shim.InstanceState, shim.ResourceConfig,
) (shim.InstanceState, error) {
	panic("schemaOnlyProvider does not implement runtime operation Refresh")
}

func (p *SchemaOnlyProvider) ReadDataDiff(
	context.Context, string, shim.ResourceConfig,
) (shim.InstanceDiff, error) {
	panic("schemaOnlyProvider does not implement runtime operation ReadDataDiff")
}

func (p *SchemaOnlyProvider) ReadDataApply(
	context.Context, string, shim.InstanceDiff,
) (shim.InstanceState, error) {
	panic("schemaOnlyProvider does not implement runtime operation ReadDataApply")
}

func (p *SchemaOnlyProvider) Meta(context.Context) interface{} {
	panic("schemaOnlyProvider does not implement runtime operation Meta")
}

func (p *SchemaOnlyProvider) Stop(context.Context) error {
	panic("schemaOnlyProvider does not implement runtime operation Stop")
}

func (p *SchemaOnlyProvider) InitLogging(context.Context) {
	panic("schemaOnlyProvider does not implement runtime operation InitLogging")
}

func (p *SchemaOnlyProvider) NewDestroyDiff(context.Context, string, shim.TimeoutOptions) shim.InstanceDiff {
	panic("schemaOnlyProvider does not implement runtime operation NewDestroyDiff")
}

func (p *SchemaOnlyProvider) NewResourceConfig(context.Context, map[string]interface{}) shim.ResourceConfig {
	panic("schemaOnlyProvider does not implement runtime operation ResourceConfig")
}

func (p *SchemaOnlyProvider) NewProviderConfig(context.Context, map[string]interface{}) shim.ResourceConfig {
	panic("schemaOnlyProvider does not implement runtime operation ProviderConfig")
}

func (p *SchemaOnlyProvider) IsSet(context.Context, interface{}) ([]interface{}, bool) {
	panic("schemaOnlyProvider does not implement runtime operation IsSet")
}

func (p *SchemaOnlyProvider) SupportsUnknownCollections() bool {
	return true
}
