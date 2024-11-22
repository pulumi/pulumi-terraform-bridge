package tfbridgetests

import (
	"context"
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hexops/autogold/v2"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/providerbuilder"
)

func ref[T any](t T) *T { return &t }

func TestSimpleNoDiff(t *testing.T) {
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

type stringDefault string

var _ defaults.String = stringDefault("default")

func (s stringDefault) DefaultString(ctx context.Context, req defaults.StringRequest, resp *defaults.StringResponse) {
	resp.PlanValue = basetypes.NewStringValue(string(s))
}

func (s stringDefault) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		resp.PlanValue = basetypes.NewStringValue(string(s))
	}
}

func (s stringDefault) Description(ctx context.Context) string {
	return "description"
}

func (s stringDefault) MarkdownDescription(ctx context.Context) string {
	return "markdown description"
}

func TestDetailedDiffStringAttribute(t *testing.T) {
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

	type testOutput struct {
		initialValue *string
		changeValue  *string
		tfOut        string
		pulumiOut    string
		detailedDiff  map[string]any
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
					diff := crosstests.Diff(t, res, map[string]cty.Value{"key": initialValue}, map[string]cty.Value{"key": changeValue})

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
