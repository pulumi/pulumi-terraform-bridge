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

package providerbuilder

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

type NewResourceArgs struct {
	Name string

	ResourceSchema schema.Schema

	// Do not inject a Computed String "id" attribute into the schema.
	CustomID bool

	CreateFunc       func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse)
	ReadFunc         func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse)
	UpdateFunc       func(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse)
	DeleteFunc       func(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse)
	ImportStateFunc  func(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse)
	UpgradeStateFunc func(context.Context) map[int64]resource.StateUpgrader
	ModifyPlanFunc   func(context.Context, resource.ModifyPlanRequest, *resource.ModifyPlanResponse)
}

// NewResource creates a new Resource with the given parameters, filling reasonable defaults.
func NewResource(args NewResourceArgs) Resource {
	if args.Name == "" {
		args.Name = "test"
	}

	if args.ResourceSchema.Attributes == nil {
		args.ResourceSchema.Attributes = map[string]schema.Attribute{}
	}

	if args.ResourceSchema.Attributes["id"] == nil && !args.CustomID {
		args.ResourceSchema.Attributes["id"] = schema.StringAttribute{
			Computed: true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		}
	}

	if args.CreateFunc == nil {
		args.CreateFunc = func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
			resp.State = tfsdk.State(req.Plan)
			resp.State.SetAttribute(ctx, path.Root("id"), "test-id")
		}
	}
	if args.UpdateFunc == nil {
		args.UpdateFunc = func(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
			resp.State = tfsdk.State(req.Plan)
		}
	}
	if args.ReadFunc == nil {
		args.ReadFunc = func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
			resp.State = req.State
		}
	}
	if args.UpgradeStateFunc == nil {
		args.UpgradeStateFunc = func(ctx context.Context) map[int64]resource.StateUpgrader {
			return nil
		}
	}
	return Resource{
		Name:             args.Name,
		ResourceSchema:   args.ResourceSchema,
		CreateFunc:       args.CreateFunc,
		ReadFunc:         args.ReadFunc,
		UpdateFunc:       args.UpdateFunc,
		DeleteFunc:       args.DeleteFunc,
		ImportStateFunc:  args.ImportStateFunc,
		UpgradeStateFunc: args.UpgradeStateFunc,
		ModifyPlanFunc:   args.ModifyPlanFunc,
	}
}

// Resource is a utility type that helps define PF resources. Prefer creating via NewResource.
type Resource struct {
	Name string

	ResourceSchema schema.Schema

	CreateFunc       func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse)
	ReadFunc         func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse)
	UpdateFunc       func(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse)
	DeleteFunc       func(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse)
	ImportStateFunc  func(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse)
	UpgradeStateFunc func(context.Context) map[int64]resource.StateUpgrader
	ModifyPlanFunc   func(context.Context, resource.ModifyPlanRequest, *resource.ModifyPlanResponse)
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, re *resource.MetadataResponse) {
	re.TypeName = req.ProviderTypeName + "_" + r.Name
}

func (r *Resource) Schema(ctx context.Context, _ resource.SchemaRequest, re *resource.SchemaResponse) {
	re.Schema = r.ResourceSchema
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.CreateFunc == nil {
		return
	}
	r.CreateFunc(ctx, req, resp)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.ReadFunc == nil {
		return
	}
	r.ReadFunc(ctx, req, resp)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.UpdateFunc == nil {
		return
	}
	r.UpdateFunc(ctx, req, resp)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.DeleteFunc == nil {
		return
	}
	r.DeleteFunc(ctx, req, resp)
}

func (r *Resource) ImportState(
	ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse,
) {
	if r.ImportStateFunc == nil {
		return
	}
	r.ImportStateFunc(ctx, req, resp)
}

func (r *Resource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	if r.UpgradeStateFunc == nil {
		return nil
	}
	return r.UpgradeStateFunc(ctx)
}

func (r *Resource) ModifyPlan(
	ctx context.Context,
	req resource.ModifyPlanRequest,
	resp *resource.ModifyPlanResponse,
) {
	if r.ModifyPlanFunc != nil {
		r.ModifyPlanFunc(ctx, req, resp)
	}
}

var (
	_ resource.ResourceWithImportState  = (*Resource)(nil)
	_ resource.ResourceWithUpgradeState = (*Resource)(nil)
	_ resource.ResourceWithModifyPlan   = (*Resource)(nil)
)
