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

	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"

	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl/hclwrite"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func yamlResource(t T, properties resource.PropertyMap) map[string]any {
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
func Diff(t T, res pb.Resource, tfConfig1, tfConfig2 map[string]cty.Value, options ...DiffOption) crosstestsimpl.DiffResult {
	skipUnlessLinux(t)

	var opts diffOpts
	for _, f := range options {
		f(&opts)
	}

	tfwd := t.TempDir()
	prov1 := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{res},
	})
	prov2 := prov1
	if opts.resource2 != nil {
		prov2 = pb.NewProvider(pb.NewProviderArgs{
			AllResources: []pb.Resource{*opts.resource2},
		})
	}

	shimProvider1 := tfbridge.ShimProvider(prov1)
	shimProvider2 := tfbridge.ShimProvider(prov2)
	// Run the TF part
	var hcl1 bytes.Buffer

	sch1 := hclSchemaPFResource(res.ResourceSchema)
	err := hclwrite.WriteResource(&hcl1, sch1, "testprovider_test", "res", tfConfig1,
		hclwrite.WithCreateBeforeDestroy(true))
	require.NoError(t, err)
	runTFPlanApply(t, prov1, tfwd, hcl1.String())

	var hcl2 bytes.Buffer
	sch2 := sch1
	if opts.resource2 != nil {
		sch2 = hclSchemaPFResource(opts.resource2.ResourceSchema)
	}
	err = hclwrite.WriteResource(&hcl2, sch2, "testprovider_test", "res", tfConfig2,
		hclwrite.WithCreateBeforeDestroy(true))
	require.NoError(t, err)
	tfChanges, tfOut := runTFPlanApply(t, prov2, tfwd, hcl2.String())

	// Run the Pulumi part
	puConfig1 := crosstestsimpl.InferPulumiValue(t,
		shimProvider1.ResourcesMap().Get("testprovider_test").Schema(),
		opts.resourceInfo,
		cty.ObjectVal(tfConfig1),
	)
	pulumiYaml1 := yamlResource(t, puConfig1)

	puConfig2 := crosstestsimpl.InferPulumiValue(t,
		shimProvider2.ResourcesMap().Get("testprovider_test").Schema(),
		opts.resourceInfo,
		cty.ObjectVal(tfConfig2),
	)
	pulumiYaml2 := yamlResource(t, puConfig2)

	bytes, err := yaml.Marshal(pulumiYaml1)
	require.NoError(t, err)
	t.Logf("Pulumi.yaml:\n%s", string(bytes))

	pt1, err := pulcheck.PulCheck(t, prov1.ToProviderInfo(), string(bytes))
	require.NoError(t, err)
	pt1.Up(t)

	state := pt1.ExportStack(t)

	bytes, err = yaml.Marshal(pulumiYaml2)
	require.NoError(t, err)
	t.Logf("Pulumi.yaml:\n%s", string(bytes))
	pt2, err := pulcheck.PulCheck(t, prov2.ToProviderInfo(), string(bytes))
	require.NoError(t, err)
	pt2.ImportStack(t, state)

	previewRes := pt2.Preview(t, optpreview.Diff())
	pulumiRes := pt2.Up(t)
	diffResponse := crosstestsimpl.GetPulumiDiffResponse(t, pt2.GrpcLog(t).Entries)

	crosstestsimpl.VerifyBasicDiffAgreement(t, tfChanges.Actions, pulumiRes.Summary, diffResponse)

	return crosstestsimpl.DiffResult{
		TFDiff:     tfChanges,
		PulumiDiff: diffResponse,
		TFOut:      tfOut,
		PulumiOut:  previewRes.StdOut,
	}
}

func runTFPlanApply(t T, pb *pb.Provider, wd string, hcl string) (tfcheck.TFChange, string) {
	driver := tfcheck.NewTfDriver(t, wd, pb.TypeName, tfcheck.NewTFDriverOpts{
		V6Provider: pb,
	})
	driver.Write(t, hcl)
	plan, err := driver.Plan(t)
	require.NoError(t, err)
	err = driver.ApplyPlan(t, plan)
	require.NoError(t, err)
	return driver.ParseChangesFromTFPlan(plan), plan.StdOut
}

type diffOpts struct {
	resourceInfo map[string]*info.Schema
	resource2    *pb.Resource
}

type DiffOption func(*diffOpts)

// DiffProviderInfo specifies a map of [info.Schema] to apply to the provider under test.
func DiffProviderInfo(info map[string]*info.Schema) DiffOption {
	return func(o *diffOpts) { o.resourceInfo = info }
}

// DiffProviderUpgradedSchema specifies the second provider schema to use for the diff.
func DiffProviderUpgradedSchema(resource2 pb.Resource) DiffOption {
	return func(o *diffOpts) { o.resource2 = &resource2 }
}
