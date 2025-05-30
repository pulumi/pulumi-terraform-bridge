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

	testutils "github.com/pulumi/providertest/replay"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
)

func TestUpdateWritesSchemaVersion(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.RandomProvider())
	require.NoError(t, err)
	testutils.Replay(t, server, `
	{
	  "method": "/pulumirpc.ResourceProvider/Update",
	  "request": {
	    "id": "0",
	    "urn": "urn:pulumi:dev::repro-pulumi-random::random:index/randomString:RandomString::s",
	    "olds": {
	      "__meta": "{\"schema_version\": \"2\"}",
	      "length": 1,
	      "result": "x",
              "id": "old-id"
	    },
	    "news": {
	      "length": 2
	    }
	  },
	  "response": {
	    "properties": {
	      "__meta": "{\"schema_version\":\"2\"}",
              "*": "*",
	      "id": "*",
	      "length": 2,
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
	    }
	  }
	}
        `)
}

func TestUpdateWithIntID(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)
	testCase := `
        {
          "method": "/pulumirpc.ResourceProvider/Update",
          "request": {
            "id": "1234",
            "olds": {
              "id": "1234"
            },
            "news": {
              "id": "5678"
            },
            "urn": "urn:pulumi:test-stack::basicprogram::testbridge:index/intID:IntID::r1",
            "preview": false
          },
          "response": {
            "properties": {
              "id": "90",
              "*": "*"
            }
          }
        }
        `
	testutils.Replay(t, server, testCase)
}
