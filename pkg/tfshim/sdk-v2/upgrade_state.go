package sdkv2

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
	ctyjson "github.com/hashicorp/go-cty/cty/json"
	"github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func upgradeResourceState(ctx context.Context, typeName string, p *schema.Provider, res *schema.Resource,
	instanceState *terraform.InstanceState) (*terraform.InstanceState, error) {
	if instanceState == nil {
		return nil, nil
	}

	if instanceState.RawState.IsNull() {
		// If RawState is not set but attributes is, we need to hydrate RawState
		// from attributes.
		state, err := instanceState.AttrsAsObjectValue(res.CoreConfigSchema().ImpliedType())
		if err != nil {
			return nil, fmt.Errorf("state from attributes: %w", err)
		}
		instanceState.RawState = state
	}

	// Ensure that we have an ID in the attributes.
	if state := instanceState.RawState.AsValueMap(); !has(state, "id") {
		state["id"] = cty.StringVal(instanceState.ID)
		instanceState.RawState = cty.ObjectVal(state)
	}

	version, hasVersion := int64(0), false
	if versionValue, ok := instanceState.Meta["schema_version"]; ok {
		versionString, ok := versionValue.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T for schema_version", versionValue)
		}
		v, err := strconv.ParseInt(versionString, 0, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse schema_version: %w", err)
		}
		version, hasVersion = v, true
	}

	json, err := ctyjson.Marshal(instanceState.RawState, res.CoreConfigSchema().ImpliedType())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RawState: %w", err)
	}

	// Now, we perform the UpgradeResourceState operation by calling into TF's UpgradeResourceState.

	resp, err := schema.NewGRPCProviderServer(p).
		UpgradeResourceState(ctx, &tfprotov5.UpgradeResourceStateRequest{
			TypeName: typeName,
			Version:  version,
			RawState: &tfprotov5.RawState{JSON: json},
		})
	if err != nil {
		return nil, fmt.Errorf("upgrade resource state GRPC: %w", err)
	}

	// Handle returned diagnostics.
	var dd diag.Diagnostics
	for _, d := range resp.Diagnostics {
		if d == nil {
			continue
		}
		rd := recoverDiagnostic(*d)
		dd = append(dd, rd)
		logDiag(ctx, rd)
	}
	if err := diagToError(dd); err != nil {
		return nil, fmt.Errorf("diag: %w", err)
	}

	// Unmarshal to get back the underlying type.
	var rawState cty.Value
	if resp.UpgradedState.JSON != nil {
		rawState, err = ctyjson.Unmarshal(resp.UpgradedState.JSON, res.CoreConfigSchema().ImpliedType())
	}
	if resp.UpgradedState.MsgPack != nil {
		rawState, err = msgpack.Unmarshal(resp.UpgradedState.MsgPack, res.CoreConfigSchema().ImpliedType())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	newState, err := res.ShimInstanceStateFromValue(rawState)
	if err != nil {
		return nil, err
	}

	newState.RawConfig = instanceState.RawConfig

	// Copy the original ID and meta to the new state and stamp in the new version.
	newState.ID = instanceState.ID

	// If state upgraders have modified the ID, respect the modification.
	if updatedID, ok := findID(rawState); ok {
		newState.ID = updatedID
	}

	newState.Meta = instanceState.Meta
	if hasVersion || version > 0 {
		if newState.Meta == nil {
			newState.Meta = map[string]interface{}{}
		}
		newState.Meta["schema_version"] = strconv.Itoa(int(version))
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
