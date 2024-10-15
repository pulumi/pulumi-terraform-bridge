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

// package meta provides a [goldmark] extension that enables reading and writing Markdown
// metadata sections. Metadata sections look like this:
//
//	---
//	# This is now parsed as YAML
//	title: My title
//	description: |
//	  My description is a bit longer
//	---
//
//	## This is markdown again
//
// [meta] adds support for parsing this data, but it doesn't support rendering it back
// out. Since we want to gradually adopt [goldmark] by parsing and then rendering back
// markdown, this is a problem. This package wraps [meta], adding rendering support as
// expected.
package meta

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"gopkg.in/yaml.v2"
)

// The [ast.NodeKind] for a [Meta] [ast.Node].
var Kind = ast.NewNodeKind("Meta")

var Extension goldmark.Extender = metaExtension{}

type Meta struct {
	ast.BaseBlock
	Yaml yaml.MapSlice
}

func (m *Meta) Dump(source []byte, level int) {
	ast.DumpHelper(m, source, level, nil, nil)
}

func (m *Meta) Kind() ast.NodeKind { return Kind }

type metaExtension struct{}

func (m metaExtension) Extend(md goldmark.Markdown) {
	meta.New().Extend(md)
	md.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(m, 1),
	))
	md.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(m, 1),
	))
}

func (metaExtension) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	items := meta.GetItems(pc)
	if items == nil {
		return
	}
	node.InsertBefore(node, node.FirstChild(), &Meta{Yaml: items})
}

func (m metaExtension) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	f := func(b util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			return ast.WalkContinue, nil
		}
		_, err := b.WriteString("---\n")
		if err != nil {
			return ast.WalkContinue, err
		}
		y, err := yaml.Marshal(n.(*Meta).Yaml)
		if err != nil {
			return ast.WalkContinue, err
		}
		_, err = b.Write(y)
		if err != nil {
			return ast.WalkContinue, err
		}
		err = b.WriteByte('\n')
		if err != nil {
			return ast.WalkContinue, err
		}
		_, err = b.WriteString("---\n")
		return ast.WalkContinue, err
	}
	r.Register(Kind, f)
}
