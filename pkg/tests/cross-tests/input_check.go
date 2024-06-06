// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
package crosstests

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
)

// Adapted from diff_check.go
type inputTestCase struct {
	// Schema for the resource under test
	Resource *schema.Resource

	Config     any
	ObjectType *tftypes.Object

	SkipCompareRawPlan   bool
	SkipCompareRawConfig bool
	SkipCompareRawState  bool
}

// Adapted from diff_check.go
func runCreateInputCheck(t T, tc inputTestCase) {
	//nolint:staticcheck
	if tc.Resource.CreateContext != nil || tc.Resource.Create != nil {
		t.Errorf("Create methods should not be set for these tests!")
	}

	var tfResData, pulResData *schema.ResourceData
	tc.Resource.CreateContext = func(_ context.Context, rd *schema.ResourceData, meta interface{}) diag.Diagnostics {
		if tfResData == nil {
			tfResData = rd
		} else {
			pulResData = rd
		}

		rd.SetId("someid") // CreateContext must pick an ID
		return make(diag.Diagnostics, 0)
	}

	tfwd := t.TempDir()

	tfd := newTfDriver(t, tfwd, defProviderShortName, defRtype, tc.Resource)
	tfd.writePlanApply(t, tc.Resource.Schema, defRtype, "example", tc.Config)

	resMap := map[string]*schema.Resource{defRtype: tc.Resource}
	bridgedProvider := pulcheck.BridgedProvider(t, defProviderShortName, resMap)
	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
		objectType:          tc.ObjectType,
	}
	yamlProgram := pd.generateYAML(t, bridgedProvider.P.ResourcesMap(), tc.Config)

	pt := pulcheck.PulCheck(t, bridgedProvider, string(yamlProgram))

	pt.Up()

	for k := range tc.Resource.Schema {
		// TODO: make this recursive
		tfVal := tfResData.Get(k)
		pulVal := pulResData.Get(k)

		tfChangeValOld, tfChangeValNew := tfResData.GetChange(k)
		pulChangeValOld, pulChangeValNew := pulResData.GetChange(k)

		assertValEqual(t, k, tfVal, pulVal)
		assertValEqual(t, k+" Change Old", tfChangeValOld, pulChangeValOld)
		assertValEqual(t, k+" Change New", tfChangeValNew, pulChangeValNew)
	}

	if !tc.SkipCompareRawConfig {
		assertCtyValEqual(t, "RawConfig", tfResData.GetRawConfig(), pulResData.GetRawConfig())
	}

	if !tc.SkipCompareRawPlan {
		assertCtyValEqual(t, "RawPlan", tfResData.GetRawPlan(), pulResData.GetRawPlan())
	}

	if !tc.SkipCompareRawState {
		assertCtyValEqual(t, "RawState", tfResData.GetRawState(), pulResData.GetRawState())
	}
}
