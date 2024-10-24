// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	mapplanmodifiers "github.com/terraform-providers/terraform-provider-random/internal/planmodifiers/map"
)

var _ resource.Resource = (*petResource)(nil)

func NewPetResource() resource.Resource {
	return &petResource{}
}

type petResource struct{}

func (r *petResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pet"
}

func (r *petResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The resource `random_pet` generates random pet names that are intended to be used as " +
			"unique identifiers for other resources.\n" +
			"\n" +
			"This resource can be used in conjunction with resources that have the `create_before_destroy` " +
			"lifecycle flag set, to avoid conflicts with unique names during the brief period where both the old " +
			"and new resources exist concurrently.",
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
				Description: "The length (in words) of the pet name. Defaults to 2",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(2),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"prefix": schema.StringAttribute{
				Description: "A string to prefix the name with.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"separator": schema.StringAttribute{
				Description: "The character to separate words in the pet name. Defaults to \"-\"",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("-"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Description: "The random pet name.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *petResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// This is necessary to ensure each call to petname is properly randomised:
	// the library uses `rand.Intn()` and does NOT seed `rand.Seed()` by default,
	// so this call takes care of that.
	petname.NonDeterministicMode()

	var plan petModelV0

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	length := plan.Length.ValueInt64()
	separator := plan.Separator.ValueString()
	prefix := plan.Prefix.ValueString()

	pet := strings.ToLower(petname.Generate(int(length), separator))

	pn := petModelV0{
		Keepers:   plan.Keepers,
		Length:    types.Int64Value(length),
		Separator: types.StringValue(separator),
	}

	if prefix != "" {
		pet = fmt.Sprintf("%s%s%s", prefix, separator, pet)
		pn.Prefix = types.StringValue(prefix)
	} else {
		pn.Prefix = types.StringNull()
	}

	pn.ID = types.StringValue(pet)

	diags = resp.State.Set(ctx, pn)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read does not need to perform any operations as the state in ReadResourceResponse is already populated.
func (r *petResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

// Update ensures the plan value is copied to the state to complete the update.
func (r *petResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model petModelV0

	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Delete does not need to explicitly call resp.State.RemoveResource() as this is automatically handled by the
// [framework](https://github.com/hashicorp/terraform-plugin-framework/pull/301).
func (r *petResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

type petModelV0 struct {
	ID        types.String `tfsdk:"id"`
	Keepers   types.Map    `tfsdk:"keepers"`
	Length    types.Int64  `tfsdk:"length"`
	Prefix    types.String `tfsdk:"prefix"`
	Separator types.String `tfsdk:"separator"`
}
