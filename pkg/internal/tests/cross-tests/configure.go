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
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func MakeConfigure(
	provider map[string]*schema.Schema, tfConfig cty.Value,
	options ...ConfigureOption,
) func(*testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		Configure(t, provider, tfConfig, options...)
	}
}

// Configure validates that a Terraform provider witnesses the same input when:
// - invoked directly with HCL on tfConfig
// - bridged and invoked via Pulumi YAML on puConfig
//
// Create only applies to resources defined with github.com/hashicorp/terraform-plugin-sdk/v2. For cross-tests
// on Plugin Framework based resources, see
// github.com/pulumi/pulumi-terraform-bridge/pkg/pf/tests/internal/cross-tests.
func Configure(
	t T, provider map[string]*schema.Schema, tfConfig cty.Value,
	options ...ConfigureOption,
) {
	var opts configureOpts
	for _, f := range options {
		f(&opts)
	}

	var puConfig resource.PropertyMap
	if opts.puConfig != nil {
		puConfig = *opts.puConfig
	} else {
		puConfig = crosstestsimpl.InferPulumiValue(t,
			shimv2.NewSchemaMap(provider),
			opts.providerInfo,
			tfConfig,
		)
	}

	tfwd := t.TempDir()
	const configureResult = "some-value"
	type result struct {
		data   *schema.ResourceData
		wasSet bool

		resourceCreated bool
	}
	var tfResult, puResult result
	makeProvider := func(writeTo *result) *schema.Provider {
		return &schema.Provider{
			Schema: provider,
			ConfigureContextFunc: func(_ context.Context, rd *schema.ResourceData) (any, diag.Diagnostics) {
				if rd == nil {
					return nil, diag.Errorf("Attempted to configure the provider with nil %T", rd)
				}
				*writeTo = result{rd, true, false}

				return configureResult, nil
			},
			ResourcesMap: map[string]*schema.Resource{
				defRtype: {
					Schema: map[string]*schema.Schema{},
					CreateContext: func(_ context.Context, data *schema.ResourceData, meta any) diag.Diagnostics {
						data.SetId("id")
						writeTo.resourceCreated = true
						s, ok := meta.(string)
						if !ok {
							return diag.Errorf("meta of unexpected type %T", meta)
						}
						if s != configureResult {
							return diag.Errorf("unexpected configuration value: %s (!= %s)",
								s, configureResult)
						}
						return nil
					},
				},
			},
		}
	}
	tfProvider := makeProvider(&tfResult)
	tfd := tfcheck.NewTfDriver(t, tfwd, defProviderShortName, tfProvider)
	tfd.Write(t, providerHCLProgram(t, defProviderShortName, tfProvider, tfConfig))
	plan, err := tfd.Plan(t)
	require.NoError(t, err)
	require.NoError(t, tfd.Apply(t, plan))

	require.True(t, tfResult.wasSet, "terraform configure result was not set")
	require.True(t, tfResult.resourceCreated, "terraform resource result was not set")

	bridgedProvider := pulcheck.BridgedProvider(
		t, defProviderShortName, makeProvider(&puResult),
		pulcheck.WithConfigInfo(opts.providerInfo),
		pulcheck.WithResourceInfo(map[string]*info.Resource{defRtype: {Tok: defRtoken}}),
	)

	data, err := generateYaml(t, "pulumi:providers:"+defProviderShortName, puConfig)
	require.NoErrorf(t, err, "generateYaml")
	data["resources"].(map[string]any)["res"] = map[string]any{
		"type":       defRtoken,
		"properties": map[string]any{},
		"options": map[string]any{
			"provider": "${example}",
		},
	}
	yamlProgram, err := yaml.Marshal(data)
	require.NoErrorf(t, err, "marshaling Pulumi.yaml")
	t.Logf("\n\n%s", yamlProgram)

	pt := pulcheck.PulCheck(t, bridgedProvider, string(yamlProgram))
	pt.Up(t)

	if !puResult.wasSet {
		log, err := pt.GrpcLog(t).Marshal()
		require.NoError(t, err)
		require.Failf(t, "puResult was not set", "pulumi configure result was not set (.resourceCreated = %t)\n%s",
			puResult.resourceCreated, string(log))
	}
	require.True(t, puResult.resourceCreated, "pulumi resource result was not set")

	assertResourceDataEqual(t, provider, tfResult.data, puResult.data)
}

type configureOpts struct {
	providerInfo map[string]*info.Schema
	puConfig     *resource.PropertyMap
}

// An option that can be used to customize [Configure].
type ConfigureOption func(*configureOpts)

// CreateResourceInfo specifies an [info.Resource] to apply to the resource under test.
func ConfigureProviderInfo(info map[string]*info.Schema) ConfigureOption {
	return func(o *configureOpts) { o.providerInfo = info }
}

// ConfigurePulumiConfig specifies an explicit pulumi value for the configure call.
func ConfigurePulumiConfig(config resource.PropertyMap) ConfigureOption {
	return func(o *configureOpts) { o.puConfig = &config }
}
