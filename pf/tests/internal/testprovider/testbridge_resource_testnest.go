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
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type testnest struct{}

var _ resource.Resource = &testnest{}
var _ resource.ResourceWithImportState = &testnest{}

func newTestnest() resource.Resource {
	return &testnest{}
}

func (*testnest) schema() rschema.Schema {
	return rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"rules": schema.ListNestedBlock{
				MarkdownDescription: "List of rules to apply to the ruleset.",
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"action_parameters": schema.ListNestedBlock{
							MarkdownDescription: "List of parameters that configure the behavior of the ruleset rule action.",
							NestedObject: schema.NestedBlockObject{
								Blocks: map[string]schema.Block{
									"phases": schema.ListNestedBlock{
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"p1": schema.BoolAttribute{
													Optional:            true,
													MarkdownDescription: "The first phase.",
												},
												"p2": schema.BoolAttribute{
													Optional:            true,
													MarkdownDescription: "The second phase.",
												},
											},
										},
									},
								},
								Attributes: map[string]schema.Attribute{
									"automatic_https_rewrites": schema.BoolAttribute{
										Optional:            true,
										MarkdownDescription: "Turn on or off Cloudflare Automatic HTTPS rewrites.",
									},
									"bic": schema.BoolAttribute{
										Optional:            true,
										MarkdownDescription: "Inspect the visitor's browser for headers commonly associated with spammers and certain bots.",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (e *testnest) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_testnest"
}

func (e *testnest) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = e.schema()
}

func (e *testnest) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	panic("unimplemented")
}

func (e *testnest) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	panic("unimplemented")
}

func (e *testnest) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	panic("unimplemented")
}

func (e *testnest) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	panic("unimplemented")
}

// ImportState is called when the provider must import the state of a resource instance. This method must return enough
// state so the Read method can properly refresh the full resource.
//
// If setting an attribute with the import identifier, it is recommended to use the ImportStatePassthroughID() call in
// this method.
func (e *testnest) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	type ruleModel struct{}

	type model struct {
		ID    types.String `tfsdk:"id"`
		Rules []ruleModel  `tfsdk:"rules"`
	}

	resp.Diagnostics = resp.State.Set(ctx, &model{
		ID:    types.StringValue(req.ID),
		Rules: []ruleModel{},
	})
}
