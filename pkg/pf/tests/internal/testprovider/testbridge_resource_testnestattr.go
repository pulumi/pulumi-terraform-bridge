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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type testnestattr struct{}

var _ resource.Resource = &testnestattr{}
var _ resource.ResourceWithImportState = &testnestattr{}

func newTestnestattr() resource.Resource {
	return &testnestattr{}
}

func (*testnestattr) schema() rschema.Schema {
	return rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// Example borrowed from https://github.com/fly-apps/terraform-provider-fly/blob/28438713f2bdf08dbd0aa2fae9d74baaca9845f1/internal/provider/machine_resource.go#L176
			"services": schema.ListNestedAttribute{
				MarkdownDescription: "services: tests ListNestedAttributes support",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ports": schema.ListNestedAttribute{
							MarkdownDescription: "External ports and handlers",
							Required:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"port": schema.Int64Attribute{
										MarkdownDescription: "External port",
										Required:            true,
									},
									"handlers": schema.ListAttribute{
										ElementType:         types.StringType,
										MarkdownDescription: "How the edge should process requests",
										Optional:            true,
									},
								},
							},
						},
						"protocol": schema.StringAttribute{
							MarkdownDescription: "network protocol",
							Required:            true,
						},
						// TODO internal_port gets mangled to internalPort by Pulumi renaming and does
						// not work end-to-end yet.
						"intport": schema.Int64Attribute{
							MarkdownDescription: "Port application listens on internally",
							Required:            true,
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
		},
	}
}

func (e *testnestattr) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_testnestattr"
}

func (e *testnestattr) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = e.schema()
}

func (e *testnestattr) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	panic("unimplemented")
}

func (e *testnestattr) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Do nothing, accepting the result from ImportState as-is
}

func (e *testnestattr) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	panic("unimplemented")
}

func (e *testnestattr) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	panic("unimplemented")
}

// ImportState is called when the provider must import the state of a resource instance. This method must return enough
// state so the Read method can properly refresh the full resource.
//
// If setting an attribute with the import identifier, it is recommended to use the ImportStatePassthroughID() call in
// this method.
func (e *testnestattr) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {

	type model struct {
		ID       types.String   `tfsdk:"id"`
		Services []ServiceModel `tfsdk:"services"`
	}

	resp.Diagnostics = resp.State.Set(ctx, &model{
		ID:       types.StringValue(req.ID),
		Services: []ServiceModel{},
	})
}
