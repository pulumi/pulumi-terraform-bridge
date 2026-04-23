// Copyright 2016-2025, Pulumi Corporation.
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

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newPrintAction() action.Action {
	return &printAction{}
}

type printAction struct{}

func (*printAction) Metadata(ctx context.Context, req action.MetadataRequest,
	resp *action.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_print"
}

func (*printAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Prints string N times",
		Attributes: map[string]schema.Attribute{
			"text": schema.StringAttribute{
				Description: "String to print",
				Required:    true,
			},
			"count": schema.Float64Attribute{
				Description: "Number of times to print the string",
				Optional:    true,
			},
		},
	}
}

func (a *printAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config printModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	text := config.Text.ValueString()

	count := 1.0
	if !config.Count.IsNull() {
		count = config.Count.ValueFloat64()
	}

	iCount := int(count)

	for i := 0; i < iCount; i++ {
		resp.SendProgress(action.InvokeProgressEvent{
			Message: fmt.Sprintf("%s", text),
		})
	}
}

type printModel struct {
	Text  types.String  `tfsdk:"text"`
	Count types.Float64 `tfsdk:"count"`
}
