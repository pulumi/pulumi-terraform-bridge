// Copyright 2016-2023, Pulumi Corporation.
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

package walk

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
)

var strSchema = (&schema.Schema{
	Type:     shim.TypeString,
	Optional: true,
}).Shim()

var testSchemaMap shim.SchemaMap = schema.SchemaMap{
	"x": (&schema.Schema{
		Type: shim.TypeMap,
		Elem: (&schema.Resource{
			Schema: schema.SchemaMap{
				"y": strSchema,
			},
		}).Shim(),
	}).Shim(),

	"list": (&schema.Schema{
		Type: shim.TypeList,
		Elem: strSchema,
	}).Shim(),

	"batching": (&schema.Schema{
		Type:     shim.TypeList,
		MaxItems: 1,
		Elem: (&schema.Resource{
			Schema: schema.SchemaMap{
				"send_after": strSchema,
			},
		}).Shim(),
	}).Shim(),
}

func TestLookupSchemaPath(t *testing.T) {
	t.Parallel()
	s := testSchemaMap

	type testCase struct {
		name   string
		path   SchemaPath
		expect any
	}

	testCases := []testCase{
		{
			"single-nested block object",
			NewSchemaPath().GetAttr("x"),
			s.Get("x"),
		},
		{
			"cannot do Element on an object",
			NewSchemaPath().GetAttr("x").Element(),
			fmt.Errorf(`LookupSchemaPath failed at walk.NewSchemaPath().GetAttr("x"): ` +
				`walk.ElementStep{} is not applicable to object types`),
		},
		{
			"nested x.y prop",
			NewSchemaPath().GetAttr("x").GetAttr("y"),
			strSchema,
		},
		{
			"list elem",
			NewSchemaPath().GetAttr("list").Element(),
			strSchema,
		},
		{
			"regress batching.send_after",
			NewSchemaPath().GetAttr("batching").Element().GetAttr("send_after"),
			strSchema,
		},
		{
			"list element object properties",
			NewSchemaPath().GetAttr("batching").Element(),
			wrapSchemaMap(s.Get("batching").Elem().(shim.Resource).Schema()),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			actual, err := LookupSchemaMapPath(tc.path, s)
			switch eerr := tc.expect.(type) {
			case error:
				assert.Error(t, err)
				assert.Equal(t, eerr.Error(), err.Error())
			default:
				assert.NoError(t, err)
				assert.Equal(t, tc.expect, actual)
			}
		})
	}
}

func TestVisitSchemaMap(t *testing.T) {
	t.Parallel()
	expectPaths := []SchemaPath{
		NewSchemaPath().GetAttr("x"),
		NewSchemaPath().GetAttr("x").GetAttr("y"),
		NewSchemaPath().GetAttr("list"),
		NewSchemaPath().GetAttr("list").Element(),
		NewSchemaPath().GetAttr("batching"),
		NewSchemaPath().GetAttr("batching").Element().GetAttr("send_after"),
	}

	VisitSchemaMap(testSchemaMap, func(p SchemaPath, s shim.Schema) {
		assert.Contains(t, expectPaths, p)
		ss, err := LookupSchemaMapPath(p, testSchemaMap)
		assert.NoError(t, err)
		assert.Equal(t, ss, s)
	})
}

func TestEncodeDecodeSchemaPath(t *testing.T) {
	t.Parallel()
	type testCase struct {
		p     SchemaPath
		s     string
		isErr bool
	}

	testCases := []testCase{
		{p: NewSchemaPath(), s: ""},
		{p: NewSchemaPath().GetAttr("a"), s: "a"},
		{p: NewSchemaPath().GetAttr("a").GetAttr("b"), s: "a.b"},
		{p: NewSchemaPath().GetAttr("a").Element().GetAttr("b"), s: "a.$.b"},
		{p: NewSchemaPath().Element(), s: "$"},
		{p: NewSchemaPath().GetAttr("$"), isErr: true},
		{p: NewSchemaPath().GetAttr("foo.bar"), isErr: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.s, func(t *testing.T) {
			ep, err := tc.p.EncodeSchemaPath()
			if tc.isErr {
				require.NotNil(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.s, ep)
				require.Equal(t, tc.p, DecodeSchemaPath(ep))
			}
		})
	}
}

func TestLookupType(t *testing.T) {
	t.Parallel()

	type testCase struct {
		p            SchemaPath
		toplevelType valueshim.Type
		expectedType valueshim.Type
		isErr        bool
	}

	testCases := []testCase{
		{
			p: NewSchemaPath(),
			toplevelType: valueshim.FromTType(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"x": tftypes.String,
				},
			}),
			expectedType: valueshim.FromTType(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"x": tftypes.String,
				},
			}),
		},
		{
			p: NewSchemaPath().GetAttr("x"),
			toplevelType: valueshim.FromTType(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"x": tftypes.String,
				},
			}),
			expectedType: valueshim.FromTType(tftypes.String),
		},
		{
			p: NewSchemaPath().GetAttr("y"),
			toplevelType: valueshim.FromTType(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"x": tftypes.String,
				},
			}),
			isErr: true,
		},
		{
			p: NewSchemaPath().GetAttr("x").GetAttr("y"),
			toplevelType: valueshim.FromTType(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"x": tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"y": tftypes.String,
						},
					},
				},
			}),
			expectedType: valueshim.FromTType(tftypes.String),
		},
		{
			p: NewSchemaPath().Element(),
			toplevelType: valueshim.FromTType(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"x": tftypes.String,
				},
			}),
			isErr: true,
		},
		{
			p: NewSchemaPath().Element(),
			toplevelType: valueshim.FromTType(tftypes.Map{
				ElementType: tftypes.String,
			}),
			expectedType: valueshim.FromTType(tftypes.String),
		},
		{
			p: NewSchemaPath().Element(),
			toplevelType: valueshim.FromTType(tftypes.Set{
				ElementType: tftypes.String,
			}),
			expectedType: valueshim.FromTType(tftypes.String),
		},
		{
			p: NewSchemaPath().Element(),
			toplevelType: valueshim.FromTType(tftypes.List{
				ElementType: tftypes.String,
			}),
			expectedType: valueshim.FromTType(tftypes.String),
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			actualType, err := LookupType(tc.p, tc.toplevelType)
			if tc.isErr {
				require.NotNil(t, err)
				t.Logf("ERROR: %v", err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedType, actualType)
			}
		})
	}
}
