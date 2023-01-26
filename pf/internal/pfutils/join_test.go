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
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoin(t *testing.T) {
	cases := []struct {
		name      string
		opts      JoinOptions
		x         tftypes.Value
		y         tftypes.Value
		expected  tftypes.Value
		expectNil bool
	}{
		{
			name:     "str-left",
			opts:     left(),
			x:        strValue("A"),
			y:        strValue("B"),
			expected: strValue("A"),
		},
		{
			name:     "str-right",
			opts:     right(),
			x:        strValue("A"),
			y:        strValue("B"),
			expected: strValue("B"),
		},
		{
			name:     "obj-replace",
			opts:     right(),
			x:        xyObjValue(strValue("A"), numValue(1)),
			y:        xyObjValue(strValue("A"), numValue(2)),
			expected: xyObjValue(strValue("A"), numValue(2)),
		},
		{
			name:     "make-list-longer",
			opts:     right(),
			x:        listValue(strValue("A")),
			y:        listValue(strValue("A"), strValue("B")),
			expected: listValue(strValue("A"), strValue("B")),
		},
		{
			name:     "make-list-shorter",
			opts:     right(),
			x:        listValue(strValue("A"), strValue("B")),
			y:        listValue(strValue("A")),
			expected: listValue(strValue("A")),
		},
		{
			name:     "make-map-longer",
			opts:     right(),
			x:        mapValue(map[string]tftypes.Value{"a": strValue("A")}),
			y:        mapValue(map[string]tftypes.Value{"a": strValue("A"), "b": strValue("B")}),
			expected: mapValue(map[string]tftypes.Value{"a": strValue("A"), "b": strValue("B")}),
		},
		{
			name:     "make-map-shorter",
			opts:     right(),
			x:        mapValue(map[string]tftypes.Value{"a": strValue("A"), "b": strValue("B")}),
			y:        mapValue(map[string]tftypes.Value{"a": strValue("A")}),
			expected: mapValue(map[string]tftypes.Value{"a": strValue("A")}),
		},
		{
			name:     "map-with-unknown",
			opts:     right(),
			x:        tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, tftypes.UnknownValue),
			y:        mapValue(map[string]tftypes.Value{"a": strValue("A")}),
			expected: mapValue(map[string]tftypes.Value{"a": strValue("A")}),
		},
		{
			name:     "union-sets",
			opts:     rightOrLeft(),
			x:        setValue(strValue("A"), strValue("B")),
			y:        setValue(strValue("A"), strValue("C")),
			expected: setValue(strValue("A"), strValue("B"), strValue("C")),
		},
		{
			name:     "union-objects-with-missing-fields",
			opts:     rightOrLeftWithNulls(),
			x:        xyObjOptValue(justValue(strValue("XV")), nil),
			y:        xyObjOptValue(nil, nil),
			expected: xyObjOptValue(justValue(strValue("XV")), nil),
		},
		{
			name:     "join-objects-injects-typed-nulls",
			opts:     removeAll(),
			x:        xyObjValue(strValue("A"), numValue(1)),
			y:        xyObjValue(strValue("B"), numValue(2)),
			expected: xyObjOptValue(nil, nil),
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			z, err := Join(c.opts, tftypes.NewAttributePath(), &c.x, &c.y)
			require.NoError(t, err)
			if c.expectNil {
				assert.Nil(t, z)
			} else {
				require.NotNil(t, z)
				assert.Equal(t, c.expected, *z)
			}
		})
	}
}

func left() JoinOptions {
	return JoinOptions{Reconcile: func(diff Diff) (*tftypes.Value, error) {
		return diff.Value1, nil
	}}
}

func right() JoinOptions {
	return JoinOptions{Reconcile: func(diff Diff) (*tftypes.Value, error) {
		return diff.Value2, nil
	}}
}

func rightOrLeft() JoinOptions {
	return JoinOptions{Reconcile: func(diff Diff) (*tftypes.Value, error) {
		if diff.Value2 != nil {
			return diff.Value2, nil
		}
		return diff.Value1, nil
	}}
}

func rightOrLeftWithNulls() JoinOptions {
	return JoinOptions{Reconcile: func(diff Diff) (*tftypes.Value, error) {
		if diff.Value2 != nil && !diff.Value2.IsNull() {
			return diff.Value2, nil
		}
		return diff.Value1, nil
	}}
}

func removeAll() JoinOptions {
	return JoinOptions{Reconcile: func(diff Diff) (*tftypes.Value, error) {
		return nil, nil
	}}
}

func xyObjValue(x, y tftypes.Value) tftypes.Value {
	return tftypes.NewValue(tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"x": tftypes.String,
			"y": tftypes.Number,
		},
	}, map[string]tftypes.Value{
		"x": x,
		"y": y,
	})
}

func xyObjOptValue(x, y *tftypes.Value) tftypes.Value {
	values := map[string]tftypes.Value{}
	if x != nil {
		values["x"] = *x
	} else {
		values["x"] = tftypes.NewValue(tftypes.String, nil)
	}
	if y != nil {
		values["y"] = *y
	} else {
		values["y"] = tftypes.NewValue(tftypes.Number, nil)
	}

	return tftypes.NewValue(tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"x": tftypes.String,
			"y": tftypes.Number,
		},
	}, values)
}

func justValue(x tftypes.Value) *tftypes.Value {
	return &x
}

func listValue(vs ...tftypes.Value) tftypes.Value {
	return tftypes.NewValue(tftypes.List{ElementType: vs[0].Type()}, vs)
}

func setValue(vs ...tftypes.Value) tftypes.Value {
	return tftypes.NewValue(tftypes.Set{ElementType: vs[0].Type()}, vs)
}

func strValue(s string) tftypes.Value {
	return tftypes.NewValue(tftypes.String, s)
}

func mapValue(elems map[string]tftypes.Value) tftypes.Value {
	var t tftypes.Type
	for _, v := range elems {
		t = v.Type()
		break
	}
	return tftypes.NewValue(tftypes.Map{ElementType: t}, elems)
}

func numValue(n int) tftypes.Value {
	return tftypes.NewValue(tftypes.Number, n)
}
