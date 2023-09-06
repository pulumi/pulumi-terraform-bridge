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

	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
