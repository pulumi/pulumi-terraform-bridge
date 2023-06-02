// Copyright 2016-2023, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

const metaKey = "__meta"

type resourceState1 struct {
	TFSchemaVersion int64
	Value           tftypes.Value
	Private         []byte
}

// Resource state where UpgradeResourceState has been already done if necessary.
type upgradedResourceState struct {
	state *resourceState1
}

func (u *upgradedResourceState) PrivateState() []byte {
	return u.state.Private
}

// TODO save private state if any.
func (u *upgradedResourceState) ToPropertyMap(rh *resourceHandle) (resource.PropertyMap, error) {
	propMap, err := convert.DecodePropertyMap(rh.decoder, u.state.Value)
	if err != nil {
		return nil, err
	}

	return updateTFSchemaVersion(propMap, u.state.TFSchemaVersion)
}

func (u *upgradedResourceState) ExtractID(rh *resourceHandle) (resource.ID, error) {
	idString, err := rh.idExtractor.extractID(u.state.Value)
	if err != nil {
		return "", err
	}
	return resource.ID(idString), nil
}

func newResourceState(ctx context.Context, rh *resourceHandle, private []byte) *upgradedResourceState {
	tfType := rh.schema.Type().TerraformType(ctx)
	value := tftypes.NewValue(tfType, nil)
	schemaVersion := rh.schema.ResourceSchemaVersion()
	return &upgradedResourceState{
		&resourceState1{
			Value:           value,
			TFSchemaVersion: schemaVersion,
			Private:         private,
		},
	}
}

func parseResourceState(rh *resourceHandle, props resource.PropertyMap) (*resourceState1, error) {
	stateVersion, err := parseTFSchemaVersion(props)
	if err != nil {
		return nil, err
	}

	if rh.pulumiResourceInfo.PreStateUpgradeHook != nil {
		var err error
		stateVersion, props, err = rh.pulumiResourceInfo.PreStateUpgradeHook(tfbridge.PreStateUpgradeHookArgs{
			ResourceSchemaVersion:   rh.schema.ResourceSchemaVersion(),
			PriorState:              props.Copy(),
			PriorStateSchemaVersion: stateVersion,
		})
		if err != nil {
			return nil, fmt.Errorf("PreStateUpgradeHook failed: %w", err)
		}
		props, err = updateTFSchemaVersion(props, stateVersion)
		if err != nil {
			return nil, fmt.Errorf("PreStateUpgradeHook failed to update schema version: %w", err)
		}
	}

	value, err := convert.EncodePropertyMap(rh.encoder, props)
	if err != nil {
		return nil, err
	}
	return &resourceState1{
		Value:           value,
		TFSchemaVersion: stateVersion,
		Private:         []byte{}, // TODO!!
	}, nil
}

func parseResourceStateFromTF(
	ctx context.Context,
	rh *resourceHandle,
	state *tfprotov6.DynamicValue,
	private []byte,
) (*upgradedResourceState, error) {
	tfType := rh.schema.Type().TerraformType(ctx)
	v, err := state.Unmarshal(tfType)
	if err != nil {
		return nil, err
	}
	return &upgradedResourceState{state: &resourceState1{
		TFSchemaVersion: rh.schema.ResourceSchemaVersion(),
		Value:           v,
		Private:         private,
	}}, nil
}

type metaBlock struct {
	SchemaVersion string `json:"schema_version"`
}

// Restores Terraform Schema version encoded in __meta.schema_version section of Pulumi state. If __meta block is
// absent, returns 0, which is the correct implied schema version.
func parseTFSchemaVersion(m resource.PropertyMap) (int64, error) {
	var meta metaBlock
	if metaProperty, hasMeta := m[metaKey]; hasMeta && metaProperty.IsString() {
		if err := json.Unmarshal([]byte(metaProperty.StringValue()), &meta); err != nil {
			err = fmt.Errorf("expected %q special property to be a JSON-marshalled string: %w",
				metaKey, err)
			return 0, err
		}
		versionN, err := strconv.Atoi(meta.SchemaVersion)
		if err != nil {
			err = fmt.Errorf(`expected props[%q]["schema_version"] to be an integer, got %q: %w`,
				metaKey, meta.SchemaVersion, err)
			return 0, err
		}
		return int64(versionN), nil
	}
	return 0, nil
}

// Stores Terraform Schema Version in the __meta.schema_version section of Pulumi state. If the __meta block is absent,
// adds it. If __meta block is present, copies it and only updates the schema_version field.
func updateTFSchemaVersion(m resource.PropertyMap, version int64) (resource.PropertyMap, error) {
	var meta map[string]interface{}
	if metaProperty, hasMeta := m[metaKey]; hasMeta && metaProperty.IsString() {
		if err := json.Unmarshal([]byte(metaProperty.StringValue()), &meta); err != nil {
			err = fmt.Errorf("expected %q special property to be a JSON-marshalled string: %w",
				metaKey, err)
			return nil, err
		}
	} else {
		meta = map[string]interface{}{}
	}
	if version != 0 {
		meta["schema_version"] = fmt.Sprintf("%d", version)
	}
	if len(meta) == 0 {
		return m, nil
	}
	updatedMeta, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	c := m.Copy()
	c[metaKey] = resource.NewStringProperty(string(updatedMeta))
	return c, nil
}
