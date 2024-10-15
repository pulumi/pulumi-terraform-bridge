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
)

type testDefaultInfoRes struct{}

var _ resource.Resource = &testDefaultInfoRes{}

func newTestDefaultInfoRes() resource.Resource {
	return &testDefaultInfoRes{}
}

func (*testDefaultInfoRes) schema() schema.Schema {
	return rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"str": rschema.StringAttribute{
				Optional: true,
			},
		},
	}
}

func (e *testDefaultInfoRes) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_test_default_info_res"
}

func (e *testDefaultInfoRes) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = e.schema()
}

func (e *testDefaultInfoRes) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resourceID := "uniqueid"
	diags1 := resp.State.SetAttribute(ctx, path.Root("id"), resourceID)
	resp.Diagnostics.Append(diags1...)

	var str *string
	diags2 := req.Plan.GetAttribute(ctx, path.Root("str"), &str)
	resp.Diagnostics.Append(diags2...)

	if str != nil {
		diags3 := resp.State.SetAttribute(ctx, path.Root("str"), &str)
		resp.Diagnostics.Append(diags3...)
	}
}

func (e *testDefaultInfoRes) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

func (e *testDefaultInfoRes) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (e *testDefaultInfoRes) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}
