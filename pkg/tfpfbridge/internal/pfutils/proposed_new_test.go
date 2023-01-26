// Copyright 2016-2022, Pulumi Corporation.
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
// limitations under the License.

package pfutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type testcase struct {
	Schema Schema
	Prior  valueBuilder
	Config valueBuilder
	Want   valueBuilder
}

func TestComputedOptionalBecomingUnknown(t *testing.T) {
	schema := rschema.Schema{Attributes: map[string]rschema.Attribute{
		"foo": rschema.SingleNestedAttribute{
			Attributes: map[string]rschema.Attribute{
				"bar": rschema.StringAttribute{
					Optional: true,
					Computed: true,
				},
			},
			Computed: true,
			Optional: true,
		},
	}}
	checkTestCase(t, "basic", testcase{
		Schema: FromResourceSchema(schema),
		Prior:  obj(field("foo", unk())),
		Config: obj(field("foo", obj(field("bar", prim(nil))))),
		Want:   obj(field("foo", obj(field("bar", unk())))),
	})
}

// The following test cases are obtained by tabulating ProposedNew from objchange.go.
//
// https://github.com/hashicorp/terraform/blob/v1.3.6/internal/plans/objchange/objchange.go
func TestProposedNewBaseCases(t *testing.T) {
	schema := rschema.Schema{Attributes: map[string]rschema.Attribute{
		"foo": rschema.StringAttribute{
			Optional: true,
			Computed: true,
		},
	}}

	testcases := map[string]testcase{
		"nested-nestedUnknown": {
			Config: object(map[string]valueBuilder{"foo": unk()}),
			Prior:  object(map[string]valueBuilder{"foo": prim("OK")}),
			Want:   object(map[string]valueBuilder{"foo": unk()}),
		},
		"nestedNull-null": {
			Config: prim(nil),
			Prior:  object(map[string]valueBuilder{"foo": prim(nil)}),
			Want:   object(map[string]valueBuilder{"foo": prim(nil)}),
		},
		"nestedUnknown-nested": {
			Config: object(map[string]valueBuilder{"foo": prim("OK")}),
			Prior:  object(map[string]valueBuilder{"foo": unk()}),
			Want:   object(map[string]valueBuilder{"foo": prim("OK")}),
		},
		"null-nested": {
			Config: object(map[string]valueBuilder{"foo": prim("OK")}),
			Prior:  prim(nil),
			Want:   object(map[string]valueBuilder{"foo": prim("OK")}),
		},
		"nestedNull-nested": {
			Config: object(map[string]valueBuilder{"foo": prim("OK")}),
			Prior:  object(map[string]valueBuilder{"foo": prim(nil)}),
			Want:   object(map[string]valueBuilder{"foo": prim("OK")}),
		},
		"nestedNull-nestedNull": {
			Config: object(map[string]valueBuilder{"foo": prim(nil)}),
			Prior:  object(map[string]valueBuilder{"foo": prim(nil)}),
			Want:   object(map[string]valueBuilder{"foo": prim(nil)}),
		},
		"nestedNull-nestedUnknown": {
			Config: object(map[string]valueBuilder{"foo": unk()}),
			Prior:  object(map[string]valueBuilder{"foo": prim(nil)}),
			Want:   object(map[string]valueBuilder{"foo": unk()}),
		},
		"nestedUnknown-unknown": {
			Config: unk(),
			Prior:  object(map[string]valueBuilder{"foo": unk()}),
			Want:   object(map[string]valueBuilder{"foo": unk()}),
		},
		"nestedUnknown-nestedNull": {
			Config: object(map[string]valueBuilder{"foo": prim(nil)}),
			Prior:  object(map[string]valueBuilder{"foo": unk()}),
			Want:   object(map[string]valueBuilder{"foo": unk()}),
		},
		"nestedUnknown-nestedUnknown": {
			Config: object(map[string]valueBuilder{"foo": unk()}),
			Prior:  object(map[string]valueBuilder{"foo": unk()}),
			Want:   object(map[string]valueBuilder{"foo": unk()}),
		},
		"null-nestedUnknown": {
			Config: object(map[string]valueBuilder{"foo": unk()}),
			Prior:  prim(nil),
			Want:   object(map[string]valueBuilder{"foo": unk()}),
		},
		"null-nestedNull": {
			Config: object(map[string]valueBuilder{"foo": prim(nil)}),
			Prior:  prim(nil),
			Want:   object(map[string]valueBuilder{"foo": prim(nil)}),
		},
		"unknown-nestedNull": {
			Config: object(map[string]valueBuilder{"foo": prim(nil)}),
			Prior:  unk(),
			Want:   object(map[string]valueBuilder{"foo": unk()}),
		},
		"unknown-unknown": {
			Config: unk(),
			Prior:  unk(),
			Want:   unk(),
		},
		"nested-unknown": {
			Config: unk(),
			Prior:  object(map[string]valueBuilder{"foo": prim("OK")}),
			Want:   object(map[string]valueBuilder{"foo": prim("OK")}),
		},
		"nested-nested": {
			Config: object(map[string]valueBuilder{"foo": prim("OK")}),
			Prior:  object(map[string]valueBuilder{"foo": prim("OK")}),
			Want:   object(map[string]valueBuilder{"foo": prim("OK")}),
		},
		"nestedNull-unknown": {
			Config: unk(),
			Prior:  object(map[string]valueBuilder{"foo": prim(nil)}),
			Want:   object(map[string]valueBuilder{"foo": prim(nil)}),
		},
		"nestedUnknown-null": {
			Config: prim(nil),
			Prior:  object(map[string]valueBuilder{"foo": unk()}),
			Want:   object(map[string]valueBuilder{"foo": unk()}),
		},
		"null-unknown": {
			Config: unk(),
			Prior:  prim(nil),
			Want:   object(map[string]valueBuilder{"foo": prim(nil)}),
		},
		"unknown-nestedUnknown": {
			Config: object(map[string]valueBuilder{"foo": unk()}),
			Prior:  unk(),
			Want:   object(map[string]valueBuilder{"foo": unk()}),
		},
		"unknown-null": {
			Config: prim(nil),
			Prior:  unk(),
			Want:   unk(),
		},
		"nested-nestedNull": {
			Config: object(map[string]valueBuilder{"foo": prim(nil)}),
			Prior:  object(map[string]valueBuilder{"foo": prim("OK")}),
			Want:   object(map[string]valueBuilder{"foo": prim("OK")}),
		},
		"nested-null": {
			Config: prim(nil),
			Prior:  object(map[string]valueBuilder{"foo": prim("OK")}),
			Want:   object(map[string]valueBuilder{"foo": prim("OK")}),
		},
		"null-null": {
			Config: prim(nil),
			Prior:  prim(nil),
			Want:   prim(nil),
		},
		"unknown-nested": {
			Config: object(map[string]valueBuilder{"foo": prim("OK")}),
			Prior:  unk(),
			Want:   object(map[string]valueBuilder{"foo": prim("OK")}),
		},
	}

	for name, tc := range testcases {
		tc.Schema = FromResourceSchema(schema)
		checkTestCase(t, name, tc)
	}
}

// The following test cases are translated from:
//
// https://github.com/hashicorp/terraform/blob/v1.3.6/internal/plans/objchange/objchange_test.go#L12
func TestProposedNewWithPortedCases(t *testing.T) {
	testAttributes := map[string]rschema.Attribute{
		"optional": rschema.StringAttribute{
			Optional: true,
		},
		"computed": rschema.StringAttribute{
			Computed: true,
		},
		"optional_computed": rschema.StringAttribute{
			Computed: true,
			Optional: true,
		},
		"required": rschema.StringAttribute{
			Required: true,
		},
	}

	tests := map[string]testcase{

		"empty": {
			FromResourceSchema(rschema.Schema{Attributes: map[string]rschema.Attribute{}}),
			prim(nil),
			prim(nil),
			prim(nil),
		},

		"no prior": (func() testcase {
			schema := rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.StringAttribute{
						Optional: true,
					},
					"bar": rschema.StringAttribute{
						Computed: true,
					},
					"bloop": rschema.SingleNestedAttribute{
						Attributes: map[string]rschema.Attribute{
							"blop": rschema.StringAttribute{
								Required: true,
							},
						},
						Computed: true,
					},
				},
				Blocks: map[string]rschema.Block{
					"baz": rschema.SingleNestedBlock{
						Attributes: map[string]rschema.Attribute{
							"boz": rschema.StringAttribute{
								Optional: true,
								Computed: true,
							},
							"biz": rschema.StringAttribute{
								Optional: true,
								Computed: true,
							},
						},
					},
				},
			}

			config := obj(
				field("foo", prim("hello")),
				field("bloop", prim(nil)),
				field("bar", prim(nil)),
				field("baz", obj(
					field("boz", prim("world")),

					// An unknown in the config represents a situation where
					// an argument is explicitly set to an expression result
					// that is derived from an unknown value. This is distinct
					// from leaving it null, which allows the provider itself
					// to decide the value during PlanResourceChange.
					field("biz", prim(tftypes.UnknownValue)),
				)),
			)

			want := obj(
				field("foo", prim("hello")),

				// unset computed attributes are null in the proposal; provider
				// usually changes them to "unknown" during PlanResourceChange,
				// to indicate that the value will be decided during apply.
				field("bar", prim(nil)),
				field("bloop", prim(nil)),
				field("baz", obj(
					field("boz", prim("world")),
					field("biz", prim(tftypes.UnknownValue)), // explicit unknown preserved from config
				)),
			)

			return testcase{
				Schema: FromResourceSchema(schema),
				Prior:  prim(nil),
				Config: config,
				Want:   want,
			}
		})(),

		"null block remains null": (func() testcase {
			schema := rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.StringAttribute{
						Optional: true,
					},
					"bloop": rschema.SingleNestedAttribute{
						Attributes: map[string]rschema.Attribute{
							"blop": rschema.StringAttribute{
								Required: true,
							},
						},
						Computed: true,
					},
				},
				Blocks: map[string]rschema.Block{
					"baz": rschema.SingleNestedBlock{
						Attributes: map[string]rschema.Attribute{
							"boz": rschema.StringAttribute{
								Optional: true,
								Computed: true,
							},
						},
					},
				},
			}
			config := obj(
				field("foo", prim("bar")),
				field("bloop", prim(nil)),
				field("baz", prim(nil)),
			)

			// The bloop attribue and baz block does not exist in the config, and therefore shouldn't be
			// planned.
			want := obj(
				field("foo", prim("bar")),
				field("bloop", prim(nil)),
				field("baz", prim(nil)),
			)
			return testcase{
				Schema: FromResourceSchema(schema),
				Prior:  prim(nil),
				Config: config,
				Want:   want,
			}
		})(),

		"no prior with set": (func() testcase {
			// This one is here because our handling of sets is more complex than others (due to the fuzzy
			// correlation heuristic) and historically that caused us some panic-related grief.
			schema := rschema.Schema{
				Blocks: map[string]rschema.Block{
					"baz": rschema.SetNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"boz": rschema.StringAttribute{
									Optional: true,
									Computed: true,
								},
							},
						},
					},
				},
				Attributes: map[string]rschema.Attribute{
					"bloop": rschema.SetNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"blop": rschema.StringAttribute{
									Required: true,
								},
							},
						},
						Computed: true,
						Optional: true,
					},
				},
			}
			config := obj(
				field("baz", set(obj(field("boz", prim("world"))))),
				field("bloop", set(obj(field("blop", prim("blub"))))),
			)
			want := obj(
				field("baz", set(obj(field("boz", prim("world"))))),
				field("bloop", set(obj(field("blop", prim("blub"))))),
			)
			return testcase{
				Schema: FromResourceSchema(schema),
				Prior:  prim(nil),
				Config: config,
				Want:   want,
			}
		})(),

		"prior attributes": (func() testcase {
			schema := rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.StringAttribute{
						Optional: true,
					},
					"bar": rschema.StringAttribute{
						Computed: true,
					},
					"baz": rschema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"boz": rschema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"bloop": rschema.SingleNestedAttribute{
						Attributes: map[string]rschema.Attribute{
							"blop": rschema.StringAttribute{
								Required: true,
							},
						},
						Optional: true,
					},
				},
			}
			prior := obj(
				field("foo", prim("bonjour")),
				field("bar", prim("petit dejeuner")),
				field("baz", prim("grande dejeuner")),
				field("boz", prim("a la monde")),
				field("bloop", obj(field("blop", prim("glub")))),
			)
			config := obj(
				field("foo", prim("hello")),
				field("bar", prim(nil)),
				field("baz", prim(nil)),
				field("boz", prim("world")),
				field("bloop", obj(field("blop", prim("bleep")))),
			)
			want := obj(
				field("foo", prim("hello")),
				field("bar", prim("petit dejeuner")),
				field("baz", prim("grande dejeuner")),
				field("boz", prim("world")),
				field("bloop", obj(field("blop", prim("bleep")))),
			)
			return testcase{
				Schema: FromResourceSchema(schema),
				Prior:  prior,
				Config: config,
				Want:   want,
			}
		})(),

		"prior nested single": {
			FromResourceSchema(rschema.Schema{
				Blocks: map[string]rschema.Block{
					"foo": rschema.SingleNestedBlock{
						Attributes: map[string]rschema.Attribute{
							"bar": rschema.StringAttribute{
								Optional: true,
								Computed: true,
							},
							"baz": rschema.StringAttribute{
								Optional: true,
								Computed: true,
							},
						},
					},
				},
				Attributes: map[string]rschema.Attribute{
					"bloop": rschema.SingleNestedAttribute{
						Attributes: map[string]rschema.Attribute{
							"blop": rschema.StringAttribute{
								Required: true,
							},
							"bleep": rschema.StringAttribute{
								Optional: true,
							},
						},
						Optional: true,
					},
				},
			}),
			object(map[string]valueBuilder{
				"foo": object(map[string]valueBuilder{
					"bar": prim("beep"),
					"baz": prim("boop"),
				}),

				"bloop": object(map[string]valueBuilder{
					"blop":  prim("glub"),
					"bleep": prim(nil),
				}),
			}),
			object(map[string]valueBuilder{
				"foo": object(map[string]valueBuilder{
					"bar": prim("bap"),
					"baz": prim(nil),
				}),
				"bloop": object(map[string]valueBuilder{
					"blop":  prim("glub"),
					"bleep": prim("beep"),
				}),
			}),
			object(map[string]valueBuilder{
				"foo": object(map[string]valueBuilder{
					"bar": prim("bap"),
					"baz": prim("boop"),
				}),
				"bloop": object(map[string]valueBuilder{
					"blop":  prim("glub"),
					"bleep": prim("beep"),
				}),
			}),
		},

		"prior nested list": {
			FromResourceSchema(rschema.Schema{
				Blocks: map[string]rschema.Block{
					"foo": rschema.ListNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{
									Optional: true,
									Computed: true,
								},
								"baz": rschema.StringAttribute{
									Optional: true,
									Computed: true,
								},
							},
						},
					},
				},
				Attributes: map[string]rschema.Attribute{
					"bloop": rschema.ListNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"blop": rschema.StringAttribute{
									Required: true,
								},
							},
						},
						Optional: true,
					},
				},
			}),
			object(map[string]valueBuilder{
				"foo": list(
					object(map[string]valueBuilder{
						"bar": prim("beep"),
						"baz": prim("boop"),
					}),
				),
				"bloop": list(
					object(map[string]valueBuilder{
						"blop": prim("bar"),
					}),

					object(map[string]valueBuilder{
						"blop": prim("baz"),
					}),
				),
			}),

			object(map[string]valueBuilder{
				"foo": list(
					object(map[string]valueBuilder{
						"bar": prim("bap"),
						"baz": prim(nil),
					}),

					object(map[string]valueBuilder{
						"bar": prim("blep"),
						"baz": prim(nil),
					})),

				"bloop": list(
					object(map[string]valueBuilder{
						"blop": prim("bar"),
					}),

					object(map[string]valueBuilder{
						"blop": prim("baz"),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": list(
					object(map[string]valueBuilder{
						"bar": prim("bap"),
						"baz": prim("boop"),
					}),

					object(map[string]valueBuilder{
						"bar": prim("blep"),
						"baz": prim(nil),
					})),

				"bloop": list(
					object(map[string]valueBuilder{
						"blop": prim("bar"),
					}),

					object(map[string]valueBuilder{
						"blop": prim("baz"),
					})),
			}),
		},

		"prior nested map": {
			FromResourceSchema(rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"bloop": rschema.MapNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"blop": rschema.StringAttribute{
									Required: true,
								},
							},
						},
						Optional: true,
					},
				},
			}),
			object(map[string]valueBuilder{
				"bloop": mapv(map[string]valueBuilder{
					"a": object(map[string]valueBuilder{
						"blop": prim("glub"),
					}),

					"b": object(map[string]valueBuilder{
						"blop": prim("blub"),
					}),
				}),
			}),

			object(map[string]valueBuilder{
				"bloop": mapv(map[string]valueBuilder{
					"a": object(map[string]valueBuilder{
						"blop": prim("glub"),
					}),

					"c": object(map[string]valueBuilder{
						"blop": prim("blub"),
					}),
				}),
			}),

			object(map[string]valueBuilder{
				"bloop": mapv(map[string]valueBuilder{
					"a": object(map[string]valueBuilder{
						"blop": prim("glub"),
					}),

					"c": object(map[string]valueBuilder{
						"blop": prim("blub"),
					}),
				}),
			}),
		},
		"prior nested set": {
			FromResourceSchema(rschema.Schema{
				Blocks: map[string]rschema.Block{
					"foo": rschema.SetNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{
									// This non-computed attribute will serve
									// as our matching key for propagating
									// "baz" from elements in the prior value.
									Optional: true,
								},
								"baz": rschema.StringAttribute{
									Optional: true,
									Computed: true,
								},
							},
						},
					},
				},
				Attributes: map[string]rschema.Attribute{
					"bloop": rschema.SetNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"blop": rschema.StringAttribute{
									Required: true,
								},
								"bleep": rschema.StringAttribute{
									Optional: true,
								},
							},
						},
						Optional: true,
					},
				},
			}),
			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": prim("beep"),
						"baz": prim("boop"),
					}),

					object(map[string]valueBuilder{
						"bar": prim("blep"),
						"baz": prim("boot"),
					})),

				"bloop": set(
					object(map[string]valueBuilder{
						"blop":  prim("glubglub"),
						"bleep": prim(nil),
					}),

					object(map[string]valueBuilder{
						"blop":  prim("glubglub"),
						"bleep": prim("beep"),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": prim("beep"),
						"baz": prim(nil),
					}),

					object(map[string]valueBuilder{
						"bar": prim("bosh"),
						"baz": prim(nil),
					})),

				"bloop": set(
					object(map[string]valueBuilder{
						"blop":  prim("glubglub"),
						"bleep": prim(nil),
					}),

					object(map[string]valueBuilder{
						"blop":  prim("glub"),
						"bleep": prim(nil),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": prim("beep"),
						"baz": prim("boop"),
					}),

					object(map[string]valueBuilder{
						"bar": prim("bosh"),
						"baz": prim(nil),
					})),

				"bloop": set(
					object(map[string]valueBuilder{
						"blop":  prim("glubglub"),
						"bleep": prim(nil),
					}),

					object(map[string]valueBuilder{
						"blop":  prim("glub"),
						"bleep": prim(nil),
					})),
			}),
		},

		"sets differing only by unknown": {
			FromResourceSchema(rschema.Schema{
				Blocks: map[string]rschema.Block{
					"multi": rschema.SetNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Attributes: map[string]rschema.Attribute{
								"optional": rschema.StringAttribute{
									Optional: true,
									Computed: true,
								},
							},
						},
					},
				},
				Attributes: map[string]rschema.Attribute{
					"bloop": rschema.SetNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"blop": schema.StringAttribute{
									Required: true,
								},
							},
						},
						Optional: true,
					},
				},
			}),
			prim(nil),
			object(map[string]valueBuilder{
				"multi": set(
					object(map[string]valueBuilder{
						"optional": unk(),
					}),

					object(map[string]valueBuilder{
						"optional": unk(),
					})),

				"bloop": set(
					object(map[string]valueBuilder{
						"blop": unk(),
					}),

					object(map[string]valueBuilder{
						"blop": unk(),
					})),
			}),

			object(map[string]valueBuilder{
				"multi": set(

					object(map[string]valueBuilder{
						"optional": unk(),
					}),

					object(map[string]valueBuilder{
						"optional": unk(),
					})),

				"bloop": set(
					object(map[string]valueBuilder{
						"blop": unk(),
					}),

					object(map[string]valueBuilder{
						"blop": unk(),
					})),
			}),
		},

		"nested list in set": {
			FromResourceSchema(rschema.Schema{
				Blocks: map[string]rschema.Block{
					"foo": rschema.SetNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Blocks: map[string]rschema.Block{
								"bar": rschema.ListNestedBlock{
									NestedObject: rschema.NestedBlockObject{
										Attributes: map[string]rschema.Attribute{
											"baz": rschema.StringAttribute{},
											"qux": rschema.StringAttribute{
												Computed: true,
												Optional: true,
											},
										},
									},
								},
							},
						},
					},
				},
			}),
			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": list(
							object(map[string]valueBuilder{
								"baz": prim("beep"),
								"qux": prim("boop"),
							})),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": list(
							object(map[string]valueBuilder{
								"baz": prim("beep"),
								"qux": prim(nil),
							})),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": list(
							object(map[string]valueBuilder{
								"baz": prim("beep"),
								"qux": prim("boop"),
							})),
					})),
			}),
		},

		"empty nested list in set": {
			FromResourceSchema(rschema.Schema{
				Blocks: map[string]rschema.Block{
					"foo": rschema.SetNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Blocks: map[string]rschema.Block{
								"bar": rschema.ListNestedBlock{
									NestedObject: rschema.NestedBlockObject{
										Blocks: map[string]rschema.Block{},
									},
								},
							},
						},
					},
				},
			}),
			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": list(),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": list(),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": list(),
					})),
			}),
		},

		// Could not port empty nested map in set since tfsdk.BlockNestingModeMap is not supported, substituting
		// an empty object instead.
		"empty nested object in set": {
			FromResourceSchema(rschema.Schema{
				Blocks: map[string]rschema.Block{
					"foo": rschema.SetNestedBlock{
						NestedObject: rschema.NestedBlockObject{
							Blocks: map[string]rschema.Block{
								"bar": rschema.SingleNestedBlock{
									Attributes: map[string]rschema.Attribute{
										"baz": rschema.StringAttribute{
											Optional: true,
										},
									},
								},
							},
						},
					},
				},
			}),
			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": object(nil),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": object(nil),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": object(nil),
					})),
			}),
		},

		// This example has a mixture of optional, computed and required in a deeply-nested NestedType attribute
		"deeply NestedType": {
			FromResourceSchema(rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.SingleNestedAttribute{
						Attributes: map[string]rschema.Attribute{
							"bar": rschema.SingleNestedAttribute{
								Attributes: testAttributes,
								Required:   true,
							},
							"baz": rschema.SingleNestedAttribute{
								Attributes: testAttributes,
								Optional:   true,
							},
						},
						Optional: true,
					},
				},
			}),

			object(map[string]valueBuilder{
				"foo": object(map[string]valueBuilder{
					"bar": prim(nil),
					"baz": object(map[string]valueBuilder{
						"optional":          prim(nil),
						"computed":          prim("hello"),
						"optional_computed": prim("prior"),
						"required":          prim("present"),
					}),
				}),
			}),

			object(map[string]valueBuilder{
				"foo": object(map[string]valueBuilder{
					"bar": unk(),

					"baz": object(map[string]valueBuilder{
						"optional":          prim(nil),
						"computed":          prim(nil),
						"optional_computed": prim("hello"),
						"required":          prim("present"),
					}),
				}),
			}),

			object(map[string]valueBuilder{
				"foo": object(map[string]valueBuilder{
					"bar": unk(),

					"baz": object(map[string]valueBuilder{
						"optional":          prim(nil),
						"computed":          prim("hello"),
						"optional_computed": prim("hello"),
						"required":          prim("present"),
					}),
				}),
			}),
		},

		"deeply nested set": {
			FromResourceSchema(rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"foo": rschema.SetNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.SetNestedAttribute{
									NestedObject: rschema.NestedAttributeObject{
										Attributes: testAttributes,
									},
									Required: true,
								},
							},
						},
					},
				},
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": set(
							object(map[string]valueBuilder{
								"optional":          prim("prior"),
								"computed":          prim("prior"),
								"optional_computed": prim("prior"),
								"required":          prim("prior"),
							})),
					}),

					object(map[string]valueBuilder{
						"bar": set(object(map[string]valueBuilder{
							"optional":          prim("other_prior"),
							"computed":          prim("other_prior"),
							"optional_computed": prim("other_prior"),
							"required":          prim("other_prior"),
						})),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": set(object(map[string]valueBuilder{
							"optional":          prim("configured"),
							"computed":          prim(nil),
							"optional_computed": prim("configured"),
							"required":          prim("configured"),
						})),
					}),

					object(map[string]valueBuilder{
						"bar": set(object(map[string]valueBuilder{
							"optional":          prim(nil),
							"computed":          prim(nil),
							"optional_computed": prim("other_configured"),
							"required":          prim("other_configured"),
						})),
					})),
			}),

			object(map[string]valueBuilder{
				"foo": set(
					object(map[string]valueBuilder{
						"bar": set(object(map[string]valueBuilder{
							"optional":          prim("configured"),
							"computed":          prim(nil),
							"optional_computed": prim("configured"),
							"required":          prim("configured"),
						})),
					}),

					object(map[string]valueBuilder{
						"bar": set(object(map[string]valueBuilder{
							"optional":          prim(nil),
							"computed":          prim(nil),
							"optional_computed": prim("other_configured"),
							"required":          prim("other_configured"),
						})),
					})),
			}),
		},

		"expected null NestedTypes": {
			FromResourceSchema(rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"single": rschema.SingleNestedAttribute{
						Attributes: map[string]rschema.Attribute{
							"bar": rschema.StringAttribute{},
						},
						Optional: true,
					},
					"list": rschema.ListNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{},
							},
						},
						Optional: true,
					},
					"set": rschema.SetNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{},
							},
						},
						Optional: true,
					},
					"map": rschema.MapNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{},
							},
						},
						Optional: true,
					},
					"nested_map": rschema.MapNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"inner": rschema.SingleNestedAttribute{
									Attributes: testAttributes,
								},
							},
						},
						Optional: true,
					},
				},
			}),
			object(map[string]valueBuilder{
				"single": object(map[string]valueBuilder{"bar": prim("baz")}),
				"list":   list(object(map[string]valueBuilder{"bar": prim("baz")})),
				"map": mapv(map[string]valueBuilder{
					"map_entry": object(map[string]valueBuilder{"bar": prim("baz")}),
				}),
				"set": set(object(map[string]valueBuilder{"bar": prim("baz")})),
				"nested_map": mapv(map[string]valueBuilder{
					"a": object(map[string]valueBuilder{
						"inner": object(map[string]valueBuilder{
							"optional":          prim("foo"),
							"computed":          prim("foo"),
							"optional_computed": prim("foo"),
							"required":          prim("foo"),
						}),
					}),
				}),
			}),

			object(map[string]valueBuilder{
				"single":     prim(nil),
				"list":       prim(nil),
				"map":        prim(nil),
				"set":        prim(nil),
				"nested_map": prim(nil),
			}),

			object(map[string]valueBuilder{
				"single":     prim(nil),
				"list":       prim(nil),
				"map":        prim(nil),
				"set":        prim(nil),
				"nested_map": prim(nil),
			}),
		},

		"expected empty NestedTypes": {
			FromResourceSchema(rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"set": rschema.SetNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{},
							},
						},
						Optional: true,
					},
					"map": rschema.MapNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{},
							},
						},
						Optional: true,
					},
				},
			}),
			object(map[string]valueBuilder{
				"map": mapv(nil),
				"set": set(),
			}),

			object(map[string]valueBuilder{
				"map": mapv(nil),
				"set": set(),
			}),

			object(map[string]valueBuilder{
				"map": mapv(nil),
				"set": set(),
			}),
		},

		"optional types set replacement": {
			FromResourceSchema(rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"set": rschema.SetNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"bar": rschema.StringAttribute{
									Required: true,
								},
							},
						},
						Optional: true,
					},
				},
			}),
			object(map[string]valueBuilder{
				"set": set(
					object(map[string]valueBuilder{
						"bar": prim("old"),
					})),
			}),

			object(map[string]valueBuilder{
				"set": set(
					object(map[string]valueBuilder{
						"bar": prim("new"),
					})),
			}),

			object(map[string]valueBuilder{
				"set": set(
					object(map[string]valueBuilder{
						"bar": prim("new"),
					})),
			}),
		},

		"prior null nested objects": {
			FromResourceSchema(rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"single": rschema.SingleNestedAttribute{
						Attributes: map[string]rschema.Attribute{
							"list": rschema.ListNestedAttribute{
								NestedObject: rschema.NestedAttributeObject{
									Attributes: map[string]rschema.Attribute{
										"foo": rschema.StringAttribute{},
									},
								},
								Optional: true,
							},
						},
						Optional: true,
					},
					"map": rschema.MapNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"list": rschema.ListNestedAttribute{
									NestedObject: rschema.NestedAttributeObject{
										Attributes: map[string]rschema.Attribute{
											"foo": rschema.StringAttribute{},
										},
									},
									Optional: true,
								},
							},
						},
						Optional: true,
					},
				},
			}),
			prim(nil),

			object(map[string]valueBuilder{
				"single": object(map[string]valueBuilder{
					"list": list(
						object(map[string]valueBuilder{
							"foo": prim("a"),
						}),

						object(map[string]valueBuilder{
							"foo": prim("b"),
						})),
				}),

				"map": mapv(map[string]valueBuilder{
					"one": object(map[string]valueBuilder{
						"list": list(
							object(map[string]valueBuilder{
								"foo": prim("a"),
							}),

							object(map[string]valueBuilder{
								"foo": prim("b"),
							})),
					}),
				}),
			}),

			object(map[string]valueBuilder{
				"single": object(map[string]valueBuilder{
					"list": list(
						object(map[string]valueBuilder{
							"foo": prim("a"),
						}),

						object(map[string]valueBuilder{
							"foo": prim("b"),
						})),
				}),

				"map": mapv(map[string]valueBuilder{
					"one": object(map[string]valueBuilder{
						"list": list(
							object(map[string]valueBuilder{
								"foo": prim("a"),
							}),

							object(map[string]valueBuilder{
								"foo": prim("b"),
							})),
					}),
				}),
			}),
		},

		// data sources are planned with an unknown value
		"unknown prior nested objects": {
			FromResourceSchema(rschema.Schema{
				Attributes: map[string]rschema.Attribute{
					"list": rschema.ListNestedAttribute{
						NestedObject: rschema.NestedAttributeObject{
							Attributes: map[string]rschema.Attribute{
								"list": rschema.ListNestedAttribute{
									NestedObject: rschema.NestedAttributeObject{
										Attributes: map[string]rschema.Attribute{
											"foo": rschema.StringAttribute{},
										},
									},
									Computed: true,
								},
							},
						},
						Computed: true,
					},
				},
			}),
			unk(),

			prim(nil),

			unk(),
		},
	}

	for name, test := range tests {
		checkTestCase(t, name, test)
	}
}

func checkTestCase(t *testing.T, name string, test testcase) {
	t.Run(name, func(t *testing.T) {
		ctx := context.Background()
		ty := test.Schema.Type().TerraformType(ctx)
		got, err := ProposedNew(ctx, test.Schema, test.Prior(ty), test.Config(ty))
		require.NoError(t, err)
		if !got.Equal(test.Want(ty)) {
			t.Errorf("wrong result\ngot:  %s\nwant: %s", got, test.Want(ty))
			diffs, err := got.Diff(test.Want(ty))
			require.NoError(t, err)
			for i, d := range diffs {
				t.Logf("Diff %d at %s:\n  got: %s\nwant: %s",
					i+1, d.Path.String(), d.Value1, d.Value2)
			}
		}
	})
}
