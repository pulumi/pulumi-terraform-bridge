// Copyright 2016-2024, Pulumi Corporation.
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

package schemashim

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// From https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/lexv2models/bot_version.go#L81
func TestExample(t *testing.T) {
	idAttribute := func() schema.StringAttribute {
		return schema.StringAttribute{
			Computed: true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		}
	}
	var botVersionLocaleDetails = map[string]attr.Type{
		"source_bot_version": types.StringType,
	}
	schema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"locale_specification": schema.MapAttribute{
				Required:    true,
				ElementType: types.ObjectType{AttrTypes: botVersionLocaleDetails},
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"id": idAttribute(),
		},
	}
	res := &testResource{"r1", schema}
	fromP := &testProvider{"prov", []resource.Resource{res}}
	p := ShimSchemaOnlyProvider(context.Background(), fromP)
	d := p.ResourcesMap().Get("prov_r1").Schema().Get("locale_specification").Description()
	t.Logf("locale_specification has this description: %q", d)
	ty := p.ResourcesMap().Get("prov_r1").Schema().Get("locale_specification").Type()
	elem := p.ResourcesMap().Get("prov_r1").Schema().Get("locale_specification").Elem()

	//t.Logf("locale_specification has this Type(): %#T", ty)

	t.Logf("locale_specification has this Elem(): %#T", elem)
	t.Logf("locale_specification has this Elem().Type(): %v", elem.(shim.Schema).Type())
	t.Logf("locale_specification has this Type(): %v", ty)
}

type testProvider struct {
	typeName  string
	resources []resource.Resource
}

var _ provider.Provider = (*testProvider)(nil)

func (p *testProvider) Metadata(
	_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = p.typeName
}

func (p *testProvider) Schema(
	context.Context, provider.SchemaRequest, *provider.SchemaResponse,
) {
}

func (p *testProvider) Configure(
	context.Context, provider.ConfigureRequest, *provider.ConfigureResponse,
) {
	panic("Unexpected call to Configure")
}

func (p *testProvider) Resources(context.Context) []func() resource.Resource {
	result := []func() resource.Resource{}
	for i := 0; i < len(p.resources); i++ {
		i := i
		result = append(result, func() resource.Resource {
			return p.resources[i]
		})
	}
	return result
}

func (p *testProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

type testResource struct {
	typeName string
	schema   schema.Schema
}

var _ resource.Resource = (*testResource)(nil)

func (r *testResource) Metadata(
	_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_" + r.typeName
}

func (r *testResource) Schema(
	_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse,
) {
	resp.Schema = r.schema
}

func (*testResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {
	panic("Unexpected call to Create")
}

func (*testResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
	panic("Unexpected call to Read")
}

func (*testResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
	panic("Unexpected call to Update")
}

func (*testResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	panic("Unexpected call to Delete")
}
