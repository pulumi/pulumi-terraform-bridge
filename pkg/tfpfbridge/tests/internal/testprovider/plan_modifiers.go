// Copyright 2016-2022, Pulumi Corporation.
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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type PropagatesNullFrom struct {
	AnotherAttribute string
}

func (mod PropagatesNullFrom) Modify(ctx context.Context, req tfsdk.ModifyAttributePlanRequest, resp *tfsdk.ModifyAttributePlanResponse) {
	var attrs map[string]tftypes.Value
	err := req.Config.Raw.As(&attrs)
	if err != nil {
		panic(err)
	}
	anotherAttr, ok := attrs[mod.AnotherAttribute]
	if !ok {
		panic(fmt.Errorf("No attribute in config: %s", mod.AnotherAttribute))
	}

	if anotherAttr.IsNull() {
		attrTy := resp.AttributePlan.Type(ctx)
		nilValue := tftypes.NewValue(attrTy.TerraformType(ctx), nil)
		nilPlan, err := attrTy.ValueFromTerraform(ctx, nilValue)
		if err != nil {
			panic(err)
		}
		resp.AttributePlan = nilPlan
	}
}

func (mod PropagatesNullFrom) Description(_ context.Context) string {
	return "Sets plan to null if AnotherAttribute is null in config"
}

func (mod PropagatesNullFrom) MarkdownDescription(ctx context.Context) string {
	return mod.Description(ctx)
}
