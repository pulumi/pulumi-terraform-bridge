// Copyright 2016-2023, Pulumi Corporation.
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

package testprovider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type privst struct{}

var _ resource.Resource = &privst{}

func newPrivst() resource.Resource {
	return &privst{}
}

func (*privst) schema() rschema.Schema {
	return rschema.Schema{
		Description: `testbridge_privst resource  tests PrivateState support`,
		Attributes: map[string]rschema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"target_private_state": schema.StringAttribute{
				Optional:    true,
				Description: "The value of this attribute is stored in PrivateState",
			},
			"observed_private_state_before": schema.StringAttribute{
				Computed:    true,
				Description: "Exports current PrivateState before the update to help test it",
			},
			"observed_private_state_after": schema.StringAttribute{
				Computed:    true,
				Description: "Exports current PrivateState after the update to help test it",
			},
		},
	}
}

func (e *privst) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_privst"
}

func (e *privst) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = e.schema()
}

func (e *privst) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.State.Raw = req.Plan.Raw.Copy()

	diags0 := resp.State.SetAttribute(ctx, path.Root("observed_private_state_before"), "")
	resp.Diagnostics.Append(diags0...)

	diags1 := e.copyTarget(ctx, req.Plan, resp.Private)
	resp.Diagnostics.Append(diags1...)

	diags2 := e.observe(ctx, "observed_private_state_after", resp.Private, &resp.State)
	resp.Diagnostics.Append(diags2...)

	diags3 := resp.State.SetAttribute(ctx, path.Root("id"), "ID0")
	resp.Diagnostics.Append(diags3...)
}

func (e *privst) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.State.Raw = req.Plan.Raw.Copy()

	diags0 := e.observe(ctx, "observed_private_state_before", resp.Private, &resp.State)
	resp.Diagnostics.Append(diags0...)

	diags1 := e.copyTarget(ctx, req.Plan, resp.Private)
	resp.Diagnostics.Append(diags1...)

	diags2 := e.observe(ctx, "observed_private_state_after", resp.Private, &resp.State)
	resp.Diagnostics.Append(diags2...)
}

type privateStateLike interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func (*privst) copyTarget(ctx context.Context, plan tfsdk.Plan, ps privateStateLike) diag.Diagnostics {
	var diags diag.Diagnostics
	var target string
	diags0 := plan.GetAttribute(ctx, path.Root("target_private_state"), &target)
	diags = append(diags, diags0...)
	if target != "" {
		bytes, err := json.Marshal(target)
		if err != nil {
			ed := diag.NewWarningDiagnostic(err.Error(), "")
			return append(diags, ed)
		}
		diags1 := ps.SetKey(ctx, "priv", bytes)
		diags = append(diags, diags1...)
	}
	return diags
}

func (*privst) observe(ctx context.Context, key string, ps privateStateLike, state *tfsdk.State) diag.Diagnostics {
	var diags diag.Diagnostics
	priv, diags0 := ps.GetKey(ctx, "priv")
	diags = append(diags, diags0...)
	var target string
	if len(priv) == 0 {
		target = ""
	} else if err := json.Unmarshal(priv, &target); err != nil {
		ed := diag.NewWarningDiagnostic(err.Error(), "")
		return append(diags, ed)
	}
	diags1 := state.SetAttribute(ctx, path.Root(key), target)
	return append(diags, diags1...)
}

func (e *privst) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	panic("Read not supported yet")
}

func (e *privst) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	typ := e.schema().Type().TerraformType(ctx)
	resp.State = tfsdk.State{
		Raw:    tftypes.NewValue(typ, nil),
		Schema: e.schema(),
	}
}
