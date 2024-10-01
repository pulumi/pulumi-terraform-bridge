// Copyright 2016-2024, Pulumi Corporation.
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

// package section provides an extension to goldmark: a section.
//
// A section is a header and any content it includes (nesting). Sections render to their
// contents, serving only as markers on [goldmark]'s parse tree.
package section

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var Extension goldmark.Extender = section{}

const priority = 901

var Kind = ast.NewNodeKind("Section")

type section struct{}

func (section) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(sectionParser{}, priority),
	))

	md.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(sectionRenderer{}, priority),
	))
}

func New(heading *ast.Heading) *Section {
	s := new(Section)
	s.AppendChild(s, heading)
	return s
}

type Section struct{ ast.BaseBlock }

func (s *Section) Heading() *ast.Heading {
	return s.FirstChild().(*ast.Heading)
}

func (s *Section) Dump(source []byte, level int) {
	ast.DumpHelper(s, source, level, nil, nil)
}

func (s *Section) Kind() ast.NodeKind { return Kind }

func (s sectionParser) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	s.transform(node, reader, pc, false)
}

func (s sectionParser) transform(node ast.Node, reader text.Reader, pc parser.Context, skipFirst bool) {
	parent := node
	node = node.FirstChild()
	if skipFirst {
		node = node.NextSibling()
	}
	for node != nil {
		heading, ok := node.(*ast.Heading)
		if !ok {
			node = node.NextSibling()
			continue
		}
		node = heading.NextSibling()

		section := &Section{}
		section.SetBlankPreviousLines(heading.HasBlankPreviousLines())
		heading.SetBlankPreviousLines(false)

		parent.ReplaceChild(parent, heading, section)
		section.AppendChild(section, heading)
		for node != nil {
			if child, ok := node.(*ast.Heading); ok && child.Level <= heading.Level {
				break
			}
			child := node
			// We are going to add c to section
			node = node.NextSibling()
			section.AppendChild(section, child)
		}
		s.transform(section, reader, pc, true)

	}
}

type sectionParser struct{}

type sectionRenderer struct{}

func (sectionRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	f := func(b util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.PreviousSibling() != nil {
			_, err := b.WriteRune('\n')
			if err != nil {
				return ast.WalkContinue, err
			}
		}
		return ast.WalkContinue, nil
	}
	r.Register(Kind, f)

}
