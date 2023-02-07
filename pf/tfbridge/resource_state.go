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
)

const metaKey = "__meta"

type resourceState struct {
	TFSchemaVersion int64
	Value           tftypes.Value
}

// Resource state where UpgradeResourceState has been already done if necessary.
type upgradedResourceState struct {
	state *resourceState
}

func (u *upgradedResourceState) ToPropertyMap(rh *resourceHandle) (resource.PropertyMap, error) {
	propMap, err := convert.DecodePropertyMap(rh.decoder, u.state.Value)
	if err != nil {
		return nil, err
	}

	return addTFSchemaVersion(propMap, u.state.TFSchemaVersion)
}

func (u *upgradedResourceState) ExtractID(rh *resourceHandle) (resource.ID, error) {
	idString, err := rh.idExtractor.extractID(u.state.Value)
	if err != nil {
		return "", err
	}
	return resource.ID(idString), nil
}

func newResourceState(ctx context.Context, rh *resourceHandle) *upgradedResourceState {
	tfType := rh.schema.Type().TerraformType(ctx)
	value := tftypes.NewValue(tfType, nil)
	schemaVersion := rh.schema.ResourceSchemaVersion()
	return &upgradedResourceState{
		&resourceState{
			Value:           value,
			TFSchemaVersion: schemaVersion,
		},
	}
}

func parseResourceState(rh *resourceHandle, props resource.PropertyMap) (*resourceState, error) {
	stateVersion, err := parseTFSchemaVersion(props)
	if err != nil {
		return nil, err
	}
	value, err := convert.EncodePropertyMap(rh.encoder, props)
	if err != nil {
		return nil, err
	}
	return &resourceState{
		Value:           value,
		TFSchemaVersion: stateVersion,
	}, nil
}

func parseResourceStateFromTF(
	ctx context.Context,
	rh *resourceHandle,
	state *tfprotov6.DynamicValue,
) (*upgradedResourceState, error) {
	tfType := rh.schema.Type().TerraformType(ctx)
	v, err := state.Unmarshal(tfType)
	if err != nil {
		return nil, err
	}
	return &upgradedResourceState{state: &resourceState{
		TFSchemaVersion: rh.schema.ResourceSchemaVersion(),
		Value:           v,
	}}, nil
}

type metaBlock struct {
	SchemaVersion string `json:"schema_version"`
}

func addTFSchemaVersion(m resource.PropertyMap, version int64) (resource.PropertyMap, error) {
	if version == 0 {
		return m, nil
	}
	if _, hasMeta := m[metaKey]; hasMeta {
		return nil, fmt.Errorf("cannot add %q property as it is already set", metaKey)
	}
	b := metaBlock{SchemaVersion: fmt.Sprintf("%d", version)}
	bytes, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	c := m.Copy()
	c[metaKey] = resource.NewStringProperty(string(bytes))
	return c, nil
}

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
