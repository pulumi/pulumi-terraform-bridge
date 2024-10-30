package tfbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
)

func TestGetModuleMajorVersion(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	assert.Equal(t, "package:module:member", MakeMember("package", "module", "member").String())
}

func TestMakeType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "package:module:member", MakeType("package", "module", "member").String())
}

func TestMakeDataSource(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "package:module/getSomething:getSomething",
		MakeDataSource("package", "module", "getSomething").String())
	assert.Equal(t, "package:module/getSomething:GetSomething",
		MakeDataSource("package", "module", "GetSomething").String())
}

func TestMakeResource(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "package:module/myResource:MyResource", MakeResource("package", "module", "MyResource").String())
}

func TestStringValue(t *testing.T) {
	t.Parallel()
	myMap := map[resource.PropertyKey]resource.PropertyValue{
		"key1": {V: "value1"},
	}
	assert.Equal(t, "value1", StringValue(myMap, "key1"))
	assert.Equal(t, "", StringValue(myMap, "keyThatDoesNotExist"))
}

func TestConfigStringValue(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
		return meBack.Unmarshal().Elem()
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

func TestDelegateIDField(t *testing.T) {
	t.Parallel()

	const (
		providerName = "test-provider"
		repoURL      = "https://example.git"
	)

	errMsg := func(msg string, a ...any) error {
		return delegateIDPropertyError{
			msg:          fmt.Sprintf(msg, a...),
			providerName: providerName,
			repoURL:      repoURL,
		}
	}

	tests := []struct {
		delegate       resource.PropertyKey
		state          resource.PropertyMap
		expectedID     resource.ID
		expectedError  error
		expectedLogMsg string
	}{
		{
			delegate: "key",
			state: resource.PropertyMap{
				"key":   resource.NewProperty("some-id"),
				"other": resource.NewProperty(3.0),
			},
			expectedID: "some-id",
		},
		{
			delegate: "other",
			state: resource.PropertyMap{
				"other": resource.NewProperty(3.0),
			},
			expectedError: errMsg("Expected 'other' property to be a string, found number"),
		},
		{
			delegate: "key",
			state: resource.PropertyMap{
				"other": resource.NewProperty(3.0),
			},
			expectedError: errMsg("Could not find required property 'key' in state"),
		},
		{
			delegate: "key",
			state: resource.PropertyMap{
				"key":   resource.MakeSecret(resource.NewProperty("some-id")),
				"other": resource.NewProperty(3.0),
			},
			expectedID:     "some-id",
			expectedLogMsg: "[warning] [] Setting non-secret resource ID as 'key' (which is secret)\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			var logs bytes.Buffer

			ctx = logging.InitLogging(ctx, logging.LogOptions{
				LogSink: &testLogSink{&logs},
			})

			computeID := DelegateIDField(tt.delegate, providerName, repoURL)
			id, err := computeID(ctx, tt.state)

			if tt.expectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, id)
			} else {
				assert.ErrorIs(t, err, tt.expectedError)
			}

			assert.Equal(t, tt.expectedLogMsg, logs.String())
		})
	}

	t.Run("panic-on-computed", func(t *testing.T) {
		assert.Panics(t, func() {
			computeID := DelegateIDField("computed", providerName, repoURL)
			_, _ = computeID(context.Background(), resource.PropertyMap{
				"computed": resource.MakeComputed(resource.NewProperty("computed")),
			})

		})
	})
}

func TestDelegateIDProperty(t *testing.T) {
	t.Parallel()

	const (
		providerName = "test-provider"
		repoURL      = "https://example.git"
	)

	errMsg := func(msg string, a ...any) error {
		return delegateIDPropertyError{
			msg:          fmt.Sprintf(msg, a...),
			providerName: providerName,
			repoURL:      repoURL,
		}
	}

	tests := []struct {
		delegate       resource.PropertyPath
		state          resource.PropertyMap
		expectedID     resource.ID
		expectedError  error
		expectedLogMsg string
	}{
		{
			delegate: resource.PropertyPath{"key"},
			state: resource.PropertyMap{
				"key":   resource.NewProperty("some-id"),
				"other": resource.NewProperty(3.0),
			},
			expectedID: "some-id",
		},
		{
			delegate: resource.PropertyPath{"nested", "id"},
			state: resource.PropertyMap{
				"nested": resource.NewProperty(resource.PropertyMap{
					"id": resource.NewProperty("my-nested-id"),
				}),
				"other": resource.NewProperty(3.0),
			},
			expectedID: "my-nested-id",
		},
		{
			delegate: resource.PropertyPath{"nested", "id"},
			state: resource.PropertyMap{
				"other": resource.NewProperty(3.0),
			},
			expectedError: errMsg("Could not find required property 'nested.id' in state"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			var logs bytes.Buffer

			ctx = logging.InitLogging(ctx, logging.LogOptions{
				LogSink: &testLogSink{&logs},
			})

			computeID := DelegateIDProperty(tt.delegate, providerName, repoURL)
			id, err := computeID(ctx, tt.state)

			if tt.expectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, id)
			} else {
				assert.ErrorIs(t, err, tt.expectedError)
			}

			assert.Equal(t, tt.expectedLogMsg, logs.String())
		})
	}
}
