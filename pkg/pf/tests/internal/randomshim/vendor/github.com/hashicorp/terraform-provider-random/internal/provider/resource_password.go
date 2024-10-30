// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"

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
	"golang.org/x/crypto/bcrypt"

	"github.com/terraform-providers/terraform-provider-random/internal/diagnostics"
	boolplanmodifiers "github.com/terraform-providers/terraform-provider-random/internal/planmodifiers/bool"
	mapplanmodifiers "github.com/terraform-providers/terraform-provider-random/internal/planmodifiers/map"
	stringplanmodifiers "github.com/terraform-providers/terraform-provider-random/internal/planmodifiers/string"
	"github.com/terraform-providers/terraform-provider-random/internal/random"
)

var (
	_ resource.Resource                 = (*passwordResource)(nil)
	_ resource.ResourceWithImportState  = (*passwordResource)(nil)
	_ resource.ResourceWithUpgradeState = (*passwordResource)(nil)
)

func NewPasswordResource() resource.Resource {
	return &passwordResource{}
}

type passwordResource struct{}

func (r *passwordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_password"
}

func (r *passwordResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = passwordSchemaV3()
}

func (r *passwordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan passwordModelV3

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

	hash, err := generateHash(string(result))
	if err != nil {
		resp.Diagnostics.Append(diagnostics.HashGenerationError(err.Error())...)
	}

	plan.BcryptHash = types.StringValue(hash)
	plan.ID = types.StringValue("none")
	plan.Result = types.StringValue(string(result))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read does not need to perform any operations as the state in ReadResourceResponse is already populated.
func (r *passwordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

// Update ensures the plan value is copied to the state to complete the update.
func (r *passwordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model passwordModelV3

	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Delete does not need to explicitly call resp.State.RemoveResource() as this is automatically handled by the
// [framework](https://github.com/hashicorp/terraform-plugin-framework/pull/301).
func (r *passwordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *passwordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID

	state := passwordModelV3{
		ID:              types.StringValue("none"),
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
		Keepers:         types.MapNull(types.StringType),
		OverrideSpecial: types.StringNull(),
	}

	hash, err := generateHash(id)
	if err != nil {
		resp.Diagnostics.Append(diagnostics.HashGenerationError(err.Error())...)
	}

	state.BcryptHash = types.StringValue(hash)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *passwordResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	schemaV0 := passwordSchemaV0()
	schemaV1 := passwordSchemaV1()
	schemaV2 := passwordSchemaV2()

	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &schemaV0,
			StateUpgrader: upgradePasswordStateV0toV3,
		},
		1: {
			PriorSchema:   &schemaV1,
			StateUpgrader: upgradePasswordStateV1toV3,
		},
		2: {
			PriorSchema:   &schemaV2,
			StateUpgrader: upgradePasswordStateV2toV3,
		},
	}
}

func upgradePasswordStateV0toV3(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	type modelV0 struct {
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

	var passwordDataV0 modelV0

	resp.Diagnostics.Append(req.State.Get(ctx, &passwordDataV0)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Setting fields that can contain null to non-null to prevent forced replacement.
	// This can occur in cases where import has been used in provider versions v3.3.1 and earlier.
	// If import has been used with v3.3.1, for instance then length, lower, number, special, upper,
	// min_lower, min_numeric, min_special and min_upper attributes will all be null in state.
	length := passwordDataV0.Length

	if length.IsNull() {
		length = types.Int64Value(int64(len(passwordDataV0.Result.ValueString())))
	}

	minNumeric := passwordDataV0.MinNumeric

	if minNumeric.IsNull() {
		minNumeric = types.Int64Value(0)
	}

	minUpper := passwordDataV0.MinUpper

	if minUpper.IsNull() {
		minUpper = types.Int64Value(0)
	}

	minLower := passwordDataV0.MinLower

	if minLower.IsNull() {
		minLower = types.Int64Value(0)
	}

	minSpecial := passwordDataV0.MinSpecial

	if minSpecial.IsNull() {
		minSpecial = types.Int64Value(0)
	}

	special := passwordDataV0.Special

	if special.IsNull() {
		special = types.BoolValue(true)
	}

	upper := passwordDataV0.Upper

	if upper.IsNull() {
		upper = types.BoolValue(true)
	}

	lower := passwordDataV0.Lower

	if lower.IsNull() {
		lower = types.BoolValue(true)
	}

	number := passwordDataV0.Number

	if number.IsNull() {
		number = types.BoolValue(true)
	}

	passwordDataV3 := passwordModelV3{
		Keepers:         passwordDataV0.Keepers,
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
		OverrideSpecial: passwordDataV0.OverrideSpecial,
		Result:          passwordDataV0.Result,
		ID:              passwordDataV0.ID,
	}

	hash, err := generateHash(passwordDataV3.Result.ValueString())
	if err != nil {
		resp.Diagnostics.Append(diagnostics.HashGenerationError(err.Error())...)
		return
	}

	passwordDataV3.BcryptHash = types.StringValue(hash)

	diags := resp.State.Set(ctx, passwordDataV3)
	resp.Diagnostics.Append(diags...)
}

func upgradePasswordStateV1toV3(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
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
		BcryptHash      types.String `tfsdk:"bcrypt_hash"`
	}

	var passwordDataV1 modelV1

	resp.Diagnostics.Append(req.State.Get(ctx, &passwordDataV1)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Setting fields that can contain null to non-null to prevent forced replacement.
	// This can occur in cases where import has been used in provider versions v3.3.1 and earlier.
	// If import has been used with v3.3.1, for instance then length, lower, number, special, upper,
	// min_lower, min_numeric, min_special and min_upper attributes will all be null in state.
	length := passwordDataV1.Length

	if length.IsNull() {
		length = types.Int64Value(int64(len(passwordDataV1.Result.ValueString())))
	}

	minNumeric := passwordDataV1.MinNumeric

	if minNumeric.IsNull() {
		minNumeric = types.Int64Value(0)
	}

	minUpper := passwordDataV1.MinUpper

	if minUpper.IsNull() {
		minUpper = types.Int64Value(0)
	}

	minLower := passwordDataV1.MinLower

	if minLower.IsNull() {
		minLower = types.Int64Value(0)
	}

	minSpecial := passwordDataV1.MinSpecial

	if minSpecial.IsNull() {
		minSpecial = types.Int64Value(0)
	}

	special := passwordDataV1.Special

	if special.IsNull() {
		special = types.BoolValue(true)
	}

	upper := passwordDataV1.Upper

	if upper.IsNull() {
		upper = types.BoolValue(true)
	}

	lower := passwordDataV1.Lower

	if lower.IsNull() {
		lower = types.BoolValue(true)
	}

	number := passwordDataV1.Number

	if number.IsNull() {
		number = types.BoolValue(true)
	}

	passwordDataV3 := passwordModelV3{
		Keepers:         passwordDataV1.Keepers,
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
		OverrideSpecial: passwordDataV1.OverrideSpecial,
		BcryptHash:      passwordDataV1.BcryptHash,
		Result:          passwordDataV1.Result,
		ID:              passwordDataV1.ID,
	}

	diags := resp.State.Set(ctx, passwordDataV3)
	resp.Diagnostics.Append(diags...)
}

func upgradePasswordStateV2toV3(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	type passwordModelV2 struct {
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
		BcryptHash      types.String `tfsdk:"bcrypt_hash"`
	}

	var passwordDataV2 passwordModelV2

	resp.Diagnostics.Append(req.State.Get(ctx, &passwordDataV2)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Setting fields that can contain null to non-null to prevent forced replacement.
	// This can occur in cases where import has been used in provider versions v3.3.1 and earlier.
	// If import has been used with v3.3.1, for instance then length, lower, number, special, upper,
	// min_lower, min_numeric, min_special and min_upper attributes will all be null in state.
	length := passwordDataV2.Length

	if length.IsNull() {
		length = types.Int64Value(int64(len(passwordDataV2.Result.ValueString())))
	}

	minNumeric := passwordDataV2.MinNumeric

	if minNumeric.IsNull() {
		minNumeric = types.Int64Value(0)
	}

	minUpper := passwordDataV2.MinUpper

	if minUpper.IsNull() {
		minUpper = types.Int64Value(0)
	}

	minLower := passwordDataV2.MinLower

	if minLower.IsNull() {
		minLower = types.Int64Value(0)
	}

	minSpecial := passwordDataV2.MinSpecial

	if minSpecial.IsNull() {
		minSpecial = types.Int64Value(0)
	}

	special := passwordDataV2.Special

	if special.IsNull() {
		special = types.BoolValue(true)
	}

	upper := passwordDataV2.Upper

	if upper.IsNull() {
		upper = types.BoolValue(true)
	}

	lower := passwordDataV2.Lower

	if lower.IsNull() {
		lower = types.BoolValue(true)
	}

	number := passwordDataV2.Number

	if number.IsNull() {
		number = types.BoolValue(true)
	}

	numeric := passwordDataV2.Number

	if numeric.IsNull() {
		numeric = types.BoolValue(true)
	}

	// Schema version 2 to schema version 3 is a duplicate of the data,
	// however the BcryptHash value may have been incorrectly generated.
	//nolint:gosimple // V3 model will expand over time so all fields are written out to help future code changes.
	passwordDataV3 := passwordModelV3{
		BcryptHash:      passwordDataV2.BcryptHash,
		ID:              passwordDataV2.ID,
		Keepers:         passwordDataV2.Keepers,
		Length:          length,
		Lower:           lower,
		MinLower:        minLower,
		MinNumeric:      minNumeric,
		MinSpecial:      minSpecial,
		MinUpper:        minUpper,
		Number:          number,
		Numeric:         numeric,
		OverrideSpecial: passwordDataV2.OverrideSpecial,
		Result:          passwordDataV2.Result,
		Special:         special,
		Upper:           upper,
	}

	// Set the duplicated data now so we can easily return early below.
	// The BcryptHash value will be adjusted later if it is incorrect.
	resp.Diagnostics.Append(resp.State.Set(ctx, passwordDataV3)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If the BcryptHash value does not correctly verify against the Result
	// value we should regenerate it.
	err := bcrypt.CompareHashAndPassword([]byte(passwordDataV2.BcryptHash.ValueString()), []byte(passwordDataV2.Result.ValueString()))

	// If the hash matched the password, there is nothing to do.
	if err == nil {
		return
	}

	if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		resp.Diagnostics.AddError(
			"Version 3 State Upgrade Error",
			"An unexpected error occurred when comparing the state version 2 password and bcrypt hash. "+
				"This is always an issue in the provider and should be reported to the provider developers.\n\n"+
				"Original Error: "+err.Error(),
		)
		return
	}

	// Regenerate the BcryptHash value.
	newBcryptHash, err := bcrypt.GenerateFromPassword([]byte(passwordDataV2.Result.ValueString()), bcrypt.DefaultCost)

	if err != nil {
		resp.Diagnostics.AddError(
			"Version 3 State Upgrade Error",
			"An unexpected error occurred when generating a new password bcrypt hash. "+
				"Check the error below and ensure the system executing Terraform can properly generate randomness.\n\n"+
				"Original Error: "+err.Error(),
		)
		return
	}

	passwordDataV3.BcryptHash = types.StringValue(string(newBcryptHash))

	resp.Diagnostics.Append(resp.State.Set(ctx, passwordDataV3)...)
}

// generateHash truncates strings that are longer than 72 bytes in
// order to avoid the error returned from bcrypt.GenerateFromPassword
// in versions v0.5.0 and above: https://pkg.go.dev/golang.org/x/crypto@v0.8.0/bcrypt#GenerateFromPassword
func generateHash(toHash string) (string, error) {
	bytesHash := []byte(toHash)
	bytesToHash := bytesHash

	if len(bytesHash) > 72 {
		bytesToHash = bytesHash[:72]
	}

	hash, err := bcrypt.GenerateFromPassword(bytesToHash, bcrypt.DefaultCost)

	return string(hash), err
}

func passwordSchemaV3() schema.Schema {
	return schema.Schema{
		Version: 3,
		Description: "Identical to [random_string](string.html) with the exception that the result is " +
			"treated as sensitive and, thus, _not_ displayed in console output. Read more about sensitive " +
			"data handling in the " +
			"[Terraform documentation](https://www.terraform.io/docs/language/state/sensitive-data.html).\n\n" +
			"This resource *does* use a cryptographic random number generator.",
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
				}},

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
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"bcrypt_hash": schema.StringAttribute{
				Description: "A bcrypt hash of the generated random string. " +
					"**NOTE**: If the generated random string is greater than 72 bytes in length, " +
					"`bcrypt_hash` will contain a hash of the first 72 bytes.",
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"id": schema.StringAttribute{
				Description: "A static value used internally by Terraform, this should not be referenced in configurations.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func passwordSchemaV2() schema.Schema {
	return schema.Schema{
		Version: 2,
		Description: "Identical to [random_string](string.html) with the exception that the result is " +
			"treated as sensitive and, thus, _not_ displayed in console output. Read more about sensitive " +
			"data handling in the " +
			"[Terraform documentation](https://www.terraform.io/docs/language/state/sensitive-data.html).\n\n" +
			"This resource *does* use a cryptographic random number generator.",
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
				Sensitive:   true,
			},

			"bcrypt_hash": schema.StringAttribute{
				Description: "A bcrypt hash of the generated random string. " +
					"**NOTE**: If the generated random string is greater than 72 bytes in length, " +
					"`bcrypt_hash` will contain a hash of the first 72 bytes.",
				Computed:  true,
				Sensitive: true,
			},

			"id": schema.StringAttribute{
				Description: "A static value used internally by Terraform, this should not be referenced in configurations.",
				Computed:    true,
			},
		},
	}
}

func passwordSchemaV1() schema.Schema {
	return schema.Schema{
		Version: 1,
		Description: "Identical to [random_string](string.html) with the exception that the result is " +
			"treated as sensitive and, thus, _not_ displayed in console output. Read more about sensitive " +
			"data handling in the " +
			"[Terraform documentation](https://www.terraform.io/docs/language/state/sensitive-data.html).\n\n" +
			"This resource *does* use a cryptographic random number generator.",
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
				Sensitive:   true,
			},

			"bcrypt_hash": schema.StringAttribute{
				Description: "A bcrypt hash of the generated random string. " +
					"**NOTE**: If the generated random string is greater than 72 bytes in length, " +
					"`bcrypt_hash` will contain a hash of the first 72 bytes.",
				Computed:  true,
				Sensitive: true,
			},

			"id": schema.StringAttribute{
				Description: "A static value used internally by Terraform, this should not be referenced in configurations.",
				Computed:    true,
			},
		},
	}
}

func passwordSchemaV0() schema.Schema {
	return schema.Schema{
		Description: "Identical to [random_string](string.html) with the exception that the result is " +
			"treated as sensitive and, thus, _not_ displayed in console output. Read more about sensitive " +
			"data handling in the " +
			"[Terraform documentation](https://www.terraform.io/docs/language/state/sensitive-data.html).\n\n" +
			"This resource *does* use a cryptographic random number generator.",
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
				Sensitive:   true,
			},

			"id": schema.StringAttribute{
				Description: "A static value used internally by Terraform, this should not be referenced in configurations.",
				Computed:    true,
			},
		},
	}
}

type passwordModelV3 struct {
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
	BcryptHash      types.String `tfsdk:"bcrypt_hash"`
}
