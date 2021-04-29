// Copyright 2016-2021, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Parses Markdown Terraform documentation.
//
// Unlike `docs.go` that is string/regex based, uses a markdown
// (blackfriday) and AST traversals to extract the docs.
//
// At the moment we still rely on `docs.go` for most of our parsing
// needs. Only `Nested Schema` sections are parsed within this module.
//
// Example document in the new markdown format that relies on `Nested
// Schema` sections:
//
// https://raw.githubusercontent.com/DataDog/terraform-provider-datadog/v2.25.0/docs/resources/dashboard.md

package tfgen

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	bf "github.com/russross/blackfriday/v2"
)

type topLevelSchema struct {
	optional       []parameter
	required       []parameter
	readonly       []parameter
	nestedSchemata []nestedSchema
}

func (ns *topLevelSchema) allParameters() []parameter {
	return append(append(ns.optional, ns.required...), ns.readonly...)
}

type nestedSchema struct {
	longName string
	linkId   *string
	optional []parameter
	required []parameter
	readonly []parameter
}

func (ns *nestedSchema) allParameters() []parameter {
	return append(append(ns.optional, ns.required...), ns.readonly...)
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

func parseTopLevelSchemaIntoDocs(
	accumulatedDocs *entityDocs,
	topLevelSchema *topLevelSchema,
	warn func(fmt string, arg ...interface{})) {
	for _, param := range topLevelSchema.allParameters() {
		args, created := accumulatedDocs.getOrCreateArgumentDocs(param.name)
		if !created && args.description != param.desc {
			warn("Descripton conflict for top-level param %s; candidates are `%s` and `%s`",
				param.name,
				args.description,
				param.desc)
		}
		args.description = param.desc
		args.isNested = false
	}

	for _, ns := range topLevelSchema.nestedSchemata {
		parseNestedSchemaIntoDocs(accumulatedDocs, &ns, warn)
	}
}

func parseTopLevelSchema(node *bf.Node, consumeNode func(node *bf.Node)) (*topLevelSchema, error) {
	if consumeNode == nil {
		consumeNode = func(node *bf.Node) {}
	}
	if node == nil || node.Type != bf.Heading {
		return nil, nil
	}
	label := node.FirstChild
	if label == nil || label.Type != bf.Text || string(label.Literal) != "Schema" {
		return nil, nil
	}
	tls := &topLevelSchema{}
	curNode := node.Next
	for curNode != nil {
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

	var nested []nestedSchema
	curNode = node.Next
	for curNode != nil {
		nestedSchema, err := parseNestedSchema(curNode, consumeNode)
		if err != nil {
			return nil, err
		}
		if nestedSchema != nil {
			nested = append(nested, *nestedSchema)
		}
		curNode = curNode.Next
	}

	tls.nestedSchemata = nested

	consumeNode(node)
	return tls, nil
}

func parseNestedSchemaIntoDocs(
	accumulatedDocs *entityDocs,
	nestedSchema *nestedSchema,
	warn func(fmt string, arg ...interface{})) {

	args, _ := accumulatedDocs.getOrCreateArgumentDocs(nestedSchema.longName)
	args.isNested = true

	for _, param := range nestedSchema.allParameters() {
		oldDesc, hasAlready := args.arguments[param.name]
		if hasAlready && oldDesc != param.desc {
			warn("Descripton conflict for param %s from %s; candidates are `%s` and `%s`",
				param.name,
				nestedSchema.longName,
				oldDesc,
				param.desc)
		}
		args.arguments[param.name] = param.desc
		fullParamName := fmt.Sprintf("%s.%s", nestedSchema.longName, param.name)
		paramArgs, created := accumulatedDocs.getOrCreateArgumentDocs(fullParamName)
		if !created && paramArgs.description != param.desc {
			warn("Descripton conflict for param %s; candidates are `%s` and `%s`",
				fullParamName,
				paramArgs.description,
				param.desc)
		}
		paramArgs.isNested = true
		paramArgs.description = param.desc
	}
}

func parseNestedSchema(node *bf.Node, consumeNode func(node *bf.Node)) (*nestedSchema, error) {
	if consumeNode == nil {
		consumeNode = func(node *bf.Node) {}
	}

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
	paramName, err := parseTextSeq(strong.FirstChild)
	if err != nil {
		return nil, err
	}
	paramDesc, err := parseTextSeq(strong.Next)
	if err != nil {
		return nil, err
	}
	return parseParameterFromDescription(paramName, cleanDesc(paramDesc)), nil
}

var seeBelowPattern *regexp.Regexp = regexp.MustCompile("[(]see \\[below for nested schema\\][(][^)]*[)][)]")

func cleanDesc(desc string) string {
	desc = seeBelowPattern.ReplaceAllString(desc, "")
	return strings.TrimSpace(desc)
}

var descriptionTypeSectionPattern *regexp.Regexp = regexp.MustCompile("^\\s*[(]([^[)]+)[)]\\s*")

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

// Unfortunately blackfriday does not include markdown renderer, or
// allow accssing source locations of nodes. Here we accept a sequence of
// inline nodes and render them as markdown text back.
func parseTextSeq(firstNode *bf.Node) (string, error) {
	var err error
	buffer := strings.Builder{}
	curNode := firstNode
	for curNode != nil {
		curNode.Walk(func(node *bf.Node, entering bool) bf.WalkStatus {
			switch node.Type {
			case bf.Text:
				buffer.WriteString(string(node.Literal))
			case bf.Code:
				buffer.WriteString("`")
				buffer.WriteString(string(node.Literal))
				buffer.WriteString("`")
			case bf.Link:
				if entering {
					buffer.WriteString("[")
				} else {
					buffer.WriteString("](")
					buffer.WriteString(string(node.Destination))
					buffer.WriteString(")")
				}
			case bf.Strong:
				buffer.WriteString("**")
			case bf.Emph:
				buffer.WriteString("*")
			default:
				err = fmt.Errorf("parseTextSeq found a tag it cannot yet render back to Markdown: %s",
					prettyPrint(node))
				return bf.Terminate
			}
			return bf.GoToNext
		})
		curNode = curNode.Next
	}
	return buffer.String(), err
}

// Useful to remember and remove nodes that were consumed
// (successfully parsed) from the AST, for example to debug which
// parts of the AST we fail to recognize.
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

func parseDoc(text string) *bf.Node {
	mdProc := bf.New(bf.WithExtensions(bf.FencedCode))
	return mdProc.Parse([]byte(text))
}

func parseNode(text string) *bf.Node {
	return parseDoc(text).FirstChild
}

// Used for debugging blackfriday parse trees by visualizing them.
func prettyPrint(n *bf.Node) string {
	if n == nil {
		return "nil"
	}
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
