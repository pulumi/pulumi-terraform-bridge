// Copyright 2016-2022, Pulumi Corporation.
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
	"io"
	"testing"
	"text/template"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bridgetesting "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
	csgen "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	tsgen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	pygen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

// TestRegress611 tests against test_data/regress-611-schema.json.
// To update the contents of test_data/regress-611-schema.json run the test with env var PULUMI_ACCEPT set to "true".
func TestRegress611(t *testing.T) {
	provider := testprovider.ProviderRegress611()
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/regress-611-schema.json", schema)
}

func TestRegressMiniRandom(t *testing.T) {
	provider := testprovider.ProviderMiniRandom()
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/regress-minirandom-schema.json", schema)
}

func TestMiniMuxed(t *testing.T) {
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

	assert.Len(t, table.Functions, 1)
	idx, found := table.Functions["minimuxed:index/muxedFunction:muxedFunction"]
	assert.True(t, found)
	assert.Equal(t, 1, idx)

	assert.Equal(t, 1, len(table.Resources))
	idx, found = table.Resources["minimuxed:index/minimuxedInteger:MinimuxedInteger"]
	assert.True(t, found)
	assert.Equal(t, 0, idx)
}

func TestMiniMuxedReplace(t *testing.T) {
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

	assert.Equal(t, 0, len(table.Functions))

	assert.Equal(t, 1, len(table.Resources))
	idx, found := table.Resources["minimuxed:index/minimuxedInteger:MinimuxedInteger"]
	assert.True(t, found)
	assert.Equal(t, 1, idx)
}

func TestCSharpMiniRandom(t *testing.T) {
	provider := testprovider.ProviderMiniRandomCSharp()
	schema, err := GenerateSchema(provider, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))
	assert.NoError(t, err)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/minirandom-schema-csharp.json", schema)
}

func TestNestedMaxItemsOne(t *testing.T) {
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

func TestRenameGeneration(t *testing.T) {
	info := testprovider.ProviderRegress611()

	g, err := NewGenerator(GeneratorOptions{
		Package:      info.Name,
		Version:      info.Version,
		Language:     Schema,
		ProviderInfo: info,
		Root:         afero.NewMemMapFs(),
		Sink: diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
			Color: colors.Never,
		}),
	})
	require.NoError(t, err)

	err = g.Generate()
	require.NoError(t, err)

	renames, err := g.Renames()
	require.NoError(t, err)
	bridgetesting.AssertEqualsJSONFile(t, "test_data/regress-611-renames.json", renames)
}

func TestAppendExample_InsertMiddle(t *testing.T) {
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
	input := "Description Text"
	markdownTmpl := `### My Example

{{ .CodeFences }}hcl
# My example content
{{ .CodeFences }}
`
	expectedTmpl :=
		`Description Text

## Example Usage

### My Example

{{ .CodeFences }}hcl
# My example content
{{ .CodeFences }}
`
	assert.Equal(t, renderTemplate(expectedTmpl), appendExample(input, renderTemplate(markdownTmpl)))
}

func TestAppendExample_NoExampleUsage_ImportsPresent(t *testing.T) {
	input := `Description Text

## Import

import content
`
	markdownTmpl := `### My Example

{{ .CodeFences }}hcl
# My example content
{{ .CodeFences }}
`
	expectedTmpl :=
		`Description Text

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

	var buf = bytes.Buffer{}
	_ = outputTemplate.Execute(&buf, data)

	return buf.String()
}

func TestGetDefaultReadme(t *testing.T) {
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
