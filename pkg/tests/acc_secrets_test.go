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
	"fmt"
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
// Check that first-class secrets work as expected: when a program configures the provider with a secret input Pulumi up
// succeeds and TF code receives the expected un-secreted value Secret material does not leak to state.
//
// Check that schema-based secrets work as expected: when a provider property is sensitive according to SchemaInfo or
// underlying TF schema, the user configures the provider with a plain value. The plain value does not leak to state but
// is secreted in the state.
//
// This tests exercise the bridge and Pulumi CLI together intentionally as secret handling for nested properties
// historically had some quirks in the Pulumi CLI.
func TestAccProviderConfigureSecrets(t *testing.T) {
	type primType struct {
		name            string
		capitalizedName string
		yamlLiteral     string
		valueInTF       any
		configValue     auto.ConfigValue
		schema          schema.Schema
	}

	secretConfigValue := func(cv auto.ConfigValue) auto.ConfigValue {
		cv.Secret = true
		return cv
	}

	primTypes := []primType{
		{
			name:            "string",
			capitalizedName: "String",
			yamlLiteral:     `"SECRET"`,
			valueInTF:       "SECRET",
			schema: schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			configValue: auto.ConfigValue{
				Value: "SECRET",
			},
		},
		{
			name:            "int",
			capitalizedName: "Int",
			yamlLiteral:     `42`,
			valueInTF:       int(42),
			schema: schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
			},
			configValue: auto.ConfigValue{
				Value: "42",
			},
		},
		{
			name:            "bool",
			capitalizedName: "Bool",
			yamlLiteral:     `false`,
			valueInTF:       false,
			schema: schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			configValue: auto.ConfigValue{
				Value: "false",
			},
		},
	}

	type testCase struct {
		name               string
		program            string
		configure          func(t *testing.T, ctx context.Context, stack *auto.Stack)
		checkConfigureCall func(t *testing.T, rd *schema.ResourceData)
		checkState         func(t *testing.T, d *apitype.DeploymentV3)
	}

	testCases := []testCase{}

	for _, ty := range primTypes {
		testCases = append(testCases, testCase{
			name: fmt.Sprintf("explicit-provider/first-class-secret/%s", ty.name),
			program: fmt.Sprintf(`
	                name: test
	                runtime: yaml
	                resources:
	                    prov:
	                        type: pulumi:providers:prov
	                        properties:
	                            basic%sConfig:
	                                fn::secret: %s
	                    mainRes:
	                        type: prov:index:Test
	                        properties:
	                            stringProp: "foo"
	                        options:
	                            provider: ${prov}
			`, ty.capitalizedName, ty.yamlLiteral),
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, ty.valueInTF, rd.Get(fmt.Sprintf("basic_%s_config", ty.name)))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				n := fmt.Sprintf("basic%sConfig", ty.capitalizedName)
				p := requireExplicitProvider(t, d)
				requireSecret(t, p.Inputs[n], fmt.Sprintf(`p.Inputs["%s"]`, n))
				requireSecret(t, p.Outputs[n], fmt.Sprintf(`p.Outputs["%s"]`, n))
			},
		})

		testCases = append(testCases, testCase{
			name: fmt.Sprintf("explicit-provider/schema-secret/%s", ty.name),
			program: fmt.Sprintf(`
	                name: test
	                runtime: yaml
	                resources:
	                    prov:
	                        type: pulumi:providers:prov
	                        properties:
	                            secret%sConfig: %s
	                    mainRes:
	                        type: prov:index:Test
	                        properties:
	                            stringProp: "foo"
	                        options:
	                            provider: ${prov}
			`, ty.capitalizedName, ty.yamlLiteral),
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, ty.valueInTF, rd.Get(fmt.Sprintf("secret_%s_config", ty.name)))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireExplicitProvider(t, d)
				n := fmt.Sprintf("secret%sConfig", ty.capitalizedName)
				requireSecret(t, p.Inputs[n], fmt.Sprintf(`p.Inputs[%s]`, n))
				requireSecret(t, p.Outputs[n], fmt.Sprintf(`p.Outputs[%s]`, n))
			},
		})

		testCases = append(testCases, testCase{
			name: fmt.Sprintf("default-provider/first-class-secret/%s", ty.name),
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
				err := stack.SetConfig(ctx,
					fmt.Sprintf("prov:basic%sConfig", ty.capitalizedName),
					secretConfigValue(ty.configValue))
				assert.NoError(t, err)
			},
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, ty.valueInTF, rd.Get(fmt.Sprintf("basic_%s_config", ty.name)))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				n := fmt.Sprintf("basic%sConfig", ty.capitalizedName)
				p := requireDefaultProvider(t, d)
				requireSecret(t, p.Inputs[n], fmt.Sprintf(`p.Inputs["%s"]`, n))
				requireSecret(t, p.Outputs[n], fmt.Sprintf(`p.Outputs["%s"]`, n))
			},
		})

		testCases = append(testCases, testCase{
			name: fmt.Sprintf("default-provider/schema-secret/%s", ty.name),
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
				err := stack.SetConfig(ctx, fmt.Sprintf("prov:secret%sConfig", ty.capitalizedName),
					ty.configValue)
				assert.NoError(t, err)
			},
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, ty.valueInTF, rd.Get(fmt.Sprintf("secret_%s_config", ty.name)))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireDefaultProvider(t, d)
				n := fmt.Sprintf("secret%sConfig", ty.capitalizedName)
				requireSecret(t, p.Inputs[n], fmt.Sprintf(`p.Inputs["%s"]`, n))
				requireSecret(t, p.Outputs[n], fmt.Sprintf(`p.Outputs["%s"]`, n))
			},
		})

		testCases = append(testCases, testCase{
			name: fmt.Sprintf("explicit-provider/first-class-secret/nested-%s", ty.name),
			program: fmt.Sprintf(`
	                name: test
	                runtime: yaml
	                resources:
	                    prov:
	                        type: pulumi:providers:prov
	                        properties:
	                            obj:
	                                nested%sConfig:
	                                    fn::secret:
	                                        %s
	                    mainRes:
	                        type: prov:index:Test
	                        properties:
	                            stringProp: "foo"
	                        options:
	                            provider: ${prov}
			`, ty.capitalizedName, ty.yamlLiteral),
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, ty.valueInTF, rd.Get(fmt.Sprintf("obj.0.nested_%s_config", ty.name)))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireExplicitProvider(t, d)
				// Current versions of the Pulumi CLI make the entire `obj` secret but it should be OK
				// to accept only the nested*Config to be secreted as well.
				requireSecret(t, p.Inputs["obj"], `p.Inputs["obj"]`)
				requireSecret(t, p.Outputs["obj"], `p.Outputs["obj"]`)
			},
		})

		testCases = append(testCases, testCase{
			name: fmt.Sprintf("explicit-provider/schema-secret/nested-%s", ty.name),
			program: fmt.Sprintf(`
	                name: test
	                runtime: yaml
	                resources:
	                    prov:
	                        type: pulumi:providers:prov
	                        properties:
	                            obj:
	                                nestedSecret%sConfig:
	                                    %s
	                    mainRes:
	                        type: prov:index:Test
	                        properties:
	                            stringProp: "foo"
	                        options:
	                            provider: ${prov}
			`, ty.capitalizedName, ty.yamlLiteral),
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, ty.valueInTF,
					rd.Get(fmt.Sprintf("obj.0.nested_secret_%s_config", ty.name)))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireExplicitProvider(t, d)
				// Current versions of the Pulumi CLI make the entire `obj` secret but it should be OK
				// to accept only the nested*Config to be secreted as well.
				requireSecret(t, p.Inputs["obj"], `p.Inputs["obj"]`)
				requireSecret(t, p.Outputs["obj"], `p.Outputs["obj"]`)
			},
		})

		testCases = append(testCases, testCase{
			name: fmt.Sprintf("default-provider/first-class-secret/nested-%s", ty.name),
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
				err := stack.SetConfigWithOptions(ctx,
					fmt.Sprintf("prov:obj.nested%sConfig", ty.capitalizedName),
					secretConfigValue(ty.configValue),
					&auto.ConfigOptions{
						Path: true,
					})
				require.NoError(t, err)
			},
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, ty.valueInTF, rd.Get(fmt.Sprintf("obj.0.nested_%s_config", ty.name)))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireDefaultProvider(t, d)
				// Current versions of the Pulumi CLI make the entire `obj` secret but it should be OK
				// to accept only the nested*Config to be secreted as well.
				requireSecret(t, p.Inputs["obj"], `p.Inputs["obj"]`)
				requireSecret(t, p.Outputs["obj"], `p.Outputs["obj"]`)
			},
		})

		testCases = append(testCases, testCase{
			name: fmt.Sprintf("default-provider/schema-secret/nested-%s", ty.name),
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
				err := stack.SetConfigWithOptions(ctx,
					fmt.Sprintf("prov:obj.nestedSecret%sConfig", ty.capitalizedName),
					ty.configValue,
					&auto.ConfigOptions{Path: true})
				require.NoError(t, err)
			},
			checkConfigureCall: func(t *testing.T, rd *schema.ResourceData) {
				assert.Equal(t, ty.valueInTF,
					rd.Get(fmt.Sprintf("obj.0.nested_secret_%s_config", ty.name)))
			},
			checkState: func(t *testing.T, d *apitype.DeploymentV3) {
				p := requireDefaultProvider(t, d)
				// Current versions of the Pulumi CLI make the entire `obj` secret but it should be OK
				// to accept only the nested*Config to be secreted as well.
				requireSecret(t, p.Inputs["obj"], `p.Inputs["obj"]`)
				requireSecret(t, p.Outputs["obj"], `p.Outputs["obj"]`)
			},
		})
	}

	nestedObjConfigSchema := &schema.Resource{
		Schema: map[string]*schema.Schema{},
	}

	for _, ty := range primTypes {
		nestedObjConfigSchema.Schema[fmt.Sprintf("nested_%s_config", ty.name)] = &ty.schema
		secretSchema := ty.schema
		secretSchema.Sensitive = true
		nestedObjConfigSchema.Schema[fmt.Sprintf("nested_secret_%s_config", ty.name)] = &secretSchema
	}

	configSchema := map[string]*schema.Schema{
		"obj": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem:     nestedObjConfigSchema,
		},
	}

	for _, ty := range primTypes {
		configSchema[fmt.Sprintf("basic_%s_config", ty.name)] = &ty.schema
		secretSchema := ty.schema
		secretSchema.Sensitive = true
		configSchema[fmt.Sprintf("secret_%s_config", ty.name)] = &secretSchema
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
