package tfbridge

import (
	"encoding/json"
	"testing"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestGetModuleMajorVersion(t *testing.T) {
	type testcase struct {
		version              string
		expectedMajorVersion string
	}

	tests := []testcase{
		{
			"v0.0.100",
			"",
		},
		{
			"v1.0.0-alpha.sdqq.dirty",
			"",
		},
		{
			"v2.0.0",
			"v2",
		},
		{
			"v3.147.2-alpha.sdqq.dirty",
			"v3",
		},
	}

	for _, test := range tests {
		majorVersion := GetModuleMajorVersion(test.version)
		assert.Equal(t, test.expectedMajorVersion, majorVersion)
	}
}

func TestMakeMember(t *testing.T) {
	assert.Equal(t, "package:module:member", MakeMember("package", "module", "member").String())
}

func TestMakeType(t *testing.T) {
	assert.Equal(t, "package:module:member", MakeType("package", "module", "member").String())
}

func TestMakeDataSource(t *testing.T) {
	assert.Equal(t, "package:module/getSomething:getSomething",
		MakeDataSource("package", "module", "getSomething").String())
	assert.Equal(t, "package:module/getSomething:GetSomething",
		MakeDataSource("package", "module", "GetSomething").String())
}

func TestMakeResource(t *testing.T) {
	assert.Equal(t, "package:module/myResource:MyResource", MakeResource("package", "module", "MyResource").String())
}

func TestStringValue(t *testing.T) {
	myMap := map[resource.PropertyKey]resource.PropertyValue{
		"key1": {V: "value1"},
	}
	assert.Equal(t, "value1", StringValue(myMap, "key1"))
	assert.Equal(t, "", StringValue(myMap, "keyThatDoesNotExist"))
}

func TestConfigStringValue(t *testing.T) {
	testConf := map[resource.PropertyKey]resource.PropertyValue{
		"strawberries": {V: "cream"},
	}
	t.Setenv("STRAWBERRIES", "shortcake")
	testEnvs := []string{
		"STRAWBERRIES",
	}

	var emptyEnvs []string
	assert.Equal(t, "cream", ConfigStringValue(testConf, "strawberries", testEnvs))
	assert.Equal(t, "cream", ConfigStringValue(testConf, "strawberries", emptyEnvs))
	assert.Equal(t, "shortcake", ConfigStringValue(testConf, "STRAWBERRIES", testEnvs))
	assert.Equal(t, "", ConfigStringValue(testConf, "STRAWBERRIES", emptyEnvs))
	assert.Equal(t, "", ConfigStringValue(testConf, "theseberriesdonotexist", emptyEnvs))
}

func TestConfigBoolValue(t *testing.T) {
	testConf := map[resource.PropertyKey]resource.PropertyValue{
		"apples": {V: true},
	}
	t.Setenv("APPLES", "true")
	testEnvs := []string{
		"APPLES",
	}

	var emptyEnvs []string
	assert.Equal(t, true, ConfigBoolValue(testConf, "apples", testEnvs))
	assert.Equal(t, true, ConfigBoolValue(testConf, "apples", emptyEnvs))
	assert.Equal(t, true, ConfigBoolValue(testConf, "APPLES", testEnvs))
	assert.Equal(t, false, ConfigBoolValue(testConf, "APPLES", emptyEnvs))
	assert.Equal(t, false, ConfigBoolValue(testConf, "thisfruitdoesnotexist", emptyEnvs))
}

func TestConfigArrayValue(t *testing.T) {
	testConf := map[resource.PropertyKey]resource.PropertyValue{
		"fruit_salad": {
			V: []resource.PropertyValue{
				{V: "orange"},
				{V: "pear"},
				{V: "banana"},
			},
		},
	}

	t.Setenv("FRUIT_SALAD", "tangerine;quince;peach")
	testEnvs := []string{
		"FRUIT_SALAD",
	}

	var emptyEnvs []string
	assert.Equal(t, []string{"orange", "pear", "banana"}, ConfigArrayValue(testConf, "fruit_salad", testEnvs))
	assert.Equal(t, []string{"orange", "pear", "banana"}, ConfigArrayValue(testConf, "fruit_salad", emptyEnvs))
	assert.Equal(t, []string{"tangerine", "quince", "peach"}, ConfigArrayValue(testConf, "FRUIT_SALAD", testEnvs))
	assert.Equal(t, []string(nil), ConfigArrayValue(testConf, "FRUIT_SALAD", emptyEnvs))
	assert.Equal(t, []string(nil), ConfigArrayValue(testConf, "idontlikefruitsalad", emptyEnvs))
}

func TestMarshalElem(t *testing.T) {
	turnaround := func(elem interface{}) interface{} {
		me := MarshallableSchema{Elem: MarshalElem(elem)}
		bytes, err := json.Marshal(me)
		if err != nil {
			panic(err)
		}
		t.Logf("me: %#v", me.Elem)
		t.Logf("bytes: %s", string(bytes))
		var meBack MarshallableSchema
		err = json.Unmarshal(bytes, &meBack)
		t.Logf("meBack: %#v", meBack.Elem)
		if err != nil {
			panic(err)
		}
		return meBack.Elem.Unmarshal()
	}

	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, turnaround(nil))
	})

	t.Run("emptySchema", func(t *testing.T) {
		var emptySchema shim.Schema = (&shimschema.Schema{}).Shim()
		actual := turnaround(emptySchema)
		s, ok := actual.(shim.Schema)
		assert.True(t, ok)
		assert.Equal(t, emptySchema, s)
	})

	t.Run("simpleSchema", func(t *testing.T) {
		var simpleSchema shim.Schema = (&shimschema.Schema{
			Type: shim.TypeInt,
		}).Shim()
		actual := turnaround(simpleSchema)
		s, ok := actual.(shim.Schema)
		assert.True(t, ok)
		assert.Equal(t, simpleSchema, s)
	})

	t.Run("emptyResource", func(t *testing.T) {
		var emptyResource shim.Resource = (&shimschema.Resource{}).Shim()
		actual := turnaround(emptyResource)
		r, ok := actual.(shim.Resource)
		assert.True(t, ok)
		assert.Equal(t, 0, r.Schema().Len())
	})

	t.Run("simpleResource", func(t *testing.T) {
		var simpleResource shim.Resource = (&shimschema.Resource{
			SchemaVersion: 1,
			Schema: (&shimschema.SchemaMap{
				"k": (&shimschema.Schema{
					Type: shim.TypeInt,
				}).Shim(),
			}),
		}).Shim()
		actual := turnaround(simpleResource)
		r, ok := actual.(shim.Resource)
		assert.True(t, ok)
		assert.Equal(t, shim.TypeInt, r.Schema().Get("k").Type())
	})
}
