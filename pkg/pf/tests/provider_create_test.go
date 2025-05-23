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

package tfbridgetests

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	testutils "github.com/pulumi/providertest/replay"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
)

func TestCreateWithComputedOptionals(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Create",
          "request": {
            "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/testres:Testcompres::r1",
            "properties": {
              "ecdsacurve": "P384"
            },
            "preview": false
          },
          "response": {
            "id": "r1",
            "properties": {
              "ecdsacurve": "P384",
              "id": "r1",
              "*": "*"
            }
          }
        }
        `
	testutils.Replay(t, server, testCase)
}

func TestCreateWithIntID(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Create",
          "request": {
            "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/intID:IntID::r1",
            "properties": {},
            "preview": false
          },
          "response": {
            "id": "1234",
            "properties": {
              "id": "1234",
              "*": "*"
            }
          }
        }
        `
	testutils.Replay(t, server, testCase)
}

func TestCreateWritesSchemaVersion(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.RandomProvider())
	require.NoError(t, err)

	testutils.Replay(t, server, `
	{
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::repro-pulumi-random::random:index/randomString:RandomString::s",
	    "properties": {
	      "length": 1
	    }
	  },
	  "response": {
	    "id": "*",
	    "properties": {
	      "__meta": "{\"schema_version\":\"2\"}",
              "*": "*",
	      "id": "*",
	      "result": "*",
	      "length": 1,
	      "lower": true,
	      "minLower": 0,
	      "minNumeric": 0,
	      "minSpecial": 0,
	      "minUpper": 0,
	      "number": true,
	      "numeric": true,
	      "special": true,
	      "upper": true
	    }
	  }
	}
        `)
}

func TestPreviewCreate(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.RandomProvider())
	require.NoError(t, err)

	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::repro::random:index/randomInteger:RandomInteger::k",
	    "properties": {
	      "max": 10,
	      "min": 0
	    },
	    "preview": true
	  },
	  "response": {
	    "properties": {
	      "id": "04da6b54-80e4-46f7-96ec-b56ff0331ba9",
	      "max": 10,
	      "min": 0,
	      "result": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
	    }
	  },
	  "metadata": {
	    "kind": "resource",
	    "mode": "client",
	    "name": "random"
	  }
	}
`
	testutils.Replay(t, server, testCase)
}

func TestMuxedAliasCreate(t *testing.T) {
	t.Parallel()
	server := newMuxedProviderServer(t, testprovider.MuxedRandomProvider())

	testCase := func(typ string) string {
		return `
	{
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::repro::` + typ + `::k"
	  },
	  "response": {
	    "id": "4",
	    "properties": {
	      "id": "4",
	      "fair": true,
	      "number": 4,
	      "suggestionUpdated": false,
              "suggestion": "*",
              "*": "*"
	    }
	  },
	  "metadata": {
	    "kind": "resource",
	    "mode": "client",
	    "name": "muxedrandom"
	  }
	}
`
	}

	t.Run("new-token", func(t *testing.T) {
		testutils.Replay(t, server,
			testCase("muxedrandom:index/randomHumanNumber:RandomHumanNumber"))
	})
	t.Run("legacy-token", func(t *testing.T) {
		testutils.Replay(t, server,
			testCase("muxedrandom:index/myNumber:MyNumber"))
	})
}

func TestCreateWithFirstClassSecrets(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.RandomProvider())
	require.NoError(t, err)
	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::pulumi-terraform-bridge-812::random:index/randomPet:RandomPet::pet",
	    "properties": {
	      "separator": {
		"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		"value": "BbAXG:}h"
	      }
	    },
	    "preview": true
	  },
	  "response": {
            "properties": {
              "id": "*",
              "length": 2,
              "separator": "BbAXG:}h"
            }
          }
	}`
	testutils.Replay(t, server, testCase)
}

func TestCreateWithSchemaBasedSecrets(t *testing.T) {
	t.Parallel()
	// Ensure that resources that mark output properties as secret in the schema return them as secrets.
	// RandomPassword is a good example. Surprisingly this test requires a Configure call first, otherwise the
	// plubming is confused about secrets bits and retursn the wrong result. The test represents production use.
	server, err := newProviderServer(t, testprovider.RandomProvider())
	require.NoError(t, err)
	testCase := `
	[
	  {
	    "method": "/pulumirpc.ResourceProvider/Configure",
	    "request": {
	      "args": {},
	      "acceptSecrets": true,
	      "acceptResources": true
	    },
	    "response": "*"
	  },
	  {
	    "method": "/pulumirpc.ResourceProvider/Create",
	    "request": {
	      "urn": "urn:pulumi:dev::secret-random-yaml::random:index/randomPassword:RandomPassword::param",
	      "properties": {
		"length": 10
	      }
	    },
	    "response": {
	      "id": "none",
	      "properties": {
		"__meta": "{\"schema_version\":\"3\"}",
                "*": "*",
		"bcryptHash": {
		  "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		  "value": "*"
		},
		"id": "none",
		"length": 10,
		"lower": true,
		"minLower": 0,
		"minNumeric": 0,
		"minSpecial": 0,
		"minUpper": 0,
		"number": true,
		"numeric": true,
		"result": {
		  "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		  "value": "*"
		},
		"special": true,
		"upper": true
	      }
	    }
	  }
	]`
	testutils.ReplaySequence(t, server, testCase)
}

func TestCreateSupportsCustomID(t *testing.T) {
	t.Parallel()
	p := testprovider.RandomProvider()
	p.Resources["random_pet"].ComputeID = func(
		ctx context.Context, state resource.PropertyMap,
	) (resource.ID, error) {
		newID := fmt.Sprintf("customID%v", state["length"].NumberValue())
		state["id"] = resource.NewStringProperty(newID)
		return resource.ID(newID), nil
	}
	server, err := newProviderServer(t, p)
	require.NoError(t, err)
	testCase := `
	{
	  "method": "/pulumirpc.ResourceProvider/Create",
	  "request": {
	    "urn": "urn:pulumi:dev::pulumi-terraform-bridge-812::random:index/randomPet:RandomPet::pet",
	    "properties": {
	      "separator": {
		"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		"value": "BbAXG:}h"
	      }
	    }
	  },
	  "response": {
            "id": "customID2",
            "properties": {
              "id": "customID2",
              "length": 2,
              "separator": "BbAXG:}h",
              "*": "*"
            }
          }
	}`
	testutils.Replay(t, server, testCase)
}

func TestPFCrossTestCreateBasic(t *testing.T) {
	t.Parallel()

	res := pb.NewResource(
		pb.NewResourceArgs{
			ResourceSchema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"hello": schema.StringAttribute{
						Optional: true,
					},
				},
			},
		},
	)

	crosstests.Create(t, res, map[string]cty.Value{
		"hello": cty.StringVal("world"),
	})
}

func TestPFCreateDynamicAttribute(t *testing.T) {
	t.Parallel()

	res := pb.NewResource(
		pb.NewResourceArgs{
			ResourceSchema: schema.Schema{
				Attributes: map[string]schema.Attribute{
					"hello": schema.StringAttribute{
						Optional: true,
					},
					"dynamic": schema.DynamicAttribute{
						Optional: true,
					},
				},
			},
		},
	)

	crosstests.Create(t, res, map[string]cty.Value{
		"hello": cty.StringVal("world"),
	})
}
