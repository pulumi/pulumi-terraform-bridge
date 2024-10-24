// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package boolplanmodifiers

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NumberNumericAttributePlanModifier returns a plan modifier that keep the values
// held in number and numeric attributes synchronised.
func NumberNumericAttributePlanModifier() planmodifier.Bool {
	return &numberNumericAttributePlanModifier{}
}

type numberNumericAttributePlanModifier struct {
}

func (d *numberNumericAttributePlanModifier) Description(ctx context.Context) string {
	return "Ensures that number and numeric attributes are kept synchronised."
}

func (d *numberNumericAttributePlanModifier) MarkdownDescription(ctx context.Context) string {
	return d.Description(ctx)
}

func (d *numberNumericAttributePlanModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	numberConfig := types.Bool{}
	diags := req.Config.GetAttribute(ctx, path.Root("number"), &numberConfig)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	numericConfig := types.Bool{}
	diags = req.Config.GetAttribute(ctx, path.Root("numeric"), &numericConfig)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !numberConfig.IsNull() && !numericConfig.IsNull() && (numberConfig.ValueBool() != numericConfig.ValueBool()) {
		resp.Diagnostics.AddError(
			"Number and numeric are both configured with different values",
			"Number is deprecated, use numeric instead",
		)
		return
	}

	// Default to true for both number and numeric when both are null.
	if numberConfig.IsNull() && numericConfig.IsNull() {
		resp.PlanValue = types.BoolValue(true)
		return
	}

	// Default to using value for numeric if number is null.
	if numberConfig.IsNull() && !numericConfig.IsNull() {
		resp.PlanValue = numericConfig
		return
	}

	// Default to using value for number if numeric is null.
	if !numberConfig.IsNull() && numericConfig.IsNull() {
		resp.PlanValue = numberConfig
		return
	}
}
