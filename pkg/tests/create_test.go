package tests

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	testutils "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func TestIntToStringOverride(t *testing.T) {
	t.Parallel()

	const largeInt = math.MaxInt64 / 2

	crosstests.Create(t,
		map[string]*schema.Schema{
			"f0": {Required: true, Type: schema.TypeInt},
		},
		cty.ObjectVal(map[string]cty.Value{
			"f0": cty.NumberIntVal(largeInt),
		}),
		crosstests.CreatePulumiConfig(resource.PropertyMap{
			"f0": resource.NewProperty(strconv.FormatInt(largeInt, 10)),
		}),
		crosstests.CreateResourceInfo(info.Resource{Fields: map[string]*info.Schema{
			"f0": {Type: "string"},
		}}),
	)
}

// Regression test for [pulumi/pulumi-terraform-bridge#1762]
func TestInputsConfigModeEqual(t *testing.T) {
	t.Parallel()

	emptyConfig := cty.ObjectVal(map[string]cty.Value{})

	emptyListConfig := cty.ObjectVal(map[string]cty.Value{
		"f0": cty.ListValEmpty(cty.Object(map[string]cty.Type{
			"x": cty.String,
		})),
	})

	nonEmptyConfig := cty.ObjectVal(map[string]cty.Value{
		"f0": cty.ListVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"x": cty.StringVal("val"),
			}),
		}),
	})

	for _, tc := range []struct {
		name       string
		config     cty.Value
		maxItems   int
		configMode schema.SchemaConfigMode
	}{
		{"MaxItems: 0, ConfigMode: Auto, Empty", emptyConfig, 0, schema.SchemaConfigModeAuto},
		{"MaxItems: 0, ConfigMode: Auto, EmptyList", emptyListConfig, 0, schema.SchemaConfigModeAuto},
		{"MaxItems: 0, ConfigMode: Auto, NonEmpty", nonEmptyConfig, 0, schema.SchemaConfigModeAuto},
		{"MaxItems: 0, ConfigMode: Block, Empty", emptyConfig, 0, schema.SchemaConfigModeBlock},
		{"MaxItems: 0, ConfigMode: Block, EmptyList", emptyListConfig, 0, schema.SchemaConfigModeBlock},
		{"MaxItems: 0, ConfigMode: Block, NonEmpty", nonEmptyConfig, 0, schema.SchemaConfigModeBlock},
		{"MaxItems: 0, ConfigMode: Attr, Empty", emptyConfig, 0, schema.SchemaConfigModeAttr},
		{"MaxItems: 0, ConfigMode: Attr, EmptyList", emptyListConfig, 0, schema.SchemaConfigModeAttr},
		{"MaxItems: 0, ConfigMode: Attr, NonEmpty", nonEmptyConfig, 0, schema.SchemaConfigModeAttr},
		{"MaxItems: 1, ConfigMode: Auto, Empty", emptyConfig, 1, schema.SchemaConfigModeAuto},
		{"MaxItems: 1, ConfigMode: Auto, EmptyList", emptyListConfig, 1, schema.SchemaConfigModeAuto},
		{"MaxItems: 1, ConfigMode: Auto, NonEmpty", nonEmptyConfig, 1, schema.SchemaConfigModeAuto},
		{"MaxItems: 1, ConfigMode: Block, Empty", emptyConfig, 1, schema.SchemaConfigModeBlock},
		{"MaxItems: 1, ConfigMode: Block, EmptyList", emptyListConfig, 1, schema.SchemaConfigModeBlock},
		{"MaxItems: 1, ConfigMode: Block, NonEmpty", nonEmptyConfig, 1, schema.SchemaConfigModeBlock},
		{"MaxItems: 1, ConfigMode: Attr, Empty", emptyConfig, 1, schema.SchemaConfigModeAttr},
		// TODO[pulumi/pulumi-terraform-bridge#2025]
		// This is not expressible in pulumi after the ConfigModeOne flattening.
		// {"MaxItems: 1, ConfigMode: Attr, EmptyList", emptyListConfig, 1, schema.SchemaConfigModeAttr},
		{"MaxItems: 1, ConfigMode: Attr, NonEmpty", nonEmptyConfig, 1, schema.SchemaConfigModeAttr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			crosstests.Create(t,
				map[string]*schema.Schema{
					"f0": {
						Optional:   true,
						Type:       schema.TypeList,
						MaxItems:   tc.maxItems,
						ConfigMode: tc.configMode,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"x": {Optional: true, Type: schema.TypeString},
							},
						},
					},
				},
				tc.config,
			)
		})
	}
}

// Regression test for [pulumi/pulumi-terraform-bridge#1767]
func TestInputsUnspecifiedMaxItemsOne(t *testing.T) {
	t.Parallel()
	crosstests.Create(t,
		map[string]*schema.Schema{
			"f0": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"x": {Optional: true, Type: schema.TypeString},
					},
				},
			},
		},
		cty.ObjectVal(map[string]cty.Value{}),
	)
}

// Regression test for [pulumi/pulumi-terraform-bridge#1970] and [pulumi/pulumi-terraform-bridge#1964]
func TestOptionalSetNotSpecified(t *testing.T) {
	t.Parallel()
	crosstests.Create(t,
		map[string]*schema.Schema{
			"f0": {
				Optional: true,
				Type:     schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"x": {Optional: true, Type: schema.TypeString},
					},
				},
			},
		},
		cty.ObjectVal(map[string]cty.Value{}),
	)
}

// Regression test for [pulumi/pulumi-terraform-bridge#1915]
func TestInputsEqualEmptyList(t *testing.T) {
	t.Parallel()
	for _, maxItems := range []int{0, 1} {
		t.Run(fmt.Sprintf("MaxItems: %v", maxItems), func(t *testing.T) {
			crosstests.Create(t,
				map[string]*schema.Schema{
					"f0": {
						Optional: true,
						Type:     schema.TypeList,
						MaxItems: maxItems,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"x": {Optional: true, Type: schema.TypeString},
							},
						},
					},
				},
				cty.ObjectVal(map[string]cty.Value{
					"f0": cty.ListValEmpty(cty.String),
				}),
			)
		})
	}
}

// TestCreateDoesNotInvokeStateUpgraders ensures that state upgrade machinery is not
// invoked during Create operations.
func TestCreateDoesNotInvokeStateUpgraders(t *testing.T) {
	t.Parallel()
	resource := func() *schema.Resource {
		return &schema.Resource{Schema: map[string]*schema.Schema{
			"f0": {
				Type:     schema.TypeString,
				Optional: true,
			},
		}}
	}

	upgradeFunc := func(ctx context.Context, rawState map[string]any, meta any) (map[string]any, error) {
		panic("State upgraders should not be called during create")
	}

	crosstests.Create(t,
		resource().Schema,
		cty.ObjectVal(map[string]cty.Value{
			"f0": cty.StringVal("default"),
		}),
		crosstests.CreateStateUpgrader(1, []schema.StateUpgrader{
			{
				Type:    resource().CoreConfigSchema().ImpliedType(),
				Upgrade: upgradeFunc,
				Version: 0,
			},
		}),
	)
}

func TestTimeouts(t *testing.T) {
	t.Parallel()
	crosstests.Create(t,
		map[string]*schema.Schema{
			"tags": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Optional: true,
					Type:     schema.TypeString,
				},
			},
		},
		cty.EmptyObjectVal,
		crosstests.CreateTimeout(&schema.ResourceTimeout{
			Create: schema.DefaultTimeout(time.Duration(120)),
		}),
	)
}

func TestMap(t *testing.T) {
	t.Parallel()

	crosstests.Create(t,
		map[string]*schema.Schema{
			"tags": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Optional: true,
					Type:     schema.TypeString,
				},
			},
		},
		cty.ObjectVal(map[string]cty.Value{
			"tags": cty.MapVal(map[string]cty.Value{
				"key":  cty.StringVal("val"),
				"key2": cty.StringVal("val2"),
			}),
		}),
	)
}

func TestEmptySetOfEmptyObjects(t *testing.T) {
	t.Parallel()

	crosstests.Create(t,
		map[string]*schema.Schema{
			"d3f0": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Resource{Schema: map[string]*schema.Schema{}},
			},
		},
		cty.ObjectVal(map[string]cty.Value{
			"d3f0": cty.SetValEmpty(cty.EmptyObject),
		}),
	)
}

func TestInputsEmptyString(t *testing.T) {
	t.Parallel()

	crosstests.Create(t,
		map[string]*schema.Schema{
			"f0": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
		cty.ObjectVal(map[string]cty.Value{
			"f0": cty.StringVal(""),
		}),
	)
}

func TestInputsNestedBlocksEmpty(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		typ1   schema.ValueType
		typ2   schema.ValueType
		config cty.Value
	}{
		{"empty list list block", schema.TypeList, schema.TypeList, cty.EmptyObjectVal},
		{"empty set set block", schema.TypeSet, schema.TypeSet, cty.EmptyObjectVal},
		{"empty list set block", schema.TypeList, schema.TypeSet, cty.EmptyObjectVal},
		{"non empty list list block", schema.TypeList, schema.TypeList, cty.ObjectVal(map[string]cty.Value{
			"f0": cty.ListValEmpty(cty.List(cty.Object(map[string]cty.Type{"f2": cty.String}))),
		})},
		{"nested non empty list list block", schema.TypeList, schema.TypeList, cty.ObjectVal(map[string]cty.Value{
			"f0": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"f2": cty.StringVal("val")})}),
		})},
		{"nested non empty set set block", schema.TypeSet, schema.TypeSet, cty.ObjectVal(map[string]cty.Value{
			"f0": cty.SetVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"f2": cty.StringVal("val")})}),
		})},
	} {
		t.Run(tc.name, crosstests.MakeCreate(
			map[string]*schema.Schema{
				"f0": {
					Type:     tc.typ1,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"f1": {
								Type:     tc.typ2,
								Optional: true,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"x": {Optional: true, Type: schema.TypeString},
									},
								},
							},
							// This allows us to specify non-empty f0s with an empty f1
							"f2": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
			},
			tc.config,
		))
	}
}

func TestExplicitNilList(t *testing.T) {
	t.Parallel()

	// This is an explicit null on the tf side:
	// resource "crossprovider_testres" "example" {
	//     f0 = null
	// }
	crosstests.Create(t,
		map[string]*schema.Schema{
			"f0": {
				Optional: true,
				Type:     schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
					Elem: &schema.Schema{
						Type: schema.TypeInt,
					},
				},
			},
		},
		cty.ObjectVal(map[string]cty.Value{"f0": cty.NullVal(cty.List(cty.Map(cty.Number)))}),
	)
}

func TestInputsEmptyCollections(t *testing.T) {
	t.Parallel()

	// signifies a block
	resourceElem := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"x": {Optional: true, Type: schema.TypeString},
		},
	}

	// signifies an attribute
	schemaElem := &schema.Schema{
		Type: schema.TypeMap,
		Elem: &schema.Schema{Type: schema.TypeString},
	}

	for _, tc := range []struct {
		name       string
		maxItems   int
		typ        schema.ValueType
		elem       any
		configMode schema.SchemaConfigMode
	}{
		{"list block", 0, schema.TypeList, resourceElem, schema.SchemaConfigModeAuto},
		{"set block", 0, schema.TypeSet, resourceElem, schema.SchemaConfigModeAuto},
		// TypeMap with Elem *Resource not supported
		// {"map block", 0, schema.TypeMap, resourceElem, schema.SchemaConfigModeAuto},
		{"list max items one block", 1, schema.TypeList, resourceElem, schema.SchemaConfigModeAuto},
		{"set max items one block", 1, schema.TypeSet, resourceElem, schema.SchemaConfigModeAuto},
		// MaxItems is only valid on lists and sets
		// {"map max items one block", 1, schema.TypeMap, resourceElem, schema.SchemaConfigModeAuto},
		{"list attr", 0, schema.TypeList, schemaElem, schema.SchemaConfigModeAuto},
		{"set attr", 0, schema.TypeSet, schemaElem, schema.SchemaConfigModeAuto},
		{"map attr", 0, schema.TypeMap, schemaElem, schema.SchemaConfigModeAuto},
		{"list max items one attr", 1, schema.TypeList, schemaElem, schema.SchemaConfigModeAuto},
		{"set max items one attr", 1, schema.TypeSet, schemaElem, schema.SchemaConfigModeAuto},
		// MaxItems is only valid on lists and sets
		// {"map max items one attr", 1, schema.TypeMap, schemaElem, schema.SchemaConfigModeAuto},
		{"list config mode attr", 0, schema.TypeList, resourceElem, schema.SchemaConfigModeAttr},
		{"set config mode attr", 0, schema.TypeSet, resourceElem, schema.SchemaConfigModeAttr},
	} {
		t.Run(tc.name, crosstests.MakeCreate(
			map[string]*schema.Schema{
				"f0": {
					Type:       tc.typ,
					MaxItems:   tc.maxItems,
					Elem:       tc.elem,
					ConfigMode: tc.configMode,
					Optional:   true,
				},
			},
			cty.EmptyObjectVal,
		))
	}
}

func TestCreateFails(t *testing.T) {
	t.Parallel()

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Required: true,
				},
			},
			CreateContext: func(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
				return diag.Errorf("CREATE FAILURE")
			},
		},
	}
	prov := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)

	pt := pulcheck.PulCheck(t, bridgedProvider, `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
	properties:
	  test: "hello"
`)

	_, err := pt.CurrentStack().Up(pt.Context())
	require.Error(t, err)
	require.ErrorContains(t, err, "CREATE FAILURE")
}

func TestCreateUnrecognizedType(t *testing.T) {
	t.Parallel()

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"test": {
					Type:     schema.TypeString,
					Required: true,
				},
			},
		},
	}
	prov := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", prov)
	providerServer, err := pulcheck.ProviderServerFromInfo(context.Background(), bridgedProvider)
	require.NoError(t, err)

	testutils.Replay(t, providerServer, `
	{
		"method": "/pulumirpc.ResourceProvider/Create",
		"request": {
			"urn": "urn:pulumi:dev::teststack::prov:index/unknownResource:UnknownResource::exres",
			"properties": {
				"test": "hello"
			}
		},
		"errors": ["unrecognized resource type (Create): prov:index/unknownResource:UnknownResource"]
	}
	`)
}
