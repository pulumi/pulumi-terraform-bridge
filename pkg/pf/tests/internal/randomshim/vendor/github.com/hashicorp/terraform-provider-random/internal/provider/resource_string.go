// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/terraform-providers/terraform-provider-random/internal/diagnostics"
	boolplanmodifiers "github.com/terraform-providers/terraform-provider-random/internal/planmodifiers/bool"
	mapplanmodifiers "github.com/terraform-providers/terraform-provider-random/internal/planmodifiers/map"
	stringplanmodifiers "github.com/terraform-providers/terraform-provider-random/internal/planmodifiers/string"
	"github.com/terraform-providers/terraform-provider-random/internal/random"
)

var (
	_ resource.Resource                 = (*stringResource)(nil)
	_ resource.ResourceWithImportState  = (*stringResource)(nil)
	_ resource.ResourceWithUpgradeState = (*stringResource)(nil)
)

func NewStringResource() resource.Resource {
	return &stringResource{}
}

type stringResource struct{}

func (r *stringResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_string"
}

func (r *stringResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = stringSchemaV3()
}

func (r *stringResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan stringModelV3

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := random.StringParams{
		Length:          plan.Length.ValueInt64(),
		Upper:           plan.Upper.ValueBool(),
		MinUpper:        plan.MinUpper.ValueInt64(),
		Lower:           plan.Lower.ValueBool(),
		MinLower:        plan.MinLower.ValueInt64(),
		Numeric:         plan.Numeric.ValueBool(),
		MinNumeric:      plan.MinNumeric.ValueInt64(),
		Special:         plan.Special.ValueBool(),
		MinSpecial:      plan.MinSpecial.ValueInt64(),
		OverrideSpecial: plan.OverrideSpecial.ValueString(),
	}

	result, err := random.CreateString(params)
	if err != nil {
		resp.Diagnostics.Append(diagnostics.RandomReadError(err.Error())...)
		return
	}

	plan.ID = types.StringValue(string(result))
	plan.Result = types.StringValue(string(result))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read does not need to perform any operations as the state in ReadResourceResponse is already populated.
func (r *stringResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

// Update ensures the plan value is copied to the state to complete the update.
func (r *stringResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model stringModelV3

	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Delete does not need to explicitly call resp.State.RemoveResource() as this is automatically handled by the
// [framework](https://github.com/hashicorp/terraform-plugin-framework/pull/301).
func (r *stringResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *stringResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID

	state := stringModelV3{
		ID:              types.StringValue(id),
		Result:          types.StringValue(id),
		Length:          types.Int64Value(int64(len(id))),
		Special:         types.BoolValue(true),
		Upper:           types.BoolValue(true),
		Lower:           types.BoolValue(true),
		Number:          types.BoolValue(true),
		Numeric:         types.BoolValue(true),
		MinSpecial:      types.Int64Value(0),
		MinUpper:        types.Int64Value(0),
		MinLower:        types.Int64Value(0),
		MinNumeric:      types.Int64Value(0),
		OverrideSpecial: types.StringNull(),
		Keepers:         types.MapNull(types.StringType),
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *stringResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	schemaV1 := stringSchemaV1()
	schemaV2 := stringSchemaV2()

	return map[int64]resource.StateUpgrader{
		1: {
			PriorSchema:   &schemaV1,
			StateUpgrader: upgradeStringStateV1toV3,
		},
		2: {
			PriorSchema:   &schemaV2,
			StateUpgrader: upgradeStringStateV2toV3,
		},
	}
}

func upgradeStringStateV1toV3(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	type modelV1 struct {
		ID              types.String `tfsdk:"id"`
		Keepers         types.Map    `tfsdk:"keepers"`
		Length          types.Int64  `tfsdk:"length"`
		Special         types.Bool   `tfsdk:"special"`
		Upper           types.Bool   `tfsdk:"upper"`
		Lower           types.Bool   `tfsdk:"lower"`
		Number          types.Bool   `tfsdk:"number"`
		MinNumeric      types.Int64  `tfsdk:"min_numeric"`
		MinUpper        types.Int64  `tfsdk:"min_upper"`
		MinLower        types.Int64  `tfsdk:"min_lower"`
		MinSpecial      types.Int64  `tfsdk:"min_special"`
		OverrideSpecial types.String `tfsdk:"override_special"`
		Result          types.String `tfsdk:"result"`
	}

	var stringDataV1 modelV1

	resp.Diagnostics.Append(req.State.Get(ctx, &stringDataV1)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Setting fields that can contain null to non-null to prevent forced replacement.
	// This can occur in cases where import has been used in provider versions v3.3.1 and earlier.
	// If import has been used with v3.3.1, for instance then length, lower, number, special, upper,
	// min_lower, min_numeric, min_special and min_upper attributes will all be null in state.
	length := stringDataV1.Length

	if length.IsNull() {
		length = types.Int64Value(int64(len(stringDataV1.Result.ValueString())))
	}

	minNumeric := stringDataV1.MinNumeric

	if minNumeric.IsNull() {
		minNumeric = types.Int64Value(0)
	}

	minUpper := stringDataV1.MinUpper

	if minUpper.IsNull() {
		minUpper = types.Int64Value(0)
	}

	minLower := stringDataV1.MinLower

	if minLower.IsNull() {
		minLower = types.Int64Value(0)
	}

	minSpecial := stringDataV1.MinSpecial

	if minSpecial.IsNull() {
		minSpecial = types.Int64Value(0)
	}

	special := stringDataV1.Special

	if special.IsNull() {
		special = types.BoolValue(true)
	}

	upper := stringDataV1.Upper

	if upper.IsNull() {
		upper = types.BoolValue(true)
	}

	lower := stringDataV1.Lower

	if lower.IsNull() {
		lower = types.BoolValue(true)
	}

	number := stringDataV1.Number

	if number.IsNull() {
		number = types.BoolValue(true)
	}

	stringDataV3 := stringModelV3{
		Keepers:         stringDataV1.Keepers,
		Length:          length,
		Special:         special,
		Upper:           upper,
		Lower:           lower,
		Number:          number,
		Numeric:         number,
		MinNumeric:      minNumeric,
		MinUpper:        minUpper,
		MinLower:        minLower,
		MinSpecial:      minSpecial,
		OverrideSpecial: stringDataV1.OverrideSpecial,
		Result:          stringDataV1.Result,
		ID:              stringDataV1.ID,
	}

	diags := resp.State.Set(ctx, stringDataV3)
	resp.Diagnostics.Append(diags...)
}

func upgradeStringStateV2toV3(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	type modelV2 struct {
		ID              types.String `tfsdk:"id"`
		Keepers         types.Map    `tfsdk:"keepers"`
		Length          types.Int64  `tfsdk:"length"`
		Special         types.Bool   `tfsdk:"special"`
		Upper           types.Bool   `tfsdk:"upper"`
		Lower           types.Bool   `tfsdk:"lower"`
		Number          types.Bool   `tfsdk:"number"`
		Numeric         types.Bool   `tfsdk:"numeric"`
		MinNumeric      types.Int64  `tfsdk:"min_numeric"`
		MinUpper        types.Int64  `tfsdk:"min_upper"`
		MinLower        types.Int64  `tfsdk:"min_lower"`
		MinSpecial      types.Int64  `tfsdk:"min_special"`
		OverrideSpecial types.String `tfsdk:"override_special"`
		Result          types.String `tfsdk:"result"`
	}

	var stringDataV2 modelV2

	resp.Diagnostics.Append(req.State.Get(ctx, &stringDataV2)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Setting fields that can contain null to non-null to prevent forced replacement.
	// This can occur in cases where import has been used in provider versions v3.3.1 and earlier.
	// If import has been used with v3.3.1, for instance then length, lower, number, special, upper,
	// min_lower, min_numeric, min_special and min_upper attributes will all be null in state.
	length := stringDataV2.Length

	if length.IsNull() {
		length = types.Int64Value(int64(len(stringDataV2.Result.ValueString())))
	}

	minNumeric := stringDataV2.MinNumeric

	if minNumeric.IsNull() {
		minNumeric = types.Int64Value(0)
	}

	minUpper := stringDataV2.MinUpper

	if minUpper.IsNull() {
		minUpper = types.Int64Value(0)
	}

	minLower := stringDataV2.MinLower

	if minLower.IsNull() {
		minLower = types.Int64Value(0)
	}

	minSpecial := stringDataV2.MinSpecial

	if minSpecial.IsNull() {
		minSpecial = types.Int64Value(0)
	}

	special := stringDataV2.Special

	if special.IsNull() {
		special = types.BoolValue(true)
	}

	upper := stringDataV2.Upper

	if upper.IsNull() {
		upper = types.BoolValue(true)
	}

	lower := stringDataV2.Lower

	if lower.IsNull() {
		lower = types.BoolValue(true)
	}

	number := stringDataV2.Number

	if number.IsNull() {
		number = types.BoolValue(true)
	}

	stringDataV3 := stringModelV3{
		Keepers:         stringDataV2.Keepers,
		Length:          length,
		Special:         special,
		Upper:           upper,
		Lower:           lower,
		Number:          number,
		Numeric:         number,
		MinNumeric:      minNumeric,
		MinUpper:        minUpper,
		MinLower:        minLower,
		MinSpecial:      minSpecial,
		OverrideSpecial: stringDataV2.OverrideSpecial,
		Result:          stringDataV2.Result,
		ID:              stringDataV2.ID,
	}

	diags := resp.State.Set(ctx, stringDataV3)
	resp.Diagnostics.Append(diags...)
}

func stringSchemaV3() schema.Schema {
	return schema.Schema{
		Version: 2,
		Description: "The resource `random_string` generates a random permutation of alphanumeric " +
			"characters and optionally special characters.\n" +
			"\n" +
			"This resource *does* use a cryptographic random number generator.\n" +
			"\n" +
			"Historically this resource's intended usage has been ambiguous as the original example used " +
			"it in a password. For backwards compatibility it will continue to exist. For unique ids please " +
			"use [random_id](id.html), for sensitive random values please use [random_password](password.html).",
		Attributes: map[string]schema.Attribute{
			"keepers": schema.MapAttribute{
				Description: "Arbitrary map of values that, when changed, will trigger recreation of " +
					"resource. See [the main provider documentation](../index.html) for more information.",
				ElementType: types.StringType,
				Optional:    true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifiers.RequiresReplaceIfValuesNotNull(),
				},
			},

			"length": schema.Int64Attribute{
				Description: "The length of the string desired. The minimum value for length is 1 and, length " +
					"must also be >= (`min_upper` + `min_lower` + `min_numeric` + `min_special`).",
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
					int64validator.AtLeastSumOf(
						path.MatchRoot("min_upper"),
						path.MatchRoot("min_lower"),
						path.MatchRoot("min_numeric"),
						path.MatchRoot("min_special"),
					),
				},
			},

			"special": schema.BoolAttribute{
				Description: "Include special characters in the result. These are `!@#$%&*()-_=+[]{}<>:?`. Default value is `true`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},

			"upper": schema.BoolAttribute{
				Description: "Include uppercase alphabet characters in the result. Default value is `true`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},

			"lower": schema.BoolAttribute{
				Description: "Include lowercase alphabet characters in the result. Default value is `true`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},

			"number": schema.BoolAttribute{
				Description: "Include numeric characters in the result. Default value is `true`. " +
					"**NOTE**: This is deprecated, use `numeric` instead.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifiers.NumberNumericAttributePlanModifier(),
					boolplanmodifier.RequiresReplace(),
				},
				DeprecationMessage: "**NOTE**: This is deprecated, use `numeric` instead.",
			},

			"numeric": schema.BoolAttribute{
				Description: "Include numeric characters in the result. Default value is `true`.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifiers.NumberNumericAttributePlanModifier(),
					boolplanmodifier.RequiresReplace(),
				},
			},

			"min_numeric": schema.Int64Attribute{
				Description: "Minimum number of numeric characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},

			"min_upper": schema.Int64Attribute{
				Description: "Minimum number of uppercase alphabet characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},

			"min_lower": schema.Int64Attribute{
				Description: "Minimum number of lowercase alphabet characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},

			"min_special": schema.Int64Attribute{
				Description: "Minimum number of special characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},

			"override_special": schema.StringAttribute{
				Description: "Supply your own list of special characters to use for string generation.  This " +
					"overrides the default character list in the special argument.  The `special` argument must " +
					"still be set to true for any overwritten characters to be used in generation.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(
						stringplanmodifiers.RequiresReplaceUnlessEmptyStringToNull(),
						"Replace on modification unless updating from empty string (\"\") to null.",
						"Replace on modification unless updating from empty string (`\"\"`) to `null`.",
					),
				},
			},

			"result": schema.StringAttribute{
				Description: "The generated random string.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"id": schema.StringAttribute{
				Description: "The generated random string.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func stringSchemaV2() schema.Schema {
	return schema.Schema{
		Version: 2,
		Description: "The resource `random_string` generates a random permutation of alphanumeric " +
			"characters and optionally special characters.\n" +
			"\n" +
			"This resource *does* use a cryptographic random number generator.\n" +
			"\n" +
			"Historically this resource's intended usage has been ambiguous as the original example used " +
			"it in a password. For backwards compatibility it will continue to exist. For unique ids please " +
			"use [random_id](id.html), for sensitive random values please use [random_password](password.html).",
		Attributes: map[string]schema.Attribute{
			"keepers": schema.MapAttribute{
				Description: "Arbitrary map of values that, when changed, will trigger recreation of " +
					"resource. See [the main provider documentation](../index.html) for more information.",
				ElementType: types.StringType,
				Optional:    true,
			},

			"length": schema.Int64Attribute{
				Description: "The length of the string desired. The minimum value for length is 1 and, length " +
					"must also be >= (`min_upper` + `min_lower` + `min_numeric` + `min_special`).",
				Required: true,
			},

			"special": schema.BoolAttribute{
				Description: "Include special characters in the result. These are `!@#$%&*()-_=+[]{}<>:?`. Default value is `true`.",
				Optional:    true,
				Computed:    true,
			},

			"upper": schema.BoolAttribute{
				Description: "Include uppercase alphabet characters in the result. Default value is `true`.",
				Optional:    true,
				Computed:    true,
			},

			"lower": schema.BoolAttribute{
				Description: "Include lowercase alphabet characters in the result. Default value is `true`.",
				Optional:    true,
				Computed:    true,
			},

			"number": schema.BoolAttribute{
				Description: "Include numeric characters in the result. Default value is `true`. " +
					"**NOTE**: This is deprecated, use `numeric` instead.",
				Optional:           true,
				Computed:           true,
				DeprecationMessage: "**NOTE**: This is deprecated, use `numeric` instead.",
			},

			"numeric": schema.BoolAttribute{
				Description: "Include numeric characters in the result. Default value is `true`.",
				Optional:    true,
				Computed:    true,
			},

			"min_numeric": schema.Int64Attribute{
				Description: "Minimum number of numeric characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
			},

			"min_upper": schema.Int64Attribute{
				Description: "Minimum number of uppercase alphabet characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
			},

			"min_lower": schema.Int64Attribute{
				Description: "Minimum number of lowercase alphabet characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
			},

			"min_special": schema.Int64Attribute{
				Description: "Minimum number of special characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
			},

			"override_special": schema.StringAttribute{
				Description: "Supply your own list of special characters to use for string generation.  This " +
					"overrides the default character list in the special argument.  The `special` argument must " +
					"still be set to true for any overwritten characters to be used in generation.",
				Optional: true,
				Computed: true,
			},

			"result": schema.StringAttribute{
				Description: "The generated random string.",
				Computed:    true,
			},

			"id": schema.StringAttribute{
				Description: "The generated random string.",
				Computed:    true,
			},
		},
	}
}

func stringSchemaV1() schema.Schema {
	return schema.Schema{
		Version: 1,
		Description: "The resource `random_string` generates a random permutation of alphanumeric " +
			"characters and optionally special characters.\n" +
			"\n" +
			"This resource *does* use a cryptographic random number generator.\n" +
			"\n" +
			"Historically this resource's intended usage has been ambiguous as the original example used " +
			"it in a password. For backwards compatibility it will continue to exist. For unique ids please " +
			"use [random_id](id.html), for sensitive random values please use [random_password](password.html).",
		Attributes: map[string]schema.Attribute{
			"keepers": schema.MapAttribute{
				Description: "Arbitrary map of values that, when changed, will trigger recreation of " +
					"resource. See [the main provider documentation](../index.html) for more information.",
				ElementType: types.StringType,
				Optional:    true,
			},

			"length": schema.Int64Attribute{
				Description: "The length of the string desired. The minimum value for length is 1 and, length " +
					"must also be >= (`min_upper` + `min_lower` + `min_numeric` + `min_special`).",
				Required: true,
			},

			"special": schema.BoolAttribute{
				Description: "Include special characters in the result. These are `!@#$%&*()-_=+[]{}<>:?`. Default value is `true`.",
				Optional:    true,
				Computed:    true,
			},

			"upper": schema.BoolAttribute{
				Description: "Include uppercase alphabet characters in the result. Default value is `true`.",
				Optional:    true,
				Computed:    true,
			},

			"lower": schema.BoolAttribute{
				Description: "Include lowercase alphabet characters in the result. Default value is `true`.",
				Optional:    true,
				Computed:    true,
			},

			"number": schema.BoolAttribute{
				Description: "Include numeric characters in the result. Default value is `true`.",
				Optional:    true,
				Computed:    true,
			},
			"min_numeric": schema.Int64Attribute{
				Description: "Minimum number of numeric characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
			},

			"min_upper": schema.Int64Attribute{
				Description: "Minimum number of uppercase alphabet characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
			},

			"min_lower": schema.Int64Attribute{
				Description: "Minimum number of lowercase alphabet characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
			},

			"min_special": schema.Int64Attribute{
				Description: "Minimum number of special characters in the result. Default value is `0`.",
				Optional:    true,
				Computed:    true,
			},

			"override_special": schema.StringAttribute{
				Description: "Supply your own list of special characters to use for string generation.  This " +
					"overrides the default character list in the special argument.  The `special` argument must " +
					"still be set to true for any overwritten characters to be used in generation.",
				Optional: true,
				Computed: true,
			},

			"result": schema.StringAttribute{
				Description: "The generated random string.",
				Computed:    true,
			},

			"id": schema.StringAttribute{
				Description: "The generated random string.",
				Computed:    true,
			},
		},
	}
}

type stringModelV3 struct {
	ID              types.String `tfsdk:"id"`
	Keepers         types.Map    `tfsdk:"keepers"`
	Length          types.Int64  `tfsdk:"length"`
	Special         types.Bool   `tfsdk:"special"`
	Upper           types.Bool   `tfsdk:"upper"`
	Lower           types.Bool   `tfsdk:"lower"`
	Number          types.Bool   `tfsdk:"number"`
	Numeric         types.Bool   `tfsdk:"numeric"`
	MinNumeric      types.Int64  `tfsdk:"min_numeric"`
	MinUpper        types.Int64  `tfsdk:"min_upper"`
	MinLower        types.Int64  `tfsdk:"min_lower"`
	MinSpecial      types.Int64  `tfsdk:"min_special"`
	OverrideSpecial types.String `tfsdk:"override_special"`
	Result          types.String `tfsdk:"result"`
}
