package sdkv2

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
	ctyjson "github.com/hashicorp/go-cty/cty/json"
	"github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func upgradeResourceStateGRPC(
	ctx context.Context, t string, res *schema.Resource,
	state cty.Value, meta map[string]any,
	server tfprotov5.ProviderServer,
) (cty.Value, map[string]any, error) {
	// TODO[pulumi/pulumi-terraform-bridge#1667]: This is not quite right but we need
	// the old TF state to get it right.
	jsonBytes, err := ctyjson.Marshal(state, state.Type())
	if err != nil {
		return cty.Value{}, nil, err
	}

	version, _, err := extractSchemaVersion(meta)
	if err != nil {
		return cty.Value{}, nil, err
	}

	//nolint:lll
	// https://github.com/opentofu/opentofu/blob/2ef3047ec6bb266e8d91c55519967212c1a0975d/internal/tofu/upgrade_resource_state.go#L52
	if version > int64(res.SchemaVersion) {
		return cty.Value{}, nil, fmt.Errorf(
			"State version %d is greater than schema version %d for resource %s. "+
				"Please upgrade the provider to work with this resource.",
			version, res.SchemaVersion, t,
		)
	}

	// Note upgrade is always called, even if the versions match
	//nolint:lll
	// https://github.com/opentofu/opentofu/blob/2ef3047ec6bb266e8d91c55519967212c1a0975d/internal/tofu/upgrade_resource_state.go#L72

	resp, err := server.UpgradeResourceState(ctx, &tfprotov5.UpgradeResourceStateRequest{
		TypeName: t,
		RawState: &tfprotov5.RawState{JSON: jsonBytes},
		Version:  version,
	})
	if err != nil {
		return cty.Value{}, nil, err
	}

	newState, err := msgpack.Unmarshal(resp.UpgradedState.MsgPack, res.CoreConfigSchema().ImpliedType())
	if err != nil {
		return cty.Value{}, nil, err
	}

	newMeta := make(map[string]interface{}, len(meta))
	// copy old meta into new meta
	for k, v := range meta {
		newMeta[k] = v
	}
	if res.SchemaVersion != 0 {
		newMeta["schema_version"] = strconv.Itoa(res.SchemaVersion)
	}

	return newState, newMeta, nil
}

func extractSchemaVersion(meta map[string]any) (int64, bool, error) {
	versionValue, ok := meta["schema_version"]
	if !ok {
		return 0, false, nil
	}

	versionString, ok := versionValue.(string)
	if !ok {
		return 0, true, fmt.Errorf("unexpected type %T for schema_version", versionValue)
	}
	v, err := strconv.ParseInt(versionString, 0, 32)
	if err != nil {
		return 0, true, err
	}
	return v, true, nil
}

func upgradeResourceState(ctx context.Context, t string, p *schema.Provider, res *schema.Resource,
	instanceState *terraform.InstanceState) (*terraform.InstanceState, error) {

	if instanceState == nil {
		return nil, nil
	}

	rawState := instanceState.RawState

	// If RawState is not set but attributes is, we need to hydrate RawState
	// from attributes.
	if rawState.IsNull() {
		// We default to assuming that the old state has the same shape as the new
		// resource.
		typ := res.CoreConfigSchema().ImpliedType()

		// Find the version, if present, to deserialize instanceState into.
		version, hasVersion, err := extractSchemaVersion(instanceState.Meta)
		if err != nil {
			return nil, err
		}

		// If we have a version, we use the schema shape that matches the version
		// specified.
		if hasVersion {
			for _, t := range res.StateUpgraders {
				if t.Version == int(version) {
					typ = t.Type
					break
				}
			}
		}

		rawState, err = instanceState.AttrsAsObjectValue(typ)
		if err != nil {
			return nil, fmt.Errorf("state from attributes: %w", err)
		}
	}

	if state := rawState.AsValueMap(); !has(state, "id") {
		state["id"] = cty.StringVal(instanceState.ID)
		rawState = cty.ObjectVal(state)
	}

	v, newMeta, err := upgradeResourceStateGRPC(ctx, t, res, rawState, instanceState.Meta, p.GRPCProvider())
	if err != nil {
		return nil, err
	}

	// Convert the value back to an InstanceState.
	newState, err := res.ShimInstanceStateFromValue(v)
	if err != nil {
		return nil, err
	}

	newState.RawConfig = instanceState.RawConfig
	newState.RawState = v
	newState.Meta = newMeta
	newState.ID = instanceState.ID

	// If state upgraders have modified the ID, respect the modification.
	if updatedID, ok := findID(v); ok {
		newState.ID = updatedID
	}
	return newState, nil
}

func findID(v cty.Value) (string, bool) {
	if !v.Type().IsObjectType() {
		return "", false
	}
	id, ok := v.AsValueMap()["id"]
	if !ok {
		return "", false
	}
	if !id.Type().Equals(cty.String) || id.IsNull() {
		return "", false
	}
	return id.AsString(), true
}

func has[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}
