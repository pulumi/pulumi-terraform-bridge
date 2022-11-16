// Copyright 2016-2022, Pulumi Corporation.
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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type idExtractor = func(tftypes.Value) (string, error)

// Unlike sdk-v3, Plugin Framework does not have a type-safe way to extract the resource ID from state as needed by
// Create and Read methods, and examples seem to rely on a convention of having an "id" attribute of type String
// present. This code encapsulates extracting ID and all the error handling. It tries to fail early at the static level
// if a schema for a resource does not specify the expected ID field, but may also fail later at runtime if the data
// does not have an ID or it is of the wrong type.
func newIdExtractor(ctx context.Context, typeName string, schema tfsdk.Schema) (idExtractor, error) {
	idPath := path.Root("id")
	idAttrPath := tftypes.NewAttributePath().WithAttributeName("id")

	idAttr, diags := schema.AttributeAtPath(ctx, idPath)
	if diags.HasError() {
		msg := "Cannot bridge Terraform resource %q to Pulumi: " +
			"schema does not define the required %q attribute"
		return nil, fmt.Errorf(msg, typeName, "id")
	}

	idAttrType := idAttr.FrameworkType().TerraformType(ctx)
	if !idAttrType.Is(tftypes.String) {
		msg := "Cannot bridge Terraform resource %q to Pulumi: " +
			"the %q attribute has type %s but only %s is supported"
		return nil, fmt.Errorf(msg, typeName, "id", idAttrType.String(), tftypes.String.String())
	}

	extractor := func(state tftypes.Value) (string, error) {
		idValue, gotIdValue, err := valueAtPath(idAttrPath, state)
		if err != nil {
			return "", fmt.Errorf(
				"Cannot extract ID from %q resource state: %w", typeName, err)
		}
		if !gotIdValue {
			return "", fmt.Errorf(
				"Cannot extract ID from %q resource state: ID attribute is missing", typeName)
		}
		var idString string
		if err := idValue.As(&idString); err != nil {
			return "", fmt.Errorf(
				"Cannot extract ID from %q resource state, expecting a string: %w", typeName, err)
		}
		if idString == "" {
			return "", fmt.Errorf(
				"Cannot extract ID from %q resource state: ID cannot be empty", typeName)
		}
		return idString, nil
	}

	return extractor, nil
}
