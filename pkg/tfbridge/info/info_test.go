package info

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	schemashim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestMarshallableProviderSensitiveProperties(t *testing.T) {
	t.Parallel()

	t.Run("resource with sensitive property is marshalled correctly", func(t *testing.T) {
		t.Parallel()

		resource := schemashim.Resource{
			Schema: schemashim.SchemaMap{
				"password": (&schemashim.Schema{
					Type:      shim.TypeString,
					Sensitive: true,
				}).Shim(),
				"username": (&schemashim.Schema{
					Type: shim.TypeString,
				}).Shim(),
			},
		}

		providerShim := &schemashim.Provider{
			ResourcesMap: schemashim.ResourceMap{
				"test_resource": resource.Shim(),
			},
		}

		provider := &Provider{
			Name: "test",
			P:    providerShim.Shim(),
			Resources: map[string]*Resource{
				"test_resource": {
					Tok: "test:index:Resource",
				},
			},
		}

		marshalled := MarshalProvider(provider)
		unmarshalled := marshalled.Unmarshal()

		// Verify the unmarshalled provider has the resource with the sensitive property
		testResource := unmarshalled.P.ResourcesMap().Get("test_resource")
		passwordSchema := testResource.Schema().Get("password")
		usernameSchema := testResource.Schema().Get("username")

		autogold.Expect(true).Equal(t, passwordSchema.Sensitive())
		autogold.Expect(false).Equal(t, usernameSchema.Sensitive())
	})

	t.Run("nested sensitive property is marshalled correctly", func(t *testing.T) {
		t.Parallel()

		resource := schemashim.Resource{
			Schema: schemashim.SchemaMap{
				"config": (&schemashim.Schema{
					Type: shim.TypeList,
					Elem: (&schemashim.Resource{
						Schema: schemashim.SchemaMap{
							"secret_key": (&schemashim.Schema{
								Type:      shim.TypeString,
								Sensitive: true,
							}).Shim(),
							"public_key": (&schemashim.Schema{
								Type: shim.TypeString,
							}).Shim(),
						},
					}).Shim(),
				}).Shim(),
			},
		}

		providerShim := &schemashim.Provider{
			ResourcesMap: schemashim.ResourceMap{
				"test_resource": resource.Shim(),
			},
		}

		provider := &Provider{
			Name: "test",
			P:    providerShim.Shim(),
			Resources: map[string]*Resource{
				"test_resource": {
					Tok: "test:index:Resource",
				},
			},
		}

		marshalled := MarshalProvider(provider)
		unmarshalled := marshalled.Unmarshal()

		// Verify the unmarshalled provider has the nested sensitive property
		testResource := unmarshalled.P.ResourcesMap().Get("test_resource")
		configSchema := testResource.Schema().Get("config")
		nestedResource := configSchema.Elem().(shim.Resource)
		secretKeySchema := nestedResource.Schema().Get("secret_key")
		publicKeySchema := nestedResource.Schema().Get("public_key")

		autogold.Expect(true).Equal(t, secretKeySchema.Sensitive())
		autogold.Expect(false).Equal(t, publicKeySchema.Sensitive())
	})
}

func TestMarshallableProviderPreservesSkipDefaultFixups(t *testing.T) {
	t.Parallel()

	provider := &Provider{
		Name:              "test",
		SkipDefaultFixups: true,
	}

	marshalled := MarshalProvider(provider)
	unmarshalled := marshalled.Unmarshal()

	autogold.Expect(true).Equal(t, unmarshalled.SkipDefaultFixups)
}

// Declared operation timeouts round-trip through the JSON wire form, so mapping
// consumers can tell which operations a resource or data source accepts a
// timeout for. Resources declaring none stay nil.
func TestMarshallableProviderTimeouts(t *testing.T) {
	t.Parallel()

	create, read, deflt := 10*time.Minute, 2*time.Minute, 5*time.Minute
	providerShim := &schemashim.Provider{
		ResourcesMap: schemashim.ResourceMap{
			"test_timed": (&schemashim.Resource{
				Schema:   schemashim.SchemaMap{"name": (&schemashim.Schema{Type: shim.TypeString}).Shim()},
				Timeouts: &shim.ResourceTimeout{Create: &create, Default: &deflt},
			}).Shim(),
			"test_untimed": (&schemashim.Resource{
				Schema: schemashim.SchemaMap{"name": (&schemashim.Schema{Type: shim.TypeString}).Shim()},
			}).Shim(),
		},
		DataSourcesMap: schemashim.ResourceMap{
			"test_source": (&schemashim.Resource{
				Schema:   schemashim.SchemaMap{"name": (&schemashim.Schema{Type: shim.TypeString}).Shim()},
				Timeouts: &shim.ResourceTimeout{Read: &read},
			}).Shim(),
		},
	}

	marshalled := MarshalProviderShim(providerShim.Shim())
	assert.Equal(t, map[string]*MarshallableResourceTimeoutShim{
		"test_timed": {Create: &create, Default: &deflt},
	}, marshalled.ResourceTimeouts)
	assert.Equal(t, map[string]*MarshallableResourceTimeoutShim{
		"test_source": {Read: &read},
	}, marshalled.DataSourceTimeouts)

	data, err := json.Marshal(marshalled)
	require.NoError(t, err)
	var decoded MarshallableProviderShim
	require.NoError(t, json.Unmarshal(data, &decoded))

	unmarshalled := decoded.Unmarshal()
	assert.Equal(t, &shim.ResourceTimeout{Create: &create, Default: &deflt},
		unmarshalled.ResourcesMap().Get("test_timed").Timeouts())
	assert.Nil(t, unmarshalled.ResourcesMap().Get("test_untimed").Timeouts())
	assert.Equal(t, &shim.ResourceTimeout{Read: &read},
		unmarshalled.DataSourcesMap().Get("test_source").Timeouts())
}

// A provider that declares no timeouts marshals to the same JSON as before the
// timeout maps existed, so older consumers of the wire form are unaffected.
func TestMarshallableProviderTimeoutsAbsentFromJSON(t *testing.T) {
	t.Parallel()

	providerShim := &schemashim.Provider{
		ResourcesMap: schemashim.ResourceMap{
			"test_untimed": (&schemashim.Resource{
				Schema: schemashim.SchemaMap{"name": (&schemashim.Schema{Type: shim.TypeString}).Shim()},
			}).Shim(),
		},
	}

	data, err := json.Marshal(MarshalProviderShim(providerShim.Shim()))
	require.NoError(t, err)
	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &keys))
	assert.Equal(t, []string{"resources"}, slices.Sorted(maps.Keys(keys)))
}
