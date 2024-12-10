package tfbridgetests

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// For the following combinations:
//
// provider-sdk: pf
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
func TestAccProviderConfigureSecretsPluginFramework(t *testing.T) {
	type configSetter func(ctx context.Context, t *testing.T, stack *auto.Stack, basePath string, secret bool)

	type primType struct {
		name                string
		capitalizedName     string
		yamlLiteral         string
		valueInTF           tftypes.Value
		configSetter        configSetter
		attrSchema          pschema.Attribute
		sensitiveAttrSchema pschema.Attribute
	}

	setConfigValue := func(cv auto.ConfigValue) configSetter {
		return func(ctx context.Context, t *testing.T, stack *auto.Stack, basePath string, secret bool) {
			err := stack.SetConfigWithOptions(ctx, basePath, auto.ConfigValue{
				Value:  cv.Value,
				Secret: secret,
			}, &auto.ConfigOptions{Path: true})
			require.NoError(t, err)
		}
	}

	primTypes := []primType{
		{
			name:            "string",
			capitalizedName: "String",
			yamlLiteral:     `"SECRET"`,
			valueInTF:       tftypes.NewValue(tftypes.String, "SECRET"),
			attrSchema: pschema.StringAttribute{
				Optional: true,
			},
			sensitiveAttrSchema: pschema.StringAttribute{
				Optional:  true,
				Sensitive: true,
			},
			configSetter: setConfigValue(auto.ConfigValue{
				Value: "SECRET",
			}),
		},
		{
			name:            "int",
			capitalizedName: "Int",
			yamlLiteral:     `42`,
			valueInTF:       tftypes.NewValue(tftypes.Number, 42),
			attrSchema: pschema.Int64Attribute{
				Optional: true,
			},
			sensitiveAttrSchema: pschema.Int64Attribute{
				Optional:  true,
				Sensitive: true,
			},
			configSetter: setConfigValue(auto.ConfigValue{
				Value: "42",
			}),
		},
		{
			name:            "bool",
			capitalizedName: "Bool",
			yamlLiteral:     `false`,
			valueInTF:       tftypes.NewValue(tftypes.Bool, false),
			attrSchema: pschema.BoolAttribute{
				Optional: true,
			},
			sensitiveAttrSchema: pschema.BoolAttribute{
				Optional:  true,
				Sensitive: true,
			},
			configSetter: setConfigValue(auto.ConfigValue{
				Value: "false",
			}),
		},
		{
			name:            "strlist",
			capitalizedName: "Strlist",
			yamlLiteral:     `["A","B"]`,
			valueInTF: tftypes.NewValue(
				tftypes.List{ElementType: tftypes.String},
				[]tftypes.Value{
					tftypes.NewValue(tftypes.String, "A"),
					tftypes.NewValue(tftypes.String, "B"),
				},
			),
			attrSchema: pschema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			sensitiveAttrSchema: pschema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Sensitive:   true,
			},
			configSetter: func(
				ctx context.Context,
				t *testing.T,
				stack *auto.Stack,
				basePath string,
				secret bool,
			) {
				o := &auto.ConfigOptions{Path: true}
				err := stack.SetConfigWithOptions(ctx, fmt.Sprintf("%s[0]", basePath),
					auto.ConfigValue{Value: "A", Secret: secret}, o)
				require.NoError(t, err)
				err = stack.SetConfigWithOptions(ctx, fmt.Sprintf("%s[1]", basePath),
					auto.ConfigValue{Value: "B", Secret: secret}, o)
				require.NoError(t, err)
			},
		},
	}

	checkConfigValue := func(t *testing.T, c *tfsdk.Config, n string, valueInTF tftypes.Value) {
		var cmap map[string]tftypes.Value
		err := c.Raw.As(&cmap)
		require.NoError(t, err)
		assert.Truef(t, cmap[n].Equal(valueInTF), "%s != %s", cmap[n], valueInTF)
	}

	checkNestedConfigValue := func(
		t *testing.T,
		c *tfsdk.Config,
		path *tftypes.AttributePath,
		valueInTF tftypes.Value,
	) {
		innerValue, remainder, err := tftypes.WalkAttributePath(c.Raw, path)
		require.NoError(t, err)
		require.Truef(t, len(remainder.Steps()) == 0, "innerValue not found at path %s", path)
		v, ok := innerValue.(tftypes.Value)
		require.Truef(t, ok, "innerValue not found")
		assert.Truef(t, v.Equal(valueInTF), "%s != %s", v, valueInTF)
	}

	type testCase struct {
		name               string
		program            string
		configure          func(t *testing.T, ctx context.Context, stack *auto.Stack)
		checkConfigureCall func(t *testing.T, ctx context.Context, config *tfsdk.Config)
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
			checkConfigureCall: func(t *testing.T, ctx context.Context, c *tfsdk.Config) {
				n := fmt.Sprintf("basic_%s_config", ty.name)
				checkConfigValue(t, c, n, ty.valueInTF)
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
			checkConfigureCall: func(t *testing.T, ctx context.Context, c *tfsdk.Config) {
				n := fmt.Sprintf("secret_%s_config", ty.name)
				checkConfigValue(t, c, n, ty.valueInTF)
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
				p := fmt.Sprintf("prov:basic%sConfig", ty.capitalizedName)
				ty.configSetter(ctx, t, stack, p, true /*secret*/)
			},
			checkConfigureCall: func(t *testing.T, ctx context.Context, c *tfsdk.Config) {
				n := fmt.Sprintf("basic_%s_config", ty.name)
				checkConfigValue(t, c, n, ty.valueInTF)
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
				p := fmt.Sprintf("prov:secret%sConfig", ty.capitalizedName)
				ty.configSetter(ctx, t, stack, p, false /*secret*/)
			},
			checkConfigureCall: func(t *testing.T, ctx context.Context, c *tfsdk.Config) {
				n := fmt.Sprintf("secret_%s_config", ty.name)
				checkConfigValue(t, c, n, ty.valueInTF)
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
			checkConfigureCall: func(t *testing.T, ctx context.Context, c *tfsdk.Config) {
				n := fmt.Sprintf("nested_%s_config", ty.name)
				p := tftypes.NewAttributePath().WithAttributeName("obj").WithAttributeName(n)
				checkNestedConfigValue(t, c, p, ty.valueInTF)
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
			checkConfigureCall: func(t *testing.T, ctx context.Context, c *tfsdk.Config) {
				n := fmt.Sprintf("nested_secret_%s_config", ty.name)
				p := tftypes.NewAttributePath().WithAttributeName("obj").WithAttributeName(n)
				checkNestedConfigValue(t, c, p, ty.valueInTF)
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
				p := fmt.Sprintf("prov:obj.nested%sConfig", ty.capitalizedName)
				ty.configSetter(ctx, t, stack, p, true /*secret*/)
			},
			checkConfigureCall: func(t *testing.T, ctx context.Context, c *tfsdk.Config) {
				n := fmt.Sprintf("nested_%s_config", ty.name)
				p := tftypes.NewAttributePath().WithAttributeName("obj").WithAttributeName(n)
				checkNestedConfigValue(t, c, p, ty.valueInTF)
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
				p := fmt.Sprintf("prov:obj.nestedSecret%sConfig", ty.capitalizedName)
				ty.configSetter(ctx, t, stack, p, false /*secret*/)
			},
			checkConfigureCall: func(t *testing.T, ctx context.Context, c *tfsdk.Config) {
				n := fmt.Sprintf("nested_secret_%s_config", ty.name)
				p := tftypes.NewAttributePath().WithAttributeName("obj").WithAttributeName(n)
				checkNestedConfigValue(t, c, p, ty.valueInTF)
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

	nestedObjS := pschema.SingleNestedAttribute{
		Optional:   true,
		Attributes: map[string]pschema.Attribute{},
	}

	for _, ty := range primTypes {
		nestedObjS.Attributes[fmt.Sprintf("nested_%s_config", ty.name)] = ty.attrSchema
		nestedObjS.Attributes[fmt.Sprintf("nested_secret_%s_config", ty.name)] = ty.sensitiveAttrSchema
	}

	configSchema := pschema.Schema{
		Attributes: map[string]pschema.Attribute{
			"obj": nestedObjS,
		},
	}

	for _, ty := range primTypes {
		configSchema.Attributes[fmt.Sprintf("basic_%s_config", ty.name)] = ty.attrSchema
		configSchema.Attributes[fmt.Sprintf("secret_%s_config", ty.name)] = ty.sensitiveAttrSchema
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			res := providerbuilder.NewResource(providerbuilder.NewResourceArgs{
				ResourceSchema: rschema.Schema{
					Attributes: map[string]rschema.Attribute{
						"string_prop": rschema.StringAttribute{Optional: true},
					},
				},
			})
			tfp := providerbuilder.NewProvider(providerbuilder.NewProviderArgs{
				TypeName: "prov",
				ConfigureFunc: func(
					ctx context.Context,
					req provider.ConfigureRequest,
					resp *provider.ConfigureResponse,
				) {
					tc.checkConfigureCall(t, ctx, &req.Config)
				},
				ProviderSchema: configSchema,
				AllResources: []providerbuilder.Resource{
					res,
				},
			})

			bp := bridgedProvider(tfp)
			bp.Config = map[string]*info.Schema{
				"basic_strlist_config": {
					// Prevent basicStrlistConfigs pluralization
					Name: "basicStrlistConfig",
				},
				"secret_strlist_config": {
					// Prevent secretStrlistConfigs pluralization
					Name: "secretStrlistConfig",
				},
				"obj": {
					Elem: &info.Schema{
						Fields: map[string]*info.Schema{
							"nested_strlist_config": {
								// Prevent nestedStrlistConfigs pluralization
								Name: "nestedStrlistConfig",
							},
							"nested_secret_strlist_config": {
								// Prevent nestedSecretStrlistConfigs pluralization
								Name: "nestedSecretStrlistConfig",
							},
						},
					},
				},
			}

			pt, err := pulcheck.PulCheck(t, bp, tc.program)
			require.NoError(t, err)
			if tc.configure != nil {
				tc.configure(t, ctx, pt.CurrentStack())
			}
			out := pt.Up(t)
			t.Logf("%s\n%s", out.StdOut, out.StdErr)
			state := pt.ExportStack(t)
			var d apitype.DeploymentV3
			err = json.Unmarshal(state.Deployment, &d)
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
