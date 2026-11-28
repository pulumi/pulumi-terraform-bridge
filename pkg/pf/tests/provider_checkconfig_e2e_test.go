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

package tfbridgetests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

const remediationReason = "Not signed in. Run `cloud login`."

func failWithRemediation(resource.PropertyMap, shim.ResourceConfig) error {
	return tfbridge0.CheckFailureError{
		Failures: []tfbridge0.CheckFailureErrorElement{{
			Reason: remediationReason,
			Remediations: []tfbridge0.CommandRemediation{{
				Description: "Sign in",
				Command:     []string{"cloud", "login", "--profile", "dev"},
				Env:         map[string]string{"CLOUD_NO_WIZARD": "true"},
			}},
		}},
	}
}

// assertRemediationReachedEngine runs a preview that fails provider configuration, then checks that the Pulumi CLI
// called CheckConfig2 and that each response carries the remediation.
func assertRemediationReachedEngine(t *testing.T, pt *pulumitest.PulumiTest) {
	_, err := pt.PreviewErr(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), remediationReason)

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
		assert.Equal(t, remediationReason, failure.Reason)
		require.Len(t, failure.Remediations, 1)
		assert.Equal(t, "Sign in", failure.Remediations[0].Description)
		assert.Equal(t, []string{"cloud", "login", "--profile", "dev"}, failure.Remediations[0].Command)
		assert.Equal(t, map[string]string{"CLOUD_NO_WIZARD": "true"}, failure.Remediations[0].Env)
	}
}

// The Pulumi CLI calls CheckConfig2 on a bridged Plugin Framework provider, and the command remediations that a
// PreConfigureCallback attaches to a check failure reach the engine.
func TestPFCheckConfigRemediationsReachEngine(t *testing.T) {
	t.Parallel()

	provider := pb.NewProvider(pb.NewProviderArgs{
		TypeName: "testprovider",
		AllResources: []pb.Resource{pb.NewResource(pb.NewResourceArgs{
			Name: "r",
		})},
	})
	providerInfo := provider.ToProviderInfo()
	providerInfo.EnableZeroDefaultSchemaVersion = true
	providerInfo.PreConfigureCallback = failWithRemediation

	pt, err := pulcheck.PulCheck(t, providerInfo, `
name: test
runtime: yaml
resources:
  mainRes:
    type: testprovider:index:R
`)
	require.NoError(t, err)

	assertRemediationReachedEngine(t, pt)
}

// A muxed SDKv2 and Plugin Framework provider broadcasts CheckConfig2 to both servers. Both report the same failure,
// and the merged response keeps its command remediations.
func TestMuxedCheckConfigRemediationsReachEngine(t *testing.T) {
	t.Parallel()

	providerInfo := testprovider.MuxedRandomProvider()
	providerInfo.PreConfigureCallback = failWithRemediation
	server := newMuxedProviderServer(t, providerInfo)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(`
name: test
runtime: yaml
resources:
  mainRes:
    type: muxedrandom:index/randomInteger:RandomInteger
    properties:
      min: 1
      max: 10
`), 0o600))

	pt := pulumitest.NewPulumiTest(t, dir,
		opttest.Env("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true"),
		opttest.TestInPlace(),
		opttest.SkipInstall(),
		opttest.AttachProviderServer(providerInfo.Name,
			func(providers.PulumiTest) (pulumirpc.ResourceProviderServer, error) { return server, nil }),
	)

	assertRemediationReachedEngine(t, pt)
}
