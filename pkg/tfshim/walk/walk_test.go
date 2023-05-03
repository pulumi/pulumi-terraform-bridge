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

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
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
