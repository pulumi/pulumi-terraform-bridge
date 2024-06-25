package sdkv2

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/logging"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	testing "github.com/mitchellh/go-testing-interface"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

var _ shim.Provider = (*v2Provider)(nil)

func NewProvider(p *schema.Provider, opts ...providerOption) shim.Provider {
	prov := v2Provider{
		tf:   p,
		opts: opts,
	}
	if cmdutil.IsTruthy(os.Getenv("PULUMI_ENABLE_PLAN_RESOURCE_CHANGE")) {
		return newProviderWithPlanResourceChange(p, prov, func(s string) bool { return true })
	}
	if opts, err := getProviderOptions(opts); err == nil && opts.planResourceChangeFilter != nil {
		return newProviderWithPlanResourceChange(p, prov, opts.planResourceChangeFilter)
	}
	return prov
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

func (p v2Provider) Apply(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	d shim.InstanceDiff,
) (shim.InstanceState, error) {
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}
	state, err := upgradeResourceState(ctx, t, p.tf, r, stateFromShim(s))
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}
	state, diags := r.Apply(ctx, state, diffFromShim(d), p.tf.Meta())
	return stateToShim(r, state), errors(diags)
}

func (p v2Provider) Refresh(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
) (shim.InstanceState, error) {
	r, ok := p.tf.ResourcesMap[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", t)
	}

	state, err := upgradeResourceState(ctx, t, p.tf, r, stateFromShim(s))
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade resource state: %w", err)
	}

	if c != nil {
		state.RawConfig = makeResourceRawConfig(configFromShim(c), r)
	}

	state, diags := r.RefreshWithoutUpgrade(context.TODO(), state, p.tf.Meta())
	return stateToShim(r, state), errors(diags)
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

func (p v2Provider) NewDestroyDiff(_ context.Context, t string, opts shim.TimeoutOptions) shim.InstanceDiff {
	d := v2InstanceDiff{&terraform.InstanceDiff{Destroy: true}}
	d.applyTimeoutOptions(opts)
	return d
}

func (p v2Provider) NewResourceConfig(
	_ context.Context, object map[string]interface{},
) shim.ResourceConfig {
	return v2ResourceConfig{&terraform.ResourceConfig{
		Raw:    object,
		Config: object,
	}}
}

func (p v2Provider) IsSet(_ context.Context, v interface{}) ([]interface{}, bool) {
	if set, ok := v.(*schema.Set); ok {
		return set.List(), true
	}
	return nil, false
}

type tfSchemaMarshaller struct {
	Type                    schema.ValueType
	ConfigMode              schema.SchemaConfigMode
	Required                bool
	Optional                bool
	Computed                bool
	ForceNew                bool
	DiffSuppressFuncDefined bool
	DiffSuppressOnRefresh   bool
	Default                 interface{}
	DefaultFuncDefined      bool
	Description             string
	InputDefault            string
	StateFuncDefined        bool
	Elem                    interface{}
	MaxItems                int
	MinItems                int
	SetDefined              bool
	ComputedWhen            []string
	ConflictsWith           []string
	ExactlyOneOf            []string
	AtLeastOneOf            []string
	RequiredWith            []string
	Deprecated              string
	ValidateFuncDefined     bool
	ValidateDiagFuncDefined bool
	Sensitive               bool
}

type tfResourceMarshaller struct {
	Schema                            map[string]*tfSchemaMarshaller
	SchemaVersion                     int
	MigrateStateDefined               bool
	StateUpgradersLen                 int
	CustomizeDiffDefined              bool
	DeprecationMessage                string
	Description                       string
	UseJSONNumber                     bool
	EnableLegacyTypeSystemApplyErrors bool
	EnableLegacyTypeSystemPlanErrors  bool
}

type tfProvMarshaller struct {
	Schema               map[string]*tfSchemaMarshaller
	ResourcesMap         map[string]*tfResourceMarshaller
	DataSourcesMap       map[string]*tfResourceMarshaller
	ConfigureFuncDefined bool
	TerraformVersion     string
}

func convertTFSchemaOrResource(s interface{}) interface{} {
	if s == nil {
		return nil
	}
	switch s := s.(type) {
	case *schema.Schema:
		return convertTFSchema(s)
	case *schema.Resource:
		return convertTfResource(s)
	}
	panic(fmt.Sprintf("unexpected type %T", s))
}

func convertTFSchema(s *schema.Schema) *tfSchemaMarshaller {
	return &tfSchemaMarshaller{
		Type:                    s.Type,
		ConfigMode:              s.ConfigMode,
		Required:                s.Required,
		Optional:                s.Optional,
		Computed:                s.Computed,
		ForceNew:                s.ForceNew,
		DiffSuppressFuncDefined: s.DiffSuppressFunc != nil,
		DiffSuppressOnRefresh:   s.DiffSuppressOnRefresh,
		Default:                 s.Default,
		DefaultFuncDefined:      s.DefaultFunc != nil,
		Description:             s.Description,
		InputDefault:            s.InputDefault,
		StateFuncDefined:        s.StateFunc != nil,
		Elem:                    convertTFSchemaOrResource(s.Elem),
		MaxItems:                s.MaxItems,
		MinItems:                s.MinItems,
		SetDefined:              s.Set != nil,
		ComputedWhen:            s.ComputedWhen,
		ConflictsWith:           s.ConflictsWith,
		ExactlyOneOf:            s.ExactlyOneOf,
		AtLeastOneOf:            s.AtLeastOneOf,
		RequiredWith:            s.RequiredWith,
		Deprecated:              s.Deprecated,
		ValidateFuncDefined:     s.ValidateFunc != nil,
		ValidateDiagFuncDefined: s.ValidateDiagFunc != nil,
		Sensitive:               s.Sensitive,
	}
}

func convertTfResource(r *schema.Resource) *tfResourceMarshaller {
	tfResource := &tfResourceMarshaller{
		Schema:                            make(map[string]*tfSchemaMarshaller),
		SchemaVersion:                     r.SchemaVersion,
		MigrateStateDefined:               r.MigrateState != nil,
		StateUpgradersLen:                 len(r.StateUpgraders),
		CustomizeDiffDefined:              r.CustomizeDiff != nil,
		DeprecationMessage:                r.DeprecationMessage,
		Description:                       r.Description,
		UseJSONNumber:                     r.UseJSONNumber,
		EnableLegacyTypeSystemApplyErrors: r.EnableLegacyTypeSystemApplyErrors,
		EnableLegacyTypeSystemPlanErrors:  r.EnableLegacyTypeSystemPlanErrors,
	}

	for k, v := range r.Schema {
		tfResource.Schema[k] = convertTFSchema(v)
	}

	return tfResource
}

func convertTfProv(p *schema.Provider) *tfProvMarshaller {
	tfProv := &tfProvMarshaller{
		Schema:               make(map[string]*tfSchemaMarshaller),
		ResourcesMap:         make(map[string]*tfResourceMarshaller),
		DataSourcesMap:       make(map[string]*tfResourceMarshaller),
		ConfigureFuncDefined: p.ConfigureFunc != nil,
		TerraformVersion:     p.TerraformVersion,
	}

	for k, v := range p.Schema {
		tfProv.Schema[k] = convertTFSchema(v)
	}

	for k, v := range p.ResourcesMap {
		tfProv.ResourcesMap[k] = convertTfResource(v)
	}

	for k, v := range p.DataSourcesMap {
		tfProv.DataSourcesMap[k] = convertTfResource(v)
	}

	return tfProv
}

func (p v2Provider) DetailedSchemaDump() []byte {
	prov := convertTfProv(p.tf)
	sch, err := json.Marshal(prov)
	contract.AssertNoErrorf(err, "failed to marshal schema")
	return sch
}
