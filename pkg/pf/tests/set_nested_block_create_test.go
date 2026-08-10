// Copyright 2016-2026, Pulumi Corporation.
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

package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pulumiresource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/zclconf/go-cty/cty"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
)

type preserveConfiguredString struct{}

func (preserveConfiguredString) Description(context.Context) string {
	return "preserve configured value"
}

func (preserveConfiguredString) MarkdownDescription(context.Context) string {
	return "preserve configured value"
}

func (preserveConfiguredString) PlanModifyString(
	_ context.Context,
	req planmodifier.StringRequest,
	resp *planmodifier.StringResponse,
) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	resp.PlanValue = types.StringValue(req.ConfigValue.ValueString())
}

type setOrderModel struct {
	ID      types.String            `tfsdk:"id"`
	GroupBy []*setOrderGroupByModel `tfsdk:"group_by"`
}

type setOrderGroupByModel struct {
	Path    types.String `tfsdk:"path"`
	TagName types.String `tfsdk:"tag_name"`
}

func TestPFCreateSetNestedBlockPreservesElementCorrelation(t *testing.T) {
	res := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: schema.Schema{
			Blocks: map[string]schema.Block{
				"group_by": schema.SetNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"path": schema.StringAttribute{
								Required: true,
							},
							"tag_name": schema.StringAttribute{
								Optional: true,
								Computed: true,
								PlanModifiers: []planmodifier.String{
									preserveConfiguredString{},
								},
							},
						},
					},
				},
			},
		},
		CreateFunc: func(
			ctx context.Context,
			req resource.CreateRequest,
			resp *resource.CreateResponse,
		) {
			var plan setOrderModel
			resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

			// Resolve computed values so Terraform can complete apply. The cross-test
			// captures req.Plan before this function returns.
			for _, item := range plan.GroupBy {
				if item.TagName.IsNull() || item.TagName.IsUnknown() {
					item.TagName = types.StringValue("")
				}
			}

			plan.ID = types.StringValue("test-id")
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		},
	})

	groupBy := func(path string, tagName cty.Value) cty.Value {
		return cty.ObjectVal(map[string]cty.Value{
			"path":     cty.StringVal(path),
			"tag_name": tagName,
		})
	}

	tfConfig := map[string]cty.Value{
		"group_by": cty.SetVal([]cty.Value{
			groupBy("env", cty.NullVal(cty.String)),
			groupBy("service", cty.NullVal(cty.String)),
			groupBy("status", cty.NullVal(cty.String)),
			groupBy("@stream", cty.StringVal("stream")),
			groupBy("@tier", cty.StringVal("tier")),
		}),
	}

	// Keep the original user order. Do not derive this value from tfConfig,
	// because cty.SetVal has already canonicalized that set.
	pulumiConfig := pulumiresource.PropertyMap{
		"groupBies": pulumiresource.NewArrayProperty([]pulumiresource.PropertyValue{
			groupByPulumi("env", ""),
			groupByPulumi("service", ""),
			groupByPulumi("status", ""),
			groupByPulumi("@stream", "stream"),
			groupByPulumi("@tier", "tier"),
		}),
	}

	crosstests.Create(
		t,
		res,
		tfConfig,
		crosstests.CreatePulumiConfig(pulumiConfig),
	)
}

func groupByPulumi(path, tagName string) pulumiresource.PropertyValue {
	value := pulumiresource.PropertyMap{
		"path": pulumiresource.NewStringProperty(path),
	}

	if tagName != "" {
		value["tagName"] = pulumiresource.NewStringProperty(tagName)
	}

	return pulumiresource.NewObjectProperty(value)
}
