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
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// MakeCreate is a helper function for calling [Create] in [testing.T.Run] subcases.
func MakeCreate(
	resource map[string]*schema.Schema, tfConfig cty.Value,
	options ...CreateOption,
) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		Create(t, resource, tfConfig, options...)
	}
}

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
	t T, resourceSchema map[string]*schema.Schema, tfConfig cty.Value,
	options ...CreateOption,
) {
	var opts createOpts
	for _, f := range options {
		f(&opts)
	}

	var puConfig resource.PropertyMap
	if opts.puConfig != nil {
		puConfig = *opts.puConfig
	} else {
		puConfig = InferPulumiValue(t,
			shimv2.NewSchemaMap(resourceSchema),
			opts.resourceInfo.GetFields(),
			tfConfig,
		)
	}

	type result struct {
		data   *schema.ResourceData
		meta   any
		wasSet bool
	}
	var tfResult, puResult result

	makeResource := func(writeTo *result) *schema.Resource {
		return &schema.Resource{
			Schema:         resourceSchema,
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
	tfd.writePlanApply(t, resourceSchema, defRtype, "example", tfConfig, lifecycleArgs{})

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

	assertResourceDataEqual(t, resourceSchema, tfResult.data, puResult.data)
}

type createOpts struct {
	resourceInfo  *info.Resource
	stateUpgrader createOptsUpgraders
	timeouts      *schema.ResourceTimeout
	puConfig      *resource.PropertyMap
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

// CreatePulumiConfig specifies an explicit config value in Pulumi's value space.
func CreatePulumiConfig(config resource.PropertyMap) CreateOption {
	return func(o *createOpts) { o.puConfig = &config }
}
