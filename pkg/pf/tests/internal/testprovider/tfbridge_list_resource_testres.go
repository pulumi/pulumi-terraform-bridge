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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type listTestres struct{}

var _ list.ListResource = &listTestres{}

func newListTestres() list.ListResource {
	return &listTestres{}
}

func (e *listTestres) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_testres"
}

func (e *listTestres) ListResourceConfigSchema(_ context.Context,
	req list.ListResourceSchemaRequest,
	res *list.ListResourceSchemaResponse) {
	res.Schema = schema.Schema{
		Description: "A test list resource",
		Attributes: map[string]schema.Attribute{
			"count": schema.Int32Attribute{
				Description: "An integer count",
				Optional:    true,
			},
		},
	}
}

func (e *listTestres) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	var model listTestresModel
	req.Config.Get(ctx, &model)
	resp.Results = func(yield func(list.ListResult) bool) {
		if !model.Count.IsNull() {
			count, _ := model.Count.ToInt32Value(ctx)
			for i := 0; i < int(count.ValueInt32()); i++ {
				result := list.ListResult{
					DisplayName: fmt.Sprintf("Example %d", i),
					Resource: &tfsdk.Resource{
						Raw: tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{
							"name":    tftypes.NewValue(tftypes.String, fmt.Sprintf("example-%d", i)),
							"current": tftypes.NewValue(tftypes.Number, float64(i)),
						}),
					},
				}

				if !yield(result) {
					return
				}
			}
		}
	}
}

type listTestresModel struct {
	Count types.Int32 `tfsdk:"count"`
}
