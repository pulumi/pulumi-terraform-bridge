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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// The Pulumi CLI calls CheckConfig2 on a bridged SDKv2 provider, and the command remediations that a
// PreConfigureCallback attaches to a check failure reach the engine.
func TestCheckConfigRemediationsReachEngine(t *testing.T) {
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
				Reason: "Not signed in. Run `cloud login`.",
				Remediations: []tfbridge.CommandRemediation{{
					Description: "Sign in",
					Command:     []string{"cloud", "login", "--profile", "dev"},
					Env:         map[string]string{"CLOUD_NO_WIZARD": "true"},
				}},
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
	assert.Contains(t, err.Error(), "Not signed in. Run `cloud login`.")

	type checkConfigResponse struct {
		Failures []struct {
			Reason       string `json:"reason"`
			Remediations []struct {
				Description string            `json:"description"`
				Command     []string          `json:"command"`
				Env         map[string]string `json:"env"`
			} `json:"remediations"`
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
		assert.Equal(t, "Not signed in. Run `cloud login`.", failure.Reason)
		require.Len(t, failure.Remediations, 1)
		assert.Equal(t, "Sign in", failure.Remediations[0].Description)
		assert.Equal(t, []string{"cloud", "login", "--profile", "dev"}, failure.Remediations[0].Command)
		assert.Equal(t, map[string]string{"CLOUD_NO_WIZARD": "true"}, failure.Remediations[0].Env)
	}
}
