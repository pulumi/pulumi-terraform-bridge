// Copyright 2016-2024, Pulumi Corporation.
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

package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	fwres "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	testutils "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
	tfpf "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func TestReadFromRefresh(t *testing.T) {
	t.Parallel()
	// This test case was obtained by running `pulumi refresh` on a simple stack with one RandomPassword.
	//
	// Specifically testing for:
	//
	// - __meta writing out the schema version
	// - implicit upgrade from version 1 to version 3 is performed ("numeric": true) appears
	// - inputs being populated

	server, err := newProviderServer(t, testprovider.RandomProvider())
	require.NoError(t, err)

	testCase := `[
	{
	  "method": "/pulumirpc.ResourceProvider/Configure",
	  "request": {
	    "args": {
	      "version": "4.8.0"
	    },
	    "acceptSecrets": true,
	    "acceptResources": true
	  },
	  "response": {
	    "supportsPreview": true,
	    "acceptResources": true,
	    "supportsAutonamingConfiguration": true
	  },
	  "metadata": {
	    "kind": "resource",
	    "mode": "client",
	    "name": "random"
	  }
	},
	{
	  "method": "/pulumirpc.ResourceProvider/Read",
	  "request": {
	    "id": "none",
	    "urn": "urn:pulumi:dev::repro-pulumi-random-258::random:index/randomPassword:RandomPassword::access-token-",
	    "properties": {
	      "__meta": "{\"schema_version\":\"1\"}",
	      "bcryptHash": "$2a$10$HHwx0gQztkpPIc7WkE4Wt.v7ibWT9Ug24/F5XLa6xNm/gOuyS5WRa",
	      "id": "none",
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "overrideSpecial": "_%@:",
	      "result": "Ps7XGKxa",
	      "special": true,
	      "upper": true
	    },
	    "inputs": {
	      "__defaults": [
		"lower",
		"minLower",
		"minNumeric",
		"minSpecial",
		"minUpper",
		"number",
		"upper"
	      ],
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "overrideSpecial": "_%@:",
	      "special": true,
	      "upper": true
	    }
	  },
	  "response": {
	    "id": "none",
	    "properties": {
	      "__meta": "{\"schema_version\":\"3\"}",
              "*": "*",
	      "bcryptHash": {
		"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		"value": "$2a$10$HHwx0gQztkpPIc7WkE4Wt.v7ibWT9Ug24/F5XLa6xNm/gOuyS5WRa"
	      },
	      "id": "none",
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
              "numeric": true,
	      "overrideSpecial": "_%@:",
	      "result": {
		"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		"value": "Ps7XGKxa"
	      },
	      "special": true,
	      "upper": true
	    },
	    "inputs": {
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "overrideSpecial": "_%@:",
	      "special": true,
	      "upper": true
	    }
	  }
	}]`

	testutils.ReplaySequence(t, server, testCase)
}

func TestImportRandomPassword(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.RandomProvider())
	require.NoError(t, err)
	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Read",
	  "request": {
	    "id": "supersecret",
	    "urn": "urn:pulumi:v2::re::random:index/randomPassword:RandomPassword::newPassword",
	    "properties": {}
	  },
	  "response": {
	    "id": "none",
	    "properties": {
	      "__meta": "{\"schema_version\":\"3\"}",
              "*": "*",
	      "bcryptHash": "*",
	      "id": "none",
	      "length": 11,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "numeric": true,
	      "result": "*",
	      "special": true,
	      "upper": true
	    },
	    "inputs": {
              "length": 11,
              "lower": true,
              "number": true,
              "numeric": true,
              "special": true,
              "upper": true
            }
	  }
	}`
	testutils.Replay(t, server, testCase)
}

func TestImportingResourcesWithBlocks(t *testing.T) {
	t.Parallel()
	// Importing a resource that has blocks such as Testnest resource used to panic. Ensure that it minimally
	// succeeds.
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	testCase := `
	{
          "method": "/pulumirpc.ResourceProvider/Read",
          "request": {
            "id": "zone/929e99f1a4152bfe415bbb3b29d1a227/my-ruleset-id",
            "urn": "urn:pulumi:testing::testing::testbridge:index/testnest:Testnest::myresource",
            "properties": {}
          },
          "response": {
            "id": "*",
            "inputs": "*",
            "properties": {
              "id": "*",
              "*": "*",
              "rules": [
                {
                  "protocol": "some-string"
                }
              ],
              "services": []
            }
          }
        }`
	testutils.Replay(t, server, testCase)
}

func TestImportingResourcesWithoutDefaults(t *testing.T) {
	t.Parallel()
	// Importing a resource that has blocks used to add a `__defaults: []` entry to the `response.inputs`
	// ensure that it no longer does so
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	testCase := `
	{
          "method": "/pulumirpc.ResourceProvider/Read",
          "request": {
            "id": "zone/929e99f1a4152bfe415bbb3b29d1a227/my-ruleset-id",
            "urn": "urn:pulumi:testing::testing::testbridge:index/testnest:Testnest::myresource",
            "properties": {}
          },
          "response": {
            "id": "*",
            "inputs": {
              "rules": [
                {
                  "protocol": "some-string"
                }
              ]
            },
            "properties": {
              "id": "*",
              "*": "*",
              "rules": [
                {
                  "protocol": "some-string"
                }
              ],
              "services": []
            }
          }
        }`
	testutils.Replay(t, server, testCase)
}

// Check that importing a resource that does not exist returns an empty property bag and
// no ID.
func TestImportingMissingResources(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	testCase := `
	{
          "method": "/pulumirpc.ResourceProvider/Read",
          "request": {
            "id": "missing",
            "urn": "urn:pulumi:testing::testing::testbridge:index/testnest:Testnest::myresource",
            "properties": {}
          },
          "response": {
            "inputs": {},
            "properties": {}
          }
        }`
	testutils.Replay(t, server, testCase)
}

func TestImportingResourcesWithNestedAttributes(t *testing.T) {
	t.Parallel()
	// Importing a resource that has attribute blocks such as Testnest resource used to panic. Ensure that it minimally
	// succeeds.
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	testCase := `
	{
          "method": "/pulumirpc.ResourceProvider/Read",
          "request": {
            "id": "zone/929e99f1a4152bfe415bbb3b29d1a227/my-ruleset-id",
            "urn": "urn:pulumi:testing::testing::testbridge:index/testnestattr:Testnestattr::someresource",
            "properties": {}
          },
          "response": {
            "id": "*",
            "inputs": "*",
            "properties": {
              "id": "zone/929e99f1a4152bfe415bbb3b29d1a227/my-ruleset-id",
              "services": [],
              "*": "*"
            }
          }
        }`
	testutils.Replay(t, server, testCase)
}

// Check that refreshing a resource that does not exist returns an empty property bag and
// no ID.
func TestRefreshMissingResources(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	testCase := `
	{
          "method": "/pulumirpc.ResourceProvider/Read",
          "request": {
            "id": "missing",
            "urn": "urn:pulumi:testing::testing::testbridge:index/testnest:Testnest::myresource",
            "properties": {
              "id": "missing",
              "rules": [
                {
                  "protocol": "some-string"
                }
              ],
              "services": []
            }
          },
          "response": {
            "inputs": {},
            "properties": {}
          }
        }`
	testutils.Replay(t, server, testCase)
}

// Provider may indicate intent with resp.State.RemoveResource(ctx) - test that Pulumi respects that.
//
// See https://github.com/pulumi/pulumi-terraform-bridge/issues/1919
func TestRefreshResourceNotFound(t *testing.T) {
	t.Parallel()
	r := pb.NewResource(pb.NewResourceArgs{
		Name: "resource",
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"id": rschema.StringAttribute{Optional: true, Computed: true},
				"x":  rschema.StringAttribute{Optional: true},
			},
		},
		ReadFunc: func(ctx context.Context, req fwres.ReadRequest, resp *fwres.ReadResponse) {
			// Even if the provider sets some attributes, RemoveResource should discard them.
			resp.State.SetAttribute(ctx, path.Root("x"), "value")
			// Indicate that it was no found and should be removed from state.
			resp.State.RemoveResource(ctx)
		},
	})

	p := pb.NewProvider(pb.NewProviderArgs{
		TypeName: "my",
		AllResources: []pb.Resource{
			r,
		},
	})
	info := tfbridge.ProviderInfo{
		Name:         "my",
		P:            tfpf.ShimProvider(p),
		MetadataInfo: &info.Metadata{},
		Version:      "0.0.1",
		Resources: map[string]*info.Resource{
			"my_resource": {
				Tok: "my:index/resource:Resource",
				ComputeID: func(ctx context.Context, state resource.PropertyMap) (resource.ID, error) {
					state["id"] = resource.NewStringProperty("customID")
					return resource.ID("customID"), nil
				},
				Fields: map[string]*info.Schema{
					"id": {
						Name: "customID",
					},
				},
			},
		},
	}
	server, err := newProviderServer(t, info)
	require.NoError(t, err)

	testCase := `
	{
          "method": "/pulumirpc.ResourceProvider/Read",
          "request": {
            "id": "myresource",
            "urn": "urn:pulumi:testing::testing::my:index/resource:Resource::myresource",
            "properties": {
              "id": "myresource"
            }
          },
          "response": {
            "inputs": {},
            "properties": {}
          }
        }`
	testutils.Replay(t, server, testCase)
}

func TestRefreshSupportsCustomID(t *testing.T) {
	t.Parallel()
	p := testprovider.RandomProvider()
	server, err := newProviderServer(t, p)
	require.NoError(t, err)

	p.Resources["random_password"].ComputeID = func(
		ctx context.Context, state resource.PropertyMap,
	) (resource.ID, error) {
		state["id"] = resource.NewStringProperty("customID")
		return resource.ID("customID"), nil
	}

	testCase := `[
	{
	  "method": "/pulumirpc.ResourceProvider/Configure",
	  "request": {
	    "args": {
	      "version": "4.8.0"
	    },
	    "acceptSecrets": true,
	    "acceptResources": true
	  },
	  "response": {
	    "supportsPreview": true,
	    "acceptResources": true,
	    "supportsAutonamingConfiguration": true
	  },
	  "metadata": {
	    "kind": "resource",
	    "mode": "client",
	    "name": "random"
	  }
	},
	{
	  "method": "/pulumirpc.ResourceProvider/Read",
	  "request": {
	    "id": "customID",
	    "urn": "urn:pulumi:dev::repro-pulumi-random-258::random:index/randomPassword:RandomPassword::access-token-",
	    "properties": {
	      "__meta": "{\"schema_version\":\"1\"}",
	      "bcryptHash": "$2a$10$HHwx0gQztkpPIc7WkE4Wt.v7ibWT9Ug24/F5XLa6xNm/gOuyS5WRa",
	      "id": "none",
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "overrideSpecial": "_%@:",
	      "result": "Ps7XGKxa",
	      "special": true,
	      "upper": true
	    },
	    "inputs": {
	      "__defaults": [
		"lower",
		"minLower",
		"minNumeric",
		"minSpecial",
		"minUpper",
		"number",
		"upper"
	      ],
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "overrideSpecial": "_%@:",
	      "special": true,
	      "upper": true
	    }
	  },
	  "response": {
	    "id": "customID",
	    "properties": {
	      "__meta": "*",
              "*": "*",
	      "bcryptHash": "*",
	      "id": "customID",
	      "length": 8,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
              "numeric": true,
	      "overrideSpecial": "_%@:",
	      "result": "*",
	      "special": true,
	      "upper": true
	    },
	    "inputs": "*"
	  }
	}]`

	testutils.ReplaySequence(t, server, testCase)
}

func TestImportSupportsCustomID(t *testing.T) {
	t.Parallel()
	p := testprovider.RandomProvider()
	p.Resources["random_password"].ComputeID = func(
		ctx context.Context, state resource.PropertyMap,
	) (resource.ID, error) {
		state["id"] = resource.NewStringProperty("customID")
		return resource.ID("customID"), nil
	}
	server, err := newProviderServer(t, p)
	require.NoError(t, err)
	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Read",
	  "request": {
	    "id": "supersecret",
	    "urn": "urn:pulumi:v2::re::random:index/randomPassword:RandomPassword::newPassword",
	    "properties": {}
	  },
	  "response": {
	    "id": "customID",
	    "properties": {
	      "__meta": "{\"schema_version\":\"3\"}",
              "*": "*",
	      "bcryptHash": "*",
	      "id": "customID",
	      "length": 11,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "numeric": true,
	      "result": "*",
	      "special": true,
	      "upper": true
	    },
	    "inputs": {
              "length": 11,
              "lower": true,
              "number": true,
              "numeric": true,
              "special": true,
              "upper": true
            }
	  }
	}`
	testutils.Replay(t, server, testCase)
}
