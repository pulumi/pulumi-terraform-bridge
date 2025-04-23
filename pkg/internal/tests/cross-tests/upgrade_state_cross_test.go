// Copyright 2016-2025, Pulumi Corporation.
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

package crosstests

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// Check a scenario where a schema change is accompanied by a migration function that compensates.
func TestUpgrade_StateUpgraders(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	resourceBefore := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}

	resourceAfter := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"prop": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
			},
		},
		SchemaVersion: 1,
		StateUpgraders: []schema.StateUpgrader{{
			Version: 0,
			Type:    resourceBefore.CoreConfigSchema().ImpliedType(),
			Upgrade: func(
				ctx context.Context,
				rawState map[string]interface{},
				meta interface{},
			) (map[string]interface{}, error) {
				// Upgrade function is receiving the data as it was written.
				autogold.Expect(map[string]interface{}{"id": "newid", "prop": "one,two,three"}).Equal(t, rawState)

				s := rawState["prop"].(string)
				parts := strings.Split(s, ",")
				partsN := []int{}
				for _, p := range parts {
					parsed := map[string]int{"one": 1, "two": 2, "three": 3}
					partsN = append(partsN, parsed[p])
				}
				return map[string]any{"prop": partsN, "id": rawState["id"]}, nil
			},
		}},
	}

	result := runUpgradeStateTest(t, upgradeStateTestCase{
		Resource1:  resourceBefore,
		Resource2:  resourceAfter,
		Inputs1:    map[string]any{"prop": "one,two,three"},
		InputsMap1: resource.PropertyMap{"prop": resource.NewStringProperty("one,two,three")},
		Inputs2:    map[string]any{"prop": []any{int(1), int(2), int(3)}},
		InputsMap2: resource.PropertyMap{"props": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewNumberProperty(1),
			resource.NewNumberProperty(2),
			resource.NewNumberProperty(3),
		})},

		// Apparently in this case TF expects RawState to be received on the new schema.
		ExpectedRawStateType: resourceAfter.CoreConfigSchema().ImpliedType(),

		// Pulumi does not upgrade this resource to V2 because of a no-op update plan.
		// TODO[pulumi/pulumi-terraform-bridge#3008]
		SkipSchemaVersionAfterUpdateCheck: true,

		SkipPulumiRefresh: "TODO[pulumi/pulumi-terraform-bridge#3024]",
	})

	autogold.Expect([]upgradeStateTrace{
		{
			Phase: upgradeStateTestPhase("preview"),
			RawState: map[string]interface{}{
				"id":   "newid",
				"prop": "one,two,three",
			},
			Result: map[string]interface{}{
				"id": "newid",
				"prop": []int{
					1,
					2,
					3,
				},
			},
		},
		{
			Phase: upgradeStateTestPhase("update"),
			RawState: map[string]interface{}{
				"id":   "newid",
				"prop": "one,two,three",
			},
			Result: map[string]interface{}{
				"id": "newid",
				"prop": []int{
					1,
					2,
					3,
				},
			},
		},
	}).Equal(t, result.pulumiUpgrades)
	autogold.Expect([]upgradeStateTrace{
		{
			Phase: upgradeStateTestPhase("refresh"),
			RawState: map[string]interface{}{
				"id":   "newid",
				"prop": "one,two,three",
			},
			Result: map[string]interface{}{
				"id": "newid",
				"prop": []int{
					1,
					2,
					3,
				},
			},
		},
		{
			Phase: upgradeStateTestPhase("preview"),
			RawState: map[string]interface{}{
				"id":   "newid",
				"prop": "one,two,three",
			},
			Result: map[string]interface{}{
				"id": "newid",
				"prop": []int{
					1,
					2,
					3,
				},
			},
		},
	}).Equal(t, result.tfUpgrades)
}

// Pulumi removing MaxItems=1 without TF schema changes should be tolerated, without calling upgraders.
func TestUpgrade_Pulumi_Removes_MaxItems1(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := map[string]*schema.Schema{
		"obj": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"str": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"bool": {
						Type:     schema.TypeBool,
						Optional: true,
					},
				},
			},
		},
	}

	trueBool := true

	resourceBeforeAndAfter := &schema.Resource{Schema: sch}
	resourceInfoBefore := &info.Resource{Fields: map[string]*info.Schema{
		"obj": {
			MaxItemsOne: &trueBool,
		},
	}}

	tfInputs := map[string]any{
		"obj": []any{
			map[string]any{
				"str":  "Hello",
				"bool": true,
			},
		},
	}

	pmBefore := resource.NewPropertyMapFromMap(map[string]any{
		"obj": map[string]any{
			"str":  "Hello",
			"bool": true,
		},
	})

	pmAfter := resource.NewPropertyMapFromMap(map[string]any{
		"objs": []any{
			map[string]any{
				"str":  "Hello",
				"bool": true,
			},
		},
	})

	result := runUpgradeStateTest(t, upgradeStateTestCase{
		Resource1:            resourceBeforeAndAfter,
		ResourceInfo1:        resourceInfoBefore,
		Resource2:            resourceBeforeAndAfter,
		Inputs1:              tfInputs,
		InputsMap1:           pmBefore,
		Inputs2:              tfInputs,
		InputsMap2:           pmAfter,
		ExpectedRawStateType: resourceBeforeAndAfter.CoreConfigSchema().ImpliedType(),
	})

	autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiRefreshResult.Summary.ResourceChanges)
	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, result.pulumiPreviewResult.ChangeSummary)
	autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiUpResult.Summary.ResourceChanges)

	autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)
	autogold.Expect([]upgradeStateTrace{}).Equal(t, result.tfUpgrades)
}

// Pulumi adding MaxItems=1 without TF schema changes should be tolerated, without calling upgraders.
func TestUpgrade_Pulumi_Adds_MaxItems1(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := map[string]*schema.Schema{
		"obj": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"str": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"bool": {
						Type:     schema.TypeBool,
						Optional: true,
					},
				},
			},
		},
	}

	trueBool := true

	resourceBeforeAndAfter := &schema.Resource{Schema: sch}
	resourceInfoAfter := &info.Resource{Fields: map[string]*info.Schema{
		"obj": {
			MaxItemsOne: &trueBool,
		},
	}}

	tfInputs := map[string]any{
		"obj": []any{
			map[string]any{
				"str":  "Hello",
				"bool": true,
			},
		},
	}

	pmAfter := resource.NewPropertyMapFromMap(map[string]any{
		"obj": map[string]any{
			"str":  "Hello",
			"bool": true,
		},
	})

	pmBefore := resource.NewPropertyMapFromMap(map[string]any{
		"objs": []any{
			map[string]any{
				"str":  "Hello",
				"bool": true,
			},
		},
	})

	result := runUpgradeStateTest(t, upgradeStateTestCase{
		Resource1:            resourceBeforeAndAfter,
		Resource2:            resourceBeforeAndAfter,
		ResourceInfo2:        resourceInfoAfter,
		Inputs1:              tfInputs,
		InputsMap1:           pmBefore,
		Inputs2:              tfInputs,
		InputsMap2:           pmAfter,
		ExpectedRawStateType: resourceBeforeAndAfter.CoreConfigSchema().ImpliedType(),
	})

	// TODO why is there a refresh update? Is it an incomplete Read test implementation?
	autogold.Expect(&map[string]int{"same": 1, "update": 1}).Equal(t, result.pulumiRefreshResult.Summary.ResourceChanges)
	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, result.pulumiPreviewResult.ChangeSummary)
	autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiUpResult.Summary.ResourceChanges)

	autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)
	autogold.Expect([]upgradeStateTrace{}).Equal(t, result.tfUpgrades)
}

// Upstream adding MaxItems=1 without TF schema changes should be tolerated, without calling upgraders.
func TestUpgrade_Upstream_Adds_MaxItems1(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := func(maxItems int) map[string]*schema.Schema {
		return map[string]*schema.Schema{
			"obj": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: maxItems,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"str": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"bool": {
							Type:     schema.TypeBool,
							Optional: true,
						},
					},
				},
			},
		}
	}

	resourceBefore := &schema.Resource{Schema: sch(0)}
	resourceAfter := &schema.Resource{Schema: sch(1)}

	tfInputs := map[string]any{
		"obj": []any{
			map[string]any{
				"str":  "Hello",
				"bool": true,
			},
		},
	}

	pmBefore := resource.NewPropertyMapFromMap(map[string]any{
		"objs": []any{
			map[string]any{
				"str":  "Hello",
				"bool": true,
			},
		},
	})

	pmAfter := resource.NewPropertyMapFromMap(map[string]any{
		"obj": map[string]any{
			"str":  "Hello",
			"bool": true,
		},
	})

	result := runUpgradeStateTest(t, upgradeStateTestCase{
		Resource1:            resourceBefore,
		Resource2:            resourceAfter,
		Inputs1:              tfInputs,
		InputsMap1:           pmBefore,
		Inputs2:              tfInputs,
		InputsMap2:           pmAfter,
		ExpectedRawStateType: resourceAfter.CoreConfigSchema().ImpliedType(),
	})

	// TODO why is there a refresh update? Is it an incomplete Read test implementation?
	autogold.Expect(&map[string]int{"same": 1, "update": 1}).Equal(t, result.pulumiRefreshResult.Summary.ResourceChanges)
	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, result.pulumiPreviewResult.ChangeSummary)
	autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiUpResult.Summary.ResourceChanges)

	autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)
	autogold.Expect([]upgradeStateTrace{}).Equal(t, result.tfUpgrades)
}

// Upstream removing MaxItems=1 without TF schema changes should be tolerated, without calling upgraders.
func TestUpgrade_Upstream_Removes_MaxItems1(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := func(maxItems int) map[string]*schema.Schema {
		return map[string]*schema.Schema{
			"obj": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: maxItems,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"str": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"bool": {
							Type:     schema.TypeBool,
							Optional: true,
						},
					},
				},
			},
		}
	}

	resourceBefore := &schema.Resource{Schema: sch(1)}
	resourceAfter := &schema.Resource{Schema: sch(0)}

	tfInputs := map[string]any{
		"obj": []any{
			map[string]any{
				"str":  "Hello",
				"bool": true,
			},
		},
	}

	pmBefore := resource.NewPropertyMapFromMap(map[string]any{
		"obj": map[string]any{
			"str":  "Hello",
			"bool": true,
		},
	})

	pmAfter := resource.NewPropertyMapFromMap(map[string]any{
		"objs": []any{
			map[string]any{
				"str":  "Hello",
				"bool": true,
			},
		},
	})

	result := runUpgradeStateTest(t, upgradeStateTestCase{
		Resource1:            resourceBefore,
		Resource2:            resourceAfter,
		Inputs1:              tfInputs,
		InputsMap1:           pmBefore,
		Inputs2:              tfInputs,
		InputsMap2:           pmAfter,
		ExpectedRawStateType: resourceAfter.CoreConfigSchema().ImpliedType(),
	})

	autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiRefreshResult.Summary.ResourceChanges)
	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, result.pulumiPreviewResult.ChangeSummary)
	autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiUpResult.Summary.ResourceChanges)

	autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)
	autogold.Expect([]upgradeStateTrace{}).Equal(t, result.tfUpgrades)
}

func TestUpgrade_UpgradersNotCalledWhenVersionIsNotChanging(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := map[string]*schema.Schema{
		"f0": {
			Type:     schema.TypeString,
			Optional: true,
		},
	}

	res1 := &schema.Resource{
		Schema:        sch,
		SchemaVersion: 1,
	}

	res2 := &schema.Resource{
		Schema:        sch,
		SchemaVersion: 1,
		StateUpgraders: []schema.StateUpgrader{{
			Version: 0,
			Type:    res1.CoreConfigSchema().ImpliedType(),
			Upgrade: nopUpgrade,
		}},
	}

	// Check when the values themselves are not changing.
	t.Run("same", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1: res1,
			Resource2: res2,
			Inputs1:   map[string]any{"f0": "val"},
			Inputs2:   map[string]any{"f0": "val"},
		})

		assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)
		autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)
	})

	// Check when the values are changing, and it is an effective update.
	t.Run("different", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1: res1,
			Resource2: res2,
			Inputs1:   map[string]any{"f0": "val1"},
			Inputs2:   map[string]any{"f0": "val2"},
		})

		assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)
		autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)
	})
}

// Basic check for upgrade logic: the type is not changing, but the schema is. Check how many times the upgrade method
// is called and if it is at parity.
func TestUpgrade_String_0to1_Version(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := map[string]*schema.Schema{
		"f0": {
			Type:     schema.TypeString,
			Optional: true,
		},
	}

	res1 := &schema.Resource{Schema: sch}

	res2 := &schema.Resource{
		Schema:        sch,
		SchemaVersion: 1,
		StateUpgraders: []schema.StateUpgrader{{
			Version: 0,
			Type:    res1.CoreConfigSchema().ImpliedType(),
			Upgrade: nopUpgrade,
		}},
	}

	// Check when the values themselves are not changing.
	t.Run("same", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1: res1,
			Resource2: res2,
			Inputs1:   map[string]any{"f0": "val"},
			Inputs2:   map[string]any{"f0": "val"},

			// Pulumi does not upgrade this resource to V2 because of a no-op update plan.
			// TODO[pulumi/pulumi-terraform-bridge#3008]
			SkipSchemaVersionAfterUpdateCheck: true,
		})

		autogold.Expect([]upgradeStateTrace{
			{
				Phase: upgradeStateTestPhase("refresh"),
				RawState: map[string]interface{}{
					"f0": "val",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val",
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("preview"),
				RawState: map[string]interface{}{
					"f0": "val",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val",
					"id": "newid",
				},
			},
		}).Equal(t, result.tfUpgrades)

		autogold.Expect([]upgradeStateTrace{
			{
				Phase: upgradeStateTestPhase("refresh"),
				RawState: map[string]interface{}{
					"f0": "val",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val",
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("preview"),
				RawState: map[string]interface{}{
					"f0": "val",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val",
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("update"),
				RawState: map[string]interface{}{
					"f0": "val",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val",
					"id": "newid",
				},
			},
		}).Equal(t, result.pulumiUpgrades)
	})

	// Check when the values are changing, and it is an effective update.
	t.Run("different", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1: res1,
			Resource2: res2,
			Inputs1:   map[string]any{"f0": "val1"},
			Inputs2:   map[string]any{"f0": "val2"},
		})

		autogold.Expect([]upgradeStateTrace{
			{
				Phase: upgradeStateTestPhase("refresh"),
				RawState: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("preview"),
				RawState: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
			},
		}).Equal(t, result.tfUpgrades)

		// Upgrade calls similar but Pulumi calls the upgrader a few times too many.
		autogold.Expect([]upgradeStateTrace{
			{
				Phase: upgradeStateTestPhase("refresh"),
				RawState: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("preview"),
				RawState: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("preview"),
				RawState: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("update"),
				RawState: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("update"),
				RawState: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": "val1",
					"id": "newid",
				},
			},
		}).Equal(t, result.pulumiUpgrades)
	})
}

// Same as the string upgrade test but with objects.
func TestUpgrade_Object_0to1_Version(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := map[string]*schema.Schema{
		"f0": {
			Required: true,
			Type:     schema.TypeList,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"x": {Optional: true, Type: schema.TypeString},
				},
			},
		},
	}

	res1 := &schema.Resource{
		Schema: sch,
	}

	res2 := &schema.Resource{
		Schema:        sch,
		SchemaVersion: 1,
		StateUpgraders: []schema.StateUpgrader{{
			Version: 0,
			Type:    res1.CoreConfigSchema().ImpliedType(),
			Upgrade: nopUpgrade,
		}},
	}

	configVal := func(val string) map[string]any {
		return map[string]any{
			"f0": []any{map[string]any{"x": val}},
		}
	}

	t.Run("same", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1: res1,
			Resource2: res2,
			Inputs1:   configVal("val"),
			Inputs2:   configVal("val"),

			SkipSchemaVersionAfterUpdateCheck: true,
		})

		autogold.Expect([]upgradeStateTrace{
			{
				Phase: upgradeStateTestPhase("refresh"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val"}},
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("preview"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val"}},
					"id": "newid",
				},
			},
		}).Equal(t, result.tfUpgrades)
		autogold.Expect([]upgradeStateTrace{
			{
				Phase: upgradeStateTestPhase("refresh"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val"}},
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("preview"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val"}},
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("update"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val"}},
					"id": "newid",
				},
			},
		}).Equal(t, result.pulumiUpgrades)
	})

	// Check when the values are changing, and it is an effective update.
	t.Run("different", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1: res1,
			Resource2: res2,
			Inputs1:   configVal("val1"),
			Inputs2:   configVal("val2"),
		})

		// Upgrade calls similar but Pulumi calls the upgrader a few times too many.
		autogold.Expect([]upgradeStateTrace{
			{
				Phase: upgradeStateTestPhase("refresh"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("preview"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
			},
		}).Equal(t, result.tfUpgrades)
		autogold.Expect([]upgradeStateTrace{
			{
				Phase: upgradeStateTestPhase("refresh"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("preview"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("preview"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("update"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
			},
			{
				Phase: upgradeStateTestPhase("update"),
				RawState: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
				Result: map[string]interface{}{
					"f0": []interface{}{map[string]interface{}{"x": "val1"}},
					"id": "newid",
				},
			},
		}).Equal(t, result.pulumiUpgrades)
	})
}

// In this upgrade scenario nothing is changing in TF but Pulumi is renaming a property. State upgraders are not
// invoked but Pulumi should handle the renaming seamlessly.
func TestUpgrade_PulumiRenamesProperty(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)

	sch := map[string]*schema.Schema{
		"f0": {
			Type:     schema.TypeString,
			Optional: true,
		},
	}

	res1 := &schema.Resource{
		Schema:        sch,
		SchemaVersion: 1,
	}

	res2 := &schema.Resource{
		Schema:        sch,
		SchemaVersion: 1,
		StateUpgraders: []schema.StateUpgrader{{
			Version: 0,
			Type:    res1.CoreConfigSchema().ImpliedType(),
			Upgrade: nopUpgrade,
		}},
	}

	res2Info := &info.Resource{
		Fields: map[string]*info.Schema{
			"f0": {Name: "f1"},
		},
	}

	t.Run("same", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1:     res1,
			Resource2:     res2,
			Inputs1:       map[string]any{"f0": "val"},
			Inputs2:       map[string]any{"f0": "val"},
			ResourceInfo2: res2Info,
		})

		assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)
		autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)

		autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiRefreshResult.Summary.ResourceChanges)
		autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiUpResult.Summary.ResourceChanges)
	})

	t.Run("different", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1:     res1,
			Resource2:     res2,
			Inputs1:       map[string]any{"f0": "val1"},
			Inputs2:       map[string]any{"f0": "val2"},
			ResourceInfo2: res2Info,
		})

		assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)
		autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)

		autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiRefreshResult.Summary.ResourceChanges)
		autogold.Expect(&map[string]int{"same": 1, "update": 1}).Equal(t, result.pulumiUpResult.Summary.ResourceChanges)
	})
}

// There are certain type changes permitted in Pulumi that coalesce naturally without state upgraders, that is
// reinterpreting a string as a number for example.
func TestUpgrade_PulumiChangesPropertyType(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)

	sch := map[string]*schema.Schema{
		"f0": {
			Type:     schema.TypeString,
			Optional: true,
		},
	}

	res1 := &schema.Resource{
		Schema:        sch,
		SchemaVersion: 1,
	}

	res2 := &schema.Resource{
		Schema:        sch,
		SchemaVersion: 1,
		StateUpgraders: []schema.StateUpgrader{{
			Version: 0,
			Type:    res1.CoreConfigSchema().ImpliedType(),
			Upgrade: nopUpgrade,
		}},
	}

	res2Info := &info.Resource{
		Fields: map[string]*info.Schema{
			"f0": {Type: "number"},
		},
	}

	t.Run("same", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1:     res1,
			Resource2:     res2,
			Inputs1:       map[string]any{"f0": "42"},
			InputsMap1:    resource.PropertyMap{"f0": resource.NewStringProperty("42")},
			Inputs2:       map[string]any{"f0": "42"},
			InputsMap2:    resource.PropertyMap{"f0": resource.NewNumberProperty(42)},
			ResourceInfo2: res2Info,
		})

		assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)
		autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)

		autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiRefreshResult.Summary.ResourceChanges)
		autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiUpResult.Summary.ResourceChanges)
	})

	t.Run("different", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1:     res1,
			Resource2:     res2,
			Inputs1:       map[string]any{"f0": "42"},
			InputsMap1:    resource.PropertyMap{"f0": resource.NewStringProperty("42")},
			Inputs2:       map[string]any{"f0": "7"},
			InputsMap2:    resource.PropertyMap{"f0": resource.NewNumberProperty(7)},
			ResourceInfo2: res2Info,
		})

		assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)
		autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)

		autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.pulumiRefreshResult.Summary.ResourceChanges)
		autogold.Expect(&map[string]int{"same": 1, "update": 1}).Equal(t, result.pulumiUpResult.Summary.ResourceChanges)
	})
}

// When downgrading to a lower schema, TF fails.
func TestUpgrade_Downgrading(t *testing.T) {
	t.Parallel()
	skipUnlessLinux(t)

	sch := map[string]*schema.Schema{
		"f0": {
			Type:     schema.TypeString,
			Optional: true,
		},
	}

	res1 := &schema.Resource{
		Schema:        sch,
		SchemaVersion: 2,
	}

	res2 := &schema.Resource{
		Schema:        sch,
		SchemaVersion: 1,
		StateUpgraders: []schema.StateUpgrader{{
			Version: 0,
			Type:    res1.CoreConfigSchema().ImpliedType(),
			Upgrade: nopUpgrade,
		}},
	}

	// Check when the values themselves are not changing.
	t.Run("same", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1:     res1,
			Resource2:     res2,
			Inputs1:       map[string]any{"f0": "val"},
			Inputs2:       map[string]any{"f0": "val"},
			ExpectFailure: true,
			SkipPulumi:    "TODO[pulumi-terraform-bridge#3009]",
		})

		assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)
		autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)
	})

	// Check when the values are changing, and it is an effective update.
	t.Run("different", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1:     res1,
			Resource2:     res2,
			Inputs1:       map[string]any{"f0": "val1"},
			Inputs2:       map[string]any{"f0": "val2"},
			ExpectFailure: true,
			SkipPulumi:    "TODO[pulumi-terraform-bridge#3009]",
		})

		assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)
		autogold.Expect([]upgradeStateTrace{}).Equal(t, result.pulumiUpgrades)
	})
}
