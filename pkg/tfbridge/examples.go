package tfbridge

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"
)

// HclExampler represents a supplemental HCL example for a given resource or function.
type HclExampler interface {
	// GetToken returns the fully qualified path to the resource or function in the schema, e.g.
	// "provider:module/getFoo:getFoo" (function), or
	// "provider:module/bar:Bar" (resource)
	GetToken() string
	// GetMarkdown returns the Markdown that comprises the entire example, including the header.
	//
	// Headers should be an H3 ("###") and the header content should not contain any prefix, e.g. "Foo with Bar" not,
	// "Example Usage - Foo with Bar".
	//
	// Code should be surrounded with code fences with an indicator of the language on the opening fence, e.g. "```hcl".
	GetMarkdown() (string, error)
}

// LocalFileHclExample represents a supplemental HCL example that is on a relative path within the Pulumi provider repo.
type LocalFileHclExample struct {
	// Token is the Pulumi identifier for the resource or function to which the example pertains, e.g.
	// "provider:module/getFoo:getFoo" (function), or
	// "provider:module/bar:Bar" (resource)
	Token string
	// Title is title of the example, e.g. "Basic Something", "Advanced Something with Something Else".
	Title string
	// RelativePath is the path to the file in the Pulumi repo relative to the repo root.
	RelativePath string
}

// InlineHclExample represents a literal HCL example and is primarily used for testing the Bridge.
type InlineHclExample struct {
	// Token is the Pulumi identifier for the resource or function to which the example pertains, e.g.
	// "provider:module/getFoo:getFoo" (function), or
	// "provider:module/bar:Bar" (resource)
	Token string
	// Title is title of the example, e.g. "Basic Something", "Advanced Something with Something Else".
	Title string
	// Contents is the HCL that comprises the example with no surrounding Markdown constructs (e.g. ```hcl`).
	Contents string
}

func (e LocalFileHclExample) GetToken() string {
	return e.Token
}

func (e LocalFileHclExample) GetMarkdown() (string, error) {
	absPath, err := filepath.Abs(e.RelativePath)
	if err != nil {
		return "", err
	}

	fileBytes, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	return renderTemplate(e.Title, string(fileBytes))
}

func (e InlineHclExample) GetToken() string {
	return e.Token
}

func (e InlineHclExample) GetMarkdown() (string, error) {
	return renderTemplate(e.Title, e.Contents)
}

func renderTemplate(title, contents string) (string, error) {
	tmpl := `### {{ .Title }}

{{ .CodeFences }}hcl
{{ .Contents }}
{{ .CodeFences }}
`

	outputTemplate, _ := template.New("dummy").Parse(tmpl)
	data := struct {
		CodeFences string
		Title      string
		Contents   string
	}{
		CodeFences: "```",
		Title:      title,
		Contents:   contents,
	}

	var buf = bytes.Buffer{}
	err := outputTemplate.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
