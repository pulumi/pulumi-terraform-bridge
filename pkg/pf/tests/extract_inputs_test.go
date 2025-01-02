package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestExtractInputsFromOutputsPF(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name      string
		props     resource.PropertyMap
		resSchema rschema.Schema
		expect    autogold.Value
	}

	testCases := []testCase{
		// ATTRIBUTES
		{
			name:  "string extracted",
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
			name:  "string with default not extracted",
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
				resource.PropertyKey("foo"): resource.PropertyValue{V: "bar"}, // wrong
			}),
		},
		{
			name:  "string with empty value not extracted",
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
			name:  "string computed not extracted",
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
			name:  "list attribute extracted",
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
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for properties with defaults in the inputs.
		{
			name: "list attribute with default not extracted",
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
					V: "bar", // wrong
				}}},
			}),
		},
		{
			name: "list attribute computed not extracted",
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
			name: "list nested attribute extracted",
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
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for properties with defaults in the inputs.
		{
			name: "list nested attribute with defaults not extracted",
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
						resource.PropertyKey("bar"): resource.PropertyValue{V: "baz"}, // wrong
					},
				}}},
			}),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for properties with defaults in the inputs.
		{
			name: "list nested attribute with nested defaults not extracted",
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
								"bar": rschema.StringAttribute{Optional: true, Default: stringdefault.StaticString("baz")},
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
						resource.PropertyKey("bar"): resource.PropertyValue{V: "baz"}, // wrong
					},
				}}},
			}),
		},
		{
			name: "list nested attribute computed not extracted",
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
			name: "list nested attribute nested computed not extracted",
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
			name: "set nested attribute extracted",
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
			name: "map nested attribute extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": map[string]interface{}{
					"key1": map[string]interface{}{"bar": "baz"},
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
					resource.PropertyKey("key1"): resource.PropertyValue{V: resource.PropertyMap{
						resource.PropertyKey("__defaults"): resource.PropertyValue{
							V: []resource.PropertyValue{},
						},
						resource.PropertyKey("bar"): resource.PropertyValue{V: "baz"},
					}},
				}},
			}),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for properties with defaults in the inputs.
		{
			name: "map nested attribute with default not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": map[string]interface{}{
					"key1": map[string]interface{}{"bar": "baz"},
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
						Default: mapdefault.StaticValue(types.MapValueMust(
							types.ObjectType{
								AttrTypes: map[string]attr.Type{"bar": types.StringType},
							}, map[string]attr.Value{
								"key1": types.ObjectValueMust(
									map[string]attr.Type{
										"bar": types.StringType,
									},
									map[string]attr.Value{
										"bar": types.StringValue("baz"), // wrong
									},
								),
							},
						)),
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
					resource.PropertyKey("key1"): resource.PropertyValue{V: resource.PropertyMap{
						resource.PropertyKey("__defaults"): resource.PropertyValue{
							V: []resource.PropertyValue{},
						},
						resource.PropertyKey("bar"): resource.PropertyValue{V: "baz"},
					}},
				}},
			}),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for properties with defaults in the inputs.
		{
			name: "map nested attribute with nested default not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": map[string]interface{}{
					"key1": map[string]interface{}{"bar": "baz"},
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.MapNestedAttribute{
						Optional: true,
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{Optional: true, Default: stringdefault.StaticString("baz")},
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
					resource.PropertyKey("key1"): resource.PropertyValue{V: resource.PropertyMap{
						resource.PropertyKey("__defaults"): resource.PropertyValue{
							V: []resource.PropertyValue{},
						},
						resource.PropertyKey("bar"): resource.PropertyValue{V: "baz"}, // wrong
					}},
				}},
			}),
		},
		{
			name: "map nested attribute computed not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": map[string]interface{}{
					"key1": map[string]interface{}{"bar": "baz"},
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.MapNestedAttribute{
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
			name: "map nested attribute nested computed not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": map[string]interface{}{
					"key1": map[string]interface{}{"bar": "baz"},
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.MapNestedAttribute{
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
				resource.PropertyKey("foo"): resource.PropertyValue{V: resource.PropertyMap{
					resource.PropertyKey("__defaults"): resource.PropertyValue{
						V: []resource.PropertyValue{},
					},
					resource.PropertyKey("key1"): resource.PropertyValue{V: resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
						V: []resource.PropertyValue{},
					}}},
				}},
			}),
		},
		{
			name: "object attribute extracted",
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
			name: "object attribute computed not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.ObjectAttribute{
						Computed: true,
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
			name: "single nested attribute extracted",
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
		{
			name: "single nested attribute computed not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			}),
			resSchema: rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]rschema.Attribute{
							"bar": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
				V: []resource.PropertyValue{},
			}}),
		},
		{
			name: "single nested attribute nested computed not extracted",
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
							"bar": rschema.StringAttribute{Computed: true},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
				V: []resource.PropertyValue{},
			}}),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2218]: Add missing defaults tests here once defaults are fixed.
		// BLOCKS
		{
			name: "list nested block extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"block_field": []interface{}{
					map[string]interface{}{"nested_field": "nested_value"},
				},
			}),
			resSchema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"block_field": rschema.ListNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"nested_field": rschema.StringAttribute{Optional: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("block_field"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: resource.PropertyMap{
						resource.PropertyKey("__defaults"): resource.PropertyValue{
							V: []resource.PropertyValue{},
						},
						resource.PropertyKey("nested_field"): resource.PropertyValue{V: "nested_value"},
					},
				}}},
			}),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for properties with defaults in the inputs.
		{
			name: "list nested block with defaults not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"block_field": []interface{}{
					map[string]interface{}{"nested_field": "nested_value"},
				},
			}),
			resSchema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"block_field": rschema.ListNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"nested_field": rschema.StringAttribute{Optional: true, Default: stringdefault.StaticString("nested_value")},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("block_field"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: resource.PropertyMap{
						resource.PropertyKey("__defaults"): resource.PropertyValue{
							V: []resource.PropertyValue{},
						},
						resource.PropertyKey("nested_field"): resource.PropertyValue{V: "nested_value"}, // wrong
					},
				}}},
			}),
		},
		{
			name: "list nested block computed not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"block_field": []interface{}{
					map[string]interface{}{"nested_field": "nested_value"},
				},
			}),
			resSchema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"block_field": rschema.ListNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"nested_field": rschema.StringAttribute{Computed: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("block_field"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
						V: []resource.PropertyValue{},
					}},
				}}},
			}),
		},
		{
			name: "set nested block extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"block_field": []interface{}{
					map[string]interface{}{"nested_field": "nested_value"},
				},
			}),
			resSchema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"block_field": rschema.SetNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"nested_field": rschema.StringAttribute{Optional: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("block_field"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: resource.PropertyMap{
						resource.PropertyKey("__defaults"): resource.PropertyValue{
							V: []resource.PropertyValue{},
						},
						resource.PropertyKey("nested_field"): resource.PropertyValue{V: "nested_value"},
					},
				}}},
			}),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for properties with defaults in the inputs.
		{
			name: "set nested block with defaults not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"block_field": []interface{}{
					map[string]interface{}{"nested_field": "nested_value"},
				},
			}),
			resSchema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"block_field": rschema.SetNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"nested_field": rschema.StringAttribute{Optional: true, Default: stringdefault.StaticString("nested_value")},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("block_field"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: resource.PropertyMap{
						resource.PropertyKey("__defaults"): resource.PropertyValue{
							V: []resource.PropertyValue{},
						},
						resource.PropertyKey("nested_field"): resource.PropertyValue{V: "nested_value"}, // wrong
					},
				}}},
			}),
		},
		{
			name: "set nested block computed not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"block_field": []interface{}{
					map[string]interface{}{"nested_field": "nested_value"},
				},
			}),
			resSchema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"block_field": rschema.SetNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"nested_field": rschema.StringAttribute{Computed: true},
							},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("block_field"): resource.PropertyValue{V: []resource.PropertyValue{{
					V: resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
						V: []resource.PropertyValue{},
					}},
				}}},
			}),
		},
		{
			name: "single nested block extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"block_field": map[string]interface{}{"nested_field": "nested_value"},
			}),
			resSchema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"block_field": rschema.SingleNestedBlock{
						Attributes: map[string]rschema.Attribute{
							"nested_field": rschema.StringAttribute{Optional: true},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("block_field"): resource.PropertyValue{V: resource.PropertyMap{
					resource.PropertyKey("__defaults"): resource.PropertyValue{
						V: []resource.PropertyValue{},
					},
					resource.PropertyKey("nested_field"): resource.PropertyValue{V: "nested_value"},
				}},
			}),
		},
		// TODO[pulumi/pulumi-terraform-bridge#2218]: This should not yield values for properties with defaults in the inputs.
		{
			name: "single nested block with default not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"block_field": map[string]interface{}{"nested_field": "nested_value"},
			}),
			resSchema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"block_field": rschema.SingleNestedBlock{
						Attributes: map[string]rschema.Attribute{
							"nested_field": rschema.StringAttribute{Optional: true, Default: stringdefault.StaticString("nested_value")},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{
				resource.PropertyKey("__defaults"): resource.PropertyValue{
					V: []resource.PropertyValue{},
				},
				resource.PropertyKey("block_field"): resource.PropertyValue{V: resource.PropertyMap{
					resource.PropertyKey("__defaults"): resource.PropertyValue{
						V: []resource.PropertyValue{},
					},
					resource.PropertyKey("nested_field"): resource.PropertyValue{V: "nested_value"}, // wrong
				}},
			}),
		},
		{
			name: "single nested block computed not extracted",
			props: resource.NewPropertyMapFromMap(map[string]interface{}{
				"block_field": map[string]interface{}{"nested_field": "nested_value"},
			}),
			resSchema: rschema.Schema{
				Blocks: map[string]rschema.Block{
					"block_field": rschema.SingleNestedBlock{
						Attributes: map[string]rschema.Attribute{
							"nested_field": rschema.StringAttribute{Computed: true},
						},
					},
				},
			},
			expect: autogold.Expect(resource.PropertyMap{resource.PropertyKey("__defaults"): resource.PropertyValue{
				V: []resource.PropertyValue{},
			}}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prov := pb.NewProvider(pb.NewProviderArgs{
				AllResources: []pb.Resource{
					pb.NewResource(pb.NewResourceArgs{
						Name:           "test",
						ResourceSchema: tc.resSchema,
					}),
				},
			})

			shimmedProvider := schemashim.ShimSchemaOnlyProvider(context.Background(), prov)
			res := shimmedProvider.ResourcesMap().Get("testprovider_test")
			result, err := tfbridge.ExtractInputsFromOutputs(nil, tc.props, res.Schema(), nil, false)
			require.NoError(t, err)
			tc.expect.Equal(t, result)
		})
	}
}
