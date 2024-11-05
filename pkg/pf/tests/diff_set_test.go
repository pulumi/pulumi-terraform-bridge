package tfbridgetests

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/zclconf/go-cty/cty"
)

func TestDetailedDiffSetAttribute(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.SetAttribute{
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
			"key": rschema.SetNestedBlock{
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
			"key": rschema.SetNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{
							Optional: true,
						},
					},
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	blockNestedReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SetNestedBlock{
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

	attrList := func(el ...string) cty.Value {
		slice := make([]cty.Value, len(el))
		for i, v := range el {
			slice[i] = cty.StringVal(v)
		}
		return cty.ListVal(slice)
	}

	blockList := func(el ...string) cty.Value {
		slice := make([]cty.Value, len(el))
		for i, v := range el {
			slice[i] = cty.ObjectVal(
				map[string]cty.Value{
					"nested": cty.StringVal(v),
				},
			)
		}
		return cty.ListVal(slice)
	}


	t.Run("unchanged non-empty", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("changed", func(t *testing.T) {
		t.Parallel()
		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": attrList("value1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": attrList("value1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{"key": blockList("value1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("added", func(t *testing.T) {
		t.Parallel()
		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("removed", func(t *testing.T) {
		t.Parallel()
		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
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
		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("null to non-null", func(t *testing.T) {
		t.Parallel()
		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": attrList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("non-null to null", func(t *testing.T) {
		t.Parallel()
		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("value")},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("changed null to empty", func(t *testing.T) {
		t.Parallel()
		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": attrList()},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
				map[string]cty.Value{"key": attrList()},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("value")},
				map[string]cty.Value{},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("changed empty to null", func(t *testing.T) {
		t.Parallel()
		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList()},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList()},
				map[string]cty.Value{"key": cty.NullVal(cty.List(cty.String))},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{},
				map[string]cty.Value{"key": blockList("value")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("removed front", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
				map[string]cty.Value{"key": attrList("val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
				map[string]cty.Value{"key": attrList("val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("removed front unordered", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val2", "val1", "val3")},
				map[string]cty.Value{"key": attrList("val1", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val2", "val1", "val3")},
				map[string]cty.Value{"key": attrList("val1", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val2", "val1", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val1", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val1", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("removed middle", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
				map[string]cty.Value{"key": attrList("val1", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
				map[string]cty.Value{"key": attrList("val1", "val3")},
			)
			autogold.Expect(`
			`).Equal(t, res.TFOut)
			autogold.Expect(`
			`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("removed middle unordered", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
				map[string]cty.Value{"key": attrList("val2", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
				map[string]cty.Value{"key": attrList("val2", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
				map[string]cty.Value{"key": blockList("val2", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
				map[string]cty.Value{"key": blockList("val2", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
				map[string]cty.Value{"key": blockList("val2", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("removed end", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
				map[string]cty.Value{"key": attrList("val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
				map[string]cty.Value{"key": attrList("val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("removed end unordered", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
				map[string]cty.Value{"key": attrList("val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
				map[string]cty.Value{"key": attrList("val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()
			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("added front", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val2", "val3")},
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val2", "val3")},
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": attrList("val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": attrList("val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": attrList("val2", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("added front unordered", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val3", "val1")},
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val3", "val1")},
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val3", "val1")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val3", "val1")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val3", "val1")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("added middle", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val1", "val3")},
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val1", "val3")},
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val1", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val3")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("added middle unordered", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val2", "val1")},
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val2", "val1")},
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val2", "val1")},
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val1")},
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val1")},
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("added end", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val1", "val2")},
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val1", "val2")},
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val1", "val2")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2")},
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("added end unordered", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val2", "val3")},
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val2", "val3")},
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val2", "val3")},
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val3")},
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val3")},
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("shuffled", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
				map[string]cty.Value{"key": attrList("val3", "val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
				map[string]cty.Value{"key": attrList("val3", "val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("shuffled unordered", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
				map[string]cty.Value{"key": attrList("val3", "val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
				map[string]cty.Value{"key": attrList("val3", "val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("shuffled with duplicates", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
				map[string]cty.Value{"key": attrList("val3", "val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val1", "val2", "val3")},
				map[string]cty.Value{"key": attrList("val3", "val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val1", "val2", "val3")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})

	t.Run("shuffled with duplicates unordered", func(t *testing.T) {
		t.Parallel()

		t.Run("attribute no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeSchema,
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
				map[string]cty.Value{"key": attrList("val3", "val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("attribute requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, attributeReplaceSchema,
				map[string]cty.Value{"key": attrList("val2", "val3", "val1")},
				map[string]cty.Value{"key": attrList("val3", "val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block no replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockSchema,
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})

		t.Run("block nested requires replace", func(t *testing.T) {
			t.Parallel()

			res := crosstests.Diff(t, blockNestedReplaceSchema,
				map[string]cty.Value{"key": blockList("val2", "val3", "val1")},
				map[string]cty.Value{"key": blockList("val3", "val1", "val2", "val3")},
			)

			autogold.Expect(`
`).Equal(t, res.TFOut)
			autogold.Expect(`
`).Equal(t, res.PulumiOut)
		})
	})
}
