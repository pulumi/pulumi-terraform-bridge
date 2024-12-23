package tests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
)

func TestDetailedDiffSet(t *testing.T) {
	t.Parallel()

	attributeSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}

	attributeSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				ForceNew: true,
			},
		},
	}

	blockSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	blockSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	blockSchemaNestedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
		},
	}

	computedAttributeSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
		CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			d.SetId("id")
			setHashFunc := d.Get("test").(*schema.Set).F
			err := d.Set("test", schema.NewSet(setHashFunc, []interface{}{"computed"}))
			contract.Assertf(err == nil, "failed to set attribute: %v", err)
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			if d.Get("test") == nil {
				setHashFunc := d.Get("test").(*schema.Set).F
				err := d.Set("test", schema.NewSet(setHashFunc, []interface{}{"computed"}))
				contract.Assertf(err == nil, "failed to set attribute: %v", err)
			}
			return nil
		},
	}

	computedAttributeSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
		CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			d.SetId("id")
			if d.Get("test") == nil {
				err := d.Set("test", schema.NewSet(schema.HashString, []interface{}{"computed"}))
				contract.Assertf(err == nil, "failed to set attribute: %v", err)
			}
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			if d.Get("test") == nil {
				err := d.Set("test", schema.NewSet(schema.HashString, []interface{}{"computed"}))
				contract.Assertf(err == nil, "failed to set attribute: %v", err)
			}
			return nil
		},
	}

	computedSetBlockAttributeFunc := func(_ context.Context, d *schema.ResourceData, _ interface{}) {
		contract.Assertf(d.Get("test") != nil, "test attribute is nil")
		testVals := d.Get("test").(*schema.Set).List()
		for _, v := range testVals {
			val := v.(map[string]interface{})
			if val["computed"] == nil {
				compVal := "computed1"
				if val["nested"] != nil {
					compVal = val["nested"].(string)
				}
				val["computed"] = compVal
			}
		}
		setHashFunc := d.Get("test").(*schema.Set).F
		err := d.Set("test", schema.NewSet(setHashFunc, testVals))
		contract.Assertf(err == nil, "failed to set attribute: %v", err)
	}

	computedSetBlockFunc := func(ctx context.Context, d *schema.ResourceData, i interface{}) {
		if d.Get("test") == nil {
			setHashFunc := d.Get("test").(*schema.Set).F
			err := d.Set("test", schema.NewSet(setHashFunc, []interface{}{
				map[string]interface{}{
					"nested":   "computed",
					"computed": "computed1",
				},
			}))
			contract.Assertf(err == nil, "failed to set attribute: %v", err)
		} else {
			computedSetBlockAttributeFunc(ctx, d, i)
		}
	}

	blockSchemaComputed := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"computed": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
					},
				},
			},
		},
		CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			d.SetId("id")
			computedSetBlockFunc(ctx, d, i)
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			computedSetBlockFunc(ctx, d, i)
			return nil
		},
	}

	blockSchemaComputedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"computed": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
					},
				},
			},
		},
		CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			d.SetId("id")
			computedSetBlockFunc(ctx, d, i)
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			computedSetBlockFunc(ctx, d, i)
			return nil
		},
	}

	blockSchemaComputedNestedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
						"computed": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
					},
				},
			},
		},
		CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			d.SetId("id")
			computedSetBlockFunc(ctx, d, i)
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			computedSetBlockFunc(ctx, d, i)
			return nil
		},
	}

	blockSchemaNestedComputed := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"computed": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
					},
				},
			},
		},
		CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			d.SetId("id")
			if d.Get("test") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			if d.Get("test") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
	}

	blockSchemaNestedComputedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"computed": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
					},
				},
			},
		},
		CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			d.SetId("id")
			if d.Get("test") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			if d.Get("test") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
	}

	blockSchemaNestedComputedNestedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"test": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
						"computed": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
					},
				},
			},
		},
		CreateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			d.SetId("id")
			if d.Get("test") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			if d.Get("test") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
	}

	attrList := func(arr *[]string) cty.Value {
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

	nestedAttrList := func(arr *[]string) cty.Value {
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

	nestedAttrListWithComputedSpecified := func(arr *[]string) cty.Value {
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
			return cty.ListValEmpty(cty.Object(map[string]cty.Type{
				"nested":   cty.String,
				"computed": cty.String,
			}))
		}
		return cty.ListVal(slice)
	}

	type schemaValueMakerPair struct {
		name       string
		res        schema.Resource
		valueMaker func(*[]string) cty.Value
	}

	schemaValueMakerPairs := []schemaValueMakerPair{
		{"attribute no force new", attributeSchema, attrList},
		{"block no force new", blockSchema, nestedAttrList},

		{"attribute force new", attributeSchemaForceNew, attrList},
		{"block top level force new", blockSchemaForceNew, nestedAttrList},
		{"block nested force new", blockSchemaNestedForceNew, nestedAttrList},
	}

	computedSchemaValueMakerPairs := []schemaValueMakerPair{
		{"computed attribute no force new", computedAttributeSchema, attrList},
		{"block with computed no replace", blockSchemaComputed, nestedAttrList},
		{"block with computed no replace computed specified in program", blockSchemaComputed, nestedAttrListWithComputedSpecified},
		{"block with nested computed no replace", blockSchemaNestedComputed, nestedAttrList},
		{"block with nested computed no replace computed specified in program", blockSchemaNestedComputed, nestedAttrListWithComputedSpecified},
		{"computed attribute force new", computedAttributeSchemaForceNew, attrList},
		{"block with computed force new", blockSchemaComputedForceNew, nestedAttrList},
		{"block with computed force new computed specified in program", blockSchemaComputedForceNew, nestedAttrListWithComputedSpecified},
		{"block with computed and nested force new", blockSchemaComputedNestedForceNew, nestedAttrList},
		{"block with computed and nested force new computed specified in program", blockSchemaComputedNestedForceNew, nestedAttrListWithComputedSpecified},
		{"block with nested computed and force new", blockSchemaNestedComputedForceNew, nestedAttrList},
		{"block with nested computed and force new computed specified in program", blockSchemaNestedComputedForceNew, nestedAttrListWithComputedSpecified},
		{"block with nested computed and nested force new", blockSchemaNestedComputedNestedForceNew, nestedAttrList},
		{"block with nested computed and nested force new computed specified in program", blockSchemaNestedComputedNestedForceNew, nestedAttrListWithComputedSpecified},
	}

	scenarios := []struct {
		name         string
		initialValue *[]string
		changeValue  *[]string
	}{
		{"unchanged non-empty", &[]string{"value"}, &[]string{"value"}},
		{"unchanged empty", &[]string{}, &[]string{}},
		{"unchanged null", nil, nil},

		{"changed non-null", &[]string{"value"}, &[]string{"value1"}},
		{"changed null to non-null", nil, &[]string{"value"}},
		{"changed non-null to null", &[]string{"value"}, nil},
		{"changed null to empty", nil, &[]string{}},
		{"changed empty to null", &[]string{}, nil},

		{"added", &[]string{}, &[]string{"value"}},
		{"removed", &[]string{"value"}, &[]string{}},

		{"removed front", &[]string{"val1", "val2", "val3"}, &[]string{"val2", "val3"}},
		{"removed front unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val3", "val1"}},
		{"removed middle", &[]string{"val1", "val2", "val3"}, &[]string{"val1", "val3"}},
		{"removed middle unordered", &[]string{"val3", "val1", "val2"}, &[]string{"val3", "val1"}},
		{"removed end", &[]string{"val1", "val2", "val3"}, &[]string{"val1", "val2"}},
		{"removed end unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val2", "val3"}},

		{"added front", &[]string{"val2", "val3"}, &[]string{"val1", "val2", "val3"}},
		{"added front unordered", &[]string{"val3", "val1"}, &[]string{"val2", "val3", "val1"}},
		{"added middle", &[]string{"val1", "val3"}, &[]string{"val1", "val2", "val3"}},
		{"added middle unordered", &[]string{"val2", "val1"}, &[]string{"val2", "val3", "val1"}},
		{"added end", &[]string{"val1", "val2"}, &[]string{"val1", "val2", "val3"}},
		{"added end unordered", &[]string{"val2", "val3"}, &[]string{"val2", "val3", "val1"}},

		{"same element updated", &[]string{"val1", "val2", "val3"}, &[]string{"val1", "val4", "val3"}},
		{"same element updated unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val2", "val4", "val1"}},

		{"shuffled", &[]string{"val1", "val2", "val3"}, &[]string{"val3", "val1", "val2"}},
		{"shuffled unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val3", "val1", "val2"}},
		{"shuffled with duplicates", &[]string{"val1", "val2", "val3"}, &[]string{"val3", "val1", "val2", "val3"}},
		{"shuffled with duplicates unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val3", "val1", "val2", "val3"}},

		{"shuffled added front", &[]string{"val2", "val3"}, &[]string{"val1", "val3", "val2"}},
		{"shuffled added middle", &[]string{"val1", "val3"}, &[]string{"val3", "val2", "val1"}},
		{"shuffled added end", &[]string{"val1", "val2"}, &[]string{"val2", "val1", "val3"}},

		{"shuffled removed front", &[]string{"val1", "val2", "val3"}, &[]string{"val3", "val2"}},
		{"shuffled removed middle", &[]string{"val1", "val2", "val3"}, &[]string{"val3", "val1"}},
		{"shuffled removed end", &[]string{"val1", "val2", "val3"}, &[]string{"val2", "val1"}},

		{"two added", &[]string{"val1", "val2"}, &[]string{"val1", "val2", "val3", "val4"}},
		{"two removed", &[]string{"val1", "val2", "val3", "val4"}, &[]string{"val1", "val2"}},
		{"two added and two removed", &[]string{"val1", "val2", "val3", "val4"}, &[]string{"val1", "val2", "val5", "val6"}},
		{"two added and two removed shuffled, one overlaps", &[]string{"val1", "val2", "val3", "val4"}, &[]string{"val1", "val5", "val6", "val2"}},
		{"two added and two removed shuffled, no overlaps", &[]string{"val1", "val2", "val3", "val4"}, &[]string{"val5", "val6", "val1", "val2"}},
		{"two added and two removed shuffled, with duplicates", &[]string{"val1", "val2", "val3", "val4"}, &[]string{"val1", "val5", "val6", "val2", "val1", "val2"}},
	}

	type testOutput struct {
		initialValue *[]string
		changeValue  *[]string
		tfOut        string
		pulumiOut    string
		detailedDiff map[string]any
	}

	runTest := func(
		t *testing.T, schema schema.Resource, valueMaker func(*[]string) cty.Value, val1 *[]string, val2 *[]string,
		disableAccurateBridgePreviews bool,
	) {
		initialValue := valueMaker(val1)
		changeValue := valueMaker(val2)

		opts := []crosstests.DiffOption{}
		if disableAccurateBridgePreviews {
			opts = append(opts, crosstests.DiffDisableAccurateBridgePreviews())
		}
		diff := crosstests.Diff(t, &schema, map[string]cty.Value{"test": initialValue}, map[string]cty.Value{"test": changeValue}, opts...)

		autogold.ExpectFile(t, testOutput{
			initialValue: val1,
			changeValue:  val2,
			tfOut:        diff.TFOut,
			pulumiOut:    diff.PulumiOut,
			detailedDiff: diff.PulumiDiff.DetailedDiff,
		})
	}

	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					runTest(t, schemaValueMakerPair.res, schemaValueMakerPair.valueMaker, scenario.initialValue, scenario.changeValue, false)
				})
			}
		})
	}

	for _, schemaValueMakerPair := range computedSchemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					runTest(
						t, schemaValueMakerPair.res, schemaValueMakerPair.valueMaker, scenario.initialValue, scenario.changeValue, false,
					)
				})
			}
		})
	}
}
