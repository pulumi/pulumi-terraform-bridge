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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// Create validates that a Terraform provider witnesses the same input when:
// - invoked directly with HCL on tfConfig
// - bridged and invoked via Pulumi YAML on puConfig
//
// Create only applies to resources defined with github.com/hashicorp/terraform-plugin-sdk/v2. For cross-tests
// on Plugin Framework based resources, see
// github.com/pulumi/pulumi-terraform-bridge/pkg/pf/tests/internal/cross-tests.
//
// Create *does not* verify the outputs of the resource, only that the provider witnessed the same inputs.
func Create(
	t T, resource map[string]*schema.Schema, tfConfig cty.Value, puConfig resource.PropertyMap,
	options ...CreateOption,
) {
	var opts createOpts
	for _, f := range options {
		f(&opts)
	}

	type result struct {
		data   *schema.ResourceData
		meta   any
		wasSet bool
	}
	var tfResult, puResult result

	makeResource := func(writeTo *result) *schema.Resource {
		return &schema.Resource{
			Schema:         resource,
			SchemaVersion:  opts.stateUpgrader.schemaVersion,
			StateUpgraders: opts.stateUpgrader.stateUpgraders,
			Timeouts:       opts.timeouts,
			CreateContext: func(_ context.Context, rd *schema.ResourceData, meta any) diag.Diagnostics {
				*writeTo = result{rd, meta, true}
				rd.SetId("someid") // CreateContext must pick an ID
				return nil
			},
		}
	}

	tfwd := t.TempDir()
	tfd := newTFResDriver(t, tfwd, defProviderShortName, defRtype, makeResource(&tfResult))
	tfd.writePlanApply(t, resource, defRtype, "example", tfConfig, lifecycleArgs{})

	require.True(t, tfResult.wasSet, "terraform test result was not set")

	resMap := map[string]*schema.Resource{defRtype: makeResource(&puResult)}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(
		t, defProviderShortName, tfp,
		pulcheck.WithResourceInfo(map[string]*info.Resource{defRtype: opts.resourceInfo}),
	)
	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
	}
	yamlProgram := pd.generateYAML(t, puConfig)

	pt := pulcheck.PulCheck(t, bridgedProvider, string(yamlProgram))

	pt.Up(t)

	require.True(t, puResult.wasSet, "pulumi test was not set")

	// Compare the result

	assert.Equal(t, tfResult.meta, puResult.meta,
		"assert that both providers were configured with the same provider metadata")

	// We are unable to assert that both providers were configured with the exact same
	// data. Type information doesn't line up in the simple case. This just doesn't work:
	//
	//	assert.Equal(t, tfResult.data, puResult.data)
	//
	// We make due by comparing raw data.
	assertCtyValEqual(t, "RawConfig", tfResult.data.GetRawConfig(), puResult.data.GetRawConfig())
	assertCtyValEqual(t, "RawPlan", tfResult.data.GetRawPlan(), puResult.data.GetRawPlan())
	assertCtyValEqual(t, "RawState", tfResult.data.GetRawState(), puResult.data.GetRawState())

	for k := range resource {
		// TODO: make this recursive
		tfVal := tfResult.data.Get(k)
		pulVal := puResult.data.Get(k)

		tfChangeValOld, tfChangeValNew := tfResult.data.GetChange(k)
		pulChangeValOld, pulChangeValNew := puResult.data.GetChange(k)

		assertValEqual(t, k, tfVal, pulVal)
		assertValEqual(t, k+" Change Old", tfChangeValOld, pulChangeValOld)
		assertValEqual(t, k+" Change New", tfChangeValNew, pulChangeValNew)
	}
}

type createOpts struct {
	resourceInfo  *info.Resource
	stateUpgrader createOptsUpgraders
	timeouts      *schema.ResourceTimeout
}

type createOptsUpgraders struct {
	schemaVersion  int
	stateUpgraders []schema.StateUpgrader
}

// An option that can be used to customize [Create].
type CreateOption func(*createOpts)

// CreateResourceInfo specifies an [info.Resource] to apply to the resource under test.
func CreateResourceInfo(info info.Resource) CreateOption {
	contract.Assertf(info.Tok == "", "cannot set info.Tok, it will not be respected")
	return func(o *createOpts) { o.resourceInfo = &info }
}

// CreateStateUpgrader specifies a schema version and list of state upgrader for [Create].
func CreateStateUpgrader(schemaVersion int, upgraders []schema.StateUpgrader) CreateOption {
	return func(o *createOpts) {
		o.stateUpgrader = createOptsUpgraders{schemaVersion, upgraders}
	}
}

// CreateTimeout specifies a timeout option for [Create].
func CreateTimeout(timeouts *schema.ResourceTimeout) CreateOption {
	return func(o *createOpts) { o.timeouts = timeouts }
}
