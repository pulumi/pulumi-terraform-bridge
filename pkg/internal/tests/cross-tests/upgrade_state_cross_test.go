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
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestUpgrade_UpgradersNotCalledWhenVersionIsNotChanging(t *testing.T) {
	t.Parallel()

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
			SkipSchemaVersionAfterUpdateCheck: true,
		})

		assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)
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

		// Almost equal but not quite - see below.
		// assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)

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
		}).Equal(t, result.tfUpgrades)

		// There seems to be a problem that Pulumi runs the upgrade function 3 times needlessly during update.
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
		assert.Equal(t, result.tfUpgrades, result.pulumiUpgrades)
	})

	// Check when the values are changing, and it is an effective update.
	t.Run("different", func(t *testing.T) {
		result := runUpgradeStateTest(t, upgradeStateTestCase{
			Resource1: res1,
			Resource2: res2,
			Inputs1:   configVal("val1"),
			Inputs2:   configVal("val2"),
		})

		// Upgrade calls are not at parity:

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
	})
}

// There are certain type changes permitted in Pulumi that coalesce naturally without state upgraders, that is
// reinterpreting a string as a number for example.
func TestUpgrade_PulumiChangesPropertyType(t *testing.T) {
	t.Parallel()

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
	})
}
