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
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

type testconfigres struct {
	config string
}

var _ resource.Resource = (*testconfigres)(nil)
var _ resource.ResourceWithConfigure = (*testconfigres)(nil)

func newTestConfigRes() resource.Resource {
	return &testconfigres{}
}

func (*testconfigres) schema() rschema.Schema {
	return rschema.Schema{
		Description: `
testbridge_testconfigres is built to test Configure support in the provider.
`,
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true,
			},
			"config_copy": rschema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (e *testconfigres) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	cfg := req.ProviderData.(*string)
	if cfg != nil {
		e.config = *cfg
	}
}

func (e *testconfigres) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_testconfigres"
}

func (e *testconfigres) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = e.schema()
}

func (e *testconfigres) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.State.SetAttribute(ctx, path.Root("id"), "id-1")
	resp.State.SetAttribute(ctx, path.Root("config_copy"), e.config)
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
