package tfbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInlineExampleGetMarkdown(t *testing.T) {
	t.Parallel()
	expected := "### Inline Title\n\n```hcl\n# HCL inline contents\n```\n"
	example := InlineHclExample{
		Title:    "Inline Title",
		Contents: "# HCL inline contents",
	}

	actual, err := example.GetMarkdown()

	assert.Equal(t, expected, actual)
	assert.Nil(t, err)
}

func TestLocalFileExampleGetMarkdown(t *testing.T) {
	t.Parallel()
	expected := "### File Title\n\n```hcl\n# HCL file contents\n```\n"

	example := LocalFileHclExample{
		Title:        "File Title",
		RelativePath: "examples_test.hcl",
	}

	actual, err := example.GetMarkdown()

	assert.Equal(t, expected, actual)
	assert.Nil(t, err)
}
