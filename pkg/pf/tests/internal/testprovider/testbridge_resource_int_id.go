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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type intIDRes struct{}

var _ resource.Resource = &intIDRes{}

func newIntIDRes() resource.Resource {
	return &intIDRes{}
}

func (*intIDRes) schema() rschema.Schema {
	return rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": rschema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": rschema.StringAttribute{
				Required: true,
			},
		},
	}
}

func (e *intIDRes) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_int_id_res"
}

func (e *intIDRes) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = e.schema()
}

func (e *intIDRes) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.State.Raw = req.Plan.Raw.Copy() // Copy plan to state.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), 1234)...)
}

func (e *intIDRes) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

func (e *intIDRes) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var id int64
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if id != 1234 {
		resp.Diagnostics.AddAttributeError(path.Root("id"), "unexpected value",
			fmt.Sprintf("expected 1234, found %d", id))
	}

	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if id != 5678 {
		resp.Diagnostics.AddAttributeError(path.Root("id"), "unexpected value",
			fmt.Sprintf("expected 5678, found %d", id))
	}

	resp.State.Raw = req.Plan.Raw.Copy() // Copy plan to state.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), 90)...)
}

func (e *intIDRes) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State = e.nilState(ctx)
}

func (e *intIDRes) nilState(ctx context.Context) tfsdk.State {
	typ := e.terraformType(ctx)
	return tfsdk.State{
		Raw:    tftypes.NewValue(typ, nil),
		Schema: e.schema(),
	}
}

func (e *intIDRes) terraformType(ctx context.Context) tftypes.Type {
	return e.schema().Type().TerraformType(ctx)
}
