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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

const metaKey = "__meta"

type resourceState struct {
	TFSchemaVersion int64
	Value           tftypes.Value
	Private         []byte
}

// Resource state where UpgradeResourceState has been already done if necessary.
type upgradedResourceState struct {
	state *resourceState
}

func (u *upgradedResourceState) PrivateState() []byte {
	return u.state.Private
}

func (u *upgradedResourceState) ToPropertyMap(rh *resourceHandle) (resource.PropertyMap, error) {
	propMap, err := convert.DecodePropertyMap(rh.decoder, u.state.Value)
	if err != nil {
		return nil, err
	}
	return updateMeta(propMap, metaState{
		SchemaVersion: u.state.TFSchemaVersion,
		PrivateState:  u.state.Private,
	})
}

func newResourceState(ctx context.Context, rh *resourceHandle, private []byte) *upgradedResourceState {
	tfType := rh.schema.Type().TerraformType(ctx)
	value := tftypes.NewValue(tfType, nil)
	schemaVersion := rh.schema.ResourceSchemaVersion()
	return &upgradedResourceState{
		&resourceState{
			Value:           value,
			TFSchemaVersion: schemaVersion,
			Private:         private,
		},
	}
}

func parseResourceState(rh *resourceHandle, props resource.PropertyMap) (*resourceState, error) {
	parsedMeta, err := parseMeta(props)
	if err != nil {
		return nil, err
	}
	stateVersion := parsedMeta.SchemaVersion

	if rh.pulumiResourceInfo.PreStateUpgradeHook != nil {
		var err error
		stateVersion, props, err = rh.pulumiResourceInfo.PreStateUpgradeHook(tfbridge.PreStateUpgradeHookArgs{
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

	value, err := convert.EncodePropertyMap(rh.encoder, props)
	if err != nil {
		return nil, err
	}
	return &resourceState{
		Value:           value,
		TFSchemaVersion: stateVersion,
		Private:         parsedMeta.PrivateState,
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
	return &upgradedResourceState{state: &resourceState{
		TFSchemaVersion: rh.schema.ResourceSchemaVersion(),
		Value:           v,
		Private:         private,
	}}, nil
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
	if metaProperty, hasMeta := m[metaKey]; hasMeta && metaProperty.IsString() {
		if err := json.Unmarshal([]byte(metaProperty.StringValue()), &meta); err != nil {
			err = fmt.Errorf("expected %q special property to be a JSON-marshalled string: %w",
				metaKey, err)
			return metaState{}, err
		}
		var versionN int64
		if meta.SchemaVersion != "" {
			versionI, err := strconv.Atoi(meta.SchemaVersion)
			if err != nil {
				err = fmt.Errorf(`expected props[%q]["schema_version"] to be an integer, got %q: %w`,
					metaKey, meta.SchemaVersion, err)
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
	if metaProperty, hasMeta := m[metaKey]; hasMeta && metaProperty.IsString() {
		if err := json.Unmarshal([]byte(metaProperty.StringValue()), &meta); err != nil {
			err = fmt.Errorf("expected %q special property to be a JSON-marshalled string: %w",
				metaKey, err)
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
	c[metaKey] = resource.NewStringProperty(string(updatedMeta))
	return c, nil
}
