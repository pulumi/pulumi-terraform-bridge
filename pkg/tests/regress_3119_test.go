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
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
)

// A no-op refresh must not rewrite the inputs recorded in state for secret-valued objects.
//
// extractInputs had no secret case, so a secret-wrapped object matched none of its array, object or
// string branches and fell through to `default: return newState`. Refresh therefore replaced the
// recorded input with the raw state, dropping nested __defaults markers. The resulting per-resource
// input drift defeats the engine's checkpoint-write elision in sameSnapshotMutation.mustWrite.
//
// The assertion is deliberately made against state rather than against a preview diff: the bridge
// reports DIFF_NONE for this drift, so ExpectNoChanges passes on both refresh and preview while the
// stored inputs have in fact changed.
//
// See https://github.com/pulumi/pulumi-terraform-bridge/issues/3119.
func TestRegress3119(t *testing.T) {
	t.Parallel()

	resMap := map[string]*schema.Resource{
		"prov_test": {
			Schema: map[string]*schema.Schema{
				"credentials": {
					Type:      schema.TypeList,
					MaxItems:  1,
					Optional:  true,
					Sensitive: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"key":       {Type: schema.TypeString, Optional: true},
							"generated": {Type: schema.TypeString, Computed: true},
						},
					},
				},
			},
			ReadContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
				require.NoError(t, rd.Set("credentials", []interface{}{
					map[string]interface{}{"key": "k", "generated": "g"},
				}))
				return nil
			},
			CreateContext: func(_ context.Context, rd *schema.ResourceData, _ interface{}) diag.Diagnostics {
				require.NoError(t, rd.Set("credentials", []interface{}{
					map[string]interface{}{"key": "k", "generated": "g"},
				}))
				rd.SetId("id0")
				return nil
			},
		},
	}

	bridgedProvider := pulcheck.BridgedProvider(t, "prov", &schema.Provider{ResourcesMap: resMap})
	pt := pulcheck.PulCheck(t, bridgedProvider, `
name: test
runtime: yaml
resources:
  mainRes:
    type: prov:index:Test
    properties:
      credentials:
        key: "k"
`)
	pt.Up(t)

	before := secretCredentials(t, pt)
	require.Equal(t, "k", before["key"])
	require.NotContains(t, before, "generated",
		"precondition: a computed field is not an input")

	pt.Refresh(t)

	require.NotContains(t, secretCredentials(t, pt), "generated",
		"refresh must not copy computed state into the recorded inputs of a secret object")
}

// secretCredentials returns the plaintext contents of the secret `credentials` input.
func secretCredentials(t *testing.T, pt *pulumitest.PulumiTest) map[string]interface{} {
	t.Helper()

	credentials, ok := stackResourceInputs(t, pt)["credentials"].(map[string]interface{})
	require.True(t, ok, "credentials should be an object")

	plaintext, ok := credentials["plaintext"].(string)
	require.True(t, ok, "credentials should be a secret with a plaintext field")

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(plaintext), &decoded))
	return decoded
}

func stackResourceInputs(t *testing.T, pt *pulumitest.PulumiTest) map[string]interface{} {
	t.Helper()

	data, err := pt.ExportStack(t).Deployment.MarshalJSON()
	require.NoError(t, err)

	var deployment struct {
		Resources []struct {
			Type   string                 `json:"type"`
			Inputs map[string]interface{} `json:"inputs"`
		} `json:"resources"`
	}
	require.NoError(t, json.Unmarshal(data, &deployment))

	for _, r := range deployment.Resources {
		if r.Type == "prov:index/test:Test" {
			return r.Inputs
		}
	}

	t.Fatal("did not find the resource in the exported stack")
	return nil
}
