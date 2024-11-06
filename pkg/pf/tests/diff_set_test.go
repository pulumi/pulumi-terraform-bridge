package tfbridgetests

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zclconf/go-cty/cty"
)

func TestDetailedDiffSet(t *testing.T) {
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

	nestedAttributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.SetNestedAttribute{
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
			"key": rschema.SetNestedAttribute{
				NestedObject: rschema.NestedAttributeObject{
					Attributes: map[string]rschema.Attribute{
						"nested": rschema.StringAttribute{Optional: true},
					},
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	nestedAttributeNestedReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.SetNestedAttribute{
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
		if len(slice) == 0 {
			return cty.ListValEmpty(cty.String)
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
		if len(slice) == 0 {
			return cty.ListValEmpty(cty.Object(map[string]cty.Type{"nested": cty.String}))
		}
		return cty.ListVal(slice)
	}

	t.Run("unchanged non-empty", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"value"}
		changedValue := []string{"value"}
	})

	t.Run("changed", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"value"}
		changedValue := []string{"value1"}
	})

	t.Run("added", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{}
		changedValue := []string{"value"}
	})

	t.Run("removed", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"value"}
		changedValue := []string{}
	})



	t.Run("null unchanged", func(t *testing.T) {
		t.Parallel()
		initialValue := cty.NullVal(cty.DynamicPseudoType)
		changedValue := cty.NullVal(cty.DynamicPseudoType)
	})

	t.Run("null to non-null", func(t *testing.T) {
		t.Parallel()
		initialValue := cty.NullVal(cty.DynamicPseudoType)
		changedValue := []string{"value"}
	})

	t.Run("non-null to null", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"value"}
		changedValue := cty.NullVal(cty.DynamicPseudoType)
	})

	t.Run("changed null to empty", func(t *testing.T) {
		t.Parallel()
		initialValue := cty.NullVal(cty.DynamicPseudoType)
		changedValue := []string{}
	})

	t.Run("changed empty to null", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{}
		changedValue := cty.NullVal(cty.DynamicPseudoType)
	})

	t.Run("removed front", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val1", "val2", "val3"}
		changedValue := []string{"val2", "val3"}
	})

	t.Run("removed front unordered", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val2", "val3", "val1"}
		changedValue := []string{"val3", "val1"}
	})

	t.Run("removed middle", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val1", "val2", "val3"}
		changedValue := []string{"val1", "val3"}
	})

	t.Run("removed middle unordered", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val3", "val1", "val2"}
		changedValue := []string{"val3", "val1"}
	})

	t.Run("removed end", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val1", "val2", "val3"}
		changedValue := []string{"val1", "val2"}
	})

	t.Run("removed end unordered", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val2", "val3", "val1"}
		changedValue := []string{"val2", "val3"}
	})

	t.Run("added front", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val2", "val3"}
		changedValue := []string{"val1", "val2", "val3"}
	})

	t.Run("added front unordered", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val3", "val1"}
		changedValue := []string{"val2", "val3", "val1"}
	})

	t.Run("added middle", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val1", "val3"}
		changedValue := []string{"val1", "val2", "val3"}
	})

	t.Run("added middle unordered", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val2", "val1"}
		changedValue := []string{"val2", "val3", "val1"}
	})

	t.Run("added end", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val1", "val2"}
		changedValue := []string{"val1", "val2", "val3"}
	})

	t.Run("added end unordered", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val2", "val3"}
		changedValue := []string{"val2", "val3", "val1"}
	})

	t.Run("shuffled", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val1", "val2", "val3"}
		changedValue := []string{"val3", "val1", "val2"}
	})

	t.Run("shuffled unordered", func(t *testing.T) {
		t.Parallel()
		initialValue := []string{"val2", "val3", "val1"}
		changedValue := []string{"val3", "val1", "val2"}
	})

	// PF does not allow duplicates in sets, so we don't test that here.
	// TODO: test pulumi behaviour
}
