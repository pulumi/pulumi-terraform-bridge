package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	pb "github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func TestExtractInputsFromOutputsPF(t *testing.T) {
	type testCase struct {
		name      string
		props     resource.PropertyMap
		resSchema rschema.Schema
		expect    autogold.Value
	}

	testCases := []testCase{
		{
			name:  "string",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{"foo": "bar"}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.StringAttribute{Optional: true},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("foo"): resource.PropertyValue{V: "bar"},
			}),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for foo in the inputs.
		{
			name:  "string with default",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{"foo": "bar"}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.StringAttribute{Optional: true, Default: stringdefault.StaticString("bar")},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("foo"): resource.PropertyValue{V: "bar"},
			}),
		},
		{
			name:  "string with empty value",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{"foo": ""}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.StringAttribute{Optional: true},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
				V: []resource.PropertyValue{},
			}}),
		},
		{
			name:  "string computed",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{"foo": "bar"}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.StringAttribute{Computed: true},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
				V: []resource.PropertyValue{},
			}}),
		},
		{
			name:  "list attribute",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{"foo": []interface{}{"bar"}}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("foo"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: "bar",
				}}},
			}),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for foo in the inputs.
		{
			name: "list attribute with default",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": []interface{}{"bar"},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{types.StringValue("bar")})),
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("foo"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: "bar",
				}}},
			}),
		},
		{
			name: "list attribute computed",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": []interface{}{"bar"},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
				V: []resource.PropertyValue{},
			}}),
		},
		{
			name: "list nested attribute",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": []interface{}{
					map[string]interface{}{"bar": "baz"},
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.ListNestedAttribute{
						Optional: true,
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{Optional: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("foo"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: resource.PropertyMap{
						resource.PropertyKey("__defaults"): resource.PropertyValue{
							V: []resource.PropertyValue{},
						},
						resource.PropertyKey("bar"): resource.PropertyValue{V: "baz"},
					},
				}}},
			}),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for foo in the inputs.
		{
			name: "list nested attribute with defaults",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": []interface{}{
					map[string]interface{}{"bar": "baz"},
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.ListNestedAttribute{
						Optional: true,
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{Optional: true},
							},
						},
						Default: listdefault.StaticValue(types.ListValueMust(types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"bar": types.StringType,
							},
						}, []attr.Value{types.ObjectValueMust(
							map[string]attr.Type{
								"bar": types.StringType,
							},
							map[string]attr.Value{
								"bar": types.StringValue("baz"),
							},
						)})),
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("foo"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: resource.PropertyMap{
						resource.PropertyKey("__defaults"): resource.PropertyValue{
							V: []resource.PropertyValue{},
						},
						resource.PropertyKey("bar"): resource.PropertyValue{V: "baz"},
					},
				}}},
			}),
		},
		{
			name: "list nested attribute computed",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": []interface{}{
					map[string]interface{}{"bar": "baz"},
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.ListNestedAttribute{
						Computed: true,
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{Optional: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
				V: []resource.PropertyValue{},
			}}),
		},
		{
			name: "list nested attribute nested computed",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": []interface{}{
					map[string]interface{}{"bar": "baz"},
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.ListNestedAttribute{
						Optional: true,
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{Computed: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("foo"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
						V: []resource.PropertyValue{},
					}},
				}}},
			}),
		},
		{
			name: "set nested attribute",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": []interface{}{
					map[string]interface{}{"bar": "baz"},
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.SetNestedAttribute{
						Optional: true,
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{Optional: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("foo"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: resource.PropertyMap{
						resource.PropertyKey("__defaults"): resource.PropertyValue{
							V: []resource.PropertyValue{},
						},
						resource.PropertyKey("bar"): resource.PropertyValue{V: "baz"},
					},
				}}},
			}),
		},
		{
			name: "map nested attribute",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.MapNestedAttribute{
						Optional: true,
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{Optional: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("foo"): resource.PropertyValue{V: resource.PropertyMap{
					resource.PropertyKey("__defaults"): resource.PropertyValue{
						V: []resource.PropertyValue{},
					},
					resource.PropertyKey("bar"): resource.PropertyValue{V: "baz"},
				}},
			}),
		},
		{
			name: "object attribute",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.ObjectAttribute{
						Optional: true,
						AttributeTypes: map[string]attr.Type{
							"bar": types.StringType,
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
				V: []resource.PropertyValue{},
			}}),
		},
		{
			name: "single nested attribute",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]rschema.Attribute{
							"bar": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("foo"): resource.PropertyValue{V: resource.PropertyMap{
					resource.PropertyKey("__defaults"): resource.PropertyValue{
						V: []resource.PropertyValue{},
					},
					resource.PropertyKey("bar"): resource.PropertyValue{V: "baz"},
				}},
			}),
		},
		// BLOCKS
		// {
		// 	name: "list nested block",
		// },
		// {
		// 	name: "set nested block",
		// },
		// {
		// 	name: "single nested block",
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prov := &pb.Provider{
				AllResources: []pb.Resource{
					{
						Name:           "test",
						ResourceSchema: tc.resSchema,
					},
				},
			}

			shimmedProvider := schemashim.ShimSchemaOnlyProvider(context.Background(), prov)
			res := shimmedProvider.ResourcesMap().Get("_test")
			result, err := tfbridge.ExtractInputsFromOutputs(nil, tc.props, res.Schema(), nil, false)
			require.NoError(t, err)
			tc.expect.Equal(t, result)
		})
	}
}
