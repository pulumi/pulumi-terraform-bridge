package util

import (
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type AliasingProvider struct {
	Provider      shim.Provider
	ResourceMap   AliasingResourceMap
	DataSourceMap AliasingResourceMap
}

var _ = (shim.Provider)((*AliasingProvider)(nil))

func (p *AliasingProvider) Schema() shim.SchemaMap {
	return p.Provider.Schema()
}

func (p *AliasingProvider) ResourcesMap() shim.ResourceMap {
	return p.ResourceMap
}

func (p *AliasingProvider) DataSourcesMap() shim.ResourceMap {
	return p.DataSourceMap
}

func (p *AliasingProvider) Validate(c shim.ResourceConfig) ([]string, []error) {
	return p.Provider.Validate(c)
}

func (p *AliasingProvider) ValidateResource(t string, c shim.ResourceConfig) ([]string, []error) {
	return p.Provider.ValidateResource(t, c)
}

func (p *AliasingProvider) ValidateDataSource(t string, c shim.ResourceConfig) ([]string, []error) {
	return p.Provider.ValidateDataSource(t, c)
}

func (p *AliasingProvider) Configure(c shim.ResourceConfig) error {
	return p.Provider.Configure(c)
}

func (p *AliasingProvider) Diff(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	return p.Provider.Diff(t, s, c)
}

func (p *AliasingProvider) Apply(t string, s shim.InstanceState, d shim.InstanceDiff) (shim.InstanceState, error) {
	return p.Provider.Apply(t, s, d)
}

func (p *AliasingProvider) Refresh(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceState, error) {
	return p.Provider.Refresh(t, s, c)
}

func (p *AliasingProvider) ReadDataDiff(t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	return p.Provider.ReadDataDiff(t, c)
}

func (p *AliasingProvider) ReadDataApply(t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	return p.Provider.ReadDataApply(t, d)
}

func (p *AliasingProvider) Meta() interface{} {
	return p.Provider.Meta()
}

func (p *AliasingProvider) Stop() error {
	return p.Provider.Stop()
}

func (p *AliasingProvider) InitLogging() {
	p.Provider.InitLogging()
}

func (p *AliasingProvider) NewDestroyDiff() shim.InstanceDiff {
	return p.Provider.NewDestroyDiff()
}

func (p *AliasingProvider) NewResourceConfig(object map[string]interface{}) shim.ResourceConfig {
	return p.Provider.NewResourceConfig(object)
}

func (p *AliasingProvider) IsSet(v interface{}) ([]interface{}, bool) {
	return p.Provider.IsSet(v)
}
