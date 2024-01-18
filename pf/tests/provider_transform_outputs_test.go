// Copyright 2016-2023, Pulumi Corporation.
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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestTransformOutputs(t *testing.T) {
	p := testprovider.SyntheticTestBridgeProvider()

	p.Resources["testbridge_testcompres"].TransformOutputs = func(
		_ context.Context,
		pm resource.PropertyMap,
	) (resource.PropertyMap, error) {
		c := pm.Copy()
		c["ecdsacurve"] = resource.NewStringProperty("TRANSFORMED")
		return c, nil
	}
	provider := newProviderServer(t, p)

	t.Run("Create preview", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Create",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::testbridge:index/testres:Testcompres::exres",
		    "properties": {},
		    "preview": true
		  },
		  "response": {
		    "properties": {
		      "id": "*",
		      "ecdsacurve": "TRANSFORMED"
		    }
		  }
		}`)
	})

	t.Run("Create", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Create",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::testbridge:index/testres:Testcompres::exres",
		    "properties": {
                      "ecdsacurve": "P384"
                    }
		  },
		  "response": {
                    "id": "*",
		    "properties": {
		      "id": "*",
		      "ecdsacurve": "TRANSFORMED"
		    }
		  }
		}`)
	})

	t.Run("Update preview", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Update",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::testbridge:index/testres:Testcompres::exres",
		    "olds": {
                      "ecdsacurve": "P384"
		    },
		    "news": {
                      "ecdsacurve": "P385"
		    },
	            "preview": true
		  },
		  "response": {
		    "properties": {
		      "id": "*",
	              "ecdsacurve": "TRANSFORMED"
		    }
		  }
		}`)
	})

	t.Run("Update", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Update",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::testbridge:index/testres:Testcompres::exres",
		    "olds": {
                      "ecdsacurve": "P384"
		    },
		    "news": {
                      "ecdsacurve": "P385"
		    }
		  },
		  "response": {
		    "properties": {
		      "id": "*",
	              "ecdsacurve": "TRANSFORMED"
		    }
		  }
		}`)
	})

	t.Run("Read to import", func(t *testing.T) {
		testutils.Replay(t, provider, `
		{
		  "method": "/pulumirpc.ResourceProvider/Read",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::testbridge:index/testres:Testcompres::exres",
	            "properties": {}
		  },
		  "response": {
	            "id": "*",
	            "inputs": "*",
		    "properties": {
			"id": "*",
                        "ecdsacurve": "TRANSFORMED"
		    }
		  }
		}`)
	})
}

func TestTransformFromState(t *testing.T) {
	provider := func(t *testing.T) pulumirpc.ResourceProviderServer {
		p := testprovider.AssertProvider(func(config tfsdk.Config, old, new *tfsdk.State) {
			// GetRawState is not available during deletes.
			ctx := context.Background()
			path := path.Root("string_property_value")
			if raw := old.Raw; !raw.IsNull() {
				var s string
				old.GetAttribute(ctx, path, &s)
				assert.Equal(t, "TRANSFORMED", s)
			}
			err := new.SetAttribute(ctx, path, "SET")
			require.Zero(t, err)
		})
		var called bool
		t.Cleanup(func() { assert.True(t, called, "Transform was not called") })

		p.Resources["assert_echo"] = &tfbridge.ResourceInfo{
			Tok: "assert:index/echo:Echo",
			TransformFromState: func(
				ctx context.Context,
				pm resource.PropertyMap,
			) (resource.PropertyMap, error) {
				p := pm.Copy()
				assert.Equal(t, "OLD", p["stringPropertyValue"].StringValue())
				p["stringPropertyValue"] =
					resource.NewStringProperty("TRANSFORMED")
				called = true
				return p, nil
			},
		}

		return newProviderServer(t, p)
	}

	t.Run("Check", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
		{
		  "method": "/pulumirpc.ResourceProvider/Check",
		  "request": {
		    "urn": "urn:pulumi:dev::teststack::assert:index/echo:Echo::exres",
		    "olds": {
		      "stringPropertyValue": "OLD"
		    },
		    "news": {
		      "stringPropertyValue": "NEW"
                    }
		  },
		  "response": {
		    "inputs": {
                      "stringPropertyValue": "NEW"
                    }
		  }
		}`)
	})

	t.Run("Update preview", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
		{
		  "method": "/pulumirpc.ResourceProvider/Update",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::assert:index/echo:Echo::exres",
		    "olds": {
		      "stringPropertyValue": "OLD"
		    },
		    "news": {
		      "stringPropertyValue": "NEW"
                    },
                    "preview": true
		  },
		  "response": {
		    "properties": {
		      "id": "*",
                      "stringPropertyValue": "NEW"
		    }
		  }
		}`)
	})

	t.Run("Update", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
		{
		  "method": "/pulumirpc.ResourceProvider/Update",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::assert:index/echo:Echo::exres",
		    "olds": {
		      "stringPropertyValue": "OLD"
		    },
		    "news": {
		      "stringPropertyValue": "NEW"
		    }
		  },
		  "response": {
		    "properties": {
		      "stringPropertyValue": "SET"
		    }
		  }
		}`)
	})

	t.Run("Diff", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
                {
		  "method": "/pulumirpc.ResourceProvider/Diff",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::assert:index/echo:Echo::exres",
		    "olds": {
		      "stringPropertyValue": "OLD"
		    },
		    "news": {
		      "stringPropertyValue": "TRANSFORMED"
		    }
		  },
		  "response": {
		    "changes": "DIFF_NONE"
		  }
               }`)
	})

	t.Run("Delete", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
                {
		  "method": "/pulumirpc.ResourceProvider/Delete",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::assert:index/echo:Echo::exres",
		    "properties": {
		      "stringPropertyValue": "OLD"
		    }
		  },
		  "response": {}
               }`)
	})

	t.Run("Read (Refresh)", func(t *testing.T) {
		testutils.Replay(t, provider(t), `
		{
		  "method": "/pulumirpc.ResourceProvider/Read",
		  "request": {
		    "id": "0",
		    "urn": "urn:pulumi:dev::teststack::assert:index/echo:Echo::exres",
	            "properties": {
	           	"stringPropertyValue": "OLD"
	            }
		  },
		  "response": {
                    "id": "0",
	            "inputs": "*",
		    "properties": {
                        "id": "0",
			"stringPropertyValue": "SET"
		    }
		  }
		}`)
	})
}
