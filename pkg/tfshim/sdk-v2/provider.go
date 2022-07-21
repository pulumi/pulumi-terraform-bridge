package sdkv2

import (
	"context"
	"fmt"
	"github.com/opentracing/opentracing-go"

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

type withSpan struct {
	wrapped shim.Provider
}

func (w withSpan) Schema() shim.SchemaMap {
	return w.wrapped.Schema()
}

func (w withSpan) ResourcesMap() shim.ResourceMap {
	return w.wrapped.ResourcesMap()
}

func (w withSpan) DataSourcesMap() shim.ResourceMap {
	return w.wrapped.DataSourcesMap()
}

func (w withSpan) Validate(c shim.ResourceConfig) ([]string, []error) {
	return w.wrapped.Validate(c)
}

func (w withSpan) ValidateResource(t string, c shim.ResourceConfig) ([]string, []error) {
	return w.wrapped.ValidateResource(t, c)
}

func (w withSpan) ValidateDataSource(t string, c shim.ResourceConfig) ([]string, []error) {
	return w.wrapped.ValidateDataSource(t, c)
}

func (w withSpan) Configure(ctx context.Context, c shim.ResourceConfig) error {
	span, spanCtx := opentracing.StartSpanFromContext(ctx, "/pulumirpc.ResourceProvider/TFBridgeConfigure")
	defer span.Finish()
	return w.wrapped.Configure(spanCtx, c)

}

func (w withSpan) Diff(ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	span, spanCtx := opentracing.StartSpanFromContext(ctx, "/pulumirpc.ResourceProvider/TFBridgeDiff", opentracing.Tag{
		Key:   "tfResource",
		Value: t,
	})
	defer span.Finish()
	return w.wrapped.Diff(spanCtx, t, s, c)
}

func (w withSpan) Apply(ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff) (shim.InstanceState, error) {
	span, spanCtx := opentracing.StartSpanFromContext(ctx, "/pulumirpc.ResourceProvider/TFBridgeApply", opentracing.Tag{
		Key:   "tfResource",
		Value: t,
	})
	defer span.Finish()
	return w.wrapped.Apply(spanCtx, t, s, d)
}

func (w withSpan) Refresh(ctx context.Context, t string, s shim.InstanceState) (shim.InstanceState, error) {
	span, spanCtx := opentracing.StartSpanFromContext(ctx, "/pulumirpc.ResourceProvider/TFBridgeRefresh", opentracing.Tag{
		Key:   "tfResource",
		Value: t,
	})
	defer span.Finish()
	return w.wrapped.Refresh(spanCtx, t, s)
}

func (w withSpan) ReadDataDiff(ctx context.Context, t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	span, spanCtx := opentracing.StartSpanFromContext(ctx, "/pulumirpc.ResourceProvider/TFBridgeReadDataDiff", opentracing.Tag{
		Key:   "tfResource",
		Value: t,
	})
	defer span.Finish()
	return w.wrapped.ReadDataDiff(spanCtx, t, c)
}

func (w withSpan) ReadDataApply(ctx context.Context, t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	span, spanCtx := opentracing.StartSpanFromContext(ctx, "/pulumirpc.ResourceProvider/TFBridgeReadDataApply", opentracing.Tag{
		Key:   "tfResource",
		Value: t,
	})
	defer span.Finish()
	return w.wrapped.ReadDataApply(spanCtx, t, d)
}

func (w withSpan) Meta() interface{} {
	return w.wrapped.Meta()
}

func (w withSpan) Stop() error {
	return w.wrapped.Stop()
}

func (w withSpan) InitLogging() {
	w.wrapped.InitLogging()
}

func (w withSpan) NewDestroyDiff() shim.InstanceDiff {
	return w.wrapped.NewDestroyDiff()
}

func (w withSpan) NewResourceConfig(object map[string]interface{}) shim.ResourceConfig {
	return w.wrapped.NewResourceConfig(object)
}

func (w withSpan) IsSet(v interface{}) ([]interface{}, bool) {
	return w.wrapped.IsSet(v)
}

func NewProvider(p *schema.Provider) shim.Provider {
	return withSpan{wrapped: v2Provider{p}}
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

func (p v2Provider) Configure(ctx context.Context, c shim.ResourceConfig) error {
	return errors(p.tf.Configure(ctx, configFromShim(c)))
}

func (p v2Provider) Diff(ctx context.Context, t string, s shim.InstanceState,
	c shim.ResourceConfig) (shim.InstanceDiff, error) {
	if c == nil {
		return diffToShim(&terraform.InstanceDiff{Destroy: true}), nil
	}
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	state, err := upgradeResourceState(p.tf, r, stateFromShim(s))
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}
	diff, err := r.SimpleDiff(ctx, state, configFromShim(c), p.tf.Meta())
	return diffToShim(diff), err
}

func (p v2Provider) Apply(ctx context.Context, t string, s shim.InstanceState,
	d shim.InstanceDiff) (shim.InstanceState,
	error) {
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	state, err := upgradeResourceState(p.tf, r, stateFromShim(s))
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}
	state, diags := r.Apply(ctx, state, diffFromShim(d), p.tf.Meta())
	return stateToShim(state), errors(diags)
}

func (p v2Provider) Refresh(ctx context.Context, t string, s shim.InstanceState) (shim.InstanceState, error) {
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	state, err := upgradeResourceState(p.tf, r, stateFromShim(s))
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}
	state, diags := r.RefreshWithoutUpgrade(ctx, state, p.tf.Meta())
	return stateToShim(state), errors(diags)
}

func (p v2Provider) ReadDataDiff(ctx context.Context, t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	r, ok := p.tf.DataSourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	diff, err := r.Diff(ctx, nil, configFromShim(c), p.tf.Meta())
	return diffToShim(diff), err
}

func (p v2Provider) ReadDataApply(ctx context.Context, t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	r, ok := p.tf.DataSourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	state, diags := r.ReadDataApply(ctx, diffFromShim(d), p.tf.Meta())
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
