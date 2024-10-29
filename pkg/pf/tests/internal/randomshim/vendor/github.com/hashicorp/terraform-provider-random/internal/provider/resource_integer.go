// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	mapplanmodifiers "github.com/terraform-providers/terraform-provider-random/internal/planmodifiers/map"
	"github.com/terraform-providers/terraform-provider-random/internal/random"
)

var (
	_ resource.Resource                = (*integerResource)(nil)
	_ resource.ResourceWithImportState = (*integerResource)(nil)
)

func NewIntegerResource() resource.Resource {
	return &integerResource{}
}

type integerResource struct{}

func (r *integerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integer"
}

func (r *integerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The resource `random_integer` generates random values from a given range, described " +
			"by the `min` and `max` attributes of a given resource.\n" +
			"\n" +
			"This resource can be used in conjunction with resources that have the `create_before_destroy` " +
			"lifecycle flag set, to avoid conflicts with unique names during the brief period where both the " +
			"old and new resources exist concurrently.",
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
			"min": schema.Int64Attribute{
				Description: "The minimum inclusive value of the range.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"max": schema.Int64Attribute{
				Description: "The maximum inclusive value of the range.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"seed": schema.StringAttribute{
				Description: "A custom seed to always produce the same value.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"result": schema.Int64Attribute{
				Description: "The random integer result.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Description: "The string representation of the integer result.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *integerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan integerModelV0

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	max := int(plan.Max.ValueInt64())
	min := int(plan.Min.ValueInt64())
	seed := plan.Seed.ValueString()

	if max < min {
		resp.Diagnostics.AddError(
			"Create Random Integer Error",
			"The minimum (min) value needs to be smaller than or equal to maximum (max) value.",
		)
		return
	}

	rand := random.NewRand(seed)
	number := rand.Intn((max+1)-min) + min

	u := &integerModelV0{
		ID:      types.StringValue(strconv.Itoa(number)),
		Keepers: plan.Keepers,
		Min:     types.Int64Value(int64(min)),
		Max:     types.Int64Value(int64(max)),
		Result:  types.Int64Value(int64(number)),
	}

	if seed != "" {
		u.Seed = types.StringValue(seed)
	} else {
		u.Seed = types.StringNull()
	}

	diags = resp.State.Set(ctx, u)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read does not need to perform any operations as the state in ReadResourceResponse is already populated.
func (r *integerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

// Update ensures the plan value is copied to the state to complete the update.
func (r *integerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model integerModelV0

	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Delete does not need to explicitly call resp.State.RemoveResource() as this is automatically handled by the
// [framework](https://github.com/hashicorp/terraform-plugin-framework/pull/301).
func (r *integerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *integerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ",")
	if len(parts) != 3 && len(parts) != 4 {
		resp.Diagnostics.AddError(
			"Import Random Integer Error",
			"Invalid import usage: expecting {result},{min},{max} or {result},{min},{max},{seed}",
		)
		return
	}

	result, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Import Random Integer Error",
			"The value supplied could not be parsed as an integer.\n\n"+
				fmt.Sprintf("Original Error: %s", err),
		)
		return
	}

	min, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Import Random Integer Error",
			"The min value supplied could not be parsed as an integer.\n\n"+
				fmt.Sprintf("Original Error: %s", err),
		)
		return
	}

	max, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Import Random Integer Error",
			"The max value supplied could not be parsed as an integer.\n\n"+
				fmt.Sprintf("Original Error: %s", err),
		)
		return
	}

	var state integerModelV0

	state.ID = types.StringValue(parts[0])
	// Using types.MapValueMust to ensure map is known.
	state.Keepers = types.MapValueMust(types.StringType, nil)
	state.Result = types.Int64Value(result)
	state.Min = types.Int64Value(min)
	state.Max = types.Int64Value(max)

	if len(parts) == 4 {
		state.Seed = types.StringValue(parts[3])
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

type integerModelV0 struct {
	ID      types.String `tfsdk:"id"`
	Keepers types.Map    `tfsdk:"keepers"`
	Min     types.Int64  `tfsdk:"min"`
	Max     types.Int64  `tfsdk:"max"`
	Seed    types.String `tfsdk:"seed"`
	Result  types.Int64  `tfsdk:"result"`
}
