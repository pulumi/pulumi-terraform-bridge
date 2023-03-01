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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateWritesSchemaVersion(t *testing.T) {
	server := newProviderServer(t, testprovider.RandomProvider())
	ctx := context.Background()
	resp, err := server.Update(ctx, testutils.NewUpdateRequest(t, `
           {
             "id": "0",
             "urn": "urn:pulumi:dev::repro-pulumi-random::random:index/randomString:RandomString::s",
             "olds": {
               "__meta": "{\"schema_version\": \"2\"}",
               "length": 1,
               "result": "x"
             },
             "news": {
               "length": 2
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
