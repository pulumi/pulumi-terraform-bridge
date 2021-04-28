package tfgen

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	bf "github.com/russross/blackfriday/v2"
)

func parseTFMarkdownNew(markdownFileName string, markdownBytes []byte) (entityDocs, error) {
	doc, err := parseMd(string(markdownBytes))
	if err != nil {
		return entityDocs{}, err
	}
	if doc == nil {
		return entityDocs{}, fmt.Errorf("Failed to parse markdown from %s", markdownFileName)
	}
	return doc.toEntityDocs(), nil
}

type doc struct {
	schema         topLevelSchema
	nestedSchemata []nestedSchema
}

func (d *doc) toEntityDocs() entityDocs {
	arguments := make(map[string]*argumentDocs)

	// Do we need to preserve optional/required/readonly info?
	for _, param := range append(append(d.schema.optional, d.schema.required...), d.schema.readonly...) {
		arguments[param.name] = &argumentDocs{
			description: param.desc,
			arguments:   make(map[string]string), // TODO this may require x-refs
			isNested:    false,
		}
	}

	for _, ns := range d.nestedSchemata {
		for _, param := range append(append(ns.optional, ns.required...), ns.readonly...) {
			// what about here? param.name  or longName?
			arguments[param.name] = &argumentDocs{
				description: param.desc,
				arguments:   make(map[string]string), // TODO this may require x-refs
				isNested:    true,
			}
		}
	}

	return entityDocs{
		Description: "TODO",
		Arguments:   arguments,
		Attributes:  make(map[string]string),
		Import:      "TODO",
	}
}

type topLevelSchema struct {
	optional []parameter
	required []parameter
	readonly []parameter
}

type nestedSchema struct {
	longName string
	linkId   *string
	optional []parameter
	required []parameter
	readonly []parameter
}

type parameter struct {
	name     string
	desc     string
	typeDecl string
}

type paramFlags int

const (
	required paramFlags = iota
	optional
	readonly
)

func (pf paramFlags) String() string {
	return [...]string{"required", "optional", "readonly"}[pf]
}

func parseDoc(docNode *bf.Node) (*doc, error) {
	var err error
	result := &doc{}
	nu := &nodeUnlinker{}
	docNode.Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
		if !entering {
			return bf.GoToNext
		}

		// detect top-level Schema section
		var tls *topLevelSchema
		tls, err = parseTopLevelSchema(node, nu.consumeNode)
		if err != nil {
			return bf.Terminate
		}
		if tls != nil {
			result.schema = *tls
			return bf.GoToNext
		}

		// detect nestedSchema sections
		var ns *nestedSchema
		ns, err = parseNestedSchema(node, nu.consumeNode)
		if err != nil {
			panic(err)
		}
		if ns != nil {
			result.nestedSchemata = append(result.nestedSchemata, *ns)
			return bf.GoToNext
		}

		// ignore code blocks
		if node.Type == bf.CodeBlock {
			nu.consumeNode(node)
		}

		return bf.GoToNext
	})
	nu.unlinkAll()
	return result, err
}

func parseTopLevelSchema(node *bf.Node, consumeNode func(node *bf.Node)) (*topLevelSchema, error) {
	if node == nil || node.Type != bf.Heading {
		return nil, nil
	}
	label := node.FirstChild
	if label == nil || label.Type != bf.Text || string(label.Literal) != "Schema" {
		return nil, nil
	}
	tls := &topLevelSchema{}
	curNode := node.Next
	for {
		flags, par, next, err := parseParameterSection(curNode, consumeNode)
		if err != nil {
			return nil, err
		}
		if par != nil {
			switch flags {
			case optional:
				tls.optional = *par
			case required:
				tls.required = *par
			case readonly:
				tls.readonly = *par
			}
			curNode = next
		} else {
			break
		}
	}
	defer consumeNode(node)
	return tls, nil
}

func parseNestedSchema(node *bf.Node, consumeNode func(node *bf.Node)) (*nestedSchema, error) {
	if node.Prev != nil && parsePreamble(node.Prev, func(x *bf.Node) {}) != nil {
		return nil, nil
	}

	linkId := parsePreamble(node, consumeNode)
	if linkId != nil {
		node = node.Next
	}

	if node == nil || node.Type != bf.Heading {
		return nil, nil
	}

	label := node.FirstChild

	if label == nil || label.Type != bf.Text || string(label.Literal) != "Nested Schema for " {
		return nil, nil
	}

	code := label.Next
	if code == nil || code.Type != bf.Code {
		return nil, fmt.Errorf("Expected a Code block, got %s", prettyPrint(code))
	}

	ns := &nestedSchema{
		longName: string(code.Literal),
		linkId:   linkId,
	}

	curNode := node.Next
	for {
		flags, par, next, err := parseParameterSection(curNode, consumeNode)
		if err != nil {
			return nil, err
		}
		if par != nil {
			switch flags {
			case optional:
				ns.optional = *par
			case required:
				ns.required = *par
			case readonly:
				ns.readonly = *par
			}
			curNode = next
		} else {
			break
		}
	}

	defer consumeNode(node)
	return ns, nil
}

var preamblePattern *regexp.Regexp = regexp.MustCompile("^[<]a id=[\"]([^\"]+)[\"][>]$")

func parsePreamble(node *bf.Node, consumeNode func(node *bf.Node)) *string {
	if node == nil || node.Type != bf.Paragraph {
		return nil
	}
	emptyText := node.FirstChild
	if emptyText == nil || emptyText.Type != bf.Text || len(emptyText.Literal) > 0 {
		return nil
	}
	openSpan := emptyText.Next
	if openSpan == nil || openSpan.Type != bf.HTMLSpan {
		return nil
	}
	emptyText2 := openSpan.Next
	if emptyText2 == nil || emptyText2.Type != bf.Text || len(emptyText2.Literal) > 0 {
		return nil
	}
	closeSpan := emptyText2.Next
	if closeSpan == nil || closeSpan.Type != bf.HTMLSpan || string(closeSpan.Literal) != "</a>" {
		return nil
	}
	matches := preamblePattern.FindStringSubmatch(string(openSpan.Literal))
	if len(matches) > 1 {
		defer consumeNode(node)
		return &matches[1]
	}
	return nil
}

func parseParameterSection(node *bf.Node, consumeNode func(node *bf.Node)) (paramFlags, *[]parameter, *bf.Node, error) {
	if node != nil && (node.Type == bf.Paragraph || node.Type == bf.Heading) {
		secLabel := node.FirstChild
		if secLabel != nil && secLabel.Type == bf.Text && secLabel.Next == nil {
			flags := parseParamFlagLiteral(string(secLabel.Literal))
			if flags == nil {
				return -1, nil, nil, nil
			}
			ps, err := parseParameterList(node.Next, consumeNode)
			if err != nil {
				return -1, nil, nil, err
			}
			if ps == nil {
				return -1, nil, nil, fmt.Errorf("Expected a parameter list, got %s", prettyPrint(node.Next))
			}
			defer consumeNode(node)
			return *flags, ps, node.Next.Next, nil
		}
	}
	return -1, nil, nil, nil
}

var optionalPattern *regexp.Regexp = regexp.MustCompile("(?i)^optional[:]?$")
var requiredPattern *regexp.Regexp = regexp.MustCompile("(?i)^required[:]?$")
var readonlyPattern *regexp.Regexp = regexp.MustCompile("(?i)^read-only[:]?$")

func parseParamFlagLiteral(text string) *paramFlags {
	if optionalPattern.MatchString(text) {
		return paramFlagsPtr(optional)
	}
	if requiredPattern.MatchString(text) {
		return paramFlagsPtr(required)
	}
	if readonlyPattern.MatchString(text) {
		return paramFlagsPtr(readonly)
	}
	return nil
}

func paramFlagsPtr(flags paramFlags) *paramFlags {
	result := new(paramFlags)
	*result = flags
	return result
}

func parseParameterList(node *bf.Node, consumeNode func(node *bf.Node)) (*[]parameter, error) {
	var out []parameter
	if node == nil || node.Type != bf.List {
		return nil, nil
	}
	item := node.FirstChild
	for item != nil {
		if item.Type != bf.Item {
			return nil, fmt.Errorf("expected an Item")
		}
		param, err := parseParameter(item)
		if err != nil {
			return nil, err
		}
		if param == nil {
			return nil, fmt.Errorf("expected a parameter, got %v", prettyPrint(item))
		}
		out = append(out, *param)
		item = item.Next
	}
	defer consumeNode(node)
	return &out, nil
}

func parseParameter(node *bf.Node) (*parameter, error) {
	if node == nil || node.Type != bf.Item {
		return nil, nil
	}
	para := node.FirstChild
	if para == nil || para.Type != bf.Paragraph || para.Next != nil {
		return nil, nil
	}
	emptyText := para.FirstChild
	if emptyText == nil || emptyText.Type != bf.Text || len(emptyText.Literal) > 0 {
		return nil, nil
	}
	strong := emptyText.Next
	if strong == nil || strong.Type != bf.Strong {
		return nil, nil
	}
	paramName, err := parseTextSeq(strong.FirstChild, bf.Text, bf.Emph)
	if err != nil {
		return nil, err
	}

	// TODO parse (see .. links), consider back-quoting `code` literals back.

	paramDesc, err := parseTextSeq(strong.Next, bf.Text, bf.Code, bf.Link, bf.Strong, bf.Emph)
	if err != nil {
		return nil, err
	}
	return parseParameterFromDescription(paramName, paramDesc), nil
}

var descriptionTypeSectionPattern *regexp.Regexp = regexp.MustCompile("^\\s*[(]([^[)]+)[)]\\s+")

func parseParameterFromDescription(name string, description string) *parameter {
	if descriptionTypeSectionPattern.MatchString(description) {
		typeDecl := descriptionTypeSectionPattern.FindStringSubmatch(description)[1]
		description = descriptionTypeSectionPattern.ReplaceAllString(description, "")

		return &parameter{
			name:     name,
			desc:     description,
			typeDecl: typeDecl,
		}
	}
	return &parameter{
		name: name,
		desc: description,
	}
}

func parseTextSeq(node *bf.Node, allowTags ...bf.NodeType) (string, error) {
	var sb strings.Builder
	for node != nil {
		allow := false
		for _, t := range allowTags {
			if node.Type == t {
				allow = true
			}
		}
		if !allow {
			return "", fmt.Errorf("parseTextSeq found a tag that is not allowed: %s",
				prettyPrint(node))
		}
		sb.WriteString(string(node.Literal))
		node = node.Next
	}
	return sb.String(), nil
}

type nodeUnlinker struct {
	nodes []*bf.Node
}

func (nu *nodeUnlinker) consumeNode(node *bf.Node) {
	nu.nodes = append(nu.nodes, node)
}

func (nu *nodeUnlinker) unlinkAll() {
	for _, n := range nu.nodes {
		if n != nil {
			n.Unlink()
		}
	}
}

func parseMd(text string) (*doc, error) {
	mdProc := bf.New(bf.WithExtensions(bf.FencedCode))
	docNode := mdProc.Parse([]byte(text))

	doc, err := parseDoc(docNode)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Parsed schema: %v\n", doc.schema)
	fmt.Printf("Parsed nested: %d\n", len(doc.nestedSchemata))
	fmt.Printf("Unparsed: %d\n", len(prettyPrint(docNode)))

	return doc, nil
}

// Used for debugging blackfriday parse trees by visualizing them.
func prettyPrint(n *bf.Node) string {
	bytes, err := json.MarshalIndent(treeify(n), "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

// Used in prettyPrint.
func treeify(n *bf.Node) interface{} {
	if n == nil {
		return "nil"
	}
	if n.Type == bf.Text {
		return string(n.Literal)
	}
	var result []interface{}
	result = append(result, fmt.Sprintf("[%s]", n.Type))

	c := n.FirstChild
	for c != nil {
		result = append(result, treeify(c))
		c = c.Next
	}

	if n.Literal != nil {
		result = append(result, string(n.Literal))
	}
	return result
}
