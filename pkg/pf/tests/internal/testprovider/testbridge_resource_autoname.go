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

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type autoNameRes struct{}

var _ resource.Resource = &autoNameRes{}

func newAutoNameRes() resource.Resource {
	return &autoNameRes{}
}

func (*autoNameRes) schema() rschema.Schema {
	return rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Test AutoNaming; Pulumi makes this property optional",
			},
		},
	}
}

func (e *autoNameRes) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_autoname_res"
}

func (e *autoNameRes) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = e.schema()
}

func (e *autoNameRes) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.State.Raw = req.Plan.Raw.Copy() // Copy plan to state.
	resp.State.SetAttribute(ctx, path.Root("id"), "some-id")
}

func (e *autoNameRes) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

func (e *autoNameRes) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.State.Raw = req.Plan.Raw.Copy() // Copy plan to state.
}

func (e *autoNameRes) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State = e.nilState(ctx)
}

func (e *autoNameRes) nilState(ctx context.Context) tfsdk.State {
	typ := e.terraformType(ctx)
	return tfsdk.State{
		Raw:    tftypes.NewValue(typ, nil),
		Schema: e.schema(),
	}
}

func (e *autoNameRes) terraformType(ctx context.Context) tftypes.Type {
	return e.schema().Type().TerraformType(ctx)
}
