package schema

import (
	"context"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type Provider struct {
	Schema         shim.SchemaMap
	ResourcesMap   shim.ResourceMap
	DataSourcesMap shim.ResourceMap
}

func (p *Provider) Shim() shim.Provider {
	c := p
	if c.Schema == nil {
		c.Schema = SchemaMap{}
	}
	if c.ResourcesMap == nil {
		c.ResourcesMap = ResourceMap{}
	}
	if c.DataSourcesMap == nil {
		c.DataSourcesMap = ResourceMap{}
	}
	return ProviderShim{c}
}

type ProviderShim struct {
	V *Provider
}

func (s ProviderShim) Schema() shim.SchemaMap {
	return s.V.Schema
}

func (s ProviderShim) ResourcesMap() shim.ResourceMap {
	return s.V.ResourcesMap
}

func (s ProviderShim) DataSourcesMap() shim.ResourceMap {
	return s.V.DataSourcesMap
}

func (ProviderShim) Validate(ctx context.Context, c shim.ResourceConfig) ([]string, []error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ValidateResource(
	ctx context.Context, t string, c shim.ResourceConfig,
) ([]string, []error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ValidateDataSource(
	ctx context.Context, t string, c shim.ResourceConfig,
) ([]string, []error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Configure(
	ctx context.Context, c shim.ResourceConfig,
) error {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Diff(
	ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig, opts shim.DiffOptions,
) (shim.InstanceDiff, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Apply(
	ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff,
) (shim.InstanceState, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Refresh(
	ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig,
) (shim.InstanceState, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ReadDataDiff(
	ctx context.Context, t string, c shim.ResourceConfig,
) (shim.InstanceDiff, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ReadDataApply(
	ctx context.Context, t string, d shim.InstanceDiff,
) (shim.InstanceState, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Meta(ctx context.Context) interface{} {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Stop(ctx context.Context) error {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) InitLogging(ctx context.Context) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) NewDestroyDiff(ctx context.Context, t string, _ shim.TimeoutOptions) shim.InstanceDiff {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) NewResourceConfig(
	ctx context.Context, object map[string]interface{},
) shim.ResourceConfig {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) IsSet(ctx context.Context, v interface{}) ([]interface{}, bool) {
	panic("this provider is schema-only and does not support runtime operations")
}
