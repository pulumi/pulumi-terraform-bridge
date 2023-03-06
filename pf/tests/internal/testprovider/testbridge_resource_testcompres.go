// Copyright 2016-2022, Pulumi Corporation.
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// Tests for Computed Attributes support.
type testCompRes struct{}

var _ resource.Resource = &testCompRes{}

func newTestCompRes() resource.Resource {
	return &testCompRes{}
}

func (*testCompRes) schema() rschema.Schema {
	return rschema.Schema{
		Description: `Additional tests for Computed attributes`,
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ecdsacurve": rschema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (e *testCompRes) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_testcompres"
}

func (e *testCompRes) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = e.schema()
}

func (e *testCompRes) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resourceID := "r1"

	// Copy plan to state.
	resp.State.Raw = req.Plan.Raw.Copy()

	var curve string
	req.Plan.GetAttribute(ctx, path.Root("ecdsacurve"), &curve)

	if curve != "P384" {
		resp.Diagnostics.AddError("invalid ecsda_curve", fmt.Sprintf(`must be "P384", got %q`, curve))
		return
	}

	diags2 := resp.State.SetAttribute(ctx, path.Root("id"), resourceID)
	resp.Diagnostics.Append(diags2...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (e *testCompRes) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	panic("TODO Read")
}

func (e *testCompRes) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	panic("TODO Update")
}

func (e *testCompRes) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	panic("TODO Delete")
}
