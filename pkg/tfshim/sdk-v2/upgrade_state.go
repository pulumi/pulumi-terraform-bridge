package sdkv2

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/rawstate"
)

func upgradeResourceStateGRPC(
	ctx context.Context,
	t string,
	res *schema.Resource,
	state rawstate.RawState,
	meta map[string]any,
	server tfprotov5.ProviderServer,
) (cty.Value, map[string]any, error) {
	jsonBytes, err := json.Marshal(state)
	if err != nil {
		return cty.Value{}, nil, err
	}
	return upgradeResourceStateGRPCInner(ctx, t, res, jsonBytes, meta, server)
}

func upgradeResourceStateGRPCInner(
	ctx context.Context,
	t string,
	res *schema.Resource,
	jsonBytes json.RawMessage, // raw state in JSON representation
	meta map[string]any,
	server tfprotov5.ProviderServer,
) (cty.Value, map[string]any, error) {
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
	if uerr := handleDiagnostics(ctx, resp.Diagnostics, err); uerr != nil {
		return cty.Value{}, nil, uerr
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
