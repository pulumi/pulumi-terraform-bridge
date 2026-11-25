// Copyright 2016-2026, Pulumi Corporation.
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

package tests

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// The Pulumi CLI calls CheckConfig2 on a bridged SDKv2 provider, and the product hint that a PreConfigureCallback
// attaches to a check failure reaches the engine.
func TestCheckConfigHintReachesEngine(t *testing.T) {
	t.Parallel()

	tfp := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"prov_test": {Schema: map[string]*schema.Schema{
				"f0": {Type: schema.TypeString, Optional: true},
			}},
		},
	}
	bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
	bridgedProvider.PreConfigureCallback = func(resource.PropertyMap, shim.ResourceConfig) error {
		return tfbridge.CheckFailureError{
			Failures: []tfbridge.CheckFailureErrorElement{{
				Reason: "No credentials",
				Hint:   &pulumirpc.Hint{Kind: pulumirpc.Hint_DYNAMIC_CREDENTIALS, Message: "Use Pulumi ESC"},
			}},
		}
	}

	pt := pulcheck.PulCheck(t, bridgedProvider, `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      f0: "val"
`)

	_, err := pt.PreviewErr(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No credentials")

	type checkConfigResponse struct {
		Failures []struct {
			Reason string `json:"reason"`
			Hint   *struct {
				Kind    string `json:"kind"`
				Message string `json:"message"`
			} `json:"hint"`
		} `json:"failures"`
	}

	var responses []checkConfigResponse
	for _, entry := range pt.GrpcLog(t).Entries {
		if entry.Method != "/pulumirpc.ResourceProvider/CheckConfig2" {
			continue
		}
		var resp checkConfigResponse
		require.NoError(t, json.Unmarshal(entry.Response, &resp))
		responses = append(responses, resp)
	}

	require.NotEmpty(t, responses, "expected the engine to call CheckConfig2")
	for _, resp := range responses {
		require.Len(t, resp.Failures, 1)
		failure := resp.Failures[0]
		assert.Equal(t, "No credentials", failure.Reason)
		require.NotNil(t, failure.Hint)
		assert.Equal(t, "DYNAMIC_CREDENTIALS", failure.Hint.Kind)
		assert.Equal(t, "Use Pulumi ESC", failure.Hint.Message)
	}
}
