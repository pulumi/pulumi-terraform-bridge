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
)

type testnest struct{}

var _ resource.Resource = &testnest{}

func newTestnest() resource.Resource {
	return &testnest{}
}

func (*testnest) schema() rschema.Schema {
	return rschema.Schema{
		Blocks: map[string]schema.Block{
			"rules": schema.ListNestedBlock{
				MarkdownDescription: "List of rules to apply to the ruleset.",
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"action_parameters": schema.ListNestedBlock{
							MarkdownDescription: "List of parameters that configure the behavior of the ruleset rule action.",
							NestedObject: schema.NestedBlockObject{
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
