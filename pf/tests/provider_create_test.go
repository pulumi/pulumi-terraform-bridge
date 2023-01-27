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
	"testing"

	testutils "github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateWithComputedOptionals(t *testing.T) {
	server := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
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
              "id": "r1"
            }
          }
        }
        `
	testutils.Replay(t, server, testCase)
}

func TestCreateWritesSchemaVersion(t *testing.T) {
	server := newProviderServer(t, testprovider.RandomProvider())
	ctx := context.Background()
	resp, err := server.Create(ctx, testutils.NewCreateRequest(t, `
           {
             "urn": "urn:pulumi:dev::repro-pulumi-random::random:index/randomString:RandomString::s",
             "properties": {
                "length": 1
              }
          }
        `))
	require.NoError(t, err)
	response := testutils.ParseResponse(t, resp, new(struct {
		Properties struct {
			META interface{} `json:"__meta"`
		} `json:"properties"`
	}))
	assert.Equal(t, `{"schema_version":"2"}`, response.Properties.META)
}
