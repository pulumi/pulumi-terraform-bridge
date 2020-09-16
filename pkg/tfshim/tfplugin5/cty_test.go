package tfplugin5

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/stretchr/testify/assert"
)

func testCtyToGo(t *testing.T, expected interface{}, val cty.Value) {
	actual, err := ctyToGo(val)
	if assert.NoError(t, err) {
		assert.Equal(t, expected, actual)
	}
}

func TestBoolean(t *testing.T) {
	testCtyToGo(t, nil, cty.NullVal(cty.Bool))
	testCtyToGo(t, UnknownVariableValue, cty.UnknownVal(cty.Bool))

	testCtyToGo(t, true, cty.True)
	testCtyToGo(t, false, cty.False)
}

func TestNumber(t *testing.T) {
	testCtyToGo(t, nil, cty.NullVal(cty.Number))
	testCtyToGo(t, UnknownVariableValue, cty.UnknownVal(cty.Number))

	testCtyToGo(t, 0.0, cty.NumberIntVal(0))
	testCtyToGo(t, 3.14, cty.NumberFloatVal(3.14))
	testCtyToGo(t, -1e10, cty.NumberFloatVal(-1e10))
}

func TestString(t *testing.T) {
	testCtyToGo(t, nil, cty.NullVal(cty.String))
	testCtyToGo(t, UnknownVariableValue, cty.UnknownVal(cty.String))

	testCtyToGo(t, "", cty.StringVal(""))
	testCtyToGo(t, "Hello, world", cty.StringVal("Hello, world"))
}

func TestTuple(t *testing.T) {
	doubleT := cty.Tuple([]cty.Type{cty.Bool, cty.String})
	tripleT := cty.Tuple([]cty.Type{cty.String, cty.String, cty.String})

	testCtyToGo(t, nil, cty.NullVal(doubleT))
	testCtyToGo(t, UnknownVariableValue, cty.UnknownVal(tripleT))

	testCtyToGo(t, []interface{}{}, cty.TupleVal([]cty.Value{}))
	testCtyToGo(t, []interface{}{true, "foo"}, cty.TupleVal([]cty.Value{cty.True, cty.StringVal("foo")}))
	testCtyToGo(t, []interface{}{"foo"}, cty.TupleVal([]cty.Value{cty.StringVal("foo")}))
	testCtyToGo(t, []interface{}{"foo", false, "bar"},
		cty.TupleVal([]cty.Value{cty.StringVal("foo"), cty.False, cty.StringVal("bar")}))
	testCtyToGo(t, []interface{}{UnknownVariableValue}, cty.TupleVal([]cty.Value{cty.UnknownVal(cty.String)}))
}

func TestList(t *testing.T) {
	listT := cty.List(cty.String)

	testCtyToGo(t, nil, cty.NullVal(listT))
	testCtyToGo(t, UnknownVariableValue, cty.UnknownVal(listT))

	testCtyToGo(t, []interface{}{}, cty.ListValEmpty(cty.String))
	testCtyToGo(t, []interface{}{"foo"}, cty.ListVal([]cty.Value{cty.StringVal("foo")}))
	testCtyToGo(t, []interface{}{"foo", "bar"}, cty.ListVal([]cty.Value{cty.StringVal("foo"), cty.StringVal("bar")}))
	testCtyToGo(t, []interface{}{UnknownVariableValue, "foo"},
		cty.ListVal([]cty.Value{cty.UnknownVal(cty.String), cty.StringVal("foo")}))
}

func TestSet(t *testing.T) {
	setT := cty.Set(cty.String)

	testCtyToGo(t, nil, cty.NullVal(setT))
	testCtyToGo(t, UnknownVariableValue, cty.UnknownVal(setT))

	setVal := cty.SetVal([]cty.Value{cty.StringVal("foo"), cty.StringVal("bar")})
	testCtyToGo(t, setVal, setVal)
}

func TestMap(t *testing.T) {
	mapT := cty.Map(cty.String)

	testCtyToGo(t, nil, cty.NullVal(mapT))
	testCtyToGo(t, UnknownVariableValue, cty.UnknownVal(mapT))

	testCtyToGo(t, map[string]interface{}{
		"foo": "bar",
	}, cty.MapVal(map[string]cty.Value{
		"foo": cty.StringVal("bar"),
	}))
	testCtyToGo(t, map[string]interface{}{
		"foo": "bar",
		"baz": "qux",
	}, cty.MapVal(map[string]cty.Value{
		"foo": cty.StringVal("bar"),
		"baz": cty.StringVal("qux"),
	}))
	testCtyToGo(t, map[string]interface{}{
		"foo": "bar",
		"baz": UnknownVariableValue,
	}, cty.MapVal(map[string]cty.Value{
		"foo": cty.StringVal("bar"),
		"baz": cty.UnknownVal(cty.String),
	}))
}

func TestObject(t *testing.T) {
	objectT := cty.Object(map[string]cty.Type{
		"foo": cty.String,
		"bar": cty.List(cty.String),
	})

	testCtyToGo(t, nil, cty.NullVal(objectT))
	testCtyToGo(t, UnknownVariableValue, cty.UnknownVal(objectT))

	testCtyToGo(t, map[string]interface{}{
		"foo": "bar",
	}, cty.ObjectVal(map[string]cty.Value{
		"foo": cty.StringVal("bar"),
	}))
	testCtyToGo(t, map[string]interface{}{
		"foo": "bar",
		"baz": "qux",
	}, cty.ObjectVal(map[string]cty.Value{
		"foo": cty.StringVal("bar"),
		"baz": cty.StringVal("qux"),
	}))
	testCtyToGo(t, map[string]interface{}{
		"foo": "bar",
		"baz": UnknownVariableValue,
	}, cty.ObjectVal(map[string]cty.Value{
		"foo": cty.StringVal("bar"),
		"baz": cty.UnknownVal(cty.String),
	}))
	testCtyToGo(t, map[string]interface{}{
		"foo": "bar",
		"baz": []interface{}{"qux", "zed"},
	}, cty.ObjectVal(map[string]cty.Value{
		"foo": cty.StringVal("bar"),
		"baz": cty.ListVal([]cty.Value{cty.StringVal("qux"), cty.StringVal("zed")}),
	}))
}
