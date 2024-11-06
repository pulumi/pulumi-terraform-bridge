package tfbridgetests

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zclconf/go-cty/cty"
)

func TestDetailedDiffList(t *testing.T) {
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

	nestedAttributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListNestedAttribute{
				NestedObject: rschema.NestedAttributeObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}

	nestedAttributeReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListNestedAttribute{
				NestedObject: rschema.NestedAttributeObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{
							Optional: true,
						},
					},
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	nestedAttributeNestedReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ListNestedAttribute{
				NestedObject: rschema.NestedAttributeObject{
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

	attrList := func(el ...string) cty.Value {
		slice := make([]cty.Value, len(el))
		for i, v := range el {
			slice[i] = cty.StringVal(v)
		}
		if len(slice) == 0 {
			return cty.ListValEmpty(cty.String)
		}
		return cty.ListVal(slice)
	}

	nestedAttrList := func(el ...string) cty.Value {
		slice := make([]cty.Value, len(el))
		for i, v := range el {
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

	t.Run("unchanged non-empty", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"value"}
		changeValue := []string{"value"}
	})

	t.Run("changed", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"value"}
		changeValue := []string{"value1"}

	})

	t.Run("added", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{}
		changeValue := []string{"value"}
	})

	t.Run("removed", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"value"}
		changeValue := []string{}
	})

	t.Run("null unchanged", func(t *testing.T) {
		t.Parallel()
		initialValue := cty.NullVal(cty.DynamicPseudoType)
		changeValue := cty.NullVal(cty.DynamicPseudoType)
	})

	t.Run("null to non-null", func(t *testing.T) {
		t.Parallel()
		initialValue := cty.NullVal(cty.DynamicPseudoType)
		changeValue := []string{"value"}
	})

	t.Run("non-null to null", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"value"}
		changeValue := cty.NullVal(cty.DynamicPseudoType)
	})

	t.Run("changed null to empty", func(t *testing.T) {
		t.Parallel()
		initialValue := cty.NullVal(cty.DynamicPseudoType)
		changeValue := []string{}
	})

	t.Run("changed empty to null", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{}
		changeValue := cty.NullVal(cty.DynamicPseudoType)
	})

	t.Run("element added", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"value"}
		changeValue := []string{"value", "value1"}
	})

	t.Run("element removed", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"value", "value1"}
		changeValue := []string{"value"}
	})
}
