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

// Compares the effect of transitioning between two randomly sampled resource configurations.
package crosstests

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/optrun"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/stretchr/testify/require"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/pulcheck"
)

type diffTestCase struct {
	// Schema for the resource to test diffing on.
	Resource *schema.Resource

	// Two resource configurations to simulate an Update from the desired state of Config1 to Config2.
	//
	// Currently they need to be non-nil, but it would make sense to also test Create and Delete flows, especially
	// Create, since there is the non-obvious detail that TF still takes Create calls through the diff logic code
	// including diff customization and PlanResource change.
	//
	// Prefer passing [tftypes.Value] representations.
	Config1, Config2 any

	// Optional object type for the resource. If left nil will be inferred from Resource schema.
	ObjectType                    *tftypes.Object
	DeleteBeforeReplace           bool
	DisableAccurateBridgePreviews bool

	// Optional second schema to use as an upgrade test with a different schema.
	Resource2 *schema.Resource

	// Whether to skip the diff equivalence check.
	SkipDiffEquivalenceCheck bool
}

func runDiffCheck(t T, tc diffTestCase) crosstestsimpl.DiffResult {
	t.Helper()
	tfwd := t.TempDir()

	lifecycleArgs := lifecycleArgs{CreateBeforeDestroy: !tc.DeleteBeforeReplace}
	resource1 := tc.Resource
	resource2 := tc.Resource2
	if resource2 == nil {
		resource2 = resource1
	}

	tfConfig1 := coalesceInputs(t, resource1.Schema, tc.Config1)
	tfd := newTFResDriver(t, tfwd, defProviderShortName, defRtype, resource1)
	_ = tfd.writePlanApply(t, resource1.Schema, defRtype, "example", tfConfig1, lifecycleArgs)

	tfConfig2 := coalesceInputs(t, resource2.Schema, tc.Config2)
	tfd2 := newTFResDriver(t, tfwd, defProviderShortName, defRtype, resource2)
	tfDiffPlan := tfd2.writePlanApply(t, resource2.Schema, defRtype, "example", tfConfig2, lifecycleArgs)

	tfp1 := &schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: resource1}}
	tfp2 := &schema.Provider{ResourcesMap: map[string]*schema.Resource{defRtype: resource2}}

	opts := []pulcheck.BridgedProviderOpt{}
	if !tc.DisableAccurateBridgePreviews {
		opts = append(opts, pulcheck.EnableAccurateBridgePreviews())
	}

	bridgedProvider1 := pulcheck.BridgedProvider(t, defProviderShortName, tfp1, opts...)
	bridgedProvider2 := pulcheck.BridgedProvider(t, defProviderShortName, tfp2, opts...)
	if tc.DeleteBeforeReplace {
		bridgedProvider1.Resources[defRtype].DeleteBeforeReplace = true
		bridgedProvider2.Resources[defRtype].DeleteBeforeReplace = true
	}

	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
	}
	yamlProgram1 := pd.generateYAML(t, crosstestsimpl.InferPulumiValue(t,
		bridgedProvider1.P.ResourcesMap().Get(defRtype).Schema(), nil, tfConfig1))

	yamlProgram2 := pd.generateYAML(t, crosstestsimpl.InferPulumiValue(t,
		bridgedProvider2.P.ResourcesMap().Get(defRtype).Schema(), nil, tfConfig2))

	// We initialize the second provider as it will be used in the preview.
	// It is temporarily overwritten by the first provider in the Run function.
	pt := pulcheck.PulCheck(t, bridgedProvider2, string(yamlProgram1))
	pt.Run(
		t,
		func(test *pulumitest.PulumiTest) {
			test.Up(t)
		},
		optrun.WithOpts(
			opttest.AttachProvider(
				defProviderShortName,
				func(ctx context.Context, pt providers.PulumiTest) (providers.Port, error) {
					handle, err := pulcheck.StartPulumiProvider(ctx, bridgedProvider1)
					require.NoError(t, err)
					return providers.Port(handle.Port), nil
				},
			),
		),
	)
	pt.WritePulumiYaml(t, string(yamlProgram2))
	previewRes := pt.Preview(t, optpreview.Diff())
	require.Empty(t, previewRes.StdErr, "preview should not have errors")

	diffResponse := crosstestsimpl.GetPulumiDiffResponse(t, pt.GrpcLog(t).Entries)
	x := pt.Up(t)

	changes := tfd.driver.ParseChangesFromTFPlan(tfDiffPlan)

	if !tc.SkipDiffEquivalenceCheck {
		crosstestsimpl.VerifyBasicDiffAgreement(t, changes.Actions, x.Summary, diffResponse)
	}

	return crosstestsimpl.DiffResult{
		TFDiff:     changes,
		PulumiDiff: diffResponse,
		TFOut:      tfDiffPlan.StdOut,
		PulumiOut:  previewRes.StdOut,
	}
}
