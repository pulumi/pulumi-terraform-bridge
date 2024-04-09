package sdkv2

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type v2Resource2 struct {
	v2Resource
	importer     shim.ImportFunc
	resourceType string
}

var _ shim.Resource = (*v2Resource2)(nil)

// This method is called to service `pulumi import` requests and maps naturally to the TF
// ImportResourceState method. When using `pulumi refresh` this is not called, and instead
// provider.Refresh() is called (see below).
func (r *v2Resource2) Importer() shim.ImportFunc {
	return r.importer
}

func (r *v2Resource2) InstanceState(
	id string, object, meta map[string]interface{},
) (shim.InstanceState, error) {
	if _, gotID := object["id"]; !gotID && id != "" {
		copy := map[string]interface{}{}
		for k, v := range object {
			copy[k] = v
		}
		copy["id"] = id
		object = copy
	}
	s, err := recoverAndCoerceCtyValueWithSchema(r.v2Resource.tf.CoreConfigSchema(), object)
	if err != nil {
		return nil, fmt.Errorf("InstanceState: %v", err)
	}
	return &v2InstanceState2{
		stateValue:   s,
		resourceType: r.resourceType,
		meta:         meta,
	}, nil
}

func (r *v2Resource2) Timeouts() *shim.ResourceTimeout {
	return v2Resource{r.tf}.Timeouts()
}

func (r *v2Resource2) DecodeTimeouts(config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	return v2Resource{r.tf}.DecodeTimeouts(config)
}

type v2InstanceState2 struct {
	resourceType string
	stateValue   cty.Value
	// Also known as private state.
	meta map[string]interface{}
}

var _ shim.InstanceState = (*v2InstanceState2)(nil)

func (s *v2InstanceState2) Type() string {
	return s.resourceType
}

func (s *v2InstanceState2) ID() string {
	id := s.stateValue.GetAttr("id")
	if !id.IsKnown() {
		return ""
	}
	contract.Assertf(id.Type() == cty.String, "expected id to be of type String")
	return id.AsString()
}

func (s *v2InstanceState2) Object(sch shim.SchemaMap) (map[string]interface{}, error) {
	res := objectFromCtyValue(s.stateValue)
	// grpc servers add a "timeouts" key to compensate for infinite diffs; this is not needed in
	// the Pulumi projection.
	delete(res, schema.TimeoutsConfigKey)
	return res, nil
}

func (s *v2InstanceState2) Meta() map[string]interface{} {
	return s.meta
}

type v2InstanceDiff2 struct {
	v2InstanceDiff

	config       cty.Value
	plannedState cty.Value
}

var _ shim.InstanceDiff = (*v2InstanceDiff2)(nil)

func (d *v2InstanceDiff2) ProposedState(
	res shim.Resource, priorState shim.InstanceState,
) (shim.InstanceState, error) {
	return &v2InstanceState2{
		stateValue: d.plannedState,
		meta:       d.v2InstanceDiff.tf.Meta,
	}, nil
}

// Provides PlanResourceChange handling for select resources.
type planResourceChangeImpl struct {
	tf     *schema.Provider
	server *grpcServer
}

var _ planResourceChangeProvider = (*planResourceChangeImpl)(nil)

func (p *planResourceChangeImpl) Diff(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
	opts shim.DiffOptions,
) (shim.InstanceDiff, error) {
	config := configFromShim(c)
	s, err := p.upgradeState(ctx, t, s)
	if err != nil {
		return nil, err
	}

	state := p.unpackInstanceState(t, s)
	res := p.tf.ResourcesMap[t]
	ty := res.CoreConfigSchema().ImpliedType()

	meta, err := p.providerMeta()
	if err != nil {
		return nil, err
	}
	cfg, err := recoverAndCoerceCtyValueWithSchema(res.CoreConfigSchema(), config.Config)
	if err != nil {
		return nil, fmt.Errorf("Resource %q: %w", t, err)
	}
	prop, err := proposedNew(res, state.stateValue, cfg)
	if err != nil {
		return nil, err
	}
	st := state.stateValue
	ic := opts.IgnoreChanges
	priv := state.meta
	to := opts.TimeoutOptions
	plan, err := p.server.PlanResourceChange(ctx, t, ty, cfg, st, prop, priv, meta, ic, to)
	if err != nil {
		return nil, err
	}
	return &v2InstanceDiff2{
		v2InstanceDiff: v2InstanceDiff{
			tf: plan.PlannedDiff,
		},
		config:       cfg,
		plannedState: plan.PlannedState,
	}, nil
}

func (p *planResourceChangeImpl) Apply(
	ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff,
) (shim.InstanceState, error) {
	res := p.tf.ResourcesMap[t]
	ty := res.CoreConfigSchema().ImpliedType()
	s, err := p.upgradeState(ctx, t, s)
	if err != nil {
		return nil, err
	}
	state := p.unpackInstanceState(t, s)
	meta, err := p.providerMeta()
	if err != nil {
		return nil, err
	}
	diff := p.unpackDiff(ty, d)
	cfg, st, pl := diff.config, state.stateValue, diff.plannedState
	priv := diff.v2InstanceDiff.tf.Meta
	resp, err := p.server.ApplyResourceChange(ctx, t, ty, cfg, st, pl, priv, meta)
	if err != nil {
		return nil, err
	}
	return &v2InstanceState2{
		resourceType: t,
		stateValue:   resp.stateValue,
		meta:         resp.meta,
	}, nil
}

// This method is called to service `pulumi refresh` requests and maps naturally to the TF
// ReadResource method. When using `pulumi import` this is not called, and instead
// resource.Importer() is called which maps to the TF ImportResourceState method..
func (p *planResourceChangeImpl) Refresh(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
) (shim.InstanceState, error) {
	res := p.tf.ResourcesMap[t]
	ty := res.CoreConfigSchema().ImpliedType()
	s, err := p.upgradeState(ctx, t, s)
	if err != nil {
		return nil, err
	}
	state := p.unpackInstanceState(t, s)
	meta, err := p.providerMeta()
	if err != nil {
		return nil, err
	}
	rr, err := p.server.ReadResource(ctx, t, ty, state.stateValue, state.meta, meta)
	if err != nil {
		return nil, err
	}
	msg := "Expected %q == %q matching resource types"
	contract.Assertf(rr.resourceType == t, msg, rr.resourceType, t)
	// When the resource is not found, the bridge expects a literal nil instead of a non-nil
	// state where the nil is encoded internally.
	if rr.stateValue.IsNull() {
		return nil, nil
	}
	return &v2InstanceState2{
		resourceType: rr.resourceType,
		stateValue:   rr.stateValue,
		meta:         rr.meta,
	}, nil
}

func (p *planResourceChangeImpl) NewDestroyDiff(
	ctx context.Context, t string, opts shim.TimeoutOptions,
) shim.InstanceDiff {
	res := p.tf.ResourcesMap[t]
	ty := res.CoreConfigSchema().ImpliedType()
	dd := (&v2Provider{}).NewDestroyDiff(ctx, t, opts).(v2InstanceDiff)
	return &v2InstanceDiff2{
		v2InstanceDiff: dd,
		config:         cty.NullVal(ty),
		plannedState:   cty.NullVal(ty),
	}
}

func (p *planResourceChangeImpl) Importer(t string) shim.ImportFunc {
	res := p.tf.ResourcesMap[t]
	ty := res.CoreConfigSchema().ImpliedType()
	return shim.ImportFunc(func(tt, id string, _ interface{}) ([]shim.InstanceState, error) {
		// Note: why are we dropping meta (3rd parameter)? Apparently this refers to
		// provider-level meta and calling ImportResourceState can already locate it in the
		// provider object, so it is redundant.
		ctx := context.TODO() // We should probably preserve Context here from the caller.
		contract.Assertf(tt == t, "Expected Import to be called with %q, got %q", t, tt)
		states, err := p.server.ImportResourceState(ctx, t, ty, id)
		if err != nil {
			return nil, nil
		}
		// Auto cast does not work, have to loop to promote to pointers.
		out := []shim.InstanceState{}
		for i := range states {
			// If the resource is not found, the outer bridge expects the state to
			// literally be nil, instead of encoding the nil as a value inside a non-nil
			// state.
			if states[i].stateValue.IsNull() {
				out = append(out, nil)
			} else {
				out = append(out, &states[i])
			}
		}
		return out, nil
	})
}

func (p *planResourceChangeImpl) providerMeta() (*cty.Value, error) {
	return nil, nil
	// TODO[pulumi/pulumi-terraform-bridge#1827]: We do not believe that this is load bearing in any providers.
}

func (*planResourceChangeImpl) unpackDiff(ty cty.Type, d shim.InstanceDiff) *v2InstanceDiff2 {
	switch d := d.(type) {
	case nil:
		contract.Failf("Unexpected nil InstanceDiff")
		return nil
	case *v2InstanceDiff2:
		return d
	default:
		contract.Failf("Unexpected concrete type for shim.InstanceDiff")
		return nil
	}
}

func (p *planResourceChangeImpl) unpackInstanceState(
	t string, s shim.InstanceState,
) *v2InstanceState2 {
	switch s := s.(type) {
	case nil:
		res := p.tf.ResourcesMap[t]
		ty := res.CoreConfigSchema().ImpliedType()
		return &v2InstanceState2{
			resourceType: t,
			stateValue:   cty.NullVal(ty),
		}
	case *v2InstanceState2:
		return s
	}
	contract.Failf("Unexpected type for shim.InstanceState: #%T", s)
	return nil
}

// Wrapping the pre-existing upgradeInstanceState method here. Since the method is written against
// terraform.InstanceState interface some adapters are needed to convert to/from cty.Value and meta
// private bag.
func (p *planResourceChangeImpl) upgradeState(
	ctx context.Context,
	t string, s shim.InstanceState,
) (shim.InstanceState, error) {
	res := p.tf.ResourcesMap[t]
	state := p.unpackInstanceState(t, s)
	instanceState, err := res.ShimInstanceStateFromValue(state.stateValue)
	if err != nil {
		return nil, err
	}
	// Looks like this definitely can happen, but upgradeResourceState assumes a non-nil map.
	if instanceState.Attributes == nil {
		instanceState.Attributes = map[string]string{}
	}
	instanceState.Meta = state.meta
	newInstanceState, err := upgradeResourceState(ctx, p.tf, res, instanceState)
	if err != nil {
		return nil, err
	}
	if newInstanceState == nil {
		return nil, nil
	}
	ty := res.CoreConfigSchema().ImpliedType()
	stateValue, err := newInstanceState.AttrsAsObjectValue(ty)
	if err != nil {
		return nil, err
	}
	return &v2InstanceState2{
		resourceType: t,
		stateValue:   stateValue,
		meta:         newInstanceState.Meta,
	}, nil
}

// Helper to unwrap gRPC types from GRPCProviderServer.
type grpcServer struct {
	gserver *schema.GRPCProviderServer
}

// This will return an error if any of the diagnostics are error-level, or a given err is non-nil.
// It will also logs the diagnostics into TF loggers, so they appear when debugging with the bridged
// provider with TF_LOG=TRACE or similar.
func (s *grpcServer) handle(ctx context.Context, diags []*tfprotov5.Diagnostic, err error) error {
	var dd diag.Diagnostics
	for _, d := range diags {
		if d == nil {
			continue
		}
		rd := recoverDiagnostic(*d)
		dd = append(dd, rd)
		logDiag(ctx, rd)
	}
	derr := errors(dd)
	if derr != nil && err != nil {
		return multierror.Append(derr, err)
	}
	if derr != nil {
		return derr
	}
	return err
}

func (s *grpcServer) PlanResourceChange(
	ctx context.Context,
	typeName string,
	ty cty.Type,
	config, priorState, proposedNewState cty.Value,
	priorMeta map[string]interface{},
	providerMeta *cty.Value,
	ignores shim.IgnoreChanges,
	timeoutOpts shim.TimeoutOptions,
) (*struct {
	PlannedState cty.Value
	PlannedMeta  map[string]interface{}
	PlannedDiff  *terraform.InstanceDiff
}, error) {
	configVal, err := msgpack.Marshal(config, ty)
	if err != nil {
		return nil, err
	}
	priorStateVal, err := msgpack.Marshal(priorState, ty)
	if err != nil {
		return nil, err
	}
	proposedNewStateVal, err := msgpack.Marshal(proposedNewState, ty)
	if err != nil {
		return nil, err
	}
	req := &schema.PlanResourceChangeExtraRequest{
		PlanResourceChangeRequest: tfprotov5.PlanResourceChangeRequest{
			TypeName:         typeName,
			PriorState:       &tfprotov5.DynamicValue{MsgPack: priorStateVal},
			ProposedNewState: &tfprotov5.DynamicValue{MsgPack: proposedNewStateVal},
			Config:           &tfprotov5.DynamicValue{MsgPack: configVal},
		},
		TransformInstanceDiff: func(d *terraform.InstanceDiff) *terraform.InstanceDiff {
			dd := &v2InstanceDiff{d}
			if ignores != nil {
				dd.processIgnoreChanges(ignores)
			}
			dd.applyTimeoutOptions(timeoutOpts)
			return dd.tf
		},
	}
	if len(priorMeta) > 0 {
		priorPrivate, err := json.Marshal(priorMeta)
		if err != nil {
			return nil, err
		}
		req.PriorPrivate = priorPrivate
	}
	if providerMeta != nil {
		providerMetaVal, err := msgpack.Marshal(*providerMeta, ty)
		if err != nil {
			return nil, err
		}
		req.ProviderMeta = &tfprotov5.DynamicValue{MsgPack: providerMetaVal}
	}
	resp, err := s.gserver.PlanResourceChangeExtra(ctx, req)
	if err := s.handle(ctx, resp.Diagnostics, err); err != nil {
		return nil, err
	}
	// Ignore resp.UnsafeToUseLegacyTypeSystem - does not matter for Pulumi bridged providers.
	// Ignore resp.RequiresReplace - expect replacement to be encoded in resp.InstanceDiff.
	plannedState, err := msgpack.Unmarshal(resp.PlannedState.MsgPack, ty)
	if err != nil {
		return nil, err
	}
	var meta map[string]interface{}
	if resp.PlannedPrivate != nil {
		if err := json.Unmarshal(resp.PlannedPrivate, &meta); err != nil {
			return nil, err
		}
	}
	return &struct {
		PlannedState cty.Value
		PlannedMeta  map[string]interface{}
		PlannedDiff  *terraform.InstanceDiff
	}{
		PlannedState: plannedState,
		PlannedMeta:  meta,
		PlannedDiff:  resp.InstanceDiff,
	}, nil
}

func (s *grpcServer) ApplyResourceChange(
	ctx context.Context,
	typeName string,
	ty cty.Type,
	config, priorState, plannedState cty.Value,
	plannedMeta map[string]interface{},
	providerMeta *cty.Value,
) (*v2InstanceState2, error) {
	configVal, err := msgpack.Marshal(config, ty)
	if err != nil {
		return nil, err
	}
	priorStateVal, err := msgpack.Marshal(priorState, ty)
	if err != nil {
		return nil, err
	}
	plannedStateVal, err := msgpack.Marshal(plannedState, ty)
	if err != nil {
		return nil, err
	}
	req := &tfprotov5.ApplyResourceChangeRequest{
		TypeName:     typeName,
		Config:       &tfprotov5.DynamicValue{MsgPack: configVal},
		PriorState:   &tfprotov5.DynamicValue{MsgPack: priorStateVal},
		PlannedState: &tfprotov5.DynamicValue{MsgPack: plannedStateVal},
	}
	if len(plannedMeta) > 0 {
		plannedPrivate, err := json.Marshal(plannedMeta)
		if err != nil {
			return nil, err
		}
		req.PlannedPrivate = plannedPrivate
	}
	if providerMeta != nil {
		providerMetaVal, err := msgpack.Marshal(*providerMeta, ty)
		if err != nil {
			return nil, err
		}
		req.ProviderMeta = &tfprotov5.DynamicValue{MsgPack: providerMetaVal}
	}
	resp, err := s.gserver.ApplyResourceChange(ctx, req)
	if err := s.handle(ctx, resp.Diagnostics, err); err != nil {
		return nil, err
	}
	newState, err := msgpack.Unmarshal(resp.NewState.MsgPack, ty)
	if err != nil {
		return nil, err
	}
	var meta map[string]interface{}
	if resp.Private != nil {
		if err := json.Unmarshal(resp.Private, &meta); err != nil {
			return nil, err
		}
	}
	return &v2InstanceState2{
		resourceType: typeName,
		stateValue:   newState,
		meta:         meta,
	}, nil
}

func (s *grpcServer) ReadResource(
	ctx context.Context,
	typeName string,
	ty cty.Type,
	currentState cty.Value,
	meta map[string]interface{},
	providerMeta *cty.Value,
) (*v2InstanceState2, error) {
	currentStateVal, err := msgpack.Marshal(currentState, ty)
	if err != nil {
		return nil, err
	}
	req := &tfprotov5.ReadResourceRequest{
		TypeName:     typeName,
		CurrentState: &tfprotov5.DynamicValue{MsgPack: currentStateVal},
	}
	if len(meta) > 0 {
		private, err := json.Marshal(meta)
		if err != nil {
			return nil, err
		}
		req.Private = private
	}
	if providerMeta != nil {
		providerMetaVal, err := msgpack.Marshal(*providerMeta, ty)
		if err != nil {
			return nil, err
		}
		req.ProviderMeta = &tfprotov5.DynamicValue{MsgPack: providerMetaVal}
	}
	resp, err := s.gserver.ReadResource(ctx, req)
	if err := s.handle(ctx, resp.Diagnostics, err); err != nil {
		return nil, err
	}
	newState, err := msgpack.Unmarshal(resp.NewState.MsgPack, ty)
	if err != nil {
		return nil, err
	}
	var meta2 map[string]interface{}
	if resp.Private != nil {
		if err := json.Unmarshal(resp.Private, &meta2); err != nil {
			return nil, err
		}
	}
	return &v2InstanceState2{
		resourceType: typeName,
		stateValue:   newState,
		meta:         meta2,
	}, nil
}

func (s *grpcServer) ImportResourceState(
	ctx context.Context,
	typeName string,
	ty cty.Type,
	id string,
) ([]v2InstanceState2, error) {
	req := &tfprotov5.ImportResourceStateRequest{
		TypeName: typeName,
		ID:       id,
	}
	resp, err := s.gserver.ImportResourceState(ctx, req)
	if err := s.handle(ctx, resp.Diagnostics, err); err != nil {
		return nil, err
	}
	out := []v2InstanceState2{}
	for _, x := range resp.ImportedResources {
		ok := x.TypeName == typeName
		contract.Assertf(ok, "Expect typeName %q=%q", x.TypeName, typeName)
		newState, err := msgpack.Unmarshal(x.State.MsgPack, ty)
		if err != nil {
			return nil, err
		}
		var meta map[string]interface{}
		if x.Private != nil {
			if err := json.Unmarshal(x.Private, &meta); err != nil {
				return nil, err
			}
		}
		s := v2InstanceState2{
			resourceType: x.TypeName,
			stateValue:   newState,
			meta:         meta,
		}
		out = append(out, s)
	}
	return out, nil
}

// Subset of shim.provider used by providerWithPlanResourceChangeDispatch.
type planResourceChangeProvider interface {
	Diff(
		ctx context.Context,
		t string,
		s shim.InstanceState,
		c shim.ResourceConfig,
		opts shim.DiffOptions,
	) (shim.InstanceDiff, error)

	Apply(
		ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff,
	) (shim.InstanceState, error)

	Refresh(
		ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig,
	) (shim.InstanceState, error)

	NewDestroyDiff(ctx context.Context, t string, opts shim.TimeoutOptions) shim.InstanceDiff

	// Moving this method to the provider object from the shim.Resource object for convenience.
	Importer(t string) shim.ImportFunc
}

// Wraps a provider to redirect select resources to use a PlanResourceChange strategy.
type providerWithPlanResourceChangeDispatch struct {
	// Fallback provider to dispatch calls to.
	shim.Provider

	// All resources to serve schema.
	resources map[string]*schema.Resource

	// Provider handling resources that use PlanResourceChange.
	planResourceChangeProvider planResourceChangeProvider

	// Predicate that returns true for TF resource names that should use PlanResourceChange.
	usePlanResourceChange func(resourceName string) bool
}

// This provider needs to override ResourceMap because it uses a different concrete implementation
// of shim.Resource and for select resources and needs to make sure the correct one is returned.
func (p *providerWithPlanResourceChangeDispatch) ResourcesMap() shim.ResourceMap {
	return &v2ResourceCustomMap{
		resources: p.resources,
		pack: func(token string, res *schema.Resource) shim.Resource {
			if p.usePlanResourceChange(token) {
				i := p.planResourceChangeProvider.Importer(token)
				return &v2Resource2{v2Resource{res}, i, token}
			}
			return v2Resource{res}
		},
	}
}

// Override Diff method to dispatch appropriately.
func (p *providerWithPlanResourceChangeDispatch) Diff(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
	opts shim.DiffOptions,
) (shim.InstanceDiff, error) {
	if p.usePlanResourceChange(t) {
		return p.planResourceChangeProvider.Diff(ctx, t, s, c, opts)
	}
	return p.Provider.Diff(ctx, t, s, c, opts)
}

// Override Apply method to dispatch appropriately.
func (p *providerWithPlanResourceChangeDispatch) Apply(
	ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff,
) (shim.InstanceState, error) {
	if p.usePlanResourceChange(t) {
		return p.planResourceChangeProvider.Apply(ctx, t, s, d)
	}
	return p.Provider.Apply(ctx, t, s, d)
}

// Override Refresh method to dispatch appropriately.
func (p *providerWithPlanResourceChangeDispatch) Refresh(
	ctx context.Context, t string, s shim.InstanceState, c shim.ResourceConfig,
) (shim.InstanceState, error) {
	if p.usePlanResourceChange(t) {
		return p.planResourceChangeProvider.Refresh(ctx, t, s, c)
	}
	return p.Provider.Refresh(ctx, t, s, c)
}

// Override NewDestroyDiff to dispatch appropriately.
func (p *providerWithPlanResourceChangeDispatch) NewDestroyDiff(
	ctx context.Context, t string, opts shim.TimeoutOptions,
) shim.InstanceDiff {
	if p.usePlanResourceChange(t) {
		return p.planResourceChangeProvider.NewDestroyDiff(ctx, t, opts)
	}
	return p.Provider.NewDestroyDiff(ctx, t, opts)
}

type v2ResourceCustomMap struct {
	resources map[string]*schema.Resource
	pack      func(string, *schema.Resource) shim.Resource
}

func (m *v2ResourceCustomMap) Len() int {
	return len(m.resources)
}

func (m *v2ResourceCustomMap) Get(key string) shim.Resource {
	r, ok := m.resources[key]
	if ok {
		return m.pack(key, r)
	}
	return nil
}

func (m *v2ResourceCustomMap) GetOk(key string) (shim.Resource, bool) {
	r, ok := m.resources[key]
	if ok {
		return m.pack(key, r), true
	}
	return nil, false
}

func (m *v2ResourceCustomMap) Range(each func(key string, value shim.Resource) bool) {
	for key, value := range m.resources {
		if !each(key, m.pack(key, value)) {
			return
		}
	}
}

func (m *v2ResourceCustomMap) Set(key string, value shim.Resource) {
	switch r := value.(type) {
	case v2Resource:
		m.resources[key] = r.tf
	case *v2Resource2:
		m.resources[key] = r.tf
	}
}

func newProviderWithPlanResourceChange(
	p *schema.Provider,
	prov shim.Provider,
	filter func(string) bool,
) shim.Provider {
	return &providerWithPlanResourceChangeDispatch{
		Provider:  prov,
		resources: p.ResourcesMap,
		planResourceChangeProvider: &planResourceChangeImpl{
			tf: p,
			server: &grpcServer{
				gserver: p.GRPCProvider().(*schema.GRPCProviderServer),
			},
		},
		usePlanResourceChange: filter,
	}
}

func recoverDiagnostic(d tfprotov5.Diagnostic) diag.Diagnostic {
	dd := diag.Diagnostic{
		Summary: d.Summary,
		Detail:  d.Detail,
	}
	switch d.Severity {
	case tfprotov5.DiagnosticSeverityError:
		dd.Severity = diag.Error
	case tfprotov5.DiagnosticSeverityWarning:
		dd.Severity = diag.Warning
	}
	if d.Attribute != nil {
		dd.AttributePath = make(cty.Path, 0)
		for _, step := range d.Attribute.Steps() {
			switch step := step.(type) {
			case tftypes.AttributeName:
				dd.AttributePath = dd.AttributePath.GetAttr(string(step))
			case tftypes.ElementKeyInt:
				dd.AttributePath = dd.AttributePath.IndexInt(int(int64(step)))
			case tftypes.ElementKeyString:
				dd.AttributePath = dd.AttributePath.IndexString(string(step))
			default:
				contract.Failf("Unexpected AttributePathStep")
			}
		}
	}
	return dd
}
