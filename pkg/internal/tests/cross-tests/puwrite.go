// Copyright 2016-2024, Pulumi Corporation.
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

//nolint:lll
package crosstests

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
)

func generateYaml(schema shim.SchemaMap, resourceToken string, objectType *tftypes.Object, tfConfig any) (map[string]any, error) {
	data := map[string]any{
		"name":    "project",
		"runtime": "yaml",
		"backend": map[string]any{
			"url": "file://./data",
		},
	}
	if tfConfig == nil {
		return data, nil
	}
	pConfig, err := convertConfigToPulumi(schema, nil, objectType, tfConfig)
	if err != nil {
		return nil, err
	}

	// TODO[pulumi/pulumi-terraform-bridge#1864]: schema secrets may be set by convertConfigToPulumi.
	pConfig = propertyvalue.RemoveSecrets(resource.NewObjectProperty(pConfig)).ObjectValue()

	// This is a bit of a leap of faith that serializing PropertyMap to YAML in this way will yield valid Pulumi
	// YAML. This probably needs refinement.
	yamlProperties := pConfig.Mappable()

	data["resources"] = map[string]any{
		"example": map[string]any{
			"type":       resourceToken,
			"properties": yamlProperties,
		},
	}
	return data, nil
}

func convertConfigToPulumi(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	objectType *tftypes.Object,
	tfConfig any,
) (resource.PropertyMap, error) {
	var v *tftypes.Value

	switch tfConfig := tfConfig.(type) {
	case tftypes.Value:
		v = &tfConfig
		if objectType == nil {
			ty := v.Type().(tftypes.Object)
			objectType = &ty
		}
	case *tftypes.Value:
		v = tfConfig
		if objectType == nil {
			ty := v.Type().(tftypes.Object)
			objectType = &ty
		}
	default:
		if objectType == nil {
			t := convert.InferObjectType(schemaMap, nil)
			objectType = &t
		}
		bytes, err := json.Marshal(tfConfig)
		if err != nil {
			return nil, err
		}
		// Knowingly using a deprecated function so we can connect back up to tftypes.Value; if this disappears
		// it should not be prohibitively difficult to rewrite or vendor.
		//
		//nolint:staticcheck
		value, err := tftypes.ValueFromJSON(bytes, *objectType)
		if err != nil {
			return nil, err
		}
		v = &value
	}

	decoder, err := convert.NewObjectDecoder(convert.ObjectSchema{
		SchemaMap:   schemaMap,
		SchemaInfos: schemaInfos,
		Object:      objectType,
	})
	if err != nil {
		return nil, err
	}

	ctx := logging.InitLogging(context.Background(), logging.LogOptions{})
	// There is not yet a way to opt out of marking schema secrets, so the resulting map might have secrets marked.
	pm, err := convert.DecodePropertyMap(ctx, decoder, *v)
	if err != nil {
		return nil, err
	}
	return pm, nil
}
