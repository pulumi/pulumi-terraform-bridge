package tfbridgetests

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/zclconf/go-cty/cty"
)

func TestDetailedDiffListAttribute(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}

	attributeReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	blockSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.ListNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}

	blockReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.ListNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{Optional: true},
					},
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	blockNestedReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.ListNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
					},
				},
			},
		},
	}

	t.Run("unchanged non-empty", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("changed", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value1")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value1")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("added", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("removed", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("null unchanged", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("null to non-null", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("non-null to null", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("changed null to empty", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": cty.ListValEmpty(cty.String)},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": cty.ListValEmpty(cty.String)},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("changed empty to null", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.ListValEmpty(cty.String)},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.ListValEmpty(cty.String)},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("element added", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value"), cty.StringVal("value1")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value"), cty.StringVal("value1")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("element removed", func(t *testing.T) {
		t.Parallel()

		t.Run("no replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value"), cty.StringVal("value1")})},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("requires replace", func(t *testing.T) {
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value"), cty.StringVal("value1")})},
				map[string]cty.Value{"key": cty.ListVal([]cty.Value{cty.StringVal("value")})},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})
}
