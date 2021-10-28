package tfplugin5

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/stretchr/testify/assert"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/tfplugin5/proto"
)

func add(new string, replace bool) shim.ResourceAttrDiff {
	return shim.ResourceAttrDiff{
		New:         new,
		RequiresNew: replace,
	}
}

func remove(old string, replace bool) shim.ResourceAttrDiff {
	return shim.ResourceAttrDiff{
		Old:         old,
		NewRemoved:  true,
		RequiresNew: replace,
	}
}

func update(old, new string, replace bool) shim.ResourceAttrDiff {
	return shim.ResourceAttrDiff{
		Old:         old,
		New:         new,
		RequiresNew: replace,
	}
}

func path(elements ...interface{}) *proto.AttributePath {
	steps := make([]*proto.AttributePath_Step, len(elements))
	for i, e := range elements {
		switch e := e.(type) {
		case string:
			steps[i] = &proto.AttributePath_Step{
				Selector: &proto.AttributePath_Step_AttributeName{AttributeName: e},
			}
		case int:
			steps[i] = &proto.AttributePath_Step{
				Selector: &proto.AttributePath_Step_ElementKeyInt{ElementKeyInt: int64(e)},
			}
		}
	}
	return &proto.AttributePath{Steps: steps}
}

func resolvePath(path *proto.AttributePath, ty cty.Type) {
	for _, s := range path.Steps {
		switch sel := s.Selector.(type) {
		case *proto.AttributePath_Step_AttributeName:
			if ty.IsMapType() {
				s.Selector = &proto.AttributePath_Step_ElementKeyString{ElementKeyString: sel.AttributeName}
			} else if ty.IsObjectType() {
				// This case only exists to handle the fact that set element keys passed to path() are strings. This
				// should be changed once set element key info is validated.
				ty = ty.AttributeType(sel.AttributeName)
				continue
			}
		case *proto.AttributePath_Step_ElementKeyInt:
			if ty.IsTupleType() {
				ty = ty.TupleElementType(int(sel.ElementKeyInt))
				continue
			}
		}

		ty = ty.ElementType()
	}
}

func diffTest(t *testing.T, attributes map[string]cty.Type, requiresReplace []*proto.AttributePath,
	planned, prior interface{}, expected map[string]shim.ResourceAttrDiff) {

	objectType := cty.Object(attributes)

	for _, p := range requiresReplace {
		resolvePath(p, objectType)
	}

	priorVal, err := goToCty(prior, objectType)
	if !assert.NoError(t, err) {
		return
	}
	plannedVal, err := goToCty(planned, objectType)
	if !assert.NoError(t, err) {
		return
	}

	expectedDiff := &instanceDiff{
		config:     plannedVal,
		planned:    plannedVal,
		attributes: expected,
	}
	for _, v := range expected {
		if v.RequiresNew {
			expectedDiff.requiresNew = true
			break
		}
	}

	actual := newInstanceDiff(plannedVal, priorVal, plannedVal, nil, requiresReplace)
	assert.Equal(t, expectedDiff, actual)
}

func TestEmptyDiff(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.String,
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{})
}

func TestSimpleAdd(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.String,
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop": add("foo", false),
		})
}

func TestSimpleAddReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.String,
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop": add("foo", true),
		})
}

func TestSimpleDelete(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.String,
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop": remove("foo", false),
		})
}

func TestSimpleDeleteReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.String,
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop": remove("foo", true),
		})
}

func TestSimpleUpdate(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.String,
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": "baz",
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop": update("foo", "baz", false),
		})
}

func TestSimpleUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.String,
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": "baz",
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop": update("foo", "baz", true),
		})
}

func TestNestedAdd(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.%":    add("1", false),
			"prop.nest": add("foo", false),
		})
}

func TestNestedAddReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.%":    add("1", true),
			"prop.nest": add("foo", true),
		})

	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop", "nest")},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.%":    add("1", false),
			"prop.nest": add("foo", true),
		})
}

func TestNestedDelete(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.%":    remove("1", false),
			"prop.nest": remove("foo", false),
		})
}

func TestNestedDeleteReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.%":    remove("1", true),
			"prop.nest": remove("foo", true),
		})

	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop", "nest")},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.%":    remove("1", false),
			"prop.nest": remove("foo", true),
		})
}

func TestNestedUpdate(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "baz"},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.nest": update("foo", "baz", false),
		})
}

func TestNestedUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "baz"},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.nest": update("foo", "baz", true),
		})

	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop", "nest")},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "baz"},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.nest": update("foo", "baz", true),
		})
}

func TestListAdd(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#": add("1", false),
			"prop.0": add("foo", false),
		})
}

func TestListAddReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#": add("1", true),
			"prop.0": add("foo", true),
		})

	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop", 0)},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#": add("1", false),
			"prop.0": add("foo", true),
		})
}

func TestListDelete(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#": remove("1", false),
			"prop.0": remove("foo", false),
		})
}

func TestListDeleteReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#": remove("1", true),
			"prop.0": remove("foo", true),
		})

	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop", 0)},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#": remove("1", false),
			"prop.0": remove("foo", true),
		})
}

func TestListUpdate(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": []interface{}{"baz"},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.0": update("foo", "baz", false),
		})
}

func TestListUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": []interface{}{"baz"},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.0": update("foo", "baz", true),
		})

	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop", 0)},
		map[string]interface{}{
			"prop": []interface{}{"baz"},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.0": update("foo", "baz", true),
		})
}

func TestSetAdd(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Set(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#":         add("1", false),
			"prop.400595255": add("foo", false),
		})
}

func TestSetAddReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Set(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#":         add("1", true),
			"prop.400595255": add("foo", true),
		})
}

func TestSetDelete(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Set(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#":         remove("1", false),
			"prop.400595255": remove("foo", false),
		})
}

func TestSetDeleteReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Set(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#":         remove("1", true),
			"prop.400595255": remove("foo", true),
		})
}

func TestUnknownSimpleUpdate(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.String,
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": UnknownVariableValue,
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop": update("foo", UnknownVariableValue, false),
		})
}

func TestUnknownSimpleUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.String,
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": UnknownVariableValue,
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": "foo",
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop": update("foo", UnknownVariableValue, true),
		})
}

func TestUnknownMapUpdate(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": UnknownVariableValue,
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.%": update("1", UnknownVariableValue, false),
		})
}

func TestUnknownNestedUpdate(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": UnknownVariableValue},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.nest": update("foo", UnknownVariableValue, false),
		})
}

func TestUnknownNestedUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": UnknownVariableValue},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.nest": update("foo", UnknownVariableValue, true),
		})

	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Map(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop", "nest")},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": UnknownVariableValue},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": map[string]interface{}{"nest": "foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.nest": update("foo", UnknownVariableValue, true),
		})
}

func TestUnknownListUpdate(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": UnknownVariableValue,
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#": update("1", UnknownVariableValue, false),
		})
}

func TestUnknownListElementUpdate(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": []interface{}{UnknownVariableValue},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.0": update("foo", UnknownVariableValue, false),
		})
}

func TestUnknownListElementUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": []interface{}{UnknownVariableValue},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.0": update("foo", UnknownVariableValue, true),
		})

	diffTest(t,
		map[string]cty.Type{
			"prop": cty.List(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop", 0)},
		map[string]interface{}{
			"prop": []interface{}{UnknownVariableValue},
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.0": update("foo", UnknownVariableValue, true),
		})
}

func TestUnknownSetUpdate(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Set(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{},
		map[string]interface{}{
			"prop": UnknownVariableValue,
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#": update("1", UnknownVariableValue, false),
		})
}

func TestUnknownSetUpdateReplace(t *testing.T) {
	diffTest(t,
		map[string]cty.Type{
			"prop": cty.Set(cty.String),
			"outp": cty.String,
		},
		[]*proto.AttributePath{path("prop")},
		map[string]interface{}{
			"prop": UnknownVariableValue,
			"outp": "bar",
		},
		map[string]interface{}{
			"prop": []interface{}{"foo"},
			"outp": "bar",
		},
		map[string]shim.ResourceAttrDiff{
			"prop.#": update("1", UnknownVariableValue, true),
		})
}
