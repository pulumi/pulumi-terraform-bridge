package tests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
)

func oneElementScenarios() []diffScenario[[]string] {
	return []diffScenario[[]string]{
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
	}
}

func setScenarios() []diffScenario[[]string] {
	scenarios := oneElementScenarios()
	multiElementScenarios := []diffScenario[[]string]{
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

	return append(scenarios, multiElementScenarios...)
}

func TestSDKv2DetailedDiffSetAttribute(t *testing.T) {
	t.Parallel()

	attributeSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
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
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				ForceNew: true,
			},
		},
	}

	diffSchemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		{"attribute no force new", attributeSchema, listValueMaker},
		{"attribute force new", attributeSchemaForceNew, listValueMaker},
	}

	runSDKv2TestMatrix(t, diffSchemaValueMakerPairs, setScenarios())
}

func TestSDKv2DetailedDiffSetBlock(t *testing.T) {
	t.Parallel()

	blockSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
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
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
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
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
		},
	}

	diffSchemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		{"block no force new", blockSchema, nestedListValueMaker},
		{"block top level force new", blockSchemaForceNew, nestedListValueMaker},
		{"block nested force new", blockSchemaNestedForceNew, nestedListValueMaker},
	}

	runSDKv2TestMatrix(t, diffSchemaValueMakerPairs, setScenarios())
}

func TestSDKv2DetailedDiffSetComputedAttribute(t *testing.T) {
	t.Parallel()

	computedAttributeSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
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
			setHashFunc := d.Get("prop").(*schema.Set).F
			err := d.Set("prop", schema.NewSet(setHashFunc, []interface{}{"computed"}))
			contract.Assertf(err == nil, "failed to set attribute: %v", err)
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			if d.Get("prop") == nil {
				setHashFunc := d.Get("prop").(*schema.Set).F
				err := d.Set("prop", schema.NewSet(setHashFunc, []interface{}{"computed"}))
				contract.Assertf(err == nil, "failed to set attribute: %v", err)
			}
			return nil
		},
	}

	computedAttributeSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
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
			if d.Get("prop") == nil {
				err := d.Set("prop", schema.NewSet(schema.HashString, []interface{}{"computed"}))
				contract.Assertf(err == nil, "failed to set attribute: %v", err)
			}
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			if d.Get("prop") == nil {
				err := d.Set("prop", schema.NewSet(schema.HashString, []interface{}{"computed"}))
				contract.Assertf(err == nil, "failed to set attribute: %v", err)
			}
			return nil
		},
	}

	diffSchemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		{"computed attribute no force new", computedAttributeSchema, listValueMaker},
		{"computed attribute force new", computedAttributeSchemaForceNew, listValueMaker},
	}

	runSDKv2TestMatrix(t, diffSchemaValueMakerPairs, setScenarios())
}

func computedSetBlockAttributeFunc(_ context.Context, d *schema.ResourceData, _ interface{}) {
	contract.Assertf(d.Get("prop") != nil, "test attribute is nil")
	testVals := d.Get("prop").(*schema.Set).List()
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
	setHashFunc := d.Get("prop").(*schema.Set).F
	err := d.Set("prop", schema.NewSet(setHashFunc, testVals))
	contract.Assertf(err == nil, "failed to set attribute: %v", err)
}

func computedSetBlockFunc(ctx context.Context, d *schema.ResourceData, i interface{}) {
	if d.Get("prop") == nil {
		setHashFunc := d.Get("prop").(*schema.Set).F
		err := d.Set("prop", schema.NewSet(setHashFunc, []interface{}{
			map[string]interface{}{
				"nested_prop": "computed",
				"computed":    "computed1",
			},
		}))
		contract.Assertf(err == nil, "failed to set attribute: %v", err)
	} else {
		computedSetBlockAttributeFunc(ctx, d, i)
	}
}

func TestSDKv2DetailedDiffSetComputedBlock(t *testing.T) {
	t.Parallel()

	blockSchemaComputed := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
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
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
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
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
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

	schemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		{"block with computed no replace", blockSchemaComputed, nestedListValueMaker},
		{"block with computed no replace computed specified in program", blockSchemaComputed, nestedListValueMakerWithComputedSpecified},
		{"block with computed force new", blockSchemaComputedForceNew, nestedListValueMaker},
		{"block with computed force new computed specified in program", blockSchemaComputedForceNew, nestedListValueMakerWithComputedSpecified},
		{"block with computed and nested force new", blockSchemaComputedNestedForceNew, nestedListValueMaker},
		{"block with computed and nested force new computed specified in program", blockSchemaComputedNestedForceNew, nestedListValueMakerWithComputedSpecified},
	}

	runSDKv2TestMatrix(t, schemaValueMakerPairs, setScenarios())
}

func TestSDKv2DetailedDiffSetNestedComputedBlock(t *testing.T) {
	t.Parallel()

	blockSchemaNestedComputed := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
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
			if d.Get("prop") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			if d.Get("prop") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
	}

	blockSchemaNestedComputedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
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
			if d.Get("prop") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			if d.Get("prop") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
	}

	blockSchemaNestedComputedNestedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
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
			if d.Get("prop") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
		UpdateContext: func(ctx context.Context, d *schema.ResourceData, i interface{}) diag.Diagnostics {
			if d.Get("prop") != nil {
				computedSetBlockAttributeFunc(ctx, d, i)
			}
			return nil
		},
	}

	schemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		{"block with nested computed no replace", blockSchemaNestedComputed, nestedListValueMaker},
		{"block with nested computed no replace computed specified in program", blockSchemaNestedComputed, nestedListValueMakerWithComputedSpecified},
		{"block with nested computed and force new", blockSchemaNestedComputedForceNew, nestedListValueMaker},
		{"block with nested computed and force new computed specified in program", blockSchemaNestedComputedForceNew, nestedListValueMakerWithComputedSpecified},
		{"block with nested computed and nested force new", blockSchemaNestedComputedNestedForceNew, nestedListValueMaker},
		{"block with nested computed and nested force new computed specified", blockSchemaNestedComputedNestedForceNew, nestedListValueMakerWithComputedSpecified},
	}

	runSDKv2TestMatrix(t, schemaValueMakerPairs, setScenarios())
}

func TestSDKv2DetailedDiffSetBlockSensitive(t *testing.T) {
	t.Parallel()

	blockSchemaSensitive := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:      schema.TypeSet,
				Optional:  true,
				Sensitive: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	blockSchemaNestedSensitive := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:      schema.TypeString,
							Optional:  true,
							Sensitive: true,
						},
					},
				},
			},
		},
	}

	diffSchemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		{"block sensitive", blockSchemaSensitive, nestedListValueMaker},
		{"block nested sensitive", blockSchemaNestedSensitive, nestedListValueMaker},
	}

	runSDKv2TestMatrix(t, diffSchemaValueMakerPairs, setScenarios())
}

func TestSDKv2DetailedDiffSetDefault(t *testing.T) {
	t.Parallel()
	// Note Default is not valid for set types.

	blockSchemaNestedDefault := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"default": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "default",
						},
					},
				},
			},
		},
	}

	diffSchemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		{"block nested default", blockSchemaNestedDefault, nestedListValueMaker},
		{"block nested default with default specified in program", blockSchemaNestedDefault, nestedListValueMakerWithDefaultSpecified},
	}

	runSDKv2TestMatrix(t, diffSchemaValueMakerPairs, setScenarios())
}

// TestSDKv2DetailedDiffSetBlockOptionalUnspecified reproduces the issue where
// adding an element to a TypeSet of blocks falls back to a whole-set UPDATE
// when the block has an Optional (non-Computed, no Default) field that the user
// doesn't specify. TF fills in the zero value ("") in the planned state, but the
// user input has null for that field, causing validInputsFromPlan to fail.
//
// This is the root cause of noisy diffs on resources like google_compute_url_map
// where host_rule has an optional description field.
//
// See https://github.com/pulumi/pulumi-terraform-bridge/issues/3324
func TestSDKv2DetailedDiffSetBlockOptionalUnspecified(t *testing.T) {
	t.Parallel()

	// Schema mimics google_compute_url_map's host_rule:
	// - nested_prop is like pathMatcher/hosts (Required)
	// - description is Optional, not Computed, no Default
	blockSchemaOptionalUnspecified := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Required: true,
						},
						"description": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	diffSchemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		// User only specifies nested_prop, never description.
		// TF plan will fill description with "" for all elements.
		{"block optional unspecified", blockSchemaOptionalUnspecified, nestedListValueMaker},
	}

	runSDKv2TestMatrix(t, diffSchemaValueMakerPairs, setScenarios())
}

func TestSDKv2DetailedDiffSetMaxItemsOne(t *testing.T) {
	t.Parallel()

	maxItemsOneAttrSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}

	maxItemsOneAttrSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}

	maxItemsOneBlockSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	maxItemsOneBlockSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}

	maxItemsOneBlockSchemaNestedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
		},
	}

	diffSchemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		{"max items one attribute", maxItemsOneAttrSchema, listValueMaker},
		{"max items one attribute force new", maxItemsOneAttrSchemaForceNew, listValueMaker},
		{"max items one block", maxItemsOneBlockSchema, nestedListValueMaker},
		{"max items one block force new", maxItemsOneBlockSchemaForceNew, nestedListValueMaker},
		{"max items one block nested force new", maxItemsOneBlockSchemaNestedForceNew, nestedListValueMaker},
	}

	runSDKv2TestMatrix(t, diffSchemaValueMakerPairs, oneElementScenarios())
}

func TestSDKv2DetailedDiffRegressGCP2953(t *testing.T) {
	t.Parallel()

	res := schema.Resource{
		Schema: map[string]*schema.Schema{
			"rule": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"src_ip_ranges": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		},
	}

	tfp := &schema.Provider{ResourcesMap: map[string]*schema.Resource{
		"prov_test": &res,
	}}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)

	program := `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      rules:
        - srcIpRanges:
            - "*"`

	pt := pulcheck.PulCheck(t, bridgedProvider, program)
	pt.Up(t)

	prevRes := pt.Preview(t, optpreview.Diff())
	require.NotContains(t, prevRes.StdErr, "Failed to calculate preview for element")
}

func TestSDKv2DetailedDiffNestedSets(t *testing.T) {
	t.Parallel()

	res := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nested_prop": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		},
	}

	nestedSetValueMaker := func(v *[]string) map[string]cty.Value {
		if v == nil {
			return map[string]cty.Value{}
		}
		if len(*v) == 0 {
			nested := map[string]cty.Value{
				"nested_prop": cty.ListValEmpty(cty.String),
			}
			return map[string]cty.Value{
				"prop": cty.ListVal([]cty.Value{cty.ObjectVal(nested)}),
			}
		}

		nestedSet := make([]cty.Value, len(*v))
		for i, v := range *v {
			nestedSet[i] = cty.StringVal(v)
		}
		nested := map[string]cty.Value{
			"nested_prop": cty.ListVal(nestedSet),
		}

		return map[string]cty.Value{
			"prop": cty.ListVal([]cty.Value{cty.ObjectVal(nested)}),
		}
	}

	diffSchemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		{"nested set", res, nestedSetValueMaker},
	}

	runSDKv2TestMatrix(t, diffSchemaValueMakerPairs, setScenarios())
}

func TestSDKv2DetailedDiffSetNestedList(t *testing.T) {
	t.Parallel()

	res := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeList,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		},
	}

	listNestedListValueMaker := func(v *[]string) map[string]cty.Value {
		if v == nil {
			return map[string]cty.Value{}
		}

		if len(*v) == 0 {
			return map[string]cty.Value{
				"prop": cty.ListVal([]cty.Value{cty.ListValEmpty(cty.String)}),
			}
		}

		slice := make([]cty.Value, len(*v))
		for i, v := range *v {
			slice[i] = cty.StringVal(v)
		}

		return map[string]cty.Value{
			"prop": cty.ListVal([]cty.Value{cty.ListVal(slice)}),
		}
	}

	diffSchemaValueMakerPairs := []diffSchemaValueMakerPair[[]string]{
		{"nested list", res, listNestedListValueMaker},
	}

	runSDKv2TestMatrix(t, diffSchemaValueMakerPairs, oneElementScenarios())
}
