package sdkv2

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/logging"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	testing "github.com/mitchellh/go-testing-interface"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = shim.Provider(v2Provider{})

func configFromShim(c shim.ResourceConfig) *terraform.ResourceConfig {
	if c == nil {
		return nil
	}
	return c.(v2ResourceConfig).tf
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
	return v2InstanceDiff{tf: d}
}

type v2Provider struct {
	tf   *schema.Provider
	opts []providerOption

	server       *grpcServer
	planEditFunc PlanStateEditFunc
	internalinter.Internal
}

var (
	_ shim.Provider                    = v2Provider{}
	_ shim.ProviderWithRawStateSupport = v2Provider{}
)

func NewProvider(p *schema.Provider, opts ...providerOption) shim.Provider {
	o, err := getProviderOptions(opts)
	contract.AssertNoErrorf(err, "provider options failed to apply")

	return v2Provider{
		planEditFunc: o.planStateEdit,
		tf:           p,
		opts:         opts,
		server: &grpcServer{
			gserver: p.GRPCProvider().(*schema.GRPCProviderServer),
		},
	}
}

func (p v2Provider) Schema() shim.SchemaMap {
	return v2SchemaMap(p.tf.Schema)
}

func (p v2Provider) DataSourcesMap() shim.ResourceMap {
	return v2ResourceMap(p.tf.DataSourcesMap)
}

func (p v2Provider) InternalValidate() error {
	return p.tf.InternalValidate()
}

func (p v2Provider) Validate(_ context.Context, c shim.ResourceConfig) ([]string, []error) {
	return warningsAndErrors(p.tf.Validate(configFromShim(c)))
}

func (p v2Provider) ValidateResource(_ context.Context, t string, c shim.ResourceConfig) ([]string, []error) {
	return warningsAndErrors(p.tf.ValidateResource(t, configFromShim(c)))
}

func (p v2Provider) ValidateDataSource(_ context.Context, t string, c shim.ResourceConfig) ([]string, []error) {
	return warningsAndErrors(p.tf.ValidateDataSource(t, configFromShim(c)))
}

func (p v2Provider) Configure(ctx context.Context, c shim.ResourceConfig) error {
	// See ConfigureProvider e.g.
	// https://github.com/hashicorp/terraform-plugin-sdk/blob/main/helper/schema/grpc_provider.go#L564
	ctxHack := context.WithValue(ctx, schema.StopContextKey, p.stopContext(context.Background()))
	return errors(p.tf.Configure(ctxHack, configFromShim(c)))
}

func (p v2Provider) stopContext(ctx context.Context) context.Context {
	// TODO may want to follow StopContext implementation to make sure calling calling p.Stop()
	// cancels the context returned here.
	//
	// See: https://github.com/hashicorp/terraform-plugin-sdk/blob/main/helper/schema/grpc_provider.go#L60C1-L60C80
	return ctx
}

func (p v2Provider) ReadDataDiff(
	ctx context.Context,
	t string,
	c shim.ResourceConfig,
) (shim.InstanceDiff, error) {
	resource, ok := p.tf.DataSourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}

	config := configFromShim(c)
	rawConfig := makeResourceRawConfig(config, resource)

	diff, err := resource.Diff(ctx, nil, config, p.tf.Meta())
	if diff != nil {
		diff.RawConfig = rawConfig
	}
	return diffToShim(diff), err
}

func (p v2Provider) ReadDataApply(
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

func (p v2Provider) Meta(_ context.Context) interface{} {
	return p.tf.Meta()
}

func (p v2Provider) Stop(_ context.Context) error {
	return nil
}

func (p v2Provider) InitLogging(_ context.Context) {
	logging.SetOutput(&testing.RuntimeT{})
}

func (p v2Provider) NewResourceConfig(
	_ context.Context, object map[string]interface{},
) shim.ResourceConfig {
	return v2ResourceConfig{&terraform.ResourceConfig{
		Raw:    object,
		Config: object,
	}}
}

func (p v2Provider) NewProviderConfig(
	_ context.Context, object map[string]interface{},
) shim.ResourceConfig {
	tfConfig := &terraform.ResourceConfig{
		Raw:    object,
		Config: object,
	}
	typ := schema.InternalMap(p.tf.Schema).CoreConfigSchema().ImpliedType()
	ctyVal, err := recoverCtyValueOfObjectType(typ, object)
	if err != nil {
		glog.V(9).Infof("Failed to recover cty value of object type: %v, falling back to old behaviour", err)
		return v2ResourceConfig{tfConfig}
	}

	tfConfig.CtyValue = ctyVal
	return v2ResourceConfig{tfConfig}
}

func (p v2Provider) IsSet(_ context.Context, v interface{}) ([]interface{}, bool) {
	if set, ok := v.(*schema.Set); ok {
		return set.List(), true
	}
	return nil, false
}

func (p v2Provider) SupportsUnknownCollections() bool {
	return true
}
