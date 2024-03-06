package sdkv2

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func upgradeResourceState(ctx context.Context, typeName string, p *schema.Provider, res *schema.Resource,
	instanceState *terraform.InstanceState) (*terraform.InstanceState, error) {
	if instanceState == nil {
		return nil, nil
	}

	m := instanceState.Attributes

	// Ensure that we have an ID in the attributes.
	m["id"] = instanceState.ID

	version, hasVersion := int64(0), false
	if versionValue, ok := instanceState.Meta["schema_version"]; ok {
		versionString, ok := versionValue.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T for schema_version", versionValue)
		}
		v, err := strconv.ParseInt(versionString, 0, 32)
		if err != nil {
			return nil, err
		}
		version, hasVersion = v, true
	}

	rawState, err := upgradeResourceStateRPC(ctx, typeName, m, p, res, version)
	if err != nil {
		return nil, err
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
	if !id.Type().Equals(cty.String) {
		return "", false
	}
	return id.AsString(), true
}

// Perform the UpgradeResourceState operation by invoking TF's underlying gRPC server.
func upgradeResourceStateRPC(
	ctx context.Context, typeName string, m map[string]string,
	p *schema.Provider, res *schema.Resource,
	version int64,
) (cty.Value, error) {
	// Call SDKv2's underlying UpgradeResourceState.
	resp, err := schema.NewGRPCProviderServer(p).
		UpgradeResourceState(ctx, &tfprotov5.UpgradeResourceStateRequest{
			TypeName: typeName,
			Version:  version,
			RawState: &tfprotov5.RawState{Flatmap: m},
		})
	if err != nil {
		return cty.Value{}, fmt.Errorf("upgrade resource state: %w", err)
	}

	// Handle returned diagnostics.
	for _, d := range resp.Diagnostics {
		msg := fmt.Sprintf("%s: %s", d.Summary, d.Detail)
		switch d.Severity {
		case tfprotov5.DiagnosticSeverityError:
			err = errors.Join(err, d.Attribute.NewError(fmt.Errorf("%s", msg)))
		case tfprotov5.DiagnosticSeverityWarning:
			// Accessing the logger (GetLogger) requires an import cycle on
			// tfbridge, so ignore for now.
		}
	}
	if err != nil {
		return cty.Value{}, err
	}

	// Unmarshal to get back the underlying type.
	return msgpack.Unmarshal(resp.UpgradedState.MsgPack, res.CoreConfigSchema().ImpliedType())
}
