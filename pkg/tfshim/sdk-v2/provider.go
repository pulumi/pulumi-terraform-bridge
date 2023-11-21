package sdkv2

import (
	"context"
	"fmt"

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

func stateToShim(r *schema.Resource, s *terraform.InstanceState) shim.InstanceState {
	if s == nil {
		return nil
	}
	return NewInstanceStateForResource(s, r)
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
	tf   *schema.Provider
	opts []providerOption
}

var _ shim.ProviderWithContext = (*v2Provider)(nil)

func NewProvider(p *schema.Provider, opts ...providerOption) shim.Provider {
	return v2Provider{
		tf:   p,
		opts: opts,
	}
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
	return errors(p.tf.Configure(context.Background(), configFromShim(c)))
}

func (p v2Provider) ConfigureWithContext(ctx context.Context, c shim.ResourceConfig) error {
	return errors(p.tf.Configure(ctx, configFromShim(c)))
}

func (p v2Provider) Apply(
	t string,
	s shim.InstanceState,
	d shim.InstanceDiff,
) (shim.InstanceState, error) {
	return p.ApplyWithContext(context.Background(), t, s, d)
}

func (p v2Provider) ApplyWithContext(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	d shim.InstanceDiff,
) (shim.InstanceState, error) {
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	state, err := upgradeResourceState(p.tf, r, stateFromShim(s))
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}
	state, diags := r.Apply(ctx, state, diffFromShim(d), p.tf.Meta())
	return stateToShim(r, state), errors(diags)
}

func (p v2Provider) Refresh(
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
) (shim.InstanceState, error) {
	return p.RefreshWithContext(context.Background(), t, s, c)
}

func (p v2Provider) RefreshWithContext(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
) (shim.InstanceState, error) {
	opts, err := getProviderOptions(p.opts)
	if err != nil {
		return nil, err
	}

	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}

	state, err := upgradeResourceState(p.tf, r, stateFromShim(s))
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}

	if c != nil {
		state.RawConfig = makeResourceRawConfig(opts.diffStrategy, configFromShim(c), r)
	}

	state, diags := r.RefreshWithoutUpgrade(context.TODO(), state, p.tf.Meta())
	return stateToShim(r, state), errors(diags)
}

func (p v2Provider) ReadDataDiff(
	t string,
	c shim.ResourceConfig,
) (shim.InstanceDiff, error) {
	return p.ReadDataDiffWithContext(context.Background(), t, c)
}

func (p v2Provider) ReadDataDiffWithContext(
	ctx context.Context,
	t string,
	c shim.ResourceConfig,
) (shim.InstanceDiff, error) {
	r, ok := p.tf.DataSourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	diff, err := r.Diff(ctx, nil, configFromShim(c), p.tf.Meta())
	return diffToShim(diff), err
}

func (p v2Provider) ReadDataApply(
	t string,
	d shim.InstanceDiff,
) (shim.InstanceState, error) {
	return p.ReadDataApplyWithContext(context.Background(), t, d)
}

func (p v2Provider) ReadDataApplyWithContext(
	ctx context.Context,
	t string,
	d shim.InstanceDiff,
) (shim.InstanceState, error) {
	r, ok := p.tf.DataSourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	state, diags := r.ReadDataApply(ctx, diffFromShim(d), p.tf.Meta())
	return stateToShim(r, state), errors(diags)
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
