// Copyright 2016-2023, Pulumi Corporation.
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

package tests

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccProviderSecrets(t *testing.T) {
	opts := accTestOptions(t).With(integration.ProgramTestOptions{
		Dir: "provider-secrets",

		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			bytes, err := json.MarshalIndent(stack.Deployment, "", "  ")
			assert.NoError(t, err)
			assert.NotContainsf(t, string(bytes), "SECRET",
				"Secret data leaked into the state")
		},
	})
	integration.ProgramTest(t, &opts)
}

// For the following combinations:
//
// provider-sdk: sdkv2
// property-type: string, number, boolean, list
// property-nesting: top-level vs nested
// provider-type: explicit, default
//
// Check that first-class secrets work as expected at integration level (via Pulumi CLI).
//
// When a program configures the provider with a secret input
// Pulumi up succeeds and TF code receives the expected un-secreted value
// Secret material does not leak to state
//
// Check that schema-based secrets work as expected:
//
// When a provider property is sensitive according to SchemaInfo or underlying TF schema User configures the provider
// with a plain value The plain value does not leak to state but is secreted instate.
func TestAccProviderConfigureSecrets(t *testing.T) {
	type testCase struct {
		name               string
		program            string
		configure          func(t *testing.T, ctx context.Context, stack *auto.Stack)
		checkConfigureCall func(t *testing.T, rd *schema.ResourceData)
		checkState         func(t *testing.T, d *apitype.DeploymentV3)
	}

	testCases := []testCase{
		{
			name: "explicit-provider/first-class-secret/string",
			program: `
                        name: test
                        runtime: yaml
                        resources:
                            prov:
                                type: pulumi:providers:prov
                                properties:
                                    stringConfig:
                                        fn::secret: "SECRET"
                            mainRes:
                                type: prov:index:Test
                                properties:
                                    stringProp: "foo"
                                options:
                                    provider: ${prov}
			`,
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, "SECRET", rd.Get("string_config"))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireExplicitProvider(t, d)
				requireSecret(t, p.Inputs["stringConfig"], `p.Inputs["stringConfig"]`)
				requireSecret(t, p.Outputs["stringConfig"], `p.Outputs["stringConfig"]`)
			},
		},
		{
			name: "explicit-provider/schema-secret/string",
			program: `
                        name: test
                        runtime: yaml
                        resources:
                            prov:
                                type: pulumi:providers:prov
                                properties:
                                    secretStringConfig: "SECRET"
                            mainRes:
                                type: prov:index:Test
                                properties:
                                    stringProp: "foo"
                                options:
                                    provider: ${prov}
			`,
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, "SECRET", rd.Get("secret_string_config"))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireExplicitProvider(t, d)
				requireSecret(t, p.Inputs["secretStringConfig"], `p.Inputs["secretStringConfig"]`)
				requireSecret(t, p.Outputs["secretStringConfig"], `p.Outputs["secretStringConfig"]`)
			},
		},
		{
			name: "default-provider/first-class-secret/string",
			program: `
                        name: test
                        runtime: yaml
                        resources:
                            mainRes:
                                type: prov:index:Test
                                properties:
                                    stringProp: "foo"
			`,
			configure: func(t *testing.T, ctx context.Context, stack *auto.Stack) {
				err := stack.SetConfig(ctx, "prov:stringConfig", auto.ConfigValue{
					Value:  "SECRET",
					Secret: true,
				})
				assert.NoError(t, err)
			},
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, "SECRET", rd.Get("string_config"))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireDefaultProvider(t, d)
				requireSecret(t, p.Inputs["stringConfig"], `p.Inputs["stringConfig"]`)
				requireSecret(t, p.Outputs["stringConfig"], `p.Outputs["stringConfig"]`)
			},
		},
		{
			name: "default-provider/schema-secret/string",
			program: `
                        name: test
                        runtime: yaml
                        resources:
                            mainRes:
                                type: prov:index:Test
                                properties:
                                    stringProp: "foo"
			`,
			configure: func(t *testing.T, ctx context.Context, stack *auto.Stack) {
				err := stack.SetConfig(ctx, "prov:secretStringConfig", auto.ConfigValue{
					Value: "SECRET",
				})
				assert.NoError(t, err)
			},
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, "SECRET", rd.Get("secret_string_config"))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireDefaultProvider(t, d)
				requireSecret(t, p.Inputs["secretStringConfig"], `p.Inputs["secretStringConfig"]`)
				requireSecret(t, p.Outputs["secretStringConfig"], `p.Outputs["secretStringConfig"]`)
			},
		},
		{
			name: "explicit-provider/first-class-secret/nested-string",
			program: `
                        name: test
                        runtime: yaml
                        resources:
                            prov:
                                type: pulumi:providers:prov
                                properties:
                                    obj:
                                        nestedStringConfig:
                                            fn::secret:
                                                "SECRET"
                            mainRes:
                                type: prov:index:Test
                                properties:
                                    stringProp: "foo"
                                options:
                                    provider: ${prov}
			`,
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, "SECRET", rd.Get("obj.0.nested_string_config"))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireExplicitProvider(t, d)
				// Current versions of the Pulumi CLI make the entire `obj` secret but it should be OK
				// to accept only the nestedStringConfig to be secreted as well.
				requireSecret(t, p.Inputs["obj"], `p.Inputs["obj"]`)
				requireSecret(t, p.Outputs["obj"], `p.Outputs["obj"]`)
			},
		},
		{
			name: "explicit-provider/schema-secret/nested-string",
			program: `
                        name: test
                        runtime: yaml
                        resources:
                            prov:
                                type: pulumi:providers:prov
                                properties:
                                    obj:
                                        nestedSecretStringConfig:
                                            "SECRET"
                            mainRes:
                                type: prov:index:Test
                                properties:
                                    stringProp: "foo"
                                options:
                                    provider: ${prov}
			`,
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, "SECRET", rd.Get("obj.0.nested_secret_string_config"))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireExplicitProvider(t, d)
				// Current versions of the Pulumi CLI make the entire `obj` secret but it should be OK
				// to accept only the nestedStringConfig to be secreted as well.
				requireSecret(t, p.Inputs["obj"], `p.Inputs["obj"]`)
				requireSecret(t, p.Outputs["obj"], `p.Outputs["obj"]`)
			},
		},
		{
			name: "default-provider/first-class-secret/nested-string",
			program: `
		        name: test
		        runtime: yaml
		        resources:
		            mainRes:
		                type: prov:index:Test
		                properties:
		                    stringProp: "foo"
			`,
			configure: func(t *testing.T, ctx context.Context, stack *auto.Stack) {
				err := stack.SetConfigWithOptions(ctx, "prov:obj.nestedStringConfig", auto.ConfigValue{
					Value:  "SECRET",
					Secret: true,
				}, &auto.ConfigOptions{
					Path: true,
				})
				require.NoError(t, err)
			},
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, "SECRET", rd.Get("obj.0.nested_string_config"))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireDefaultProvider(t, d)
				// Current versions of the Pulumi CLI make the entire `obj` secret but it should be OK
				// to accept only the nestedStringConfig to be secreted as well.
				requireSecret(t, p.Inputs["obj"], `p.Inputs["obj"]`)
				requireSecret(t, p.Outputs["obj"], `p.Outputs["obj"]`)
			},
		},
		{
			name: "default-provider/schema-secret/nested-string",
			program: `
		        name: test
		        runtime: yaml
		        resources:
		            mainRes:
		                type: prov:index:Test
		                properties:
		                    stringProp: "foo"
			`,
			configure: func(t *testing.T, ctx context.Context, stack *auto.Stack) {
				err := stack.SetConfigWithOptions(ctx, "prov:obj.nestedSecretStringConfig",
					auto.ConfigValue{
						Value:  "SECRET",
						Secret: true,
					},
					&auto.ConfigOptions{Path: true})
				require.NoError(t, err)
			},
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, "SECRET", rd.Get("obj.0.nested_secret_string_config"))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireDefaultProvider(t, d)
				// Current versions of the Pulumi CLI make the entire `obj` secret but it should be OK
				// to accept only the nestedStringConfig to be secreted as well.
				requireSecret(t, p.Inputs["obj"], `p.Inputs["obj"]`)
				requireSecret(t, p.Outputs["obj"], `p.Outputs["obj"]`)
			},
		},
	}

	configSchema := map[string]*schema.Schema{
		"string_config": {
			Type:     schema.TypeString,
			Optional: true,
		},
		"secret_string_config": {
			Type:      schema.TypeString,
			Optional:  true,
			Sensitive: true,
		},
		"obj": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"nested_string_config": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"nested_secret_string_config": {
						Type:      schema.TypeString,
						Optional:  true,
						Sensitive: true,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			res := &schema.Resource{
				Schema: map[string]*schema.Schema{
					"string_prop": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			}
			tfp := &schema.Provider{
				ConfigureContextFunc: func(
					ctx context.Context,
					rd *schema.ResourceData,
				) (interface{}, diag.Diagnostics) {
					tc.checkConfigureCall(t, rd)
					return &struct{}{}, diag.Diagnostics{}
				},
				Schema:       configSchema,
				ResourcesMap: map[string]*schema.Resource{"prov_test": res},
			}
			bridgedProvider := pulcheck.BridgedProvider(t, "prov", tfp)
			pt := pulcheck.PulCheck(t, bridgedProvider, tc.program)
			if tc.configure != nil {
				tc.configure(t, ctx, pt.CurrentStack())
			}
			out := pt.Up(t)
			t.Logf("%s\n%s", out.StdOut, out.StdErr)
			state := pt.ExportStack(t)
			var d apitype.DeploymentV3
			err := json.Unmarshal(state.Deployment, &d)
			require.NoError(t, err)
			tc.checkState(t, &d)
		})
	}
}

// Requires an explicit provider record in the state and returns it.
func requireExplicitProvider(t *testing.T, d *apitype.DeploymentV3) apitype.ResourceV3 {
	t.Helper()
	for _, r := range d.Resources {
		if r.URN == "urn:pulumi:test::test::pulumi:providers:prov::prov" {
			return r
		}
	}
	require.Fail(t, "Expected to find an explicit provider record")
	return apitype.ResourceV3{}
}

// Requires a default provider record in the state and returns it.
func requireDefaultProvider(t *testing.T, d *apitype.DeploymentV3) apitype.ResourceV3 {
	t.Helper()
	for _, r := range d.Resources {
		if r.URN == "urn:pulumi:test::test::pulumi:providers:prov::default" {
			return r
		}
	}
	require.Fail(t, "Expected to find an explicit provider record")
	return apitype.ResourceV3{}
}

// Ensures that isSecret(value) assertion holds.
func requireSecret(t *testing.T, value any, expr string) {
	t.Helper()
	require.Truef(t, isSecret(value), "Expected %s to be a secret, got %#v", expr, value)
}

// Detects the secret-marked values in Pulumi state files.
func isSecret(value any) bool {
	switch value := value.(type) {
	case map[string]any:
		v, ok := value["4dabf18193072939515e22adb298388d"]
		if !ok {
			return false
		}
		vs, ok := v.(string)
		if !ok {
			return false
		}
		return vs == "1b47061264138c4ac30d75fd1eb44270"
	default:
		return false
	}
}
