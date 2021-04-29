package tfgen

import (
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

// TODO work to remove (see below) automatically.
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

func TestParseNestedSections(t *testing.T) {
	bytes, err := ioutil.ReadFile("test_data/mini.md")
	if err != nil {
		t.Fatal(err)
	}
	markdown := string(bytes)

	schemata := make(map[string]*nestedSchema)

	parseDoc(markdown).Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
		if entering {
			nested, err := parseNestedSchema(node, nil)
			if err != nil {
				t.Fatal(err)
			}
			if nested != nil {
				schemata[nested.longName] = nested
			}
		}
		return bf.GoToNext
	})

	assert.Equal(t, 6, len(schemata))

	assert.Equal(t, "The layout type of the group, only 'ordered' for now.",
		param(t, schemata["widget.group_definition"], "layout_type").desc)

	assert.Equal(t, "The definition for a Group widget.",
		param(t, schemata["widget"], "group_definition").desc)

	assert.Equal(t, "",
		param(t, schemata["widget.group_definition"], "title").desc)
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
