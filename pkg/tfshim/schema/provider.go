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
	return ProviderShim{p}
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

func (ProviderShim) Validate(c shim.ResourceConfig) ([]string, []error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ValidateResource(t string, c shim.ResourceConfig) ([]string, []error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ValidateDataSource(t string, c shim.ResourceConfig) ([]string, []error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Configure(ctx context.Context, c shim.ResourceConfig) error {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Diff(ctx context.Context, t string, s shim.InstanceState,
	c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Apply(ctx context.Context, t string, s shim.InstanceState,
	d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Refresh(ctx context.Context, t string, s shim.InstanceState) (shim.InstanceState, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ReadDataDiff(ctx context.Context, t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ReadDataApply(ctx context.Context, t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Meta() interface{} {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Stop() error {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) InitLogging() {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) NewDestroyDiff() shim.InstanceDiff {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) NewResourceConfig(object map[string]interface{}) shim.ResourceConfig {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) IsSet(v interface{}) ([]interface{}, bool) {
	panic("this provider is schema-only and does not support runtime operations")
}
