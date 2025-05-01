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
// limitations under the License.

package tfgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"sort"
	"testing"
	"text/template"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	csgen "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	tsgen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	pygen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bridgetesting "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/testprovider"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/x/muxer"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

// TestRegress611 tests against test_data/regress-611-schema.json.
// To update the contents of test_data/regress-611-schema.json run the test with env var PULUMI_ACCEPT set to "true".
func TestRegress611(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderRegress611()
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/regress-611-schema.json", schema)
}

func TestRegressMiniRandom(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderMiniRandom()
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/regress-minirandom-schema.json", schema)
}

func TestMiniMuxed(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderMiniMuxed()
	provider.MetadataInfo = tfbridge.NewProviderMetadata(nil)
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)

	bridgetesting.AssertEqualsJSONFile(t, "test_data/minimuxed-schema.json", schema)

	table, found, err := metadata.Get[muxer.DispatchTable](provider.GetMetadata(), "mux")
	assert.NoError(t, err)
	assert.True(t, found)

	assert.Len(t, table.Functions, 2)
	idx, found := table.Functions["minimuxed:index/muxedFunction:muxedFunction"]
	assert.True(t, found)
	assert.Equal(t, 1, idx)

	assert.Equal(t, 1, len(table.Resources))
	idx, found = table.Resources["minimuxed:index/minimuxedInteger:MinimuxedInteger"]
	assert.True(t, found)
	assert.Equal(t, 0, idx)
}

func TestMiniMuxedReplace(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderMiniMuxedReplace()
	provider.MetadataInfo = tfbridge.NewProviderMetadata(nil)
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)

	bridgetesting.AssertEqualsJSONFile(t, "test_data/minimuxed-replace-schema.json", schema)

	table, found, err := metadata.Get[muxer.DispatchTable](provider.GetMetadata(), "mux")
	assert.NoError(t, err)
	assert.True(t, found)

	assert.Equal(t, 1, len(table.Functions))

	assert.Equal(t, 1, len(table.Resources))
	idx, found := table.Resources["minimuxed:index/minimuxedInteger:MinimuxedInteger"]
	assert.True(t, found)
	assert.Equal(t, 1, idx)
}

func TestCSharpMiniRandom(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderMiniRandomCSharp()
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/minirandom-schema-csharp.json", schema)
}

// Test the ability to force type sharing. Some of the upstream providers generate very large concrete schemata in Go,
// with TF not being materially affected. The example is inspired by QuickSight types in AWS. In Pulumi the default
// projection is going to generate named types for every instance of the shared schema. This may lead to SDK bloat. Test
// the ability of the provider author to curb the bloat and force an explicit sharing.
func TestTypeSharing(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on Windows due to a test setup issue")
	}

	tmpdir := t.TempDir()
	barCharVisualSchema := func() *schema.Schema {
		return &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			MinItems: 1,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"nest": {
						Type:     schema.TypeList,
						MaxItems: 1,
						Optional: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"nested_prop": {
									Type:     schema.TypeBool,
									Optional: true,
								},
							},
						},
					},
				},
			},
		}
	}
	visualsSchema := func() *schema.Schema {
		return &schema.Schema{
			Type:     schema.TypeList,
			MinItems: 1,
			MaxItems: 50,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"bar_chart_visual": barCharVisualSchema(),
					"box_plot_visual":  barCharVisualSchema(),
				},
			},
		}
	}
	provider := info.Provider{
		Name: "testprov",
		P: sdkv2.NewProvider(&schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"testprov_r1": {
					Schema: map[string]*schema.Schema{
						"sheets": {
							Type:     schema.TypeList,
							MinItems: 1,
							MaxItems: 20,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"visuals": visualsSchema(),
								},
							},
						},
					},
				},
				"testprov_r2": {
					Schema: map[string]*schema.Schema{
						"x": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"sheets": {
							Type:     schema.TypeList,
							MinItems: 1,
							MaxItems: 20,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"y": {
										Type:     schema.TypeBool,
										Optional: true,
									},
									"visuals": visualsSchema(),
								},
							},
						},
					},
				},
			},
		}),
		UpstreamRepoPath: tmpdir,
		Repository:       "https://github.com/pulumi/pulumi-testprov",
		Resources: map[string]*info.Resource{
			"testprov_r1": {
				Tok: "testprov:index:R1",
				Fields: map[string]*info.Schema{
					"sheets": {
						Elem: &info.Schema{
							Fields: map[string]*info.Schema{
								"visuals": {
									Elem: &info.Schema{
										TypeName: tfbridge.Ref("Visual"),
									},
								},
							},
						},
					},
				},
			},
			"testprov_r2": {
				Tok: "testprov:index:R2",
				Fields: map[string]*info.Schema{
					"sheets": {
						Elem: &info.Schema{
							Fields: map[string]*info.Schema{
								"visuals": {
									Elem: &info.Schema{
										Type:     "testprov:index/Visual:Visual",
										OmitType: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	schema, err := GenerateSchema(provider, diag.DefaultSink(&buf, &buf, diag.FormatOptions{
		Color: colors.Never,
	}))
	require.NoError(t, err)

	t.Logf("%s", buf.String())

	keys := []string{}
	for k := range schema.Types {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Note that there is only one set of helper types, and they are not prefixed by any of the resource names.
	autogold.Expect([]string{
		"testprov:index/R1Sheet:R1Sheet", "testprov:index/R2Sheet:R2Sheet",
		"testprov:index/Visual:Visual",
		"testprov:index/VisualBarChartVisual:VisualBarChartVisual",
		"testprov:index/VisualBarChartVisualNest:VisualBarChartVisualNest",
		"testprov:index/VisualBoxPlotVisual:VisualBoxPlotVisual",
		"testprov:index/VisualBoxPlotVisualNest:VisualBoxPlotVisualNest",
	}).Equal(t, keys)

	bytes, err := json.MarshalIndent(schema, "", "  ")
	require.NoError(t, err)

	autogold.ExpectFile(t, autogold.Raw(string(bytes)))
}

// TestPropertyDocumentationEdits tests that documentation edits are applied to
// individual properties. This includes both the property description and
// deprecation message. This tests the following workflow
//  1. The generator finds markdown documentation for the `aws_s3_bucket`
//     resource
//  2. The generator applies `DocsEdit` rules to the markdown documentation
//  3. The generator parses the markdown documentation and pulls out the `acl`
//     argument description and merges that into the schema
//  3. The generator cleans up the `acl` description and deprecation message,
//     replacing terraform references with pulumi references e.g.
//     `aws_s3_bucket_acl` -> `aws.s3.BucketAclV2`
func TestPropertyDocumentationEdits(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderMiniAws()
	provider.MetadataInfo = tfbridge.NewProviderMetadata(nil)
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)

	// asserts that `aws_s3_bucket_acl` has been changed to `aws.s3.BucketAclV2`
	assert.Equal(t,
		"The [canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl) to apply."+
			" Valid values are `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`,"+
			" and `log-delivery-write`. Defaults to `private`.  Conflicts with `grant`. The provider will only perform drift "+
			"detection if a configuration value is provided. Use the resource `aws.s3.BucketAclV2` instead.\n",
		schema.Resources["aws:s3/bucketV2:BucketV2"].InputProperties["acl"].Description,
	)
	assert.Equal(t,
		"Use the aws.s3.BucketAclV2 resource instead",
		schema.Resources["aws:s3/bucketV2:BucketV2"].InputProperties["acl"].DeprecationMessage,
	)
}

func TestNestedMaxItemsOne(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderMiniCloudflare()
	meta, err := metadata.New(nil)
	require.NoError(t, err)
	provider.MetadataInfo = &tfbridge.MetadataInfo{
		Path: "non-nil",
		Data: meta,
	}
	err = provider.ApplyAutoAliases()
	require.NoError(t, err)
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/regress-minicloudflare-schema.json", schema)

	// We will now remove manual MaxItemsOne and assert that AutoAliasing re-applies them.
	provider = testprovider.ProviderMiniCloudflare()
	actionParameters := provider.Resources["cloudflare_ruleset"].Fields["rules"].Elem.Fields["action_parameters"]
	actionParameters.MaxItemsOne = nil
	actionParameters.Elem.Fields["phases"].MaxItemsOne = nil
	// Round trip through serialization to simulate writing out and reading from disk.
	meta, err = metadata.New(meta.Marshal())
	require.NoError(t, err)
	provider.MetadataInfo = &tfbridge.MetadataInfo{
		Path: "non-nil",
		Data: meta,
	}
	err = provider.ApplyAutoAliases()
	require.NoError(t, err)

	schema2, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)
	assert.Equal(t, schema, schema2)
}

// TestNestedTypeSingularization shows that we singularize types associated with list
// properties.
//
// The test also shows that we can override this behavior by setting [tfbridge.SchemaInfo.NestedType].
//
//nolint:lll // Long type names make it arduous to stay under the 120 char limit.
func TestNestedTypeSingularization(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		provider := testprovider.ProviderMiniCloudflare()
		{
			actionParameters := provider.Resources["cloudflare_ruleset"].Fields["rules"].Elem.Fields["action_parameters"]
			actionParameters.MaxItemsOne = nil
			actionParameters.Elem.Fields["phases"].MaxItemsOne = nil
		}
		actual, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
			Color: colors.Never,
		}))
		assert.NoError(t, err)

		autogold.Expect(pschema.PropertySpec{
			TypeSpec: pschema.TypeSpec{
				Type:  "array",
				Items: &pschema.TypeSpec{Ref: "#/types/cloudflare:index/RulesetRuleActionParameterPhase:RulesetRuleActionParameterPhase"},
			},
			Language: map[string]pschema.RawMessage{},
		}).Equal(t, actual.Types["cloudflare:index/RulesetRuleActionParameter:RulesetRuleActionParameter"].Properties["phases"])
	})

	t.Run("disable", func(t *testing.T) {
		provider := testprovider.ProviderMiniCloudflare()
		{
			actionParameters := provider.Resources["cloudflare_ruleset"].Fields["rules"].Elem.Fields["action_parameters"]
			actionParameters.MaxItemsOne = nil
			actionParameters.Elem.Fields["phases"].MaxItemsOne = nil
			actionParameters.Elem.Fields["phases"].Elem = &tfbridge.SchemaInfo{NestedType: "RulesetRuleActionParameterPhases"}
		}
		actual, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
			Color: colors.Never,
		}))
		assert.NoError(t, err)

		autogold.Expect(pschema.PropertySpec{
			TypeSpec: pschema.TypeSpec{
				Type: "array",
				Items: &pschema.TypeSpec{
					Ref: "#/types/cloudflare:index/RulesetRuleActionParameterPhases:RulesetRuleActionParameterPhases",
				},
			},
			Language: map[string]pschema.RawMessage{},
		}).Equal(t, actual.Types["cloudflare:index/RulesetRuleActionParameter:RulesetRuleActionParameter"].Properties["phases"])

		// Check that a type of the correct name gets generated.
		autogold.Expect(pschema.ComplexTypeSpec{ObjectTypeSpec: pschema.ObjectTypeSpec{
			Properties: map[string]pschema.PropertySpec{"phase1": {
				TypeSpec: pschema.TypeSpec{Type: "string"},
				Language: map[string]pschema.RawMessage{},
			}},
			Type:     "object",
			Required: []string{"phase1"},
		}}).Equal(t, actual.Types["cloudflare:index/RulesetRuleActionParameterPhases:RulesetRuleActionParameterPhases"])
	})
}

func TestNestedDescriptions(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderNestedDescriptions()
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	require.NoError(t, err)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/nested-descriptions-schema.json", schema)
}

func TestRequiredInputWithDefault(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderRequiredInputWithDefaultFunc()
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	require.NoError(t, err)
	require.Empty(t, schema.Resources["testprovider:index:Res"].RequiredInputs)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/required-input-with-default-schema.json", schema)
}

func TestNestedFullyComputed(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderNestedFullyComputed()
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	require.NoError(t, err)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/nested-fully-computed-schema.json", schema)
}

func TestAppendExample_InsertMiddle(t *testing.T) {
	t.Parallel()
	descTmpl := `Description text

## Example Usage

### Basic

Basic example content

## Import

Import content
`
	markdownTmpl := `### My Example

{{ .CodeFences }}hcl
# My example
{{ .CodeFences }}
`

	expectedTmpl := `Description text

## Example Usage

### Basic

Basic example content

### My Example

{{ .CodeFences }}hcl
# My example
{{ .CodeFences }}

## Import

Import content
`
	assert.Equal(t, renderTemplate(expectedTmpl), appendExample(renderTemplate(descTmpl), renderTemplate(markdownTmpl)))
}

func TestAppendExample_InsertEnd(t *testing.T) {
	t.Parallel()
	descTmpl := `Description text

## Example Usage

### Basic

Basic example content
`
	markdownTmpl := `### My Example

{{ .CodeFences }}hcl
# My example content
{{ .CodeFences }}
`

	expectedTmpl := `Description text

## Example Usage

### Basic

Basic example content

### My Example

{{ .CodeFences }}hcl
# My example content
{{ .CodeFences }}
`
	assert.Equal(t, renderTemplate(expectedTmpl), appendExample(renderTemplate(descTmpl), renderTemplate(markdownTmpl)))
}

// Extra test case to ensure that we do not modify the source material internally in the function.
func TestAppendExample_NoOp(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", appendExample("", ""))
	assert.Equal(t, "foo\nbar", appendExample("foo\nbar", ""))

	input := `Description

## Example Usage

example usage content
`
	assert.Equal(t, input, appendExample(input, ""))
}

// There are resources (or more commonly, functions) that do not have ## Example Usage in the in the source description.
// Therefore, we need to add the H2 if none exists to emit a well-formed doc page.
func TestAppendExample_NoExampleUsage(t *testing.T) {
	t.Parallel()
	input := "Description Text"
	markdownTmpl := `### My Example

{{ .CodeFences }}hcl
# My example content
{{ .CodeFences }}
`
	expectedTmpl := `Description Text

## Example Usage

### My Example

{{ .CodeFences }}hcl
# My example content
{{ .CodeFences }}
`
	assert.Equal(t, renderTemplate(expectedTmpl), appendExample(input, renderTemplate(markdownTmpl)))
}

func TestAppendExample_NoExampleUsage_ImportsPresent(t *testing.T) {
	t.Parallel()
	input := `Description Text

## Import

import content
`
	markdownTmpl := `### My Example

{{ .CodeFences }}hcl
# My example content
{{ .CodeFences }}
`
	expectedTmpl := `Description Text

## Example Usage

### My Example

{{ .CodeFences }}hcl
# My example content
{{ .CodeFences }}

## Import

import content
`

	assert.Equal(t, renderTemplate(expectedTmpl), appendExample(input, renderTemplate(markdownTmpl)))
}

// / renderTemplate allows us to easily use code fences with herestrings
func renderTemplate(tmpl string) string {
	outputTemplate, _ := template.New("dummy").Parse(tmpl)
	data := struct {
		CodeFences string
	}{
		CodeFences: "```",
	}

	buf := bytes.Buffer{}
	_ = outputTemplate.Execute(&buf, data)

	return buf.String()
}

func TestGetDefaultReadme(t *testing.T) {
	t.Parallel()
	//nolint:lll
	expected := "> This provider is a derived work of the [Terraform Provider](https://github.com/hashicorp/terraform-provider-aws)\n" +
		"> distributed under [MPL 2.0](https://www.mozilla.org/en-US/MPL/2.0/). If you encounter a bug or missing feature,\n" +
		"> first check the [`pulumi-aws` repo](https://github.com/pulumi/pulumi-aws/issues); however, if that doesn't turn up anything,\n" +
		"> please consult the source [`terraform-provider-aws` repo](https://github.com/hashicorp/terraform-provider-aws/issues)."

	actual := getDefaultReadme("aws", "aws", "hashicorp",
		tfbridge.MPL20LicenseType, "https://www.mozilla.org/en-US/MPL/2.0/", "github.com",
		"https://github.com/pulumi/pulumi-aws")
	assert.Equal(t, expected, actual)
}

func TestPropagateLanguageOptions(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderMiniRandom() // choice of provider is arbitrary here

	require.Nil(t, provider.Golang)

	provider.Golang = &tfbridge.GolangInfo{
		RespectSchemaVersion:          true,
		DisableFunctionOutputVersions: true,
	}

	require.Nil(t, provider.Python)

	provider.Python = &tfbridge.PythonInfo{
		RespectSchemaVersion: true,
	}

	require.Nil(t, provider.JavaScript)

	provider.JavaScript = &tfbridge.JavaScriptInfo{
		RespectSchemaVersion: true,
	}

	require.Nil(t, provider.CSharp)

	provider.CSharp = &tfbridge.CSharpInfo{
		RespectSchemaVersion: true,
	}

	require.Nil(t, provider.Java)
	provider.Java = &tfbridge.JavaInfo{
		BuildFiles: "gradle",
	}

	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)

	t.Run("all-languages", func(t *testing.T) {
		// If this test fails, you may run the test with PULUMI_ACCEPT=1 environment variable to reset expected
		// schema file with the actually generated schema.
		bridgetesting.AssertEqualsJSONFile(t, "test_data/test-propagate-language-options.json", schema)
	})

	t.Run("golang", func(t *testing.T) {
		actual := gogen.GoPackageInfo{}
		err = json.Unmarshal(schema.Language["go"], &actual)
		require.NoError(t, err)
		assert.True(t, actual.RespectSchemaVersion)
	})

	t.Run("python", func(t *testing.T) {
		actual := pygen.PackageInfo{}
		err = json.Unmarshal(schema.Language["python"], &actual)
		assert.True(t, actual.RespectSchemaVersion)
	})

	t.Run("typescript", func(t *testing.T) {
		actual := tsgen.NodePackageInfo{}
		err = json.Unmarshal(schema.Language["nodejs"], &actual)
		require.NoError(t, err)
		assert.True(t, actual.RespectSchemaVersion)
	})

	t.Run("csharp", func(t *testing.T) {
		actual := csgen.CSharpPackageInfo{}
		err = json.Unmarshal(schema.Language["csharp"], &actual)
		require.NoError(t, err)
		assert.True(t, actual.RespectSchemaVersion)
	})

	t.Run("java", func(t *testing.T) {
		actual := map[string]any{}
		err = json.Unmarshal(schema.Language["java"], &actual)
		require.NoError(t, err)
		assert.Equal(t, "gradle", actual["buildFiles"])
	})
}

func TestDefaultInfoFails(t *testing.T) {
	t.Parallel()
	provider := testprovider.ProviderDefaultInfo()
	meta, err := metadata.New(nil)
	require.NoError(t, err)
	provider.MetadataInfo = &tfbridge.MetadataInfo{
		Path: "non-nil",
		Data: meta,
	}
	require.NoError(t, err)
	defer func() {
		r := recover()
		assert.Contains(
			t,
			r,
			"Property id has a DefaultInfo Value [default_id] of kind slice which is not currently supported.",
		)
	}()
	// Should panic
	_, _ = GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}))
}

func TestRegress1626(t *testing.T) {
	t.Parallel()
	info := testprovider.ProviderMiniTalos()
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	s, err := GenerateSchema(info, sink)
	t.Logf("SPEC: %v", s)
	require.NoError(t, err)
}

func TestSinkHclDiagnostics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    hcl.Diagnostics
		expected autogold.Value
	}{
		{
			name:     "No diagnostics",
			input:    hcl.Diagnostics{},
			expected: autogold.Expect(captureDiagSink{}),
		},
		{
			name: "Only errors, fewer than the threshold",
			input: hcl.Diagnostics{
				{Severity: hcl.DiagError, Summary: "Error 1", Detail: "Detail 1"},
				{Severity: hcl.DiagError, Summary: "Error 2", Detail: "Detail 2"},
			},
			expected: autogold.Expect(captureDiagSink{
				{Diag: &diag.Diag{Message: "Error 1: Detail 1"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Error 2: Detail 2"}, Sev: diag.Error},
			}),
		},
		{
			name: "Only errors, more than the threshold",
			input: hcl.Diagnostics{
				{Severity: hcl.DiagError, Summary: "Error 1", Detail: "Detail 1"},
				{Severity: hcl.DiagError, Summary: "Error 2", Detail: "Detail 2"},
				{Severity: hcl.DiagError, Summary: "Error 3", Detail: "Detail 3"},
				{Severity: hcl.DiagError, Summary: "Error 4", Detail: "Detail 4"},
				{Severity: hcl.DiagError, Summary: "Error 5", Detail: "Detail 5"},
				{Severity: hcl.DiagError, Summary: "Error 6", Detail: "Detail 6"},
				{Severity: hcl.DiagError, Summary: "Error 7", Detail: "Detail 7"},
			},
			expected: autogold.Expect(captureDiagSink{
				{Diag: &diag.Diag{Message: "Error 1: Detail 1"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Error 2: Detail 2"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Error 3: Detail 3"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Error 4: Detail 4"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Error 5: Detail 5"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Error 6: Detail 6"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "1 additional errors"}, Sev: diag.Error},
			}),
		},
		{
			name: "Only warnings, fewer than the threshold",
			input: hcl.Diagnostics{
				{Severity: hcl.DiagWarning, Summary: "Warning 1", Detail: "Detail 1"},
				{Severity: hcl.DiagWarning, Summary: "Warning 2", Detail: "Detail 2"},
			},
			expected: autogold.Expect(captureDiagSink{
				{Diag: &diag.Diag{Message: "Warning 1: Detail 1"}, Sev: diag.Warning},
				{Diag: &diag.Diag{Message: "Warning 2: Detail 2"}, Sev: diag.Warning},
			}),
		},
		{
			name: "Only warnings, more than the threshold",
			input: hcl.Diagnostics{
				{Severity: hcl.DiagWarning, Summary: "Warning 1", Detail: "Detail 1"},
				{Severity: hcl.DiagWarning, Summary: "Warning 2", Detail: "Detail 2"},
				{Severity: hcl.DiagWarning, Summary: "Warning 3", Detail: "Detail 3"},
				{Severity: hcl.DiagWarning, Summary: "Warning 4", Detail: "Detail 4"},
				{Severity: hcl.DiagWarning, Summary: "Warning 5", Detail: "Detail 5"},
				{Severity: hcl.DiagWarning, Summary: "Warning 6", Detail: "Detail 6"},
				{Severity: hcl.DiagWarning, Summary: "Warning 7", Detail: "Detail 7"},
			},
			expected: autogold.Expect(captureDiagSink{
				{Diag: &diag.Diag{Message: "Warning 1: Detail 1"}, Sev: diag.Warning},
				{Diag: &diag.Diag{Message: "Warning 2: Detail 2"}, Sev: diag.Warning},
				{Diag: &diag.Diag{Message: "Warning 3: Detail 3"}, Sev: diag.Warning},
				{Diag: &diag.Diag{Message: "Warning 4: Detail 4"}, Sev: diag.Warning},
				{Diag: &diag.Diag{Message: "Warning 5: Detail 5"}, Sev: diag.Warning},
				{Diag: &diag.Diag{Message: "Warning 6: Detail 6"}, Sev: diag.Warning},
				{Diag: &diag.Diag{Message: "1 additional warnings"}, Sev: diag.Warning},
			}),
		},
		{
			name: "Mix of errors and warnings, fewer than the threshold",
			input: hcl.Diagnostics{
				{Severity: hcl.DiagError, Summary: "Error 1", Detail: "Detail 1"},
				{Severity: hcl.DiagError, Summary: "Error 2", Detail: "Detail 2"},
				{Severity: hcl.DiagWarning, Summary: "Warning 1", Detail: "Detail 1"},
				{Severity: hcl.DiagWarning, Summary: "Warning 2", Detail: "Detail 2"},
			},
			expected: autogold.Expect(captureDiagSink{
				{Diag: &diag.Diag{Message: "Error 1: Detail 1"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Error 2: Detail 2"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Warning 1: Detail 1"}, Sev: diag.Warning},
				{Diag: &diag.Diag{Message: "Warning 2: Detail 2"}, Sev: diag.Warning},
			}),
		},
		{
			name: "Mix of errors and warnings, exceeding the threshold",
			input: hcl.Diagnostics{
				{Severity: hcl.DiagError, Summary: "Error 1", Detail: "Detail 1"},
				{Severity: hcl.DiagError, Summary: "Error 2", Detail: "Detail 2"},
				{Severity: hcl.DiagError, Summary: "Error 3", Detail: "Detail 3"},
				{Severity: hcl.DiagError, Summary: "Error 4", Detail: "Detail 4"},
				{Severity: hcl.DiagWarning, Summary: "Warning 1", Detail: "Detail 1"},
				{Severity: hcl.DiagWarning, Summary: "Warning 2", Detail: "Detail 2"},
				{Severity: hcl.DiagWarning, Summary: "Warning 3", Detail: "Detail 3"},
				{Severity: hcl.DiagWarning, Summary: "Warning 4", Detail: "Detail 4"},
			},
			expected: autogold.Expect(captureDiagSink{
				{Diag: &diag.Diag{Message: "Error 1: Detail 1"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Error 2: Detail 2"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Error 3: Detail 3"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Error 4: Detail 4"}, Sev: diag.Error},
				{Diag: &diag.Diag{Message: "Warning 1: Detail 1"}, Sev: diag.Warning},
				{Diag: &diag.Diag{Message: "Warning 2: Detail 2"}, Sev: diag.Warning},
				{Diag: &diag.Diag{Message: "2 additional warnings"}, Sev: diag.Warning},
			}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var actual captureDiagSink
			sinkHclDiagnostics(&actual, tt.input)
			tt.expected.Equal(t, actual)
		})
	}
}

type captureDiagSink []capturedDiag

type capturedDiag struct {
	*diag.Diag
	Sev diag.Severity
}

var _ diag.Sink = &captureDiagSink{}

func (c *captureDiagSink) Debugf(d *diag.Diag, a ...any)   { c.Logf(diag.Debug, d, a...) }
func (c *captureDiagSink) Errorf(d *diag.Diag, a ...any)   { c.Logf(diag.Error, d, a...) }
func (c *captureDiagSink) Infoerrf(d *diag.Diag, a ...any) { c.Logf(diag.Infoerr, d, a...) }
func (c *captureDiagSink) Infof(d *diag.Diag, a ...any)    { c.Logf(diag.Info, d, a...) }
func (c *captureDiagSink) Warningf(d *diag.Diag, a ...any) { c.Logf(diag.Warning, d, a...) }

func (c *captureDiagSink) Logf(s diag.Severity, d *diag.Diag, a ...any) {
	d.Message = fmt.Sprintf(d.Message, a...)
	*c = append(*c, capturedDiag{d, s})
}

func (c *captureDiagSink) Stringify(diag.Severity, *diag.Diag, ...any) (string, string) {
	panic("unimplemented")
}
