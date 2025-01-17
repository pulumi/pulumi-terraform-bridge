package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestSDKv2DetailedDiffList(t *testing.T) {
	t.Parallel()

	listAttrSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}

	listAttrSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				ForceNew: true,
			},
		},
	}

	maxItemsOneAttrSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}

	maxItemsOneAttrSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem:     &schema.Schema{Type: schema.TypeString},
				ForceNew: true,
			},
		},
	}

	listBlockSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
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

	listBlockSchemaForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
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

	listBlockSchemaNestedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
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

	maxItemsOneBlockSchema := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
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
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
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

	maxItemsOneBlockSchemaNestedForceNew := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
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

	listBlockSchemaSensitive := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:      schema.TypeList,
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

	listBlockSchemaNestedSensitive := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
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

	listBlockSchemaNestedDefault := schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
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

	listPairs := []diffSchemaValueMakerPair[[]string]{
		{"list attribute", listAttrSchema, listValueMaker},
		{"list attribute force new", listAttrSchemaForceNew, listValueMaker},
		{"list block", listBlockSchema, nestedListValueMaker},
		{"list block force new", listBlockSchemaForceNew, nestedListValueMaker},
		{"list block nested force new", listBlockSchemaNestedForceNew, nestedListValueMaker},
		{"list block sensitive", listBlockSchemaSensitive, nestedListValueMaker},
		{"list block nested sensitive", listBlockSchemaNestedSensitive, nestedListValueMaker},
		{"list block nested default", listBlockSchemaNestedDefault, nestedListValueMaker},
		{
			"list block nested default with default specified in program",
			listBlockSchemaNestedDefault, nestedListValueMakerWithDefaultSpecified,
		},
	}

	maxItemsOnePairs := []diffSchemaValueMakerPair[[]string]{
		{"max items one attribute", maxItemsOneAttrSchema, listValueMaker},
		{"max items one attribute force new", maxItemsOneAttrSchemaForceNew, listValueMaker},
		{"max items one block", maxItemsOneBlockSchema, nestedListValueMaker},
		{"max items one block force new", maxItemsOneBlockSchemaForceNew, nestedListValueMaker},
		{"max items one block nested force new", maxItemsOneBlockSchemaNestedForceNew, nestedListValueMaker},
	}

	oneElementScenarios := []diffScenario[[]string]{
		{"unchanged empty", nil, nil},
		{"unchanged non-empty", ref([]string{"val1"}), ref([]string{"val1"})},
		{"added non-empty", nil, ref([]string{"val1"})},
		{"added empty", nil, ref([]string{})},
		{"removed non-empty", ref([]string{"val1"}), nil},
		{"removed empty", ref([]string{}), nil},
		{"changed", ref([]string{"val1"}), ref([]string{"val2"})},
	}

	longList := &[]string{}
	for i := 0; i < 20; i++ {
		*longList = append(*longList, fmt.Sprintf("value%d", i))
	}
	longListAddedBack := append([]string{}, *longList...)
	longListAddedBack = append(longListAddedBack, "value20")
	longListAddedFront := append([]string{"value20"}, *longList...)

	multiElementScenarios := []diffScenario[[]string]{
		{"list element added front", ref([]string{"val2", "val3"}), ref([]string{"val1", "val2", "val3"})},
		{"list element added back", ref([]string{"val1", "val2"}), ref([]string{"val1", "val2", "val3"})},
		{"list element added middle", ref([]string{"val1", "val3"}), ref([]string{"val1", "val2", "val3"})},
		{"list element removed front", ref([]string{"val1", "val2", "val3"}), ref([]string{"val2", "val3"})},
		{"list element removed middle", ref([]string{"val1", "val2", "val3"}), ref([]string{"val1", "val3"})},
		{"list element removed end", ref([]string{"val1", "val2", "val3"}), ref([]string{"val1", "val2"})},
		{"one added, one removed", ref([]string{"val1", "val2", "val3"}), ref([]string{"val2", "val3", "val4"})},
		{"long list added back", longList, &longListAddedBack},
		// TODO[pulumi/pulumi-terraform-bridge#2239]: These cases present as multiple changes instead of just one
		{"long list added front", longList, &longListAddedFront},
		{"long list removed front", &longListAddedFront, longList},
		{"long list removed back", &longListAddedBack, longList},
	}

	scenarios := append(oneElementScenarios, multiElementScenarios...)

	runSDKv2TestMatrix(t, listPairs, scenarios)
	runSDKv2TestMatrix(t, maxItemsOnePairs, oneElementScenarios)
}
