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
// limitations under the License.

package tfbridge

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	ctymsgpack "github.com/zclconf/go-cty/cty/msgpack"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/configs/configschema"
	opentofuconvert "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/plans/objchange"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/terraform-plugin-go/tfprotov6/toproto"
)

// detectPFInconsistentApplyWithTerraform uses OpenTofu's vendored AssertObjectCompatible
// to detect inconsistencies between planned and actual states for Plugin Framework providers.
//
// This function uses Terraform's exact logic for consistency checking, replacing the custom
// detection logic with the authoritative implementation.
func detectPFInconsistentApplyWithTerraform(
	ctx context.Context,
	resourceType string,
	plannedState *tfprotov6.DynamicValue,
	actualState *tfprotov6.DynamicValue,
	schema runtypes.Schema,
) error {
	// Skip if inconsistency detection is disabled for this resource
	if !tfbridge.ShouldDetectInconsistentApply(resourceType) {
		return nil
	}

	if plannedState == nil || actualState == nil || schema == nil {
		return nil
	}

	// Get the resource type
	tfType := schema.Type(ctx)

	// Convert tftypes.Type to cty.Type
	ctyType, err := convertTfTypeToCtyType(tfType)
	if err != nil {
		return fmt.Errorf("error converting type: %w", err)
	}

	// Create conversion helper
	conv := &pfConversion{
		tfType:  tfType,
		ctyType: ctyType,
	}

	// Convert planned state to cty.Value
	plannedValue, err := conv.toCtyValue(plannedState)
	if err != nil {
		return fmt.Errorf("error converting planned state: %w", err)
	}

	// Convert actual state to cty.Value
	actualValue, err := conv.toCtyValue(actualState)
	if err != nil {
		return fmt.Errorf("error converting actual state: %w", err)
	}

	// Convert schema to configschema.Block
	schemaBlock, err := convertSchemaToBlock(ctx, schema)
	if err != nil {
		return fmt.Errorf("error converting schema: %w", err)
	}

	// Use OpenTofu's vendored consistency check
	errs := objchange.AssertObjectCompatible(
		schemaBlock,
		plannedValue,
		actualValue,
	)

	if len(errs) > 0 {
		// Format and log the inconsistency using the common formatter
		msg := tfbridge.FormatTerraformStyleInconsistency(resourceType, errs)
		logger := tfbridge.GetLogger(ctx)
		logger.Warn(msg)
	}

	return nil
}

// pfConversion handles conversions between tftypes.Value and cty.Value for PF providers
type pfConversion struct {
	tfType  tftypes.Type
	ctyType cty.Type
}

// toCtyValue converts a tfprotov6.DynamicValue to cty.Value
func (c *pfConversion) toCtyValue(dv *tfprotov6.DynamicValue) (cty.Value, error) {
	if dv == nil {
		return cty.NullVal(c.ctyType), nil
	}

	// Prefer msgpack for efficiency
	if dv.MsgPack != nil {
		return ctymsgpack.Unmarshal(dv.MsgPack, c.ctyType)
	}

	// Fall back to JSON
	if dv.JSON != nil {
		return ctyjson.Unmarshal(dv.JSON, c.ctyType)
	}

	return cty.NilVal, fmt.Errorf("DynamicValue has neither MsgPack nor JSON data")
}

// convertTfTypeToCtyType converts a tftypes.Type to cty.Type
func convertTfTypeToCtyType(t tftypes.Type) (cty.Type, error) {
	// tftypes.Type.MarshalJSON() is deprecated but remains the most reliable way
	// to convert tftypes.Type to cty.Type. The Plugin Framework team recommends
	// this approach until a direct conversion API is available.
	//nolint:staticcheck
	ctyTypeJSON, err := t.MarshalJSON()
	if err != nil {
		return cty.NilType, fmt.Errorf("tftypes.Type.MarshalJSON() failed: %w", err)
	}
	return ctyjson.UnmarshalType(ctyTypeJSON)
}

// convertSchemaToBlock converts a runtypes.Schema to configschema.Block
func convertSchemaToBlock(ctx context.Context, schema runtypes.Schema) (*configschema.Block, error) {
	// Get the proto schema from the runtime schema
	protoSchema, err := schema.ResourceProtoSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting proto schema: %w", err)
	}

	if protoSchema == nil {
		return nil, fmt.Errorf("ResourceProtoSchema returned nil")
	}

	// Convert proto schema to OpenTofu proto format
	opentofuProto := toproto.Schema(protoSchema)
	if opentofuProto == nil {
		return nil, fmt.Errorf("toproto.Schema returned nil")
	}

	protoBlock := opentofuProto.GetBlock()
	if protoBlock == nil {
		return nil, fmt.Errorf("proto schema has no block")
	}

	// Convert to configschema.Block using vendored conversion
	block := opentofuconvert.ProtoToConfigSchema(protoBlock)
	if block == nil {
		return nil, fmt.Errorf("ProtoToConfigSchema returned nil")
	}

	return block, nil
}

// detectAndReportPFInconsistenciesWithTerraform detects and reports inconsistencies
// using Terraform's AssertObjectCompatible logic.
// This replaces detectAndReportPFInconsistencies with the Terraform-based implementation.
func detectAndReportPFInconsistenciesWithTerraform(
	ctx context.Context,
	resourceType string,
	plannedState *tfprotov6.DynamicValue,
	actualState *tfprotov6.DynamicValue,
	schema runtypes.Schema,
) error {
	return detectPFInconsistentApplyWithTerraform(ctx, resourceType, plannedState, actualState, schema)
}
