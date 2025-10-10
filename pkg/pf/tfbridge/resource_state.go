// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tfbridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/reservedkeys"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
)

// Resource state where UpgradeResourceState has been already done if necessary.
type upgradedResourceState struct {
	TFSchemaVersion int64
	Value           tftypes.Value
	Private         []byte
}

func (u *upgradedResourceState) PrivateState() []byte {
	return u.Private
}

func (u *upgradedResourceState) ToPropertyMap(ctx context.Context, rh *resourceHandle) (resource.PropertyMap, error) {
	propMap, err := convert.DecodePropertyMap(ctx, rh.decoder, u.Value)
	if err != nil {
		return nil, err
	}
	return updateMeta(propMap, metaState{
		SchemaVersion: u.TFSchemaVersion,
		PrivateState:  u.Private,
	})
}

func newResourceState(ctx context.Context, rh *resourceHandle, private []byte) *upgradedResourceState {
	tfType := rh.schema.Type(ctx)
	value := tftypes.NewValue(tfType, nil)
	schemaVersion := rh.schema.ResourceSchemaVersion()
	return &upgradedResourceState{
		Value:           value,
		TFSchemaVersion: schemaVersion,
		Private:         private,
	}
}

func parseResourceStateFromTF(
	ctx context.Context,
	rh *resourceHandle,
	state *tfprotov6.DynamicValue,
	private []byte,
) (*upgradedResourceState, error) {
	tfType := rh.schema.Type(ctx)
	return parseResourceStateFromTFInner(ctx, tfType, rh.schema.ResourceSchemaVersion(), state, private)
}

func parseResourceStateFromTFInner(
	_ context.Context,
	resourceTerraformType tftypes.Type,
	resourceSchemaVersion int64,
	state *tfprotov6.DynamicValue,
	private []byte,
) (*upgradedResourceState, error) {
	rs := &upgradedResourceState{
		TFSchemaVersion: resourceSchemaVersion,
		Private:         private,
	}
	if state == nil {
		rs.Value = tftypes.NewValue(resourceTerraformType, nil)
	} else {
		v, err := state.Unmarshal(resourceTerraformType)
		if err != nil {
			return nil, err
		}
		rs.Value = v
	}
	return rs, nil
}

type metaState struct {
	SchemaVersion int64
	PrivateState  []byte
}

// Restores Terraform Schema version encoded in __meta.schema_version section of Pulumi state. If __meta block is
// absent, returns 0, which is the correct implied schema version. Reads __meta.private_state similarly.
func parseMeta(m resource.PropertyMap) (metaState, error) {
	type metaBlock struct {
		SchemaVersion string `json:"schema_version"`
		PrivateState  string `json:"private_state"`
	}
	var meta metaBlock
	if metaProperty, hasMeta := m[reservedkeys.Meta]; hasMeta && metaProperty.IsString() {
		if err := json.Unmarshal([]byte(metaProperty.StringValue()), &meta); err != nil {
			err = fmt.Errorf("expected %q special property to be a JSON-marshalled string: %w",
				reservedkeys.Meta, err)
			return metaState{}, err
		}
		var versionN int64
		if meta.SchemaVersion != "" {
			versionI, err := strconv.Atoi(meta.SchemaVersion)
			if err != nil {
				err = fmt.Errorf(`expected props[%q]["schema_version"] to be an integer, got %q: %w`,
					reservedkeys.Meta, meta.SchemaVersion, err)
				return metaState{}, err
			}
			versionN = int64(versionI)
		}

		var privateStateBytes []byte
		if meta.PrivateState != "" {
			var err error
			privateStateBytes, err = base64.StdEncoding.DecodeString(meta.PrivateState)
			if err != nil {
				return metaState{}, err
			}
		}

		return metaState{
			SchemaVersion: versionN,
			PrivateState:  privateStateBytes,
		}, nil
	}
	return metaState{}, nil
}

// Stores Terraform Schema Version in the __meta.schema_version section of Pulumi state. If the __meta block is absent,
// adds it. If __meta block is present, copies it and only updates the schema_version field. Updates
// __meta.private_state similarly.
func updateMeta(m resource.PropertyMap, newMeta metaState) (resource.PropertyMap, error) {
	var meta map[string]interface{}
	if metaProperty, hasMeta := m[reservedkeys.Meta]; hasMeta && metaProperty.IsString() {
		if err := json.Unmarshal([]byte(metaProperty.StringValue()), &meta); err != nil {
			err = fmt.Errorf("expected %q special property to be a JSON-marshalled string: %w",
				reservedkeys.Meta, err)
			return nil, err
		}
	} else {
		meta = map[string]interface{}{}
	}
	if newMeta.SchemaVersion != 0 {
		meta["schema_version"] = fmt.Sprintf("%d", newMeta.SchemaVersion)
	}
	if len(newMeta.PrivateState) > 0 {
		meta["private_state"] = base64.StdEncoding.EncodeToString(newMeta.PrivateState)
	}
	if len(meta) == 0 {
		return m, nil
	}
	updatedMeta, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	c := m.Copy()
	c[reservedkeys.Meta] = resource.NewStringProperty(string(updatedMeta))
	return c, nil
}

// Stores delta under reservedkeys.RawStateDelta; should be called right before returning to the engine.
func insertRawStateDelta(ctx context.Context, rh *resourceHandle, pm resource.PropertyMap, state tftypes.Value) error {
	schemaInfos := rh.pulumiResourceInfo.GetFields()
	v := valueshim.FromTValue(state)

	st := valueshim.FromTType(rh.schema.Type(ctx))
	delta, err := tfbridge.RawStateComputeDelta(ctx, rh.schema.Shim(), schemaInfos, pm, st, v)
	if err != nil {
		return err
	}
	pm[reservedkeys.RawStateDelta] = delta.Marshal()
	return nil
}

func (p *provider) parseAndUpgradeResourceState(
	ctx context.Context,
	rh *resourceHandle,
	props resource.PropertyMap,
) (*upgradedResourceState, error) {
	parsedMeta, err := parseMeta(props)
	if err != nil {
		return nil, err
	}

	// The version in state starts off with the value from parsedMeta, but may be modified by PreStateUpgradeHook.
	stateVersion := parsedMeta.SchemaVersion

	if rh.pulumiResourceInfo.PreStateUpgradeHook != nil {
		var err error

		// Possibly modify stateVersion.
		stateVersion, props, err = rh.pulumiResourceInfo.PreStateUpgradeHook(info.PreStateUpgradeHookArgs{
			ResourceSchemaVersion:   rh.schema.ResourceSchemaVersion(),
			PriorState:              props.Copy(),
			PriorStateSchemaVersion: parsedMeta.SchemaVersion,
		})
		if err != nil {
			return nil, fmt.Errorf("PreStateUpgradeHook failed: %w", err)
		}
		props, err = updateMeta(props, metaState{
			SchemaVersion: stateVersion,
			PrivateState:  parsedMeta.PrivateState,
		})
		if err != nil {
			return nil, fmt.Errorf("PreStateUpgradeHook failed to update schema version: %w", err)
		}
	}

	if stateVersion > rh.schema.ResourceSchemaVersion() {
		return nil, fmt.Errorf("The current state of %s was created by a newer provider version "+
			" and cannot be downgraded from resource schema version %d to %d. Please upgrade the provider",
			rh.token, stateVersion, rh.schema.ResourceSchemaVersion())
	}

	// States written by newer version of the bridge should be able to recover the raw state.
	if delta, hasDelta := props[reservedkeys.RawStateDelta]; hasDelta && p.info.RawStateDeltaEnabled() {
		rawState, err := recoverRawState(props, delta)
		if err != nil {
			// Log details at Debug level since they may contain secrets.
			tflog.Debug(ctx, "[pf/tfbridge] Failed to recover raw state for Plugin Framework",
				map[string]any{
					"token": rh.token,
					"props": props.Mappable(),
				})
			return nil, fmt.Errorf("[pf/tfbridge] Failed to recover raw state for Plugin Framework")
		}

		// Always call the upgrade method, even if at current schema version.
		return p.upgradeResourceState(ctx, rh, rawState, parsedMeta.PrivateState, stateVersion)
	}

	// Otherwise fallback to imprecise legacy parsing.
	value, err := convert.EncodePropertyMap(rh.encoder, props)
	if err != nil {
		return nil, fmt.Errorf("[pf/tfbridge] Error calling EncodePropertyMap: %w", err)
	}

	// Before EnableRawStateDelta rollout, the behavior used to be to skip the upgrade method in case of an exact
	// version match. This seems incorrect, but to derisk fixing this problem it is flagged together with
	// EnableRawStateDelta so it participates in the phased rollout. Remove once rollout completes.
	if stateVersion == rh.schema.ResourceSchemaVersion() && !p.info.RawStateDeltaEnabled() {
		return &upgradedResourceState{
			TFSchemaVersion: stateVersion,
			Private:         parsedMeta.PrivateState,
			Value:           value,
		}, nil
	}

	tfType := rh.schema.Type(ctx).(tftypes.Object)
	rawStateBytes, err := valueshim.FromTValue(value).Marshal(valueshim.FromTType(tfType))
	if err != nil {
		return nil, fmt.Errorf("[pf/tfbridge] Error calling NewRawState: %w", err)
	}
	rawState := &tfprotov6.RawState{JSON: []byte(rawStateBytes)}

	return p.upgradeResourceState(ctx, rh, rawState, parsedMeta.PrivateState, stateVersion)
}

// Wraps running state migration via the underlying TF upgradeResourceState method.
func (p *provider) upgradeResourceState(
	ctx context.Context,
	rh *resourceHandle,
	rawState *tfprotov6.RawState,
	privateState []byte,
	stateVersion int64,
) (*upgradedResourceState, error) {
	tfType := rh.schema.Type(ctx).(tftypes.Object)
	req := &tfprotov6.UpgradeResourceStateRequest{
		TypeName: rh.terraformResourceName,
		Version:  stateVersion,
		RawState: rawState,
	}

	resp, err := p.tfServer.UpgradeResourceState(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error calling UpgradeResourceState: %w", err)
	}
	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return nil, err
	}
	v, err := resp.UpgradedState.Unmarshal(tfType)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling the response from UpgradeResourceState: %w", err)
	}

	if p.info.RawStateDeltaEnabled() {
		// Downgrade float precision to 53. This is important because pulumi.PropertyValue stores float64, and
		// tftypes.Value originating from Pulumi would have this precision, but the native precision coming
		// from UpgradeResourceState is 512. Mismatches in precision may lead to spurious diffs.
		v, err = tftypes.Transform(v, func(ap *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
			if v.IsKnown() && !v.IsNull() && v.Type().Is(tftypes.Number) {
				var n big.Float
				err := v.As(&n)
				contract.AssertNoErrorf(err, "Values of tftypes.Number type should unpack to big.Float")
				if n.Prec() != 53 {
					v2 := tftypes.NewValue(tftypes.Number, new(big.Float).Copy(&n).SetPrec(53))
					return v2, nil
				}
			}
			return v, nil
		})
		contract.AssertNoErrorf(err, "float precision downgrade transform should not fail")
	}

	return &upgradedResourceState{
		TFSchemaVersion: rh.schema.ResourceSchemaVersion(),
		Value:           v,
		Private:         privateState,
	}, nil
}

func recoverRawState(props resource.PropertyMap, deltaPV resource.PropertyValue) (*tfprotov6.RawState, error) {
	delta, err := tfbridge.UnmarshalRawStateDelta(deltaPV)
	if err != nil {
		return nil, fmt.Errorf("NewRawStateDeltaFromPropertyValue failed: %w", err)
	}
	stateValue, err := delta.Recover(resource.NewObjectProperty(props))
	if err != nil {
		return nil, fmt.Errorf("delta.Recover failed: %w", err)
	}
	return &tfprotov6.RawState{JSON: stateValue}, nil
}
