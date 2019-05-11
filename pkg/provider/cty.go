// Copyright 2016-2019, Pulumi Corporation.
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

package provider

import (
	"encoding/json"
	"unicode"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/hashicorp/terraform/states"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// structpbToCtyObject converts a Protocol Buffers struct into a go-cty object, which
// is what Terraform uses for configuration for remote state backends.
func structpbToCtyObject(input *structpb.Struct) (cty.Value, error) {
	propertyMap, err := plugin.UnmarshalProperties(input, plugin.MarshalOptions{
		SkipNulls:      true,
		RejectUnknowns: true,
		Label:          "cty object",
	})
	if err != nil {
		return cty.NilVal, errors.Wrap(err, "error unmarshalling properties")
	}

	configJSONBytes, err := json.Marshal(propertyMap.Mappable())
	if err != nil {
		return cty.NilVal, errors.Wrap(err, "error marshaling PropertyMap to JSON")
	}

	impliedType, err := ctyjson.ImpliedType(configJSONBytes)
	if err != nil {
		return cty.NilVal, errors.Wrap(err, "error deducing implied cty type")
	}

	unmarshaledCty, err := ctyjson.Unmarshal(configJSONBytes, impliedType)
	if err != nil {
		return cty.NilVal, errors.Wrap(err, "error unmarshaling cty from JSON")
	}

	return unmarshaledCty, nil
}

// structpbNamesPulumiToTerraform uses the standard convention for mapping the top-level keys of
// a structpb with Pulumi-convention field names into a new structpb with Terraform-convention
// field names.
func structpbNamesPulumiToTerraform(input *structpb.Struct) *structpb.Struct {
	result := &structpb.Struct{
		Fields: map[string]*structpb.Value{},
	}

	for key, value := range input.GetFields() {
		tfName := pulumiNameToTfName(key)
		result.Fields[tfName] = value
	}

	return result
}

// tfNameToPulumiName uses the standard convention to map a Pulumi name to a Terraform name, without
// requiring a Terraform schema.
func pulumiNameToTfName(tfName string) string {
	var result string
	for i, c := range tfName {
		if c >= 'A' && c <= 'Z' {
			// if upper case, add an underscore (if it's not #1), and then the lower case version.
			if i != 0 {
				result += "_"
			}
			result += string(unicode.ToLower(c))
		} else {
			result += string(c)
		}
	}

	return result
}

// outputsToStructpb converts the map structure found in Terraform State to a structpb,
// without touching the names.
func outputsToStructpb(outputs map[string]*states.OutputValue) (*structpb.Struct, error) {
	output := &structpb.Struct{
		Fields: map[string]*structpb.Value{},
	}

	for key, value := range outputs {
		jsonBytes, err := ctyjson.Marshal(value.Value, value.Value.Type())
		if err != nil {
			return nil, errors.Wrap(err, "error marshaling cty to JSON")
		}

		var actual interface{}
		if err := json.Unmarshal(jsonBytes, &actual); err != nil {
			return nil, errors.Wrap(err, "error unmarshaling JSON")
		}

		val, err := plugin.MarshalPropertyValue(resource.NewPropertyValue(actual), plugin.MarshalOptions{
			SkipNulls: true,
		})
		if err != nil {
			return nil, errors.Wrap(err, "error marshalling PropertyValue")
		}

		output.GetFields()[key] = val
	}

	return output, nil
}
