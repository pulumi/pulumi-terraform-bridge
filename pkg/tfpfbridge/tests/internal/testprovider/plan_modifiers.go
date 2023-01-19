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

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type PropagatesNullFrom struct {
	AnotherAttribute string
}

var _ planmodifier.String = PropagatesNullFrom{}
var _ planmodifier.Number = PropagatesNullFrom{}
var _ planmodifier.Bool = PropagatesNullFrom{}
var _ planmodifier.List = PropagatesNullFrom{}
var _ planmodifier.Map = PropagatesNullFrom{}

func (mod PropagatesNullFrom) Description(_ context.Context) string {
	return "Sets plan to null if AnotherAttribute is null in config"
}

func (mod PropagatesNullFrom) MarkdownDescription(ctx context.Context) string {
	return mod.Description(ctx)
}

func (mod PropagatesNullFrom) PlanModifyNumber(ctx context.Context, req planmodifier.NumberRequest,
	resp *planmodifier.NumberResponse) {
	if mod.anotherAttributeIsNull(req.Config) {
		resp.PlanValue = types.NumberNull()
	}
}

func (mod PropagatesNullFrom) PlanModifyString(ctx context.Context, req planmodifier.StringRequest,
	resp *planmodifier.StringResponse) {
	if mod.anotherAttributeIsNull(req.Config) {
		resp.PlanValue = types.StringNull()
	}
}

func (mod PropagatesNullFrom) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest,
	resp *planmodifier.BoolResponse) {
	if mod.anotherAttributeIsNull(req.Config) {
		resp.PlanValue = types.BoolNull()
	}
}

func (mod PropagatesNullFrom) PlanModifyList(ctx context.Context, req planmodifier.ListRequest,
	resp *planmodifier.ListResponse) {
	if mod.anotherAttributeIsNull(req.Config) {
		resp.PlanValue = types.ListNull(req.PlanValue.ElementType(ctx))
	}
}

func (mod PropagatesNullFrom) PlanModifyMap(ctx context.Context, req planmodifier.MapRequest,
	resp *planmodifier.MapResponse) {
	if mod.anotherAttributeIsNull(req.Config) {
		resp.PlanValue = types.MapNull(req.PlanValue.ElementType(ctx))
	}
}

func (mod PropagatesNullFrom) anotherAttributeIsNull(config tfsdk.Config) bool {
	var attrs map[string]tftypes.Value
	err := config.Raw.As(&attrs)
	if err != nil {
		panic(err)
	}
	anotherAttr, ok := attrs[mod.AnotherAttribute]
	if !ok {
		panic(fmt.Errorf("PropagatesNullFrom{%q} plan modifier did not find the %q attribute in config",
			mod.AnotherAttribute, mod.AnotherAttribute))
	}
	return anotherAttr.IsNull()
}
