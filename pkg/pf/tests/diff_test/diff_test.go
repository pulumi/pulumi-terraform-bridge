package tfbridgetests

import (
	"context"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hexops/autogold/v2"
	"github.com/zclconf/go-cty/cty"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
)

func TestPFSimpleNoDiff(t *testing.T) {
	t.Parallel()

	sch := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{Optional: true},
		},
	}

	res := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: sch,
	})
	diff := crosstests.Diff(t, res,
		map[string]cty.Value{"key": cty.StringVal("value")},
		map[string]cty.Value{"key": cty.StringVal("value1")},
	)

	autogold.Expect(`
Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place

Terraform will perform the following actions:

  # testprovider_test.res will be updated in-place
  ~ resource "testprovider_test" "res" {
        id  = "test-id"
      ~ key = "value" -> "value1"
    }

Plan: 0 to add, 1 to change, 0 to destroy.

`).Equal(t, diff.TFOut)
	autogold.Expect(`Previewing update (test):
  pulumi:pulumi:Stack: (same)
    [urn=urn:pulumi:test::project::pulumi:pulumi:Stack::project-test]
    ~ testprovider:index/test:Test: (update)
        [id=test-id]
        [urn=urn:pulumi:test::project::testprovider:index/test:Test::p]
      ~ key: "value" => "value1"
Resources:
    ~ 1 to update
    1 unchanged
`).Equal(t, diff.PulumiOut)
}

func TestPFDetailedDiffStringAttribute(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{Optional: true},
		},
	}

	attributeReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}

	attributeSchemaWithDefault := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringDefault("default"),
			},
		},
	}

	attributeSchemaWithDefaultReplace := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringDefault("default"),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}

	attributeSchemaWitPlanModifierDefault := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringDefault("default")},
			},
		},
	}

	attributeSchemaWithPlanModifierDefaultReplace := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringDefault("default"), stringplanmodifier.RequiresReplace()},
			},
		},
	}

	schemas := []struct {
		name   string
		schema rschema.Schema
	}{
		{"no replace", attributeSchema},
		{"replace", attributeReplaceSchema},
		{"default", attributeSchemaWithDefault},
		{"default replace", attributeSchemaWithDefaultReplace},
		{"plan modifier default", attributeSchemaWitPlanModifierDefault},
		{"plan modifier default replace", attributeSchemaWithPlanModifierDefaultReplace},
	}

	makeValue := func(s *string) cty.Value {
		if s == nil {
			return cty.NullVal(cty.DynamicPseudoType)
		}
		return cty.StringVal(*s)
	}

	scenarios := []struct {
		name         string
		initialValue *string
		changeValue  *string
	}{
		{"unchanged", ref("value"), ref("value")},
		{"changed", ref("value"), ref("value1")},
		{"added", nil, ref("value")},
		{"removed", ref("value"), nil},
	}

	for _, schema := range schemas {
		t.Run(schema.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					initialValue := makeValue(scenario.initialValue)
					changeValue := makeValue(scenario.changeValue)

					res := pb.NewResource(pb.NewResourceArgs{
						ResourceSchema: schema.schema,
					})
					diff := crosstests.Diff(
						t, res, map[string]cty.Value{"key": initialValue}, map[string]cty.Value{"key": changeValue},
					)

					autogold.ExpectFile(t, testOutput{
						initialValue: scenario.initialValue,
						changeValue:  scenario.changeValue,
						tfOut:        diff.TFOut,
						pulumiOut:    diff.PulumiOut,
						detailedDiff: diff.PulumiDiff.DetailedDiff,
					})
				})
			}
		})
	}
}

func TestPFDetailedDiffDynamicType(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.DynamicAttribute{
				Optional: true,
			},
		},
	}
	res := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: attributeSchema,
	})

	t.Run("no change", func(t *testing.T) {
		crosstests.Diff(t, res,
			map[string]cty.Value{"key": cty.StringVal("value")},
			map[string]cty.Value{"key": cty.StringVal("value")},
		)
	})

	t.Run("change", func(t *testing.T) {
		crosstests.Diff(t, res,
			map[string]cty.Value{"key": cty.StringVal("value")},
			map[string]cty.Value{"key": cty.StringVal("value1")},
		)
	})

	t.Run("int no change", func(t *testing.T) {
		crosstests.Diff(t, res,
			map[string]cty.Value{"key": cty.NumberVal(big.NewFloat(1))},
			map[string]cty.Value{"key": cty.NumberVal(big.NewFloat(1))},
		)
	})

	t.Run("type change", func(t *testing.T) {
		// TODO[pulumi/pulumi-terraform-bridge#3078]
		t.Skip(`Error converting tftypes.Number<"1"> (value2) at "AttributeName(\"key\")": can't unmarshal tftypes.Number into *string, expected string`)
		crosstests.Diff(t, res,
			map[string]cty.Value{"key": cty.StringVal("value")},
			map[string]cty.Value{"key": cty.NumberVal(big.NewFloat(1))},
		)
	})
}

func TestPFDetailedDiffDynamicTypeWithMigration(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.DynamicAttribute{
				Optional: true,
			},
		},
	}
	res1 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: attributeSchema,
	})

	schema2 := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.DynamicAttribute{
				Optional: true,
			},
		},
		Version: 1,
	}
	res2 := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: schema2,
		UpgradeStateFunc: func(ctx context.Context) map[int64]resource.StateUpgrader {
			return map[int64]resource.StateUpgrader{
				0: {
					PriorSchema: &res1.ResourceSchema,
					StateUpgrader: func(ctx context.Context, usr1 resource.UpgradeStateRequest, usr2 *resource.UpgradeStateResponse) {
						usr2.State = *usr1.State
					},
				},
			}
		},
	})

	t.Run("no change", func(t *testing.T) {
		crosstests.Diff(t, res1,
			map[string]cty.Value{"key": cty.StringVal("value")},
			map[string]cty.Value{"key": cty.StringVal("value")},
			crosstests.DiffProviderUpgradedSchema(
				res2,
			),
		)
	})

	t.Run("change", func(t *testing.T) {
		crosstests.Diff(t, res1,
			map[string]cty.Value{"key": cty.StringVal("value")},
			map[string]cty.Value{"key": cty.StringVal("value1")},
			crosstests.DiffProviderUpgradedSchema(
				res2,
			),
		)
	})
}
