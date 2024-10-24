// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/terraform-providers/terraform-provider-random/internal/diagnostics"
)

var (
	_ resource.Resource                = (*bytesResource)(nil)
	_ resource.ResourceWithImportState = (*bytesResource)(nil)
)

func NewBytesResource() resource.Resource {
	return &bytesResource{}
}

type bytesResource struct {
}

func (r *bytesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bytes"
}

func (r *bytesResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = bytesSchemaV0()
}

func (r *bytesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan bytesModelV0

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	bytes := make([]byte, plan.Length.ValueInt64())
	_, err := rand.Read(bytes)
	if err != nil {
		resp.Diagnostics.AddError(
			"Create Random bytes error",
			"There was an error during random generation.\n\n"+
				diagnostics.RetryMsg+
				fmt.Sprintf("Original Error: %s", err),
		)
		return
	}

	u := &bytesModelV0{
		Length:  plan.Length,
		Base64:  types.StringValue(base64.StdEncoding.EncodeToString(bytes)),
		Hex:     types.StringValue(hex.EncodeToString(bytes)),
		Keepers: plan.Keepers,
	}

	diags = resp.State.Set(ctx, u)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read does not need to perform any operations as the state in ReadResourceResponse is already populated.
func (r *bytesResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

func (r *bytesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model bytesModelV0

	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Delete does not need to explicitly call resp.State.RemoveResource() as this is automatically handled by the
// [framework](https://github.com/hashicorp/terraform-plugin-framework/pull/301).
func (r *bytesResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func (r *bytesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	bytes, err := base64.StdEncoding.DecodeString(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Import Random bytes Error",
			"There was an error during the parsing of the base64 string.\n\n"+
				diagnostics.RetryMsg+
				fmt.Sprintf("Original Error: %s", err),
		)
		return
	}

	var state bytesModelV0

	state.Length = types.Int64Value(int64(len(bytes)))
	state.Base64 = types.StringValue(req.ID)
	state.Hex = types.StringValue(hex.EncodeToString(bytes))
	state.Keepers = types.MapNull(types.StringType)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

type bytesModelV0 struct {
	Length  types.Int64  `tfsdk:"length"`
	Keepers types.Map    `tfsdk:"keepers"`
	Base64  types.String `tfsdk:"base64"`
	Hex     types.String `tfsdk:"hex"`
}

func bytesSchemaV0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Description: "The resource `random_bytes` generates random bytes that are intended to be " +
			"used as a secret, or key. Use this in preference to `random_id` when the output is " +
			"considered sensitive, and should not be displayed in the CLI.",
		Attributes: map[string]schema.Attribute{
			"keepers": schema.MapAttribute{
				Description: "Arbitrary map of values that, when changed, will trigger recreation of " +
					"resource. See [the main provider documentation](../index.html) for more information.",
				ElementType: types.StringType,
				Optional:    true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"length": schema.Int64Attribute{
				Description: "The number of bytes requested. The minimum value for length is 1.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"base64": schema.StringAttribute{
				Description: "The generated bytes presented in base64 string format.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"hex": schema.StringAttribute{
				Description: "The generated bytes presented in hex string format.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
