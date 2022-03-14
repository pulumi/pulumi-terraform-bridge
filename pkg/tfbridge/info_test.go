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
	assert.Equal(t, "package:module/getSomething:getSomething", MakeDataSource("package", "module", "getSomething").String())
	assert.Equal(t, "package:module/getSomething:GetSomething", MakeDataSource("package", "module", "GetSomething").String())
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

func TestTfDataSourceToPulumi(t *testing.T) {
	assert.Equal(t, "", TfDataSourceToPulumi("invalid"))
	assert.Equal(t, "getFoo", TfDataSourceToPulumi("provider_foo"))
	assert.Equal(t, "getFooBar", TfDataSourceToPulumi("provider_foo_bar"))
	assert.Equal(t, "getFooBarBaz", TfDataSourceToPulumi("provider_foo_bar_baz"))
}

func TestTfResourceToPulumi(t *testing.T) {
	assert.Equal(t, "", TfResourceToPulumi("invalid"))
	assert.Equal(t, "Foo", TfResourceToPulumi("provider_foo"))
	assert.Equal(t, "FooBarBaz", TfResourceToPulumi("provider_foo_bar_baz"))
}
