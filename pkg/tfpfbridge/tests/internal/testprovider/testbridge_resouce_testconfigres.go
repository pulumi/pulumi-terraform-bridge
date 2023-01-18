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

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type testconfigres struct {
	config string
}

var _ resource.Resource = (*testconfigres)(nil)
var _ resource.ResourceWithConfigure = (*testconfigres)(nil)

func newTestConfigRes() resource.Resource {
	return &testconfigres{}
}

func (*testconfigres) schema() tfsdk.Schema {
	return tfsdk.Schema{
		Description: `
testbridge_testconfigres is built to test Configure support in the provider.
`,
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"configCopy": {
				Type:     types.StringType,
				Computed: true,
			},
		},
	}
}

func (e *testconfigres) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	e.config = req.ProviderData.(string)
}

func (e *testconfigres) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_testconfigres"
}

func (e *testconfigres) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return e.schema(), nil
}

func (e *testconfigres) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.State.SetAttribute(ctx, path.Root("id"), "id-1")
	resp.State.SetAttribute(ctx, path.Root("configCopy"), e.config)
}

func (e *testconfigres) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	panic("Read not implemented")
}

func (e *testconfigres) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	panic("Update not implemented")
}

func (e *testconfigres) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	panic("Delete not implemented")
}
