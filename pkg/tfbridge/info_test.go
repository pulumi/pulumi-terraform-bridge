package tfbridge

import (
	"testing"

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
