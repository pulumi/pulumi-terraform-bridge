package main

import (
	"context"
	"fmt"
	"math/big"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/numberplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &resourceValidateInputs{}
	_ resource.ResourceWithImportState = &resourceValidateInputs{}
)

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
			"attr_string_default": schema.StringAttribute{
				Computed: true,
				Optional: true,
				Default:  stringdefault.StaticString("default-value"),
			},
			"attr_string_default_overridden": schema.StringAttribute{
				Computed: true,
				Optional: true,
				Default:  stringdefault.StaticString("should-be-overridden"),
			},
			"attr_string_write_only": schema.StringAttribute{
				Optional:            true,
				WriteOnly:           true,
				MarkdownDescription: "A write-only attribute that is not stored in state",
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

type data struct {
	SR types.String `tfsdk:"attr_string_required"`
	BR types.Bool   `tfsdk:"attr_bool_required"`
	IR types.Int64  `tfsdk:"attr_int_required"`
	NR types.Number `tfsdk:"attr_number_required"`

	SC types.String `tfsdk:"attr_string_computed"`
	BC types.Bool   `tfsdk:"attr_bool_computed"`
	IC types.Int64  `tfsdk:"attr_int_computed"`
	NC types.Number `tfsdk:"attr_number_computed"`

	SD  types.String `tfsdk:"attr_string_default"`
	SDO types.String `tfsdk:"attr_string_default_overridden"`

	WriteOnly types.String `tfsdk:"attr_string_write_only"`

	ID types.String `tfsdk:"id"`
}

func (r *resourceValidateInputs) Create(
	ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse,
) {
	var data data
	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	check{&resp.Diagnostics}.inputAttributes(data)

	data.SC = types.StringValue("t") // "t" is after "s"
	data.BC = types.BoolValue(false)
	data.IC = types.Int64Value(128)
	data.NC = types.NumberValue(big.NewFloat(12.3456))

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	data.ID = types.StringValue("example-id")

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *resourceValidateInputs) Read(
	ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse,
) {
	var data data
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ID.ValueString() == "imported" {
		check{&resp.Diagnostics}.equal(path.Root("attr_string_required"),
			data.SR.ValueString(), "imported value")
	} else {
		check{&resp.Diagnostics}.inputAttributes(data)
	}

	data.IC = types.Int64Value(10 * 1000)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *resourceValidateInputs) Update(
	ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse,
) {
	var data data
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	check := check{&resp.Diagnostics}
	check.inputAttributes(data)
	var b bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("attr_bool_required"), &b)...)
	check.equal(path.Root("attr_bool_required"), b, false)

	data.IC = types.Int64Value(256)
	data.SC = types.StringNull()

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type check struct{ *diag.Diagnostics }

func (c check) equal(path path.Path, actual, expected any) {
	if expected != actual {
		c.AddAttributeError(path, fmt.Sprintf("%#v != %#v", expected, actual), "test validation failed")
	}
}

func (c check) inputAttributes(data data) {
	// Required attributes
	c.equal(path.Root("attr_string_required"), data.SR.ValueString(), "s")
	c.equal(path.Root("attr_bool_required"), data.BR.ValueBool(), true)
	c.equal(path.Root("attr_int_required"), data.IR.ValueInt64(), int64(64))
	f, _ := data.NR.ValueBigFloat().Float64()
	c.equal(path.Root("attr_int_required"), f, 12.3456)

	// Default attributes
	c.equal(path.Root("attr_string_default"), data.SD.ValueString(), "default-value")
	c.equal(path.Root("attr_string_default_overridden"), data.SDO.ValueString(), "overridden")
}

func (r *resourceValidateInputs) Delete(
	ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse,
) {
	var data data
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c := check{&resp.Diagnostics}
	c.inputAttributes(data)

	c.equal(path.Root("attr_string_computed"), data.SC.ValueString(), "t")
}

func (r *resourceValidateInputs) ImportState(
	ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse,
) {
	var data data

	data.BR = types.BoolValue(true)
	data.IR = types.Int64Value(1234)
	data.SR = types.StringValue("imported value")
	data.NR = types.NumberValue(big.NewFloat(43.21))
	data.ID = types.StringValue("imported")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
