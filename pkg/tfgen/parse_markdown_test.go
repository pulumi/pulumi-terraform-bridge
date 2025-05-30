package tfgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	bf "github.com/russross/blackfriday/v2"
	"github.com/stretchr/testify/assert"
)

var pulumiAccept = func() bool {
	b, _ := strconv.ParseBool(os.Getenv("PULUMI_ACCEPT"))
	return b
}()

func TestParseTextSeq(t *testing.T) {
	t.Parallel()
	turnaround := func(src string) {
		res, err := parseTextSeq(parseNode(src).FirstChild, true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, src, res)
	}

	turnaround("plain")
	turnaround("`code`")
	turnaround("*emph*")
	turnaround("**strong**")
	turnaround("[link](http://pulumi.com)")
	turnaround("plain `code` *emph* **strong** [link](http://pulumi.com)")
	turnaround(`(Block List, Max: 1) The definition for a Change  widget. (see [below for nested schema]` +
		`(#nestedblock--widget--group_definition--widget--change_definition))`)

	res, err := parseTextSeq(parseNode("request_max_bytes").FirstChild, false)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "request_max_bytes", res)
}

func TestParsePreamble(t *testing.T) {
	t.Parallel()
	node := parseNode(`<a id="nestedblock--widget--group_definition--widget--id--request"></a>`)
	actual := parsePreamble(node, func(x *bf.Node) {})
	if actual == nil {
		t.Errorf("parsePreamble(node) == nil")
		return
	}
	expected := "nestedblock--widget--group_definition--widget--id--request"
	if *actual != expected {
		t.Errorf("parsePreamble(node) == `%s`, expected `%s`", *actual, expected)
	}
}

func TestParseParameterFromDescription(t *testing.T) {
	t.Parallel()
	actual := parseParameterFromDescription(
		"heatmap_definition",
		"(Block List, Max: 1) The definition for a Heatmap widget. (see below for nested schema)")
	expectedDesc := "The definition for a Heatmap widget. (see below for nested schema)"
	if actual.desc != expectedDesc {
		t.Errorf("Expected desc to be %s but got %s", expectedDesc, actual.desc)
	}
	expectedTypeDecl := "Block List, Max: 1"
	if actual.typeDecl != expectedTypeDecl {
		t.Errorf("Expected typeDecl to be %s but got %s", expectedTypeDecl, actual.typeDecl)
	}
}

func TestParseTopLevelSchema(t *testing.T) {
	t.Parallel()
	markdown := readTestFile(t, "mini.md")

	var schema *topLevelSchema

	parseDoc([]byte(markdown)).Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
		if entering {
			tls, err := parseTopLevelSchema(node, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tls != nil && schema != nil {
				t.Fatalf("Parsed Schema twice")
			}
			if tls != nil {
				schema = tls
			}
		}
		return bf.GoToNext
	})

	assert.NotNil(t, schema)

	assert.Equal(t, 6, len(schema.nestedSchemata))

	assert.Equal(t, topParam(t, schema, "layout_type").desc,
		"The layout type of the dashboard, either 'free' or 'ordered'.")

	assert.Equal(t, "The layout type of the group, only 'ordered' for now.",
		param(t, nested(t, schema, "widget.group_definition"), "layout_type").desc)

	assert.Equal(t, "The definition for a Group widget.",
		param(t, nested(t, schema, "widget"), "group_definition").desc)

	assert.Equal(t, "",
		param(t, nested(t, schema, "widget.group_definition"), "title").desc)

	assert.Equal(t, topParam(t, schema, "theme").desc,
		"The theme of the dashboard.")
}

func TestParseNestedSchemaIntoDoc(t *testing.T) {
	t.Parallel()
	markdown := readTestFile(t, "mini.md")
	out := &entityDocs{}
	parseDoc([]byte(markdown)).Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
		if entering {
			nested, err := parseNestedSchema(node, nil)
			if err != nil {
				t.Fatal(err)
			}
			if nested != nil {
				parseNestedSchemaIntoDocs(out, nested, nil)
			}
		}
		return bf.GoToNext
	})
	actual, err := json.MarshalIndent(out.Arguments, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	compareTestFile(t, "mini.json", string(actual), assert.JSONEq)
}

func compareTestFile(
	t *testing.T, path, actual string,
	comp func(t assert.TestingT, expected string, actual string, msgAndArgs ...interface{}) bool,
) {
	if pulumiAccept {
		err := os.WriteFile(testFilePath(path), []byte(actual), 0o600)
		assert.NoError(t, err)
	} else {
		comp(t, readTestFile(t, path), actual)
	}
}

func testFilePath(path ...string) string {
	return filepath.Join(append([]string{"test_data"}, path...)...)
}

func readTestFile(t *testing.T, name string) string {
	bytes, err := os.ReadFile(testFilePath(name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(bytes), "\r\n", "\n")
}

func nested(t *testing.T, tls *topLevelSchema, name string) *nestedSchema {
	for _, s := range tls.nestedSchemata {
		if s.longName == name {
			return &s
		}
	}
	t.Errorf("Could not find nested schema %s", name)
	return &nestedSchema{}
}

func param(t *testing.T, s *nestedSchema, name string) parameter {
	for _, p := range s.allParameters() {
		if p.name == name {
			return p
		}
	}
	t.Errorf("Could not find parameter %s in schema %s", name, s.longName)
	return parameter{}
}

func topParam(t *testing.T, s *topLevelSchema, name string) parameter {
	for _, p := range s.allParameters() {
		if p.name == name {
			return p
		}
	}
	t.Errorf("Could not find parameter %s in top-level schema", name)
	return parameter{}
}
