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
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/pfutils"
)

type idExtractor struct {
	typeName string
}

// Unlike sdk-v3, Plugin Framework does not have a type-safe way to extract the resource ID from state as needed by
// Create and Read methods, and examples seem to rely on a convention of having an "id" attribute of type String
// present. This code encapsulates extracting ID and all the error handling. It tries to fail early at the static level
// if a schema for a resource does not specify the expected ID field, but may also fail later at runtime if the data
// does not have an ID or it is of the wrong type.
func newIdExtractor(ctx context.Context, typeName string, schema pfutils.Schema) (idExtractor, error) {
	idPath := path.Root("id")

	idAttr, diags := schema.AttributeAtPath(ctx, idPath)
	if diags.HasError() {
		msg := "Cannot bridge Terraform resource %q to Pulumi: " +
			"schema does not define the required %q attribute"
		return idExtractor{}, fmt.Errorf(msg, typeName, "id")
	}

	idAttrType := idAttr.GetType().TerraformType(ctx)
	if !idAttrType.Is(tftypes.String) {
		msg := "Cannot bridge Terraform resource %q to Pulumi: " +
			"the %q attribute has type %s but only %s is supported"
		return idExtractor{}, fmt.Errorf(msg, typeName, "id", idAttrType.String(), tftypes.String.String())
	}

	return idExtractor{typeName: typeName}, nil
}

func (ie idExtractor) extractID(state tftypes.Value) (string, error) {
	typeName := ie.typeName
	idAttrPath := tftypes.NewAttributePath().WithAttributeName("id")
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

// Drills down into a Value. Returns the found value and a flag indicating if it was found or not.
func valueAtPath(p *tftypes.AttributePath, root tftypes.Value) (tftypes.Value, bool, error) {
	result, _, err := tftypes.WalkAttributePath(root, p)
	if err == tftypes.ErrInvalidStep {
		return tftypes.Value{}, false, nil // not found
	}
	if err != nil {
		return tftypes.Value{}, false, err // error
	}
	resultValue, ok := result.(tftypes.Value)
	if !ok {
		return tftypes.Value{}, false, fmt.Errorf(
			"Expected a value of type tftypes.Value but got: %v", result)
	}
	return resultValue, true, nil
}
