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

package tfbridgetests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	ct "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// Check a scenario where a schema change is accompanied by a migration function that compensates.
func TestPFUpgrade_StateUpgraders(t *testing.T) {
	t.Parallel()
	ct.SkipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	resourceBefore := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"prop": rschema.StringAttribute{Optional: true},
			},
		},
	})

	type v0Model struct {
		ID   string `tfsdk:"id"`
		Prop string `tfsdk:"prop"`
	}

	type v1Model struct {
		ID   string  `tfsdk:"id"`
		Prop []int32 `tfsdk:"prop"`
	}

	resourceAfter := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"prop": rschema.ListAttribute{Optional: true, ElementType: basetypes.Int64Type{}},
			},
			Version: 1,
		},
		UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
			return map[int64]resource.StateUpgrader{
				0: {
					PriorSchema: &resourceBefore.ResourceSchema,
					StateUpgrader: func(
						ctx context.Context,
						req resource.UpgradeStateRequest,
						resp *resource.UpgradeStateResponse,
					) {
						var priorState v0Model
						d0 := req.State.Get(ctx, &priorState)
						resp.Diagnostics = append(resp.Diagnostics, d0...)

						partsN := []int32{}
						parts := strings.Split(priorState.Prop, ",")
						for _, p := range parts {
							parsed := map[string]int32{"one": 1, "two": 2, "three": 3}
							partsN = append(partsN, parsed[p])
						}

						updState := v1Model{
							ID:   priorState.ID,
							Prop: partsN,
						}

						d := resp.State.Set(ctx, updState)
						resp.Diagnostics = append(resp.Diagnostics, d...)

						if d.HasError() {
							panic(fmt.Sprintf("D!: %v", resp.Diagnostics))
						}
					},
				},
			}
		},
	})

	tc := ct.UpgradeStateTestCase{
		Resource1:  &resourceBefore,
		Resource2:  &resourceAfter,
		Inputs1:    cty.ObjectVal(map[string]cty.Value{"prop": cty.StringVal("one,two,three")}),
		InputsMap1: presource.PropertyMap{"prop": presource.NewStringProperty("one,two,three")},
		Inputs2: cty.ObjectVal(map[string]cty.Value{"prop": cty.ListVal([]cty.Value{
			cty.NumberIntVal(1),
			cty.NumberIntVal(2),
			cty.NumberIntVal(3),
		})}),
		InputsMap2: presource.PropertyMap{"props": presource.NewArrayProperty([]presource.PropertyValue{
			presource.NewNumberProperty(1),
			presource.NewNumberProperty(2),
			presource.NewNumberProperty(3),
		})},

		// Apparently in this case TF expects RawState to be received on the new schema.
		ExpectedRawStateType: resourceAfter.ResourceSchema.Type().TerraformType(context.Background()),

		// Pulumi does not upgrade this resource to V2 because of a no-op update plan.
		// TODO[pulumi/pulumi-terraform-bridge#3008]
		SkipSchemaVersionAfterUpdateCheck: true,
	}

	result := tc.Run(t)

	autogold.Expect([]ct.UpgradeStateTrace{
		{
			Phase: ct.UpgradeStateTestPhase("preview"),
			PriorState: map[string]interface{}{
				"id":   "test-id",
				"prop": "one,two,three",
			},
			ReturnedState: map[string]interface{}{
				"id": "test-id",
				"prop": []interface{}{
					1,
					2,
					3,
				},
			},
		},
		{
			Phase: ct.UpgradeStateTestPhase("update"),
			PriorState: map[string]interface{}{
				"id":   "test-id",
				"prop": "one,two,three",
			},
			ReturnedState: map[string]interface{}{
				"id": "test-id",
				"prop": []interface{}{
					1,
					2,
					3,
				},
			},
		},
	}).Equal(t, result.PulumiUpgrades)
	autogold.Expect([]ct.UpgradeStateTrace{
		{
			Phase: ct.UpgradeStateTestPhase("refresh"),
			PriorState: map[string]interface{}{
				"id":   "test-id",
				"prop": "one,two,three",
			},
			ReturnedState: map[string]interface{}{
				"id": "test-id",
				"prop": []interface{}{
					1,
					2,
					3,
				},
			},
		},
		{
			Phase: ct.UpgradeStateTestPhase("preview"),
			PriorState: map[string]interface{}{
				"id":   "test-id",
				"prop": "one,two,three",
			},
			ReturnedState: map[string]interface{}{
				"id": "test-id",
				"prop": []interface{}{
					1,
					2,
					3,
				},
			},
		},
		{
			Phase: ct.UpgradeStateTestPhase("update"),
			PriorState: map[string]interface{}{
				"id":   "test-id",
				"prop": "one,two,three",
			},
			ReturnedState: map[string]interface{}{
				"id": "test-id",
				"prop": []interface{}{
					1,
					2,
					3,
				},
			},
		},
	}).Equal(t, result.TFUpgrades)
}

// Pulumi removing MaxItems=1 without TF schema changes should be tolerated, without calling upgraders.
func TestPFUpgrade_Pulumi_Removes_MaxItems1(t *testing.T) {
	t.Parallel()
	ct.SkipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	resourceBeforeAndAfter := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"obj": rschema.ListNestedAttribute{
					Optional: true,
					NestedObject: rschema.NestedAttributeObject{
						Attributes: map[string]rschema.Attribute{
							"str":  rschema.StringAttribute{Optional: true},
							"bool": rschema.BoolAttribute{Optional: true},
						},
					},
				},
			},
		},
	})

	trueBool := true

	resourceInfoBefore := &info.Resource{Fields: map[string]*info.Schema{
		"obj": {
			MaxItemsOne: &trueBool,
		},
	}}

	tfInputs := cty.ObjectVal(map[string]cty.Value{
		"obj": cty.ListVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"str":  cty.StringVal("Hello"),
				"bool": cty.BoolVal(true),
			}),
		}),
	})

	pmBefore := presource.NewPropertyMapFromMap(map[string]any{
		"obj": map[string]any{
			"str":  "Hello",
			"bool": true,
		},
	})

	pmAfter := presource.NewPropertyMapFromMap(map[string]any{
		"objs": []any{
			map[string]any{
				"str":  "Hello",
				"bool": true,
			},
		},
	})

	testCase := ct.UpgradeStateTestCase{
		Resource1:            &resourceBeforeAndAfter,
		ResourceInfo1:        resourceInfoBefore,
		Resource2:            &resourceBeforeAndAfter,
		Inputs1:              tfInputs,
		InputsMap1:           pmBefore,
		Inputs2:              tfInputs,
		InputsMap2:           pmAfter,
		ExpectedRawStateType: resourceBeforeAndAfter.ResourceSchema.Type().TerraformType(context.Background()),
	}
	result := testCase.Run(t)

	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, result.PulumiPreviewResult.ChangeSummary)
	autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.PulumiUpResult.Summary.ResourceChanges)

	autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.PulumiUpgrades)
	autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.TFUpgrades)
}

// Pulumi adding MaxItems=1 without TF schema changes should be tolerated, without calling upgraders.
func TestPFUpgrade_Pulumi_Adds_MaxItems1(t *testing.T) {
	t.Parallel()
	ct.SkipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	resourceBeforeAndAfter := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"obj": rschema.ListNestedAttribute{
					Optional: true,
					NestedObject: rschema.NestedAttributeObject{
						Attributes: map[string]rschema.Attribute{
							"str":  rschema.StringAttribute{Optional: true},
							"bool": rschema.BoolAttribute{Optional: true},
						},
					},
				},
			},
		},
	})

	trueBool := true

	resourceInfoAfter := &info.Resource{Fields: map[string]*info.Schema{
		"obj": {
			MaxItemsOne: &trueBool,
		},
	}}

	tfInputs := cty.ObjectVal(map[string]cty.Value{
		"obj": cty.ListVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"str":  cty.StringVal("Hello"),
				"bool": cty.BoolVal(true),
			}),
		}),
	})

	pmBefore := presource.NewPropertyMapFromMap(map[string]any{
		"objs": []any{
			map[string]any{
				"str":  "Hello",
				"bool": true,
			},
		},
	})

	pmAfter := presource.NewPropertyMapFromMap(map[string]any{
		"obj": map[string]any{
			"str":  "Hello",
			"bool": true,
		},
	})

	testCase := ct.UpgradeStateTestCase{
		Resource1:            &resourceBeforeAndAfter,
		Resource2:            &resourceBeforeAndAfter,
		ResourceInfo2:        resourceInfoAfter,
		Inputs1:              tfInputs,
		InputsMap1:           pmBefore,
		Inputs2:              tfInputs,
		InputsMap2:           pmAfter,
		ExpectedRawStateType: resourceBeforeAndAfter.ResourceSchema.Type().TerraformType(context.Background()),
	}

	result := testCase.Run(t)

	autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}).Equal(t, result.PulumiPreviewResult.ChangeSummary)
	autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.PulumiUpResult.Summary.ResourceChanges)

	autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.PulumiUpgrades)
	autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.TFUpgrades)
}

func TestPFUpgrade_UpgradersNotCalledWhenVersionIsNotChanging(t *testing.T) {
	t.Parallel()
	ct.SkipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := func(version int64) rschema.Schema {
		return rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"f0": rschema.StringAttribute{
					Optional: true,
				},
			},
			Version: version,
		}
	}

	res1 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(1),
	})

	res2 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(1),
		UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
			return map[int64]resource.StateUpgrader{
				0: {
					PriorSchema:   &res1.ResourceSchema,
					StateUpgrader: ct.NopUpgrader,
				},
			}
		},
	})

	configVal := func(val string) cty.Value {
		return cty.ObjectVal(map[string]cty.Value{
			"f0": cty.StringVal(val),
		})
	}

	propMap := func(val string) presource.PropertyMap {
		return presource.PropertyMap{
			"f0": presource.NewStringProperty(val),
		}
	}

	// Check when the values themselves are not changing.
	t.Run("same", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:  &res1,
			Resource2:  &res2,
			Inputs1:    configVal("val"),
			InputsMap1: propMap("val"),
			Inputs2:    configVal("val"),
			InputsMap2: propMap("val"),
		}
		result := tc.Run(t)

		assert.Equal(t, result.TFUpgrades, result.PulumiUpgrades)
		autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.PulumiUpgrades)
	})

	// Check when the values are changing, and it is an effective update.
	t.Run("different", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:  &res1,
			Resource2:  &res2,
			Inputs1:    configVal("val1"),
			InputsMap1: propMap("val1"),
			Inputs2:    configVal("val2"),
			InputsMap2: propMap("val2"),
		}
		result := tc.Run(t)
		assert.Equal(t, result.TFUpgrades, result.PulumiUpgrades)
		autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.PulumiUpgrades)
	})
}

// Basic check for upgrade logic: the type is not changing, but the schema is. Check how many times the upgrade method
// is called and if it is at parity.
func TestPFUpgrade_String_0to1_Version(t *testing.T) {
	t.Parallel()
	ct.SkipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := func(version int64) rschema.Schema {
		return rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"f0": rschema.StringAttribute{
					Optional: true,
				},
			},
			Version: version,
		}
	}

	res1 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(0),
	})

	res2 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(1),
		UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
			return map[int64]resource.StateUpgrader{
				0: {
					PriorSchema:   &res1.ResourceSchema,
					StateUpgrader: ct.NopUpgrader,
				},
			}
		},
	})

	configVal := func(val string) cty.Value {
		return cty.ObjectVal(map[string]cty.Value{
			"f0": cty.StringVal(val),
		})
	}

	propMap := func(val string) presource.PropertyMap {
		return presource.PropertyMap{
			"f0": presource.NewStringProperty(val),
		}
	}

	// Check when the values themselves are not changing.
	t.Run("same", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:  &res1,
			Resource2:  &res2,
			Inputs1:    configVal("val"),
			InputsMap1: propMap("val"),
			Inputs2:    configVal("val"),
			InputsMap2: propMap("val"),

			// Pulumi does not upgrade this resource to V2 because of a no-op update plan.
			// TODO[pulumi/pulumi-terraform-bridge#3008]
			SkipSchemaVersionAfterUpdateCheck: true,
		}
		result := tc.Run(t)

		autogold.Expect([]ct.UpgradeStateTrace{
			{
				Phase: ct.UpgradeStateTestPhase("refresh"),
				PriorState: map[string]interface{}{
					"f0": "val",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val",
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("preview"),
				PriorState: map[string]interface{}{
					"f0": "val",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val",
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("update"),
				PriorState: map[string]interface{}{
					"f0": "val",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val",
					"id": "test-id",
				},
			},
		}).Equal(t, result.TFUpgrades)

		autogold.Expect([]ct.UpgradeStateTrace{
			{
				Phase: ct.UpgradeStateTestPhase("preview"),
				PriorState: map[string]interface{}{
					"f0": "val",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val",
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("update"),
				PriorState: map[string]interface{}{
					"f0": "val",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val",
					"id": "test-id",
				},
			},
		}).Equal(t, result.PulumiUpgrades)
	})

	// Check when the values are changing, and it is an effective update.
	t.Run("different", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:  &res1,
			Resource2:  &res2,
			Inputs1:    configVal("val1"),
			InputsMap1: propMap("val1"),
			Inputs2:    configVal("val1"),
			InputsMap2: propMap("val2"),
		}
		result := tc.Run(t)

		autogold.Expect([]ct.UpgradeStateTrace{
			{
				Phase: ct.UpgradeStateTestPhase("refresh"),
				PriorState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("preview"),
				PriorState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("update"),
				PriorState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
			},
		}).Equal(t, result.TFUpgrades)

		// Upgrade calls similar but Pulumi calls the upgrader a few times too many.
		autogold.Expect([]ct.UpgradeStateTrace{
			{
				Phase: ct.UpgradeStateTestPhase("preview"),
				PriorState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("preview"),
				PriorState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("update"),
				PriorState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("update"),
				PriorState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": "val1",
					"id": "test-id",
				},
			},
		}).Equal(t, result.PulumiUpgrades)
	})
}

// Same as the string upgrade test but with objects.
func TestPFUpgrade_Object_0to1_Version(t *testing.T) {
	t.Parallel()
	ct.SkipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := func(version int64) rschema.Schema {
		return rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"f0": rschema.ObjectAttribute{
					Required: true,
					AttributeTypes: map[string]attr.Type{
						"x": basetypes.StringType{},
					},
				},
			},
			Version: version,
		}
	}

	res1 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(0),
	})

	res2 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(1),
		UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
			return map[int64]resource.StateUpgrader{
				0: {
					PriorSchema:   &res1.ResourceSchema,
					StateUpgrader: ct.NopUpgrader,
				},
			}
		},
	})

	configVal := func(val string) cty.Value {
		return cty.ObjectVal(map[string]cty.Value{
			"f0": cty.ObjectVal(map[string]cty.Value{
				"x": cty.StringVal(val),
			}),
		})
	}

	propMap := func(val string) presource.PropertyMap {
		return presource.PropertyMap{
			"f0": presource.NewObjectProperty(presource.PropertyMap{
				"x": presource.NewStringProperty(val),
			}),
		}
	}

	t.Run("same", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:  &res1,
			Resource2:  &res2,
			Inputs1:    configVal("val"),
			InputsMap1: propMap("val"),
			Inputs2:    configVal("val"),
			InputsMap2: propMap("val"),

			SkipSchemaVersionAfterUpdateCheck: true,
		}
		result := tc.Run(t)

		autogold.Expect([]ct.UpgradeStateTrace{
			{
				Phase: ct.UpgradeStateTestPhase("refresh"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val"},
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("preview"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val"},
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("update"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val"},
					"id": "test-id",
				},
			},
		}).Equal(t, result.TFUpgrades)
		autogold.Expect([]ct.UpgradeStateTrace{
			{
				Phase: ct.UpgradeStateTestPhase("preview"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val"},
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("update"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val"},
					"id": "test-id",
				},
			},
		}).Equal(t, result.PulumiUpgrades)
	})

	// Check when the values are changing, and it is an effective update.
	t.Run("different", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:  &res1,
			Resource2:  &res2,
			Inputs1:    configVal("val1"),
			InputsMap1: propMap("val1"),
			Inputs2:    configVal("val2"),
			InputsMap2: propMap("val2"),
		}
		result := tc.Run(t)

		// Upgrade calls similar but Pulumi calls the upgrader a few times too many.
		autogold.Expect([]ct.UpgradeStateTrace{
			{
				Phase: ct.UpgradeStateTestPhase("refresh"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("preview"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("update"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
			},
		}).Equal(t, result.TFUpgrades)
		autogold.Expect([]ct.UpgradeStateTrace{
			{
				Phase: ct.UpgradeStateTestPhase("preview"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("preview"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("update"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
			},
			{
				Phase: ct.UpgradeStateTestPhase("update"),
				PriorState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
				ReturnedState: map[string]interface{}{
					"f0": map[string]interface{}{"x": "val1"},
					"id": "test-id",
				},
			},
		}).Equal(t, result.PulumiUpgrades)
	})
}

// In this upgrade scenario nothing is changing in TF but Pulumi is renaming a property. State upgraders are not
// invoked but Pulumi should handle the renaming seamlessly.
func TestPFUpgrade_PulumiRenamesProperty(t *testing.T) {
	t.Parallel()
	ct.SkipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := func(version int64) rschema.Schema {
		return rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"f0": rschema.StringAttribute{Optional: true},
			},
			Version: version,
		}
	}

	res1 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(1),
	})

	res2 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(1),
		UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
			return map[int64]resource.StateUpgrader{
				0: {
					PriorSchema:   &res1.ResourceSchema,
					StateUpgrader: ct.NopUpgrader,
				},
			}
		},
	})

	res2Info := &info.Resource{
		Fields: map[string]*info.Schema{
			"f0": {Name: "f1"},
		},
	}

	t.Run("same", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:     &res1,
			Resource2:     &res2,
			Inputs1:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("val")}),
			InputsMap1:    presource.PropertyMap{"f0": presource.NewStringProperty("val")},
			Inputs2:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("val")}),
			InputsMap2:    presource.PropertyMap{"f1": presource.NewStringProperty("val")},
			ResourceInfo2: res2Info,
		}
		result := tc.Run(t)

		assert.Equal(t, result.TFUpgrades, result.PulumiUpgrades)
		autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.PulumiUpgrades)

		autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.PulumiUpResult.Summary.ResourceChanges)
	})

	t.Run("different", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:     &res1,
			Resource2:     &res2,
			Inputs1:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("val1")}),
			InputsMap1:    presource.PropertyMap{"f0": presource.NewStringProperty("val1")},
			Inputs2:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("val2")}),
			InputsMap2:    presource.PropertyMap{"f1": presource.NewStringProperty("val2")},
			ResourceInfo2: res2Info,
		}
		result := tc.Run(t)

		assert.Equal(t, result.TFUpgrades, result.PulumiUpgrades)
		autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.PulumiUpgrades)

		autogold.Expect(&map[string]int{"same": 1, "update": 1}).Equal(t, result.PulumiUpResult.Summary.ResourceChanges)
	})
}

// There are certain type changes permitted in Pulumi that coalesce naturally without state upgraders, that is
// reinterpreting a string as a number for example.
func TestPFUpgrade_PulumiChangesPropertyType(t *testing.T) {
	t.Parallel()
	ct.SkipUnlessLinux(t)
	skipUnlessDeltasEnabled(t)

	sch := func(version int64) rschema.Schema {
		return rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"f0": rschema.StringAttribute{Optional: true},
			},
			Version: version,
		}
	}

	res1 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(1),
	})

	res2 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(1),
		UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
			return map[int64]resource.StateUpgrader{
				0: {
					PriorSchema:   &res1.ResourceSchema,
					StateUpgrader: ct.NopUpgrader,
				},
			}
		},
	})

	res2Info := &info.Resource{
		Fields: map[string]*info.Schema{
			"f0": {Type: "number"},
		},
	}

	t.Run("same", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:     &res1,
			Resource2:     &res2,
			Inputs1:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("42")}),
			InputsMap1:    presource.PropertyMap{"f0": presource.NewStringProperty("42")},
			Inputs2:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("42")}),
			InputsMap2:    presource.PropertyMap{"f0": presource.NewNumberProperty(42)},
			ResourceInfo2: res2Info,
		}
		result := tc.Run(t)

		assert.Equal(t, result.TFUpgrades, result.PulumiUpgrades)
		autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.PulumiUpgrades)

		autogold.Expect(&map[string]int{"same": 2}).Equal(t, result.PulumiUpResult.Summary.ResourceChanges)
	})

	t.Run("different", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:     &res1,
			Resource2:     &res2,
			Inputs1:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("42")}),
			InputsMap1:    presource.PropertyMap{"f0": presource.NewStringProperty("42")},
			Inputs2:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("7")}),
			InputsMap2:    presource.PropertyMap{"f0": presource.NewNumberProperty(7)},
			ResourceInfo2: res2Info,
		}
		result := tc.Run(t)

		assert.Equal(t, result.TFUpgrades, result.PulumiUpgrades)
		autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.PulumiUpgrades)

		autogold.Expect(&map[string]int{"same": 1, "update": 1}).Equal(t, result.PulumiUpResult.Summary.ResourceChanges)
	})
}

// When downgrading to a lower schema, TF fails.
func TestPFUpgrade_Downgrading(t *testing.T) {
	t.Parallel()
	ct.SkipUnlessLinux(t)

	sch := func(version int64) rschema.Schema {
		return rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"f0": rschema.StringAttribute{Optional: true},
			},
			Version: version,
		}
	}

	res1 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(2),
	})

	res2 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch(1),
		UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
			return map[int64]resource.StateUpgrader{
				0: {
					PriorSchema:   &res1.ResourceSchema,
					StateUpgrader: ct.NopUpgrader,
				},
			}
		},
	})

	// Check when the values themselves are not changing.
	t.Run("same", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:     &res1,
			Resource2:     &res2,
			Inputs1:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("val")}),
			Inputs2:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("val")}),
			ExpectFailure: true,
			SkipPulumi:    "TODO[pulumi-terraform-bridge#3009]",
		}
		result := tc.Run(t)
		assert.Equal(t, result.TFUpgrades, result.PulumiUpgrades)
		autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.PulumiUpgrades)
	})

	// Check when the values are changing, and it is an effective update.
	t.Run("different", func(t *testing.T) {
		tc := ct.UpgradeStateTestCase{
			Resource1:     &res1,
			Resource2:     &res2,
			Inputs1:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("val1")}),
			Inputs2:       cty.ObjectVal(map[string]cty.Value{"f0": cty.StringVal("val2")}),
			ExpectFailure: true,
			SkipPulumi:    "TODO[pulumi-terraform-bridge#3009]",
		}
		result := tc.Run(t)
		assert.Equal(t, result.TFUpgrades, result.PulumiUpgrades)
		autogold.Expect([]ct.UpgradeStateTrace{}).Equal(t, result.PulumiUpgrades)
	})
}

func skipUnlessDeltasEnabled(t *testing.T) {
	if d, ok := os.LookupEnv("PULUMI_RAW_STATE_DELTA_ENABLED"); !ok || !cmdutil.IsTruthy(d) {
		t.Skip("This test requires PULUMI_RAW_STATE_DELTA_ENABLED=true environment")
	}
}
