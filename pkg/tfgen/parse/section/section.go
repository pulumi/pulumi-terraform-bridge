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

package section

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var _ goldmark.Extender = section{}

func Extension(priority int) goldmark.Extender {
	return section{priority}
}

var Kind = ast.NewNodeKind("Section")

type section struct{ priority int }

func (s section) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(sectionParser{}, s.priority),
	))

	md.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(sectionRenderer{}, s.priority),
	))
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
	f := func(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
		return ast.WalkContinue, nil
	}
	r.Register(Kind, f)

}
