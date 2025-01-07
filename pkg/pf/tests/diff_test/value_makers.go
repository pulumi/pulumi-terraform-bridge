package tfbridgetests

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/zclconf/go-cty/cty"
)

func ref[T any](t T) *T { return &t }

type stringDefault string

var _ defaults.String = stringDefault("default")

func (s stringDefault) DefaultString(ctx context.Context, req defaults.StringRequest, resp *defaults.StringResponse) {
	resp.PlanValue = basetypes.NewStringValue(string(s))
}

func (s stringDefault) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		resp.PlanValue = basetypes.NewStringValue(string(s))
	}
}

func (s stringDefault) Description(ctx context.Context) string {
	return "description"
}

func (s stringDefault) MarkdownDescription(ctx context.Context) string {
	return "markdown description"
}

type objectDefault basetypes.ObjectValue

var _ defaults.Object = objectDefault{}

func (o objectDefault) DefaultObject(ctx context.Context, req defaults.ObjectRequest, resp *defaults.ObjectResponse) {
	resp.PlanValue = basetypes.ObjectValue(o)
}

func (o objectDefault) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		resp.PlanValue = basetypes.ObjectValue(o)
	}
}

func (o objectDefault) Description(ctx context.Context) string {
	return "description"
}

func (o objectDefault) MarkdownDescription(ctx context.Context) string {
	return "markdown description"
}

type setDefault string

var _ defaults.Set = setDefault("default")

func (s setDefault) DefaultSet(ctx context.Context, req defaults.SetRequest, resp *defaults.SetResponse) {
	resp.PlanValue = basetypes.NewSetValueMust(types.StringType, []attr.Value{
		basetypes.NewStringValue("value"),
	})
}

func (s setDefault) Description(ctx context.Context) string {
	return "description"
}

func (s setDefault) MarkdownDescription(ctx context.Context) string {
	return "markdown description"
}

func listValueMaker(arr *[]string) cty.Value {
	if arr == nil {
		return cty.NullVal(cty.DynamicPseudoType)
	}
	slice := make([]cty.Value, len(*arr))
	for i, v := range *arr {
		slice[i] = cty.StringVal(v)
	}
	if len(slice) == 0 {
		return cty.ListValEmpty(cty.String)
	}
	return cty.ListVal(slice)
}

func nestedListValueMaker(arr *[]string) cty.Value {
	if arr == nil {
		return cty.NullVal(cty.DynamicPseudoType)
	}
	slice := make([]cty.Value, len(*arr))
	for i, v := range *arr {
		slice[i] = cty.ObjectVal(
			map[string]cty.Value{
				"nested": cty.StringVal(v),
			},
		)
	}
	if len(slice) == 0 {
		return cty.ListValEmpty(cty.Object(map[string]cty.Type{"nested": cty.String}))
	}
	return cty.ListVal(slice)
}

func nestedListValueMakerWithComputedSpecified(arr *[]string) cty.Value {
	if arr == nil {
		return cty.NullVal(cty.DynamicPseudoType)
	}
	slice := make([]cty.Value, len(*arr))
	for i, v := range *arr {
		slice[i] = cty.ObjectVal(
			map[string]cty.Value{
				"nested":   cty.StringVal(v),
				"computed": cty.StringVal("non-computed-" + v),
			},
		)
	}
	if len(slice) == 0 {
		return cty.ListValEmpty(cty.Object(map[string]cty.Type{"nested": cty.String}))
	}
	return cty.ListVal(slice)
}

type testOutput struct {
	initialValue any
	changeValue  any
	tfOut        string
	pulumiOut    string
	detailedDiff map[string]any
}
