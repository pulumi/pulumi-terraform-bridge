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

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

type Resource struct {
	Name           string
	ResourceSchema schema.Schema

	CreateFunc      func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse)
	ReadFunc        func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse)
	UpdateFunc      func(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse)
	DeleteFunc      func(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse)
	ImportStateFunc func(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse)
	ModifyPlanFunc func(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse)
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

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.ImportStateFunc == nil {
		return
	}
	r.ImportStateFunc(ctx, req, resp)
}

func (r *Resource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
    if r.ModifyPlanFunc == nil {
		return
	}
	r.ModifyPlanFunc(ctx, req, resp)
}

var _ resource.ResourceWithImportState = &Resource{}
var _ resource.ResourceWithModifyPlan = &Resource{}