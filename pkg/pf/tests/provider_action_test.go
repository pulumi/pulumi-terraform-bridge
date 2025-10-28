// Copyright 2016-2025, Pulumi Corporation.
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

	testutils "github.com/pulumi/providertest/replay"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
)

func TestActionSchema(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Invoke",
          "request": {
            "tok": "testbridge:index/print:Print",
            "args": {
              "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
              "preview": true,
              "text": "hello world",
              "count": 3
            }
          },
          "response": {
            "return": {}
          }
        }
        `
	testutils.Replay(t, server, testCase)

	testCase = `
        {
          "method": "/pulumirpc.ResourceProvider/Invoke",
          "request": {
            "tok": "testbridge:index/print:Print",
            "args": {
              "urn": "urn:pulumi:st::pg::testprovider:index/res:Res::r",
              "preview": false,
              "text": "hello world"
            }
          },
          "response": {
            "return": {}
          }
        }
        `
	testutils.Replay(t, server, testCase)
}
