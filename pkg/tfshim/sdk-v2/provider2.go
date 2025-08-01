package sdkv2

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/hashicorp/go-cty/cty"
	ctyjson "github.com/hashicorp/go-cty/cty/json"
	"github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/internalinter"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/rawstate"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/log"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
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

// The the legacy method of reconstructing an InstanceState. For new providers prefer UpgradeState method.
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
	s, err := recoverAndCoerceCtyValueWithSchema(r.tf.CoreConfigSchema(), object)
	if err != nil {
		glog.V(9).Infof("failed to coerce config: %v, proceeding with imprecise value", err)
		original := schema.HCL2ValueFromConfigValue(object)
		s = original
	}
	s = normalizeBlockCollections(s, r.tf)

	// This state has not passed through the upgrade method and is marked as such.
	return newNonUpgradedInstanceState(r.resourceType, s, meta), nil
}

func (r *v2Resource2) Timeouts() *shim.ResourceTimeout {
	return v2Resource{tf: r.tf}.Timeouts()
}

func (r *v2Resource2) DecodeTimeouts(config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	return v2Resource{tf: r.tf}.DecodeTimeouts(config)
}

func (r *v2Resource2) SchemaType() valueshim.Type {
	return valueshim.FromHctyResourceType(r.tf.CoreConfigSchema().ImpliedType())
}

type v2InstanceState2 struct {
	resourceType string
	stateValue   cty.Value

	// Also known as private state.
	meta map[string]interface{}

	// The state has passed through the state upgrade method and does not need to do it again.
	isUpgraded bool
	internalinter.Internal
}

var (
	_ shim.InstanceState               = (*v2InstanceState2)(nil)
	_ shim.InstanceStateWithTypedValue = (*v2InstanceState2)(nil)
)

func newNonUpgradedInstanceState(resourceType string, stateValue cty.Value, meta map[string]any) *v2InstanceState2 {
	return &v2InstanceState2{
		resourceType: resourceType,
		stateValue:   stateValue,
		isUpgraded:   false,
		meta:         meta,
	}
}

func newUpgradedInstanceState(resourceType string, stateValue cty.Value, meta map[string]any) *v2InstanceState2 {
	return &v2InstanceState2{
		resourceType: resourceType,
		stateValue:   stateValue,
		isUpgraded:   true,
		meta:         meta,
	}
}

func (s *v2InstanceState2) markUpgraded() *v2InstanceState2 {
	copy := *s
	copy.isUpgraded = true
	return &copy
}

func (s *v2InstanceState2) Type() string {
	return s.resourceType
}

func (s *v2InstanceState2) ID() string {
	if s.stateValue.IsNull() {
		return ""
	}
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

func (s *v2InstanceState2) Value() valueshim.Value {
	return valueshim.FromHCtyValue(s.stateValue)
}

type v2InstanceDiff2 struct {
	v2InstanceDiff

	resourceType              string
	config                    cty.Value
	plannedState              cty.Value
	plannedPrivate            map[string]interface{}
	diffEqualDecisionOverride shim.DiffOverride
	prior                     cty.Value
	priorMeta                 map[string]interface{}
}

func (d *v2InstanceDiff2) String() string {
	return d.GoString()
}

func (d *v2InstanceDiff2) GoString() string {
	if d == nil {
		return "nil"
	}
	return fmt.Sprintf(`&v2InstanceDiff2{
    v2InstanceDiff: v2InstanceDiff{
        tf: %#v,
    },
    config:         %#v,
    plannedState:   %#v,
    plannedPrivate:    %#v,
}`, d.tf, d.config, d.plannedState, d.plannedPrivate)
}

var _ shim.InstanceDiff = (*v2InstanceDiff2)(nil)

func (d *v2InstanceDiff2) ProposedState(
	res shim.Resource, priorState shim.InstanceState,
) (shim.InstanceState, error) {
	// The proposed state is always upgraded, which is a bit of a technicality since the code will not try
	// upgrading it anyway.
	return newUpgradedInstanceState(d.resourceType, d.plannedState, d.tf.Meta), nil
}

func (d *v2InstanceDiff2) PriorState() (shim.InstanceState, error) {
	return &v2InstanceState2{
		stateValue: d.prior,
		meta:       d.priorMeta,
	}, nil
}

func (d *v2InstanceDiff2) DiffEqualDecisionOverride() shim.DiffOverride {
	return d.diffEqualDecisionOverride
}

// If the user specifies custom timeout overrides for the resource, encode them in the magic `timeouts` key under config
// and plannedState. This is how TF communicates this information on the gRPC protocol. The schema default timeouts need
// not be specially encoded because the gRPC implementation already respects them. A slight complication here is that
// some resources do not seem to allow customizing timeouts for certain operations, and their impliedType will not have
// the corresponding slot. Warn the user if the custom timeout is a no-op.
func (p *v2Provider) configWithTimeouts(
	ctx context.Context,
	c shim.ResourceConfig,
	topts shim.TimeoutOptions,
	impliedType cty.Type,
) map[string]any {
	configWithoutTimeouts := configFromShim(c).Config
	config := map[string]any{}
	for k, v := range configWithoutTimeouts {
		if k == schema.TimeoutsConfigKey {
			p.warnIgnoredCustomTimeouts(ctx, topts.TimeoutOverrides)
			return configWithoutTimeouts
		}
		config[k] = v
	}
	impliedAttrs := impliedType.AttributeTypes()
	timeoutsObj, supportCustomTimeouts := impliedAttrs[schema.TimeoutsConfigKey]
	if !supportCustomTimeouts || !timeoutsObj.IsObjectType() {
		p.warnIgnoredCustomTimeouts(ctx, topts.TimeoutOverrides)
		return configWithoutTimeouts
	}
	timeoutAttrs := timeoutsObj.AttributeTypes()
	tn := 0
	tt := map[string]any{}
	for k := range timeoutAttrs {
		if override, ok := topts.TimeoutOverrides[shim.TimeoutKey(k)]; ok {
			tt[k] = override.String()
			tn++
		} else {
			tt[k] = nil
		}
	}
	if tn > 0 {
		config[schema.TimeoutsConfigKey] = tt
	}
	unusedOverrides := map[shim.TimeoutKey]time.Duration{}
	for k, d := range topts.TimeoutOverrides {
		_, used := tt[string(k)]
		if !used {
			unusedOverrides[k] = d
		}
	}
	p.warnIgnoredCustomTimeouts(ctx, unusedOverrides)
	return config
}

func (*v2Provider) warnIgnoredCustomTimeouts(
	ctx context.Context,
	timeoutOverrides map[shim.TimeoutKey]time.Duration,
) {
	if len(timeoutOverrides) == 0 {
		return
	}
	var parts []string
	for k, v := range timeoutOverrides {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v.String()))
	}
	sort.Strings(parts)
	keys := strings.Join(parts, ", ")
	msg := fmt.Sprintf("Resource does not support customTimeouts, ignoring: %s", keys)
	log.GetLogger(ctx).Warn(msg)
}

func (p v2Provider) Diff(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
	opts shim.DiffOptions,
) (shim.InstanceDiff, error) {
	s, err := p.ensureStateIsUpgraded(ctx, t, s)
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

	cfg, err := recoverAndCoerceCtyValueWithSchema(res.CoreConfigSchema(),
		p.configWithTimeouts(ctx, c, opts.TimeoutOptions, ty))
	if err != nil {
		return nil, fmt.Errorf("Resource %q: %w", t, err)
	}
	cfg = normalizeBlockCollections(cfg, res)

	prop, err := proposedNew(res, state.stateValue, cfg)
	if err != nil {
		return nil, err
	}
	st := state.stateValue
	ic := opts.IgnoreChanges
	priv := state.meta
	plan, err := p.server.PlanResourceChange(ctx, t, ty, cfg, st, prop, priv, meta, ic)
	if err != nil {
		return nil, err
	}

	plannedState, err := p.planEdit(ctx, PlanStateEditRequest{
		NewInputs:      opts.NewInputs,
		ProviderConfig: opts.ProviderConfig,
		TfToken:        t,
		PlanState:      plan.PlannedState,
	})

	//nolint:lll
	// Taken from https://github.com/opentofu/opentofu/blob/864aa9d1d629090cfc4ddf9fdd344d34dee9793e/internal/tofu/node_resource_abstract_instance.go#L1024
	// We need to unmark the values to make sure Equals works.
	// Equals will return unknown if either value is unknown.
	// START
	unmarkedPrior, _ := st.UnmarkDeep()
	unmarkedPlan, _ := plannedState.UnmarkDeep()
	eqV := unmarkedPrior.Equals(unmarkedPlan)
	eq := eqV.IsKnown() && eqV.True()
	// END

	diffOverride := shim.DiffOverrideUpdate
	if eq {
		diffOverride = shim.DiffOverrideNoUpdate
	}

	return &v2InstanceDiff2{
		v2InstanceDiff: v2InstanceDiff{
			tf: plan.PlannedDiff,
		},
		config:                    cfg,
		plannedState:              plannedState,
		resourceType:              t,
		diffEqualDecisionOverride: diffOverride,
		plannedPrivate:            plan.PlannedPrivate,
		prior:                     st,
		priorMeta:                 priv,
	}, err
}

func (p *v2Provider) planEdit(ctx context.Context, e PlanStateEditRequest) (cty.Value, error) {
	if p.planEditFunc == nil {
		return e.PlanState, nil
	}
	return p.planEditFunc(ctx, e)
}

func (p v2Provider) Apply(
	ctx context.Context, t string, s shim.InstanceState, d shim.InstanceDiff,
) (shim.InstanceState, error) {
	res := p.tf.ResourcesMap[t]
	ty := res.CoreConfigSchema().ImpliedType()
	s, err := p.ensureStateIsUpgraded(ctx, t, s)
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

	// Merge plannedPrivate and v2InstanceDiff.tf.Meta into a single map. This is necessary because
	// timeouts are stored in the Meta and not in plannedPrivate.
	priv := make(map[string]interface{})
	if len(diff.plannedPrivate) > 0 {
		maps.Copy(priv, diff.plannedPrivate)
	}
	if len(diff.tf.Meta) > 0 {
		maps.Copy(priv, diff.tf.Meta)
	}

	return p.server.ApplyResourceChange(ctx, t, ty, cfg, st, pl, priv, meta)
}

// This method is called to service `pulumi refresh` requests and maps naturally to the TF
// ReadResource method. When using `pulumi import` this is not called, and instead
// resource.Importer() is called which maps to the TF ImportResourceState method..
func (p v2Provider) Refresh(
	ctx context.Context,
	t string,
	s shim.InstanceState,
	c shim.ResourceConfig,
) (shim.InstanceState, error) {
	res := p.tf.ResourcesMap[t]
	ty := res.CoreConfigSchema().ImpliedType()
	s, err := p.ensureStateIsUpgraded(ctx, t, s)
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
	return newUpgradedInstanceState(rr.resourceType, rr.stateValue, rr.meta), nil
}

func (p v2Provider) NewDestroyDiff(
	ctx context.Context, t string, opts shim.TimeoutOptions,
) shim.InstanceDiff {
	res := p.tf.ResourcesMap[t]
	ty := res.CoreConfigSchema().ImpliedType()

	dd := v2InstanceDiff{tf: &terraform.InstanceDiff{Destroy: true}}
	dd.applyTimeoutOptions(opts)

	return &v2InstanceDiff2{
		resourceType:   t,
		v2InstanceDiff: dd,
		config:         cty.NullVal(ty),
		plannedState:   cty.NullVal(ty),
	}
}

func (p *v2Provider) Importer(t string) shim.ImportFunc {
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

func (p *v2Provider) providerMeta() (*cty.Value, error) {
	return nil, nil
	// TODO[pulumi/pulumi-terraform-bridge#1827]: We do not believe that this is load bearing in any providers.
}

func (*v2Provider) unpackDiff(_ cty.Type, d shim.InstanceDiff) *v2InstanceDiff2 {
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

func (p *v2Provider) unpackInstanceState(t string, s shim.InstanceState) *v2InstanceState2 {
	switch s := s.(type) {
	case nil:
		return p.newEmptyStateImpl(t)
	case *v2InstanceState2:
		return s
	}
	contract.Failf("Unexpected type for shim.InstanceState: #%T", s)
	return nil
}

func (p *v2Provider) ensureStateIsUpgraded(
	ctx context.Context,
	t string,
	s shim.InstanceState,
) (shim.InstanceState, error) {
	res := p.tf.ResourcesMap[t]
	state := p.unpackInstanceState(t, s)

	// If it has already been upgraded, nothing is to be done here.
	if state.isUpgraded {
		return state, nil
	}

	// In the case of Create, prior state is encoded as Null and should not be subject to upgrades.
	if state.stateValue.IsNull() {
		// An empty state counts as an upgraded state.
		return state.markUpgraded(), nil
	}

	// Otherwise translate to shim.RawState and pass through an gRPC TF upgrade method.
	rawState := state.stateValue

	jsonBytes, err := ctyjson.Marshal(rawState, rawState.Type())
	if err != nil {
		return nil, err
	}

	newState, newMeta, err := upgradeResourceStateGRPCInner(ctx, t, res, jsonBytes, state.meta, p.server.gserver)
	if err != nil {
		return nil, err
	}

	return newUpgradedInstanceState(t, newState, newMeta), nil
}

// New-style constructor for empty states.
func (p v2Provider) newEmptyStateImpl(t string) *v2InstanceState2 {
	res := p.tf.ResourcesMap[t]
	ty := res.CoreConfigSchema().ImpliedType()
	return newUpgradedInstanceState(t, cty.NullVal(ty), nil)
}

// New-style upgrade method exposed through the shim layer. Should never be called on creates with nil state.
func (p v2Provider) UpgradeState(
	ctx context.Context,
	t string,
	s rawstate.RawState,
	meta map[string]any,
) (shim.InstanceState, error) {
	res := p.tf.ResourcesMap[t]

	newState, newMeta, err := upgradeResourceStateGRPC(ctx, t, res, s, meta, p.server.gserver)
	if err != nil {
		return nil, err
	}

	return newUpgradedInstanceState(t, newState, newMeta), nil
}

// Helper to unwrap gRPC types from GRPCProviderServer.
type grpcServer struct {
	gserver *schema.GRPCProviderServer
}

// This will return an error if any of the diagnostics are error-level, or a given err is non-nil.
// It will also logs the diagnostics into TF loggers, so they appear when debugging with the bridged
// provider with TF_LOG=TRACE or similar.
func handleDiagnostics(ctx context.Context, diags []*tfprotov5.Diagnostic, err error) error {
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
) (*struct {
	PlannedState   cty.Value
	PlannedPrivate map[string]interface{}
	PlannedDiff    *terraform.InstanceDiff
}, error,
) {
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
			dd := &v2InstanceDiff{tf: d}
			if ignores != nil {
				dd.processIgnoreChanges(ignores)
			}
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
	if err := handleDiagnostics(ctx, resp.Diagnostics, err); err != nil {
		return nil, err
	}
	// Ignore resp.UnsafeToUseLegacyTypeSystem - does not matter for Pulumi bridged providers.
	// Ignore resp.RequiresReplace - expect replacement to be encoded in resp.InstanceDiff.
	plannedState, err := msgpack.Unmarshal(resp.PlannedState.MsgPack, ty)
	if err != nil {
		return nil, err
	}

	// There are cases where planned state is equal to the original state, but InstanceDiff still displays changes.
	// Pulumi considers this to be a no-change diff, and as a workaround here any InstanceDiff changes are deleted
	// and ignored (simlar to processIgnoreChanges).
	//
	// See pulumi/pulumi-aws#3880
	same := plannedState.Equals(priorState)
	if same.IsKnown() && same.True() {
		resp.InstanceDiff.Attributes = map[string]*terraform.ResourceAttrDiff{}
	}

	var meta map[string]interface{}
	if resp.PlannedPrivate != nil {
		if err := json.Unmarshal(resp.PlannedPrivate, &meta); err != nil {
			return nil, err
		}
	}
	return &struct {
		PlannedState   cty.Value
		PlannedPrivate map[string]interface{}
		PlannedDiff    *terraform.InstanceDiff
	}{
		PlannedState:   plannedState,
		PlannedPrivate: meta,
		PlannedDiff:    resp.InstanceDiff,
	}, nil
}

func (s *grpcServer) ApplyResourceChange(
	ctx context.Context,
	typeName string,
	ty cty.Type,
	config, priorState, plannedState cty.Value,
	plannedMeta map[string]interface{},
	providerMeta *cty.Value,
) (shim.InstanceState, error) {
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

	var providerMetaVal []byte
	if providerMeta != nil {
		providerMetaVal, err = msgpack.Marshal(*providerMeta, providerMeta.Type())
		if err != nil {
			return nil, err
		}
	} else {
		providerMetaVal, err = msgpack.Marshal(cty.NullVal(cty.EmptyObject), cty.EmptyObject)
		if err != nil {
			return nil, err
		}
	}

	req := &tfprotov5.ApplyResourceChangeRequest{
		TypeName:     typeName,
		Config:       &tfprotov5.DynamicValue{MsgPack: configVal},
		PriorState:   &tfprotov5.DynamicValue{MsgPack: priorStateVal},
		PlannedState: &tfprotov5.DynamicValue{MsgPack: plannedStateVal},
		ProviderMeta: &tfprotov5.DynamicValue{MsgPack: providerMetaVal},
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
	resp, applyErr := s.gserver.ApplyResourceChange(ctx, req)
	newState := cty.Value{}
	var meta map[string]interface{}
	if resp != nil {
		if resp.NewState != nil {
			newState, err = msgpack.Unmarshal(resp.NewState.MsgPack, ty)
			if err != nil {
				return nil, err
			}
		}
		if resp.Private != nil {
			if err := json.Unmarshal(resp.Private, &meta); err != nil {
				return nil, err
			}
		}
	}
	returnErr := handleDiagnostics(ctx, resp.Diagnostics, applyErr)
	if newState.IsNull() {
		return nil, returnErr
	}
	return newUpgradedInstanceState(typeName, newState, meta), returnErr
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
	if err := handleDiagnostics(ctx, resp.Diagnostics, err); err != nil {
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
	return newUpgradedInstanceState(typeName, newState, meta2), nil
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
	if err := handleDiagnostics(ctx, resp.Diagnostics, err); err != nil {
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

// This provider needs to override ResourceMap because it uses a different concrete implementation
// of shim.Resource and for select resources and needs to make sure the correct one is returned.
func (p v2Provider) ResourcesMap() shim.ResourceMap {
	return &v2ResourceCustomMap{
		resources: p.tf.ResourcesMap,
		pack: func(token string, res *schema.Resource) shim.Resource {
			i := p.Importer(token)
			return &v2Resource2{v2Resource{tf: res}, i, token}
		},
	}
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

func normalizeBlockCollections(val cty.Value, res *schema.Resource) cty.Value {
	// Full rules about block vs attr
	//nolint:lll
	// https://github.com/hashicorp/terraform-plugin-sdk/blob/1f499688ebd9420768f501d4ed622a51b2135ced/helper/schema/core_schema.go#L60
	sch := res.CoreConfigSchema()
	if !val.Type().IsObjectType() {
		contract.Failf("normalizeBlockCollections: Expected object type, got %v", val.Type().GoString())
	}

	if val.IsNull() || !val.IsKnown() {
		return val
	}

	valMap := val.AsValueMap()

	for fieldName := range sch.BlockTypes {
		if !val.Type().HasAttribute(fieldName) {
			continue
		}
		subVal := val.GetAttr(fieldName)
		if subVal.IsNull() {
			fieldType := val.Type().AttributeType(fieldName)
			// Only lists and sets can be blocks and pass InternalValidate
			// Ignore other types.
			if fieldType.IsListType() {
				glog.V(10).Info(
					"normalizeBlockCollections: replacing a nil list with an empty list because the underlying "+
						"TF property is a block %s, %s",
					fieldName, fieldType.ElementType())
				valMap[fieldName] = cty.ListValEmpty(fieldType.ElementType())
			} else if fieldType.IsSetType() {
				glog.V(10).Info(
					"normalizeBlockCollections: replacing a nil set with an empty set because the underlying "+
						"TF property is a block %s, %s",
					fieldName, fieldType.ElementType())
				valMap[fieldName] = cty.SetValEmpty(fieldType.ElementType())
			}
		} else {
			subBlockSchema := res.SchemaMap()[fieldName]
			if subBlockSchema == nil {
				glog.V(5).Info("normalizeBlockCollections: Unexpected nil subBlockSchema for %s", fieldName)
				continue
			}
			subBlockRes, ok := subBlockSchema.Elem.(*schema.Resource)
			if !ok {
				glog.V(5).Info("normalizeBlockCollections: Unexpected schema type %s", fieldName)
				continue
			}
			normalizedVal := normalizeSubBlock(subVal, subBlockRes)
			valMap[fieldName] = normalizedVal
		}
	}
	return cty.ObjectVal(valMap)
}

func normalizeSubBlock(val cty.Value, subBlockRes *schema.Resource) cty.Value {
	if !val.IsKnown() || val.IsNull() {
		// Blocks shouldn't be unknown, but if they are, we can't do anything with them.
		// val should also not be nil here as that case is handled separately above.
		return val
	}

	if val.Type().IsListType() {
		blockValSlice := val.AsValueSlice()
		newSlice := make([]cty.Value, len(blockValSlice))
		for i, v := range blockValSlice {
			newSlice[i] = normalizeBlockCollections(v, subBlockRes)
		}
		if len(newSlice) != 0 {
			return cty.ListVal(newSlice)
		}
		return cty.ListValEmpty(val.Type().ElementType())
	} else if val.Type().IsSetType() {
		blockValSlice := val.AsValueSlice()
		newSlice := make([]cty.Value, len(blockValSlice))
		for i, v := range blockValSlice {
			newSlice[i] = normalizeBlockCollections(v, subBlockRes)
		}
		if len(newSlice) != 0 {
			return cty.SetVal(newSlice)
		}
		return cty.SetValEmpty(val.Type().ElementType())
	}
	return val
}
