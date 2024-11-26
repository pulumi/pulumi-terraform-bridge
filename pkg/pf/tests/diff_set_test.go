package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/providerbuilder"
)

type setDefault string

var _ defaults.Set = setDefault("default")

func (s setDefault) DefaultSet(ctx context.Context, req defaults.SetRequest, resp *defaults.SetResponse) {
	resp.PlanValue = basetypes.NewSetValueMust(types.StringType, []attr.Value{
		basetypes.NewStringValue("value"),
	})
}

func (s setDefault) Description(ctx context.Context) string {
	return "description"
}

func (s setDefault) MarkdownDescription(ctx context.Context) string {
	return "markdown description"
}

func TestDetailedDiffSet(t *testing.T) {
	t.Parallel()

	attributeSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.SetAttribute{
					Optional:    true,
					ElementType: types.StringType,
				},
			},
		},
	})

	attributeSchemaWithDefault := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.SetAttribute{
					Optional:    true,
					Computed:    true,
					ElementType: types.StringType,
					Default:     setDefault("default"),
					PlanModifiers: []planmodifier.Set{
						setplanmodifier.UseStateForUnknown(),
					},
				},
			},
		},
	})

	attributeReplaceSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.SetAttribute{
					Optional:    true,
					ElementType: types.StringType,
					PlanModifiers: []planmodifier.Set{
						setplanmodifier.RequiresReplace(),
					},
				},
			},
		},
	})

	nestedAttributeSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.SetNestedAttribute{
					Optional: true,
					NestedObject: rschema.NestedAttributeObject{
						Attributes: map[string]rschema.Attribute{
							"nested": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
		},
	})

	nestedAttributeReplaceSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.SetNestedAttribute{
					Optional: true,
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
		},
	})

	nestedAttributeNestedReplaceSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.SetNestedAttribute{
					Optional: true,
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
		},
	})

	blockSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Blocks: map[string]rschema.Block{
				"key": rschema.SetNestedBlock{
					NestedObject: rschema.NestedBlockObject{
						Attributes: map[string]rschema.Attribute{
							"nested": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
		},
	})

	blockSchemaWithDefault := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Blocks: map[string]rschema.Block{
				"key": rschema.SetNestedBlock{
					NestedObject: rschema.NestedBlockObject{
						Attributes: map[string]rschema.Attribute{
							"nested": rschema.StringAttribute{
								Optional: true,
								Default:  stringDefault("default"),
								Computed: true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.UseStateForUnknown(),
								},
							},
						},
					},
				},
			},
		},
	})

	blockReplaceSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
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
		},
	})

	blockNestedReplaceSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
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
		},
	})

	computedAttributeCreateFunc := func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
		type ObjectModel struct {
			ID   types.String `tfsdk:"id"`
			Keys types.Set    `tfsdk:"key"`
		}
		reqVal := ObjectModel{}
		diags := req.Plan.Get(ctx, &reqVal)
		contract.Assertf(diags.ErrorsCount() == 0, "failed to get attribute: %v", diags)

		respVal := ObjectModel{
			ID: types.StringValue("test-id"),
		}
		if reqVal.Keys.IsUnknown() {
			respVal.Keys = types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("value"),
			})
		} else {
			respVal.Keys = reqVal.Keys
		}

		diags = resp.State.Set(ctx, &respVal)
		contract.Assertf(diags.ErrorsCount() == 0, "failed to set attribute: %v", diags)
	}

	computedAttributeUpdateFunc := func(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
		createResp := resource.CreateResponse{
			State:       resp.State,
			Diagnostics: resp.Diagnostics,
		}
		computedAttributeCreateFunc(ctx, resource.CreateRequest{
			Plan:         req.Plan,
			Config:       req.Config,
			ProviderMeta: req.ProviderMeta,
		}, &createResp)
		resp.State = createResp.State
		resp.Diagnostics = createResp.Diagnostics
	}

	computedSetAttributeSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.SetAttribute{
					Optional:    true,
					ElementType: types.StringType,
					Computed:    true,
					PlanModifiers: []planmodifier.Set{
						setplanmodifier.UseStateForUnknown(),
					},
				},
			},
		},
		CreateFunc: computedAttributeCreateFunc,
		UpdateFunc: computedAttributeUpdateFunc,
	})

	computedSetAttributeReplaceSchema := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"key": rschema.SetAttribute{
					Optional:    true,
					ElementType: types.StringType,
					Computed:    true,
					PlanModifiers: []planmodifier.Set{
						setplanmodifier.RequiresReplace(),
						setplanmodifier.UseStateForUnknown(),
					},
				},
			},
		},
		CreateFunc: computedAttributeCreateFunc,
		UpdateFunc: computedAttributeUpdateFunc,
	})

	computedBlockCreateFunc := func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
		type Nested struct {
			Nested   types.String `tfsdk:"nested"`
			Computed types.String `tfsdk:"computed"`
		}

		type ObjectModel struct {
			ID   types.String `tfsdk:"id"`
			Keys []Nested     `tfsdk:"key"`
		}

		reqObj := ObjectModel{}
		diags := req.Plan.Get(ctx, &reqObj)
		contract.Assertf(diags.ErrorsCount() == 0, "failed to get attribute: %v", diags)

		respObj := ObjectModel{
			ID:   types.StringValue("test-id"),
			Keys: make([]Nested, len(reqObj.Keys)),
		}

		for i, key := range reqObj.Keys {
			newKey := Nested{}
			if key.Computed.IsNull() || key.Computed.IsUnknown() {
				nestedVal := ""
				if !key.Nested.IsNull() && !key.Nested.IsUnknown() {
					nestedVal = key.Nested.ValueString()
				}
				computedVal := "computed-" + nestedVal
				newKey.Nested = types.StringValue(nestedVal)
				newKey.Computed = types.StringValue(computedVal)
			} else {
				newKey.Nested = key.Nested
				newKey.Computed = key.Computed
			}
			respObj.Keys[i] = newKey
		}

		diags = resp.State.Set(ctx, &respObj)
		contract.Assertf(diags.ErrorsCount() == 0, "failed to set attribute: %v", diags)
	}

	computedBlockUpdateFunc := func(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
		createResp := resource.CreateResponse{
			State:       resp.State,
			Diagnostics: resp.Diagnostics,
		}
		computedBlockCreateFunc(ctx, resource.CreateRequest{
			Plan:         req.Plan,
			Config:       req.Config,
			ProviderMeta: req.ProviderMeta,
		}, &createResp)

		resp.State = createResp.State
		resp.Diagnostics = createResp.Diagnostics
	}

	blockSchemaWithComputed := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Blocks: map[string]rschema.Block{
				"key": rschema.SetNestedBlock{
					NestedObject: rschema.NestedBlockObject{
						Attributes: map[string]rschema.Attribute{
							"nested": rschema.StringAttribute{Optional: true},
							"computed": rschema.StringAttribute{
								Computed: true,
								Optional: true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.UseStateForUnknown(),
								},
							},
						},
					},
				},
			},
		},
		CreateFunc: computedBlockCreateFunc,
		UpdateFunc: computedBlockUpdateFunc,
	})

	blockSchemaWithComputedNoStateForUnknown := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Blocks: map[string]rschema.Block{
				"key": rschema.SetNestedBlock{
					NestedObject: rschema.NestedBlockObject{
						Attributes: map[string]rschema.Attribute{
							"nested": rschema.StringAttribute{Optional: true},
							"computed": rschema.StringAttribute{
								Computed: true,
								Optional: true,
							},
						},
					},
				},
			},
		},
		CreateFunc: computedBlockCreateFunc,
		UpdateFunc: computedBlockUpdateFunc,
	})

	blockSchemaWithComputedReplace := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Blocks: map[string]rschema.Block{
				"key": rschema.SetNestedBlock{
					NestedObject: rschema.NestedBlockObject{
						Attributes: map[string]rschema.Attribute{
							"nested": rschema.StringAttribute{Optional: true},
							"computed": rschema.StringAttribute{
								Computed: true,
								Optional: true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.UseStateForUnknown(),
								},
							},
						},
					},
					PlanModifiers: []planmodifier.Set{
						setplanmodifier.RequiresReplace(),
					},
				},
			},
		},
		CreateFunc: computedBlockCreateFunc,
		UpdateFunc: computedBlockUpdateFunc,
	})

	blockSchemaWithComputedNestedReplace := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
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
							"computed": rschema.StringAttribute{
								Computed: true,
								Optional: true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.UseStateForUnknown(),
								},
							},
						},
					},
				},
			},
		},
		CreateFunc: computedBlockCreateFunc,
		UpdateFunc: computedBlockUpdateFunc,
	})

	blockSchemaWithComputedComputedRequiresReplace := pb.NewResource(pb.NewResourceArgs{
		ResourceSchema: rschema.Schema{
			Blocks: map[string]rschema.Block{
				"key": rschema.SetNestedBlock{
					NestedObject: rschema.NestedBlockObject{
						Attributes: map[string]rschema.Attribute{
							"nested": rschema.StringAttribute{Optional: true},
							"computed": rschema.StringAttribute{
								Computed: true,
								Optional: true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.UseStateForUnknown(),
									stringplanmodifier.RequiresReplace(),
								},
							},
						},
					},
				},
			},
		},
		CreateFunc: computedBlockCreateFunc,
		UpdateFunc: computedBlockUpdateFunc,
	})

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
			return cty.ListValEmpty(cty.Object(map[string]cty.Type{"nested": cty.String}))
		}
		return cty.ListVal(slice)
	}

	schemaValueMakerPairs := []struct {
		name       string
		res        pb.Resource
		valueMaker func(*[]string) cty.Value
	}{
		{"attribute no replace", attributeSchema, attrList},
		{"attribute requires replace", attributeReplaceSchema, attrList},
		{"nested attribute no replace", nestedAttributeSchema, nestedAttrList},
		{"nested attribute requires replace", nestedAttributeReplaceSchema, nestedAttrList},
		{"nested attribute nested requires replace", nestedAttributeNestedReplaceSchema, nestedAttrList},
		{"block no replace", blockSchema, nestedAttrList},
		{"block requires replace", blockReplaceSchema, nestedAttrList},
		{"block nested requires replace", blockNestedReplaceSchema, nestedAttrList},

		// Defaults
		{"attribute with default", attributeSchemaWithDefault, attrList},
		{"block with default", blockSchemaWithDefault, nestedAttrList},

		// Computed attributes
		{"attribute with computed no replace", computedSetAttributeSchema, attrList},
		{"attribute with computed requires replace", computedSetAttributeReplaceSchema, attrList},

		// Computed blocks, each state we test both the behaviour when the computed value is specified in the program and when it is not.
		{"block with computed no replace computed", blockSchemaWithComputed, nestedAttrList},
		{"block with computed no replace computed specified in program", blockSchemaWithComputed, nestedAttrListWithComputedSpecified},
		{"block with computed requires replace", blockSchemaWithComputedReplace, nestedAttrList},
		{"block with computed requires replace computed specified in program", blockSchemaWithComputedReplace, nestedAttrListWithComputedSpecified},
		{"block with computed and nested requires replace", blockSchemaWithComputedNestedReplace, nestedAttrList},
		{"block with computed and nested requires replace computed specified in program", blockSchemaWithComputedNestedReplace, nestedAttrListWithComputedSpecified},
		{"block with computed and computed requires replace", blockSchemaWithComputedComputedRequiresReplace, nestedAttrList},
		{"block with computed and computed requires replace computed specified in program", blockSchemaWithComputedComputedRequiresReplace, nestedAttrListWithComputedSpecified},
		// Rarely used, but supported
		{"block with computed no state for unknown", blockSchemaWithComputedNoStateForUnknown, nestedAttrList},
		{"block with computed no state for unknown computed specified in program", blockSchemaWithComputedNoStateForUnknown, nestedAttrListWithComputedSpecified},
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
		{"shuffled", &[]string{"val1", "val2", "val3"}, &[]string{"val3", "val1", "val2"}},
		{"shuffled unordered", &[]string{"val2", "val3", "val1"}, &[]string{"val3", "val1", "val2"}},
	}

	type testOutput struct {
		initialValue *[]string
		changeValue  *[]string
		tfOut        string
		pulumiOut    string
		detailedDiff map[string]any
	}

	for _, schemaValueMakerPair := range schemaValueMakerPairs {
		t.Run(schemaValueMakerPair.name, func(t *testing.T) {
			t.Parallel()
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					t.Parallel()
					initialValue := schemaValueMakerPair.valueMaker(scenario.initialValue)
					changeValue := schemaValueMakerPair.valueMaker(scenario.changeValue)

					diff := crosstests.Diff(t, schemaValueMakerPair.res, map[string]cty.Value{"key": initialValue}, map[string]cty.Value{"key": changeValue})

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

	// Both pulumi and TF do not allow duplicates in sets, so we don't test that here.
}
