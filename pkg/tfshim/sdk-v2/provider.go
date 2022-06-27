package sdkv2

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/hcl2shim"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/logging"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	testing "github.com/mitchellh/go-testing-interface"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.Provider(v2Provider{})

func configFromShim(c shim.ResourceConfig) *terraform.ResourceConfig {
	if c == nil {
		return nil
	}
	return c.(v2ResourceConfig).tf
}

func stateFromShim(s shim.InstanceState) *terraform.InstanceState {
	if s == nil {
		return nil
	}
	return s.(v2InstanceState).tf
}

func stateToShim(s *terraform.InstanceState) shim.InstanceState {
	if s == nil {
		return nil
	}
	return v2InstanceState{s, nil}
}

func diffFromShim(d shim.InstanceDiff) *terraform.InstanceDiff {
	if d == nil {
		return nil
	}
	return d.(v2InstanceDiff).tf
}

func diffToShim(d *terraform.InstanceDiff) shim.InstanceDiff {
	if d == nil {
		return nil
	}
	return v2InstanceDiff{d}
}

type v2Provider struct {
	tf *schema.Provider
}

func NewProvider(p *schema.Provider) shim.Provider {
	return v2Provider{p}
}

func (p v2Provider) Schema() shim.SchemaMap {
	return v2SchemaMap(p.tf.Schema)
}

func (p v2Provider) ResourcesMap() shim.ResourceMap {
	return v2ResourceMap(p.tf.ResourcesMap)
}

func (p v2Provider) DataSourcesMap() shim.ResourceMap {
	return v2ResourceMap(p.tf.DataSourcesMap)
}

func (p v2Provider) Validate(c shim.ResourceConfig) ([]string, []error) {
	return warningsAndErrors(p.tf.Validate(configFromShim(c)))
}

func (p v2Provider) ValidateResource(t string, c shim.ResourceConfig) ([]string, []error) {
	return warningsAndErrors(p.tf.ValidateResource(t, configFromShim(c)))
}

func (p v2Provider) ValidateDataSource(t string, c shim.ResourceConfig) ([]string, []error) {
	return warningsAndErrors(p.tf.ValidateDataSource(t, configFromShim(c)))
}

func (p v2Provider) Configure(c shim.ResourceConfig) error {
	return errors(p.tf.Configure(context.TODO(), configFromShim(c)))
}

func (p v2Provider) Diff(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	if c == nil {
		return diffToShim(&terraform.InstanceDiff{Destroy: true}), nil
	}
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	config, state := configFromShim(c), stateFromShim(s)
	if state != nil {
		state.RawConfig = hcl2shim.HCL2ValueFromConfigValue(config.Raw)
	}

	state, err := upgradeResourceState(p.tf, r, state)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}
	diff, err := r.SimpleDiff(context.TODO(), state, config, p.tf.Meta())
	return diffToShim(diff), err
}

func (p v2Provider) Apply(t string, s shim.InstanceState, d shim.InstanceDiff) (shim.InstanceState, error) {
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	state, err := upgradeResourceState(p.tf, r, stateFromShim(s))
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}
	state, diags := r.Apply(context.TODO(), state, diffFromShim(d), p.tf.Meta())
	return stateToShim(state), errors(diags)
}

func (p v2Provider) Refresh(t string, s shim.InstanceState) (shim.InstanceState, error) {
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	state, err := upgradeResourceState(p.tf, r, stateFromShim(s))
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}
	state, diags := r.RefreshWithoutUpgrade(context.TODO(), state, p.tf.Meta())
	return stateToShim(state), errors(diags)
}

func (p v2Provider) ReadDataDiff(t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	r, ok := p.tf.DataSourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	diff, err := r.Diff(context.TODO(), nil, configFromShim(c), p.tf.Meta())
	return diffToShim(diff), err
}

func (p v2Provider) ReadDataApply(t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	r, ok := p.tf.DataSourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	state, diags := r.ReadDataApply(context.TODO(), diffFromShim(d), p.tf.Meta())
	return stateToShim(state), errors(diags)
}

func (p v2Provider) Meta() interface{} {
	return p.tf.Meta()
}

func (p v2Provider) Stop() error {
	return nil
}

func (p v2Provider) InitLogging() {
	logging.SetOutput(&testing.RuntimeT{})
}

func (p v2Provider) NewDestroyDiff() shim.InstanceDiff {
	return v2InstanceDiff{&terraform.InstanceDiff{Destroy: true}}
}

func (p v2Provider) NewResourceConfig(object map[string]interface{}) shim.ResourceConfig {
	return v2ResourceConfig{&terraform.ResourceConfig{
		Raw:    object,
		Config: object,
	}}
}

func (p v2Provider) IsSet(v interface{}) ([]interface{}, bool) {
	if set, ok := v.(*schema.Set); ok {
		return set.List(), true
	}
	return nil, false
}
