package schema

import (
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

func (ProviderShim) Validate(c shim.ResourceConfig) ([]string, []error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ValidateResource(t string, c shim.ResourceConfig) ([]string, []error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ValidateDataSource(t string, c shim.ResourceConfig) ([]string, []error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Configure(c shim.ResourceConfig) error {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Diff(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Apply(t string, s shim.InstanceState, d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) Refresh(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceState, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ReadDataDiff(t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("this provider is schema-only and does not support runtime operations")
}

func (ProviderShim) ReadDataApply(t string, d shim.InstanceDiff) (shim.InstanceState, error) {
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
