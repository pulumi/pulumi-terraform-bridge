package tfgen

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
	"text/template"
)

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

/// renderTemplate allows us to easily use code fences with herestrings
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
