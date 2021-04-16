package sdkv1

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/logging"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.Provider(v1Provider{})

func instanceInfo(t string) *terraform.InstanceInfo {
	return &terraform.InstanceInfo{Type: t}
}

func configFromShim(c shim.ResourceConfig) *terraform.ResourceConfig {
	if c == nil {
		return nil
	}
	return c.(v1ResourceConfig).tf
}

func stateFromShim(s shim.InstanceState) *terraform.InstanceState {
	if s == nil {
		return nil
	}
	return s.(v1InstanceState).tf
}

func stateToShim(s *terraform.InstanceState) shim.InstanceState {
	if s == nil {
		return nil
	}
	return v1InstanceState{s, nil}
}

func diffFromShim(d shim.InstanceDiff) *terraform.InstanceDiff {
	if d == nil {
		return nil
	}
	return d.(v1InstanceDiff).tf
}

func diffToShim(d *terraform.InstanceDiff) shim.InstanceDiff {
	if d == nil {
		return nil
	}
	return v1InstanceDiff{d}
}

type v1Provider struct {
	tf *schema.Provider
}

func NewProvider(p *schema.Provider) shim.Provider {
	return v1Provider{p}
}

func (p v1Provider) Schema() shim.SchemaMap {
	return v1SchemaMap(p.tf.Schema)
}

func (p v1Provider) ResourcesMap() shim.ResourceMap {
	return v1ResourceMap(p.tf.ResourcesMap)
}

func (p v1Provider) DataSourcesMap() shim.ResourceMap {
	return v1ResourceMap(p.tf.DataSourcesMap)
}

func (p v1Provider) Validate(c shim.ResourceConfig) ([]string, []error) {
	return p.tf.Validate(configFromShim(c))
}

func (p v1Provider) ValidateResource(t string, c shim.ResourceConfig) ([]string, []error) {
	return p.tf.ValidateResource(t, configFromShim(c))
}

func (p v1Provider) ValidateDataSource(t string, c shim.ResourceConfig) ([]string, []error) {
	return p.tf.ValidateDataSource(t, configFromShim(c))
}

func (p v1Provider) Configure(c shim.ResourceConfig) error {
	return p.tf.Configure(configFromShim(c))
}

func (p v1Provider) Diff(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	if c == nil {
		return diffToShim(&terraform.InstanceDiff{Destroy: true}), nil
	}

	diff, err := p.tf.SimpleDiff(instanceInfo(t), stateFromShim(s), configFromShim(c))
	return diffToShim(diff), err
}

func (p v1Provider) Apply(t string, s shim.InstanceState, d shim.InstanceDiff) (shim.InstanceState, error) {
	state, err := p.tf.Apply(instanceInfo(t), stateFromShim(s), diffFromShim(d))
	return stateToShim(state), err
}

func (p v1Provider) Refresh(t string, s shim.InstanceState) (shim.InstanceState, error) {
	state, err := p.tf.Refresh(instanceInfo(t), stateFromShim(s))
	return stateToShim(state), err
}

func (p v1Provider) ReadDataDiff(t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	diff, err := p.tf.ReadDataDiff(instanceInfo(t), configFromShim(c))
	return diffToShim(diff), err
}

func (p v1Provider) ReadDataApply(t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	state, err := p.tf.ReadDataApply(instanceInfo(t), diffFromShim(d))
	return stateToShim(state), err
}

func (p v1Provider) Meta() interface{} {
	return p.tf.Meta()
}

func (p v1Provider) Stop() error {
	return p.tf.Stop()
}

func (p v1Provider) InitLogging() {
	logging.SetOutput()
}

func (p v1Provider) NewDestroyDiff() shim.InstanceDiff {
	return v1InstanceDiff{&terraform.InstanceDiff{Destroy: true}}
}

func (p v1Provider) NewResourceConfig(object map[string]interface{}) shim.ResourceConfig {
	return v1ResourceConfig{&terraform.ResourceConfig{
		Raw:    object,
		Config: object,
	}}
}

func (p v1Provider) IsSet(v interface{}) ([]interface{}, bool) {
	if set, ok := v.(*schema.Set); ok {
		return set.List(), true
	}
	return nil, false
}
