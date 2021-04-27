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

func nodeFromStr(text string) *bf.Node {
	md := bf.New()
	return md.Parse([]byte(text)).FirstChild
}
