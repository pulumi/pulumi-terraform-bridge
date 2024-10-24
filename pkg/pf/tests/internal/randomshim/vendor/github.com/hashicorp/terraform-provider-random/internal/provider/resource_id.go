// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/terraform-providers/terraform-provider-random/internal/diagnostics"
	mapplanmodifiers "github.com/terraform-providers/terraform-provider-random/internal/planmodifiers/map"
)

var (
	_ resource.Resource                = (*idResource)(nil)
	_ resource.ResourceWithImportState = (*idResource)(nil)
)

func NewIdResource() resource.Resource {
	return &idResource{}
}

type idResource struct{}

func (r *idResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_id"
}

func (r *idResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
The resource ` + "`random_id`" + ` generates random numbers that are intended to be
used as unique identifiers for other resources. If the output is considered 
sensitive, and should not be displayed in the CLI, use ` + "`random_bytes`" + `
instead.

This resource *does* use a cryptographic random number generator in order
to minimize the chance of collisions, making the results of this resource
when a 16-byte identifier is requested of equivalent uniqueness to a
type-4 UUID.

This resource can be used in conjunction with resources that have
the ` + "`create_before_destroy`" + ` lifecycle flag set to avoid conflicts with
unique names during the brief period where both the old and new resources
exist concurrently.
`,
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
			"byte_length": schema.Int64Attribute{
				Description: "The number of random bytes to produce. The minimum value is 1, which produces " +
					"eight bits of randomness.",
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"prefix": schema.StringAttribute{
				Description: "Arbitrary string to prefix the output value with. This string is supplied as-is, " +
					"meaning it is not guaranteed to be URL-safe or base64 encoded.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"b64_url": schema.StringAttribute{
				Description: "The generated id presented in base64, using the URL-friendly character set: " +
					"case-sensitive letters, digits and the characters `_` and `-`.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"b64_std": schema.StringAttribute{
				Description: "The generated id presented in base64 without additional transformations.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"hex": schema.StringAttribute{
				Description: "The generated id presented in padded hexadecimal digits. This result will " +
					"always be twice as long as the requested byte length.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dec": schema.StringAttribute{
				Description: "The generated id presented in non-padded decimal digits.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Description: "The generated id presented in base64 without additional transformations or prefix.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *idResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan idModelV0

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	byteLength := plan.ByteLength.ValueInt64()
	bytes := make([]byte, byteLength)

	n, err := rand.Reader.Read(bytes)
	if int64(n) != byteLength {
		resp.Diagnostics.Append(diagnostics.RandomnessGenerationError(err.Error())...)
		return
	}
	if err != nil {
		resp.Diagnostics.Append(diagnostics.RandomReadError(err.Error())...)
		return
	}

	id := base64.RawURLEncoding.EncodeToString(bytes)
	prefix := plan.Prefix.ValueString()
	b64Std := base64.StdEncoding.EncodeToString(bytes)
	hexStr := hex.EncodeToString(bytes)

	bigInt := big.Int{}
	bigInt.SetBytes(bytes)
	dec := bigInt.String()

	i := idModelV0{
		ID:         types.StringValue(id),
		Keepers:    plan.Keepers,
		ByteLength: types.Int64Value(plan.ByteLength.ValueInt64()),
		Prefix:     plan.Prefix,
		B64URL:     types.StringValue(prefix + id),
		B64Std:     types.StringValue(prefix + b64Std),
		Hex:        types.StringValue(prefix + hexStr),
		Dec:        types.StringValue(prefix + dec),
	}

	diags = resp.State.Set(ctx, i)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read does not need to perform any operations as the state in ReadResourceResponse is already populated.
func (r *idResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update ensures the plan value is copied to the state to complete the update.
func (r *idResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var model idModelV0

	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// Delete does not need to explicitly call resp.State.RemoveResource() as this is automatically handled by the
// [framework](https://github.com/hashicorp/terraform-plugin-framework/pull/301).
func (r *idResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func (r *idResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	var prefix string

	sep := strings.LastIndex(id, ",")
	if sep != -1 {
		prefix = id[:sep]
		id = id[sep+1:]
	}

	bytes, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Import Random ID Error",
			"While attempting to import a random id there was a decoding error.\n\n+"+
				diagnostics.RetryMsg+
				fmt.Sprintf("Original Error: %s", err),
		)
		return
	}

	b64Std := base64.StdEncoding.EncodeToString(bytes)
	hexStr := hex.EncodeToString(bytes)

	bigInt := big.Int{}
	bigInt.SetBytes(bytes)
	dec := bigInt.String()

	var state idModelV0

	state.ID = types.StringValue(id)
	state.ByteLength = types.Int64Value(int64(len(bytes)))
	// Using types.MapValueMust to ensure map is known.
	state.Keepers = types.MapValueMust(types.StringType, nil)
	state.B64Std = types.StringValue(prefix + b64Std)
	state.B64URL = types.StringValue(prefix + id)
	state.Hex = types.StringValue(prefix + hexStr)
	state.Dec = types.StringValue(prefix + dec)

	if prefix == "" {
		state.Prefix = types.StringNull()
	} else {
		state.Prefix = types.StringValue(prefix)
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

type idModelV0 struct {
	ID         types.String `tfsdk:"id"`
	Keepers    types.Map    `tfsdk:"keepers"`
	ByteLength types.Int64  `tfsdk:"byte_length"`
	Prefix     types.String `tfsdk:"prefix"`
	B64URL     types.String `tfsdk:"b64_url"`
	B64Std     types.String `tfsdk:"b64_std"`
	Hex        types.String `tfsdk:"hex"`
	Dec        types.String `tfsdk:"dec"`
}
