package tfgen

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"

	bridgetesting "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestRegress611(t *testing.T) {
	provider := testprovider.ProviderRegress611()
	schema, err := GenerateSchema(provider, nil)
	assert.NoError(t, err)
	bridgetesting.AssertPackageSpecEquals(t, "test_data/regress-611-schema.json", schema)
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
