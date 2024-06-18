package main

import (
	"context"
	"fmt"
	"math/big"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/numberplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &resourceValidateInputs{}
var _ resource.ResourceWithImportState = &resourceValidateInputs{}

func NewExampleResource() resource.Resource { return &resourceValidateInputs{} }

// resourceValidateInputs defines the resource implementation.
type resourceValidateInputs struct {
	client *http.Client
}

func (r *resourceValidateInputs) Metadata(
	ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_primitive"
}

func (r *resourceValidateInputs) Schema(
	ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Example resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Example identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
	mapInsert(resp.Schema.Attributes,
		primitiveAttributes(mkRequired),
		primitiveAttributes(mkComputed))
}

func mapInsert[K comparable, V any](dst map[K]V, src ...map[K]V) {
	for _, src := range src {
		for k, v := range src {
			dst[k] = v
		}
	}
}

type attrOpts struct {
	computed, optional, required, sensitive bool
}

type attrOpt func(*attrOpts)

func mkComputed(o *attrOpts) { o.computed = true }
func mkRequired(o *attrOpts) { o.required = true }

//nolint:unused
func mkOptional(o *attrOpts) { o.optional = true }

//nolint:unused
func mkSensitive(o *attrOpts) { o.sensitive = true }

func primitiveAttributes(opts ...attrOpt) map[string]schema.Attribute {
	var o attrOpts
	for _, opt := range opts {
		opt(&o)
	}
	name := func(typ string) string {
		name := "attr_" + typ
		if o.computed {
			name += "_computed"
		}
		if o.optional {
			name += "_optional"
		}
		if o.required {
			name += "_required"
		}
		if o.sensitive {
			name += "_sensitive"
		}
		return name
	}
	return map[string]schema.Attribute{
		name("string"): schema.StringAttribute{
			Required:            o.required,
			Optional:            o.optional,
			Computed:            o.computed,
			Sensitive:           o.sensitive,
			MarkdownDescription: "The description for " + name("string"),
			PlanModifiers: when(o.computed, []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			}),
		},
		name("bool"): schema.BoolAttribute{
			Required:            o.required,
			Optional:            o.optional,
			Computed:            o.computed,
			Sensitive:           o.sensitive,
			MarkdownDescription: "The description for " + name("string"),
			PlanModifiers: when(o.computed, []planmodifier.Bool{
				boolplanmodifier.UseStateForUnknown(),
			}),
		},
		name("int"): schema.Int64Attribute{
			Required:            o.required,
			Optional:            o.optional,
			Computed:            o.computed,
			Sensitive:           o.sensitive,
			MarkdownDescription: "The description for " + name("int"),
			PlanModifiers: when(o.computed, []planmodifier.Int64{
				int64planmodifier.UseStateForUnknown(),
			}),
		},
		name("number"): schema.NumberAttribute{
			Required:            o.required,
			Optional:            o.optional,
			Computed:            o.computed,
			Sensitive:           o.sensitive,
			MarkdownDescription: "The description for " + name("number"),
			PlanModifiers: when(o.computed, []planmodifier.Number{
				numberplanmodifier.UseStateForUnknown(),
			}),
		},
	}
}

func when[T any](check bool, value T) T {
	if !check {
		var t T
		return t
	}
	return value
}

func (r *resourceValidateInputs) Configure(
	ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse,
) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*http.Client)

	if !ok {
		details := fmt.Sprintf(
			"Expected *http.Client, got: %T. Please report this issue to the provider developers.",
			req.ProviderData,
		)
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", details)

		return
	}

	r.client = client
}

func (r *resourceValidateInputs) Create(
	ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse,
) {
	var data struct {
		S types.String `tfsdk:"attr_string_required"`
		B types.Bool   `tfsdk:"attr_bool_required"`
		I types.Int64  `tfsdk:"attr_int_required"`
		N types.Number `tfsdk:"attr_number_required"`

		SR types.String `tfsdk:"attr_string_computed"`
		BR types.Bool   `tfsdk:"attr_bool_computed"`
		IR types.Int64  `tfsdk:"attr_int_computed"`
		NR types.Number `tfsdk:"attr_number_computed"`

		ID types.String `tfsdk:"id"`
	}

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.S.ValueString() != "s" {
		resp.Diagnostics.AddError("string_required: s != "+data.S.String(), "test validation failed")
	}

	if !data.B.ValueBool() {
		resp.Diagnostics.AddError("bool_required: true != "+data.B.String(), "test validation failed")
	}

	if data.I.ValueInt64() != 64 {
		resp.Diagnostics.AddError("int_required: 64 != "+data.I.String(), "test validation failed")
	}

	if f, _ := data.N.ValueBigFloat().Float64(); f != 12.3456 {
		resp.Diagnostics.AddError("int_required: 12.3456 != "+data.N.String(), "test validation failed")
	}

	data.SR = types.StringValue("t") // "t" is after "s"
	data.BR = types.BoolValue(false)
	data.IR = types.Int64Value(128)
	data.NR = types.NumberValue(big.NewFloat(12.3456))

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	data.ID = types.StringValue("example-id")

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *resourceValidateInputs) Read(
	ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse,
) {
	resp.Diagnostics.AddError("Not implemented yet", "")
}

func (r *resourceValidateInputs) Update(
	ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse,
) {
	resp.Diagnostics.AddError("Not implemented yet", "")
}

func (r *resourceValidateInputs) Delete(
	ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse,
) {
	resp.Diagnostics.AddError("Not implemented yet", "")
}

func (r *resourceValidateInputs) ImportState(
	ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse,
) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
