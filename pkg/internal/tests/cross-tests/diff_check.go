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
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
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
	ObjectType          *tftypes.Object
	DeleteBeforeReplace bool
}

func runDiffCheck(t T, tc diffTestCase) crosstestsimpl.DiffResult {
	t.Helper()
	tfwd := t.TempDir()

	lifecycleArgs := lifecycleArgs{CreateBeforeDestroy: !tc.DeleteBeforeReplace}

	tfConfig1 := coalesceInputs(t, tc.Resource.Schema, tc.Config1)
	tfConfig2 := coalesceInputs(t, tc.Resource.Schema, tc.Config2)
	tfd := newTFResDriver(t, tfwd, defProviderShortName, defRtype, tc.Resource)
	_ = tfd.writePlanApply(t, tc.Resource.Schema, defRtype, "example", tfConfig1, lifecycleArgs)
	tfDiffPlan := tfd.writePlanApply(t, tc.Resource.Schema, defRtype, "example", tfConfig2, lifecycleArgs)

	resMap := map[string]*schema.Resource{defRtype: tc.Resource}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(t, defProviderShortName, tfp)
	if tc.DeleteBeforeReplace {
		bridgedProvider.Resources[defRtype].DeleteBeforeReplace = true
	}

	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
	}

	yamlProgram := pd.generateYAML(t, crosstestsimpl.InferPulumiValue(t,
		bridgedProvider.P.ResourcesMap().Get(defRtype).Schema(), nil, tfConfig1))
	pt := pulcheck.PulCheck(t, bridgedProvider, string(yamlProgram))
	pt.Up(t)

	yamlProgram = pd.generateYAML(t, crosstestsimpl.InferPulumiValue(t,
		bridgedProvider.P.ResourcesMap().Get(defRtype).Schema(), nil, tfConfig2))
	err := os.WriteFile(filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml"), yamlProgram, 0o600)
	require.NoErrorf(t, err, "writing Pulumi.yaml")

	previewRes := pt.Preview(t)
	x := pt.Up(t)

	changes := tfd.driver.ParseChangesFromTFPlan(tfDiffPlan)

	diffResponse := crosstestsimpl.GetPulumiDiffResponse(t, pt)
	crosstestsimpl.VerifyBasicDiffAgreement(t, changes.Actions, x.Summary, diffResponse)

	return crosstestsimpl.DiffResult{
		TFDiff:     changes,
		PulumiDiff: diffResponse,
		TFOut:      tfDiffPlan.StdOut,
		PulumiOut:  previewRes.StdOut,
	}
}
