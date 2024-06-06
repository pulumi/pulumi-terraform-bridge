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
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	ObjectType *tftypes.Object
}

func runDiffCheck(t T, tc diffTestCase) {
	tfwd := t.TempDir()

	tfd := newTfDriver(t, tfwd, defProviderShortName, defRtype, tc.Resource)
	_ = tfd.writePlanApply(t, tc.Resource.Schema, defRtype, "example", tc.Config1)
	tfDiffPlan := tfd.writePlanApply(t, tc.Resource.Schema, defRtype, "example", tc.Config2)

	resMap := map[string]*schema.Resource{defRtype: tc.Resource}
	bridgedProvider := pulcheck.BridgedProvider(t, defProviderShortName, resMap)

	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
		objectType:          nil,
	}
	yamlProgram := pd.generateYAML(t, bridgedProvider.P.ResourcesMap(), tc.Config1)
	pt := pulcheck.PulCheck(t, bridgedProvider, string(yamlProgram))

	pt.Up()

	yamlProgram = pd.generateYAML(t, bridgedProvider.P.ResourcesMap(), tc.Config2)
	p := filepath.Join(pt.CurrentStack().Workspace().WorkDir(), "Pulumi.yaml")
	err := os.WriteFile(p, yamlProgram, 0o600)
	require.NoErrorf(t, err, "writing Pulumi.yaml")
	x := pt.Up()

	tfAction := tfd.parseChangesFromTFPlan(*tfDiffPlan)

	tc.verifyBasicDiffAgreement(t, tfAction, x.Summary)
}

func (tc *diffTestCase) verifyBasicDiffAgreement(t T, tfAction string, us auto.UpdateSummary) {
	t.Logf("UpdateSummary.ResourceChanges: %#v", us.ResourceChanges)
	switch tfAction {
	case "update":
		require.NotNilf(t, us.ResourceChanges, "UpdateSummary.ResourceChanges should not be nil")
		rc := *us.ResourceChanges
		assert.Equalf(t, 1, rc[string(apitype.OpSame)], "expected one resource to stay the same - the stack")
		assert.Equalf(t, 1, rc[string(apitype.Update)], "expected the test resource to get an update plan")
		assert.Equalf(t, 2, len(rc), "expected two entries in UpdateSummary.ResourceChanges")
	case "no-op":
		require.NotNilf(t, us.ResourceChanges, "UpdateSummary.ResourceChanges should not be nil")
		rc := *us.ResourceChanges
		assert.Equalf(t, 2, rc[string(apitype.OpSame)], "expected the test resource and stack to stay the same")
		assert.Equalf(t, 1, len(rc), "expected one entry in UpdateSummary.ResourceChanges")
	default:
		panic("TODO: do not understand this TF action yet: " + tfAction)
	}
}
