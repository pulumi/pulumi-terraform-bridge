package sdkv2

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func upgradeResourceState(ctx context.Context, p *schema.Provider, res *schema.Resource,
	instanceState *terraform.InstanceState) (*terraform.InstanceState, error) {

	if instanceState == nil {
		return nil, nil
	}

	m := instanceState.Attributes

	// Ensure that we have an ID in the attributes.
	m["id"] = instanceState.ID

	var json map[string]any
	version, hasVersion := 0, false
	if instanceState.RawState.IsNull() {
		if versionValue, ok := instanceState.Meta["schema_version"]; ok {
			versionString, ok := versionValue.(string)
			if !ok {
				return nil, fmt.Errorf("unexpected type %T for schema_version", versionValue)
			}
			v, err := strconv.ParseInt(versionString, 0, 32)
			if err != nil {
				return nil, err
			}
			version, hasVersion = int(v), true
		}

		var err error
		// First, build a JSON state from the InstanceState.
		json, version, err = schema.UpgradeFlatmapState(ctx, version, m, res, p.Meta())
		if err != nil {
			return nil, err
		}
	} else {
		v := instanceState.RawState.AsValueMap()
		if _, ok := v["id"]; !ok {
			v["id"] = cty.StringVal(instanceState.ID)
			instanceState.RawState = cty.ObjectVal(v)
		}

		var err error
		// Cannot call schema.StateValueToJSONMap since it round-trips through float64
		json, err = schema.StateValueToJSONMap(instanceState.RawState, res.CoreConfigSchema().ImpliedType())
		if err != nil {
			return nil, err
		}
	}

	fmt.Printf("- %#v\n", json)
	fmt.Printf("-x %#v (%[1]T)\n", json["x"])

	// Next, migrate the JSON state up to the current version.

	// Discovery: schema.UpgradeJSONState works on a lossy representation; numbers as
	// float64. To maintain fidelity, we must avoid it if possible.
	//
	// The best way I can think of is to only round-trip through JSON when upgrades
	// are needed.
	json, err := schema.UpgradeJSONState(ctx, version, json, res, p.Meta())
	if err != nil {
		return nil, err
	}

	configBlock := res.CoreConfigSchema()

	// Strip out removed fields.
	schema.RemoveAttributes(ctx, json, configBlock.ImpliedType())

	// now we need to turn the state into the default json representation, so
	// that it can be re-decoded using the actual schema.
	v, err := schema.JSONMapToStateValue(json, configBlock)
	if err != nil {
		return nil, err
	}

	// Now we need to make sure blocks are represented correctly, which means
	// that missing blocks are empty collections, rather than null.
	// First we need to CoerceValue to ensure that all object types match.
	v, err = configBlock.CoerceValue(v)
	if err != nil {
		return nil, err
	}

	// Normalize the value and fill in any missing blocks.
	v = schema.NormalizeObjectFromLegacySDK(v, configBlock)

	fmt.Printf("!- %s\n", v.GoString())
	fmt.Printf("!-x %#v\n", v.GetAttr("x").AsBigFloat().Text('f', 0))

	// Convert the value back to an InstanceState.
	newState, err := res.ShimInstanceStateFromValue(v)
	if err != nil {
		return nil, err
	}
	newState.RawConfig = instanceState.RawConfig

	fmt.Printf("!!- %s\n", newState.Attributes)

	// Copy the original ID and meta to the new state and stamp in the new version.
	newState.ID = instanceState.ID

	// If state upgraders have modified the ID, respect the modification.
	if updatedID, ok := findID(v); ok {
		newState.ID = updatedID
	}

	newState.Meta = instanceState.Meta
	if hasVersion || version > 0 {
		if newState.Meta == nil {
			newState.Meta = map[string]interface{}{}
		}
		newState.Meta["schema_version"] = strconv.Itoa(version)
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
