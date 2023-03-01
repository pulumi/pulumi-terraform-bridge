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
	"testing"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
	testutils "github.com/pulumi/pulumi-terraform-bridge/testing/x"
)

func TestConfigure(t *testing.T) {
	server := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	testCase := `
[
  {
    "method": "/pulumirpc.ResourceProvider/Configure",
    "request": {
      "args": {
        "stringConfigProp": "example"
      }
    },
    "response": {
      "acceptSecrets": true,
      "supportsPreview": true,
      "acceptResources": true
    }
  },
  {
    "method": "/pulumirpc.ResourceProvider/Create",
    "request": {
      "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/testres:TestConfigRes::r1",
      "preview": false
    },
    "response": {
      "id": "id-1",
      "properties": {
        "configCopy": "example",
        "id": "id-1"
      }
    }
  }
]
        `
	testutils.ReplaySequence(t, server, testCase)
}
