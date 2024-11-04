// Copyright 2016-2024, Pulumi Corporation.
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

package crosstests

import (
	"bytes"
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"
)

func yamlResource(t *testing.T, properties resource.PropertyMap) map[string]any {
	return map[string]any{
		"name":    "project",
		"runtime": "yaml",
		"resources": map[string]any{
			"p": map[string]any{
				"type":       "testprovider:index:Test",
				"properties": crosstests.ConvertResourceValue(t, properties),
			},
		},
	}
}

// Diff will assert that given two resource configurations, the diffs are the same
// when computed by Terraform and Pulumi.
//
// Diff should be safe to run in parallel.
func Diff(t *testing.T, schema rschema.Schema, tfConfig1, tfConfig2 map[string]cty.Value, options ...DiffOption) crosstestsimpl.DiffResult {
	skipUnlessLinux(t)

	var opts diffOpts
	for _, f := range options {
		f(&opts)
	}

	// By default, logs only show when they are on a failed test. By logging to
	// topLevelT, we can log items to be shown if downstream tests fail.
	topLevelT := t

	prov := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{{
			Name:           "test",
			ResourceSchema: schema,
		}},
	})

	shimProvider := tfbridge.ShimProvider(prov)

	var tfOut string
	var pulumiOut string
	var tfChanges tfcheck.TFChange
	var pulumiRes auto.UpResult
	var diffResponse crosstestsimpl.PulumiDiffResp
	t.Run("tf", func(t *testing.T) {
		defer propagateSkip(topLevelT, t)
		var hcl1 bytes.Buffer

		err := writeResource(&hcl1, schema, "testprovider_test", "res", tfConfig1)
		require.NoError(t, err)

		driver := tfcheck.NewTfDriver(t, t.TempDir(), prov.TypeName, prov)

		driver.Write(t, hcl1.String())
		plan, err := driver.Plan(t)
		require.NoError(t, err)
		err = driver.Apply(t, plan)
		require.NoError(t, err)

		var hcl2 bytes.Buffer
		err = writeResource(&hcl2, schema, "testprovider_test", "res", tfConfig2)
		require.NoError(t, err)
		driver.Write(t, hcl2.String())
		plan, err = driver.Plan(t)
		require.NoError(t, err)
		tfChanges = driver.ParseChangesFromTFPlan(plan)
		tfOut = plan.StdOut
	})

	t.Run("bridged", func(t *testing.T) {
		defer propagateSkip(topLevelT, t)

		puConfig1 := crosstestsimpl.InferPulumiValue(t,
			shimProvider.ResourcesMap().Get("testprovider_test").Schema(),
			opts.resourceInfo,
			cty.ObjectVal(tfConfig1),
		)
		pulumiYaml1 := yamlResource(t, puConfig1)

		puConfig2 := crosstestsimpl.InferPulumiValue(t,
			shimProvider.ResourcesMap().Get("testprovider_test").Schema(),
			opts.resourceInfo,
			cty.ObjectVal(tfConfig2),
		)
		pulumiYaml2 := yamlResource(t, puConfig2)

		bytes, err := yaml.Marshal(pulumiYaml1)
		require.NoError(t, err)
		topLevelT.Logf("Pulumi.yaml:\n%s", string(bytes))

		pt, err := pulcheck.PulCheck(t, bridgedProvider(prov), string(bytes))
		require.NoError(t, err)
		pt.Up(t)

		bytes, err = yaml.Marshal(pulumiYaml2)
		require.NoError(t, err)
		topLevelT.Logf("Pulumi.yaml:\n%s", string(bytes))
		pt.WritePulumiYaml(t, string(bytes))

		previewRes := pt.Preview(t)
		pulumiOut = previewRes.StdOut

		pulumiRes = pt.Up(t)

		diffResponse = crosstestsimpl.GetPulumiDiffResponse(t, pt.GrpcLog(t).Entries)
	})

	skipCompare := t.Failed() || t.Skipped()
	t.Run("compare", func(t *testing.T) {
		if skipCompare {
			t.Skipf("skipping since earlier steps did not complete")
		}

		crosstestsimpl.VerifyBasicDiffAgreement(t, tfChanges.Actions, pulumiRes.Summary, diffResponse)
	})

	return crosstestsimpl.DiffResult{
		TFDiff:     tfChanges,
		PulumiDiff: diffResponse,
		TFOut:      tfOut,
		PulumiOut:  pulumiOut,
	}
}

type diffOpts struct {
	resourceInfo map[string]*info.Schema
}

type DiffOption func(*diffOpts)

// DiffProviderInfo specifies a map of [info.Schema] to apply to the provider under test.
func DiffProviderInfo(info map[string]*info.Schema) DiffOption {
	return func(o *diffOpts) { o.resourceInfo = info }
}
