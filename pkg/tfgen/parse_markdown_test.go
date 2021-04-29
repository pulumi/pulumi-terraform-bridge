package tfgen

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	bf "github.com/russross/blackfriday/v2"
	"github.com/stretchr/testify/assert"
)

func TestParseTextSeq(t *testing.T) {
	res, err := parseTextSeq(parseNode(`(Block List, Max: 1) The definition for a Change  widget. (see [below for nested schema](#nestedblock--widget--group_definition--widget--change_definition))`).FirstChild,
		bf.Text, bf.Link)
	if err != nil {
		t.Fatal(err)
	}

	// link nodes get erased on purpose:
	assert.Equal(t, "(Block List, Max: 1) The definition for a Change  widget. (see )", res)
}

func TestParsePreamble(t *testing.T) {
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
	markdown := readTestFile(t, "mini.md")
	var schema *topLevelSchema

	parseDoc(markdown).Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
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
}

func TestParseNestedSchemaIntoDoc(t *testing.T) {
	markdown := readTestFile(t, "mini.md")
	out := &entityDocs{}
	parseDoc(markdown).Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
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
	actual, err := json.MarshalIndent(out.Arguments, "  ", "")
	if err != nil {
		t.Fatal(err)
	}
	assert.JSONEq(t, readTestFile(t, "mini.json"), string(actual))
}

func readTestFile(t *testing.T, name string) string {
	bytes, err := ioutil.ReadFile(fmt.Sprintf("test_data/%s", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}

func nested(t *testing.T, tls *topLevelSchema, name string) *nestedSchema {
	for _, s := range tls.nestedSchemata {
		if s.longName == name {
			return &s
		}
	}
	t.Errorf("Could not find nested schema %s", name)
	return nil
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
