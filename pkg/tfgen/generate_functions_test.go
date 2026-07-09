// Copyright 2016-2026, Pulumi Corporation.
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

package tfgen

import (
	"io"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func generateFunctionSchema(t *testing.T, info tfbridge.ProviderInfo) (pschema.PackageSpec, error) {
	t.Helper()
	t.Setenv("PULUMI_SKIP_MISSING_MAPPING_ERROR", "")
	t.Setenv("PULUMI_SKIP_EXTRA_MAPPING_ERROR", "")
	nilSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	r, err := GenerateSchemaWithOptions(GenerateSchemaOptions{
		DiagnosticsSink: nilSink,
		ProviderInfo:    info,
	})
	if err != nil {
		return pschema.PackageSpec{}, err
	}
	return r.PackageSpec, nil
}

func TestGenerateProviderFunction(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	info := tfbridge.ProviderInfo{
		Name: "test",
		P: (&shimschema.Provider{
			Functions: map[string]shim.Function{
				"parse_arn": {
					Parameters: []shim.FunctionParameter{
						{Name: "arn", Type: tftypes.String, Description: "The ARN to parse."},
						{Name: "strict_mode", Type: tftypes.Bool, AllowNullValue: true},
					},
					Return: tftypes.Object{
						AttributeTypes: map[string]tftypes.Type{
							"service":    tftypes.String,
							"account_id": tftypes.String,
						},
					},
					Summary:     "Parses an ARN.",
					Description: "Splits an ARN into its parts.",
				},
			},
		}).Shim(),
		Functions: map[string]*info.Function{
			"parse_arn": {Tok: "test:index/parseArn:parseArn"},
		},
	}

	spec, err := generateFunctionSchema(t, info)
	require.NoError(t, err)

	assert.Equal(t, pschema.FunctionSpec{
		Description: "Parses an ARN.\n\nSplits an ARN into its parts.\n",
		Inputs: &pschema.ObjectTypeSpec{
			Type: "object",
			Properties: map[string]pschema.PropertySpec{
				"arn": {
					TypeSpec:    pschema.TypeSpec{Type: "string"},
					Description: "The ARN to parse.\n",
				},
				"strictMode": {
					TypeSpec: pschema.TypeSpec{Type: "boolean"},
				},
			},
			Required: []string{"arn"},
		},
		MultiArgumentInputs: []string{"arn", "strictMode"},
		ReturnType: &pschema.ReturnTypeSpec{
			ObjectTypeSpec: &pschema.ObjectTypeSpec{
				Type: "object",
				Properties: map[string]pschema.PropertySpec{
					"service":   {TypeSpec: pschema.TypeSpec{Type: "string"}},
					"accountId": {TypeSpec: pschema.TypeSpec{Type: "string"}},
				},
				Required: []string{"accountId", "service"},
			},
		},
	}, spec.Functions["test:index/parseArn:parseArn"])
}

func TestGenerateProviderFunctionVariadic(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	info := tfbridge.ProviderInfo{
		Name: "test",
		P: (&shimschema.Provider{
			Functions: map[string]shim.Function{
				"join": {
					Parameters: []shim.FunctionParameter{
						{Name: "separator", Type: tftypes.String},
					},
					VariadicParameter: &shim.FunctionParameter{
						Name:        "parts",
						Type:        tftypes.String,
						Description: "Strings to join.",
					},
					Return:             tftypes.String,
					DeprecationMessage: "use something else",
				},
			},
		}).Shim(),
		Functions: map[string]*info.Function{
			"join": {Tok: "test:index/join:join"},
		},
	}

	spec, err := generateFunctionSchema(t, info)
	require.NoError(t, err)

	assert.Equal(t, pschema.FunctionSpec{
		DeprecationMessage: "use something else",
		Inputs: &pschema.ObjectTypeSpec{
			Type: "object",
			Properties: map[string]pschema.PropertySpec{
				"separator": {TypeSpec: pschema.TypeSpec{Type: "string"}},
				"parts": {
					TypeSpec: pschema.TypeSpec{
						Type:  "array",
						Items: &pschema.TypeSpec{Type: "string"},
					},
					Description: "Strings to join.\n",
				},
			},
			Required: []string{"separator"},
		},
		MultiArgumentInputs: []string{"separator", "parts"},
		ReturnType: &pschema.ReturnTypeSpec{
			TypeSpec: &pschema.TypeSpec{Type: "string"},
		},
	}, spec.Functions["test:index/join:join"])
}

func TestGenerateProviderFunctionRequiredOrder(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	info := tfbridge.ProviderInfo{
		Name: "test",
		P: (&shimschema.Provider{
			Functions: map[string]shim.Function{
				"lookup": {
					Parameters: []shim.FunctionParameter{
						{Name: "zone", Type: tftypes.String},
						{Name: "address", Type: tftypes.String},
					},
					Return: tftypes.String,
				},
			},
		}).Shim(),
		Functions: map[string]*info.Function{
			"lookup": {Tok: "test:index/lookup:lookup"},
		},
	}

	spec, err := generateFunctionSchema(t, info)
	require.NoError(t, err)

	fn := spec.Functions["test:index/lookup:lookup"]
	// Required keeps parameter order, matching multiArgumentInputs, rather than
	// sorting by Pulumi name.
	assert.Equal(t, []string{"zone", "address"}, fn.Inputs.Required)
	assert.Equal(t, []string{"zone", "address"}, fn.MultiArgumentInputs)
}

func TestGenerateProviderFunctionNamingEdgeCases(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	info := tfbridge.ProviderInfo{
		Name: "test",
		P: (&shimschema.Provider{
			Functions: map[string]shim.Function{
				"weird": {
					Parameters: []shim.FunctionParameter{
						{Type: tftypes.String},
						{Name: "value", Type: tftypes.String},
						{Name: "value", Type: tftypes.String},
					},
					Return: tftypes.DynamicPseudoType,
				},
			},
		}).Shim(),
		Functions: map[string]*info.Function{
			"weird": {Tok: "test:index/weird:weird"},
		},
	}

	spec, err := generateFunctionSchema(t, info)
	require.NoError(t, err)

	fn := spec.Functions["test:index/weird:weird"]
	assert.Equal(t, []string{"arg1", "value", "value2"}, fn.MultiArgumentInputs)
	assert.Equal(t, &pschema.ReturnTypeSpec{
		TypeSpec: &pschema.TypeSpec{Ref: "pulumi.json#/Any"},
	}, fn.ReturnType)
}

func TestGenerateProviderFunctionObjectParameter(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	info := tfbridge.ProviderInfo{
		Name: "test",
		P: (&shimschema.Provider{
			Functions: map[string]shim.Function{
				"describe": {
					Parameters: []shim.FunctionParameter{
						{
							Name: "config",
							Type: tftypes.Object{
								AttributeTypes: map[string]tftypes.Type{
									"region":  tftypes.String,
									"verbose": tftypes.Bool,
								},
								OptionalAttributes: map[string]struct{}{
									"verbose": {},
								},
							},
						},
					},
					Return: tftypes.String,
				},
			},
		}).Shim(),
		Functions: map[string]*info.Function{
			"describe": {Tok: "test:index/describe:describe"},
		},
	}

	spec, err := generateFunctionSchema(t, info)
	require.NoError(t, err)

	fn := spec.Functions["test:index/describe:describe"]
	assert.Equal(t, pschema.TypeSpec{
		Ref: "#/types/test:index/describeConfig:describeConfig",
	}, fn.Inputs.Properties["config"].TypeSpec)

	assert.Equal(t, pschema.ComplexTypeSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Type: "object",
			Properties: map[string]pschema.PropertySpec{
				"region":  {TypeSpec: pschema.TypeSpec{Type: "string"}},
				"verbose": {TypeSpec: pschema.TypeSpec{Type: "boolean"}},
			},
			Required: []string{"region"},
		},
	}, spec.Types["test:index/describeConfig:describeConfig"])
}

func TestGenerateProviderFunctionTupleError(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	info := tfbridge.ProviderInfo{
		Name: "test",
		P: (&shimschema.Provider{
			Functions: map[string]shim.Function{
				"tuple_fn": {
					Parameters: []shim.FunctionParameter{
						{Name: "pair", Type: tftypes.Tuple{
							ElementTypes: []tftypes.Type{tftypes.String, tftypes.Number},
						}},
					},
					Return: tftypes.String,
				},
			},
		}).Shim(),
		Functions: map[string]*info.Function{
			"tuple_fn": {Tok: "test:index/tupleFn:tupleFn"},
		},
	}

	_, err := generateFunctionSchema(t, info)
	assert.ErrorContains(t, err, "tuple types are not supported")
	assert.ErrorContains(t, err, `function "tuple_fn"`)
}

func TestGenerateProviderFunctionDataSourceTokenCollision(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	info := tfbridge.ProviderInfo{
		Name: "test",
		P: (&shimschema.Provider{
			DataSourcesMap: schemaOnlyDataSource(),
			Functions: map[string]shim.Function{
				"foo": {Return: tftypes.String},
			},
		}).Shim(),
		DataSources: map[string]*tfbridge.DataSourceInfo{
			"test_ds": {Tok: "test:index/foo:foo"},
		},
		Functions: map[string]*info.Function{
			"foo": {Tok: "test:index/foo:foo"},
		},
	}

	_, err := generateFunctionSchema(t, info)
	assert.ErrorContains(t, err,
		`TF function "foo" and TF data source "test_ds" are both mapped to the Pulumi token "test:index/foo:foo"`)
}

func TestGenerateProviderFunctionMappingErrors(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	provider := func() shim.Provider {
		return (&shimschema.Provider{
			Functions: map[string]shim.Function{
				"real_fn": {Return: tftypes.String},
			},
		}).Shim()
	}

	t.Run("missing mapping", func(t *testing.T) {
		_, err := generateFunctionSchema(t, tfbridge.ProviderInfo{
			Name: "test",
			P:    provider(),
		})
		assert.ErrorContains(t, err, `TF function "real_fn" not mapped to the Pulumi provider`)
	})

	t.Run("ignore mapping", func(t *testing.T) {
		spec, err := generateFunctionSchema(t, tfbridge.ProviderInfo{
			Name:           "test",
			P:              provider(),
			IgnoreMappings: []string{"real_fn"},
		})
		require.NoError(t, err)
		// The spec always carries the synthesized terraformConfig provider method;
		// no function generated from real_fn may appear.
		assert.NotContains(t, spec.Functions, "test:index/realFn:realFn")
	})

	t.Run("extra mapping", func(t *testing.T) {
		_, err := generateFunctionSchema(t, tfbridge.ProviderInfo{
			Name: "test",
			P:    provider(),
			Functions: map[string]*info.Function{
				"real_fn": {Tok: "test:index/realFn:realFn"},
				"gone_fn": {Tok: "test:index/goneFn:goneFn"},
			},
		})
		assert.ErrorContains(t, err,
			`Pulumi token "test:index/goneFn:goneFn" is mapped to TF provider function "gone_fn", but no such function found`)
	})
}

func schemaOnlyDataSource() shimschema.ResourceMap {
	return shimschema.ResourceMap{
		"test_ds": (&shimschema.Resource{
			Schema: shimschema.SchemaMap{
				"x": (&shimschema.Schema{Type: shim.TypeString, Optional: true}).Shim(),
			},
		}).Shim(),
	}
}
