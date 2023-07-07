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

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

type compoptRes struct{}

var _ resource.Resource = &compoptRes{}

func newCompOptRes() resource.Resource {
	return &compoptRes{}
}

func (*compoptRes) schema() rschema.Schema {
	return rschema.Schema{
		Description: `testbridge_compopt_res resource tests computed optional fields`,
		Attributes: map[string]rschema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"foo": rschema.SingleNestedAttribute{
				Attributes: map[string]rschema.Attribute{
					"bar": rschema.StringAttribute{
						Optional: true,
						Computed: true,
					},
				},
				Computed: true,
				Optional: true,
			},
		},
	}
}

func (e *compoptRes) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compopt_res"
}

func (e *compoptRes) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = e.schema()
}

func (e *compoptRes) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	panic("Create not supported yet")
}

func (e *compoptRes) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	panic("Update not supported yet")
}

func (e *compoptRes) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	panic("Read not supported yet")
}

func (e *compoptRes) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	panic("Delete not supported yet")
}
