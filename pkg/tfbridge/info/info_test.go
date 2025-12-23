package info

import (
	"testing"

	"github.com/hexops/autogold/v2"

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
