package tfgen

import (
	"testing"

	bf "github.com/russross/blackfriday/v2"
)

func TestParsePreamble(t *testing.T) {
	node := nodeFromStr(`<a id="nestedblock--widget--group_definition--widget--id--request"></a>`)
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

func nodeFromStr(text string) *bf.Node {
	md := bf.New()
	return md.Parse([]byte(text)).FirstChild
}
