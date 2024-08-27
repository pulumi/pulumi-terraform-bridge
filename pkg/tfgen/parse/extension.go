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

package parse

import (
	"bytes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse/meta"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse/section"
)

// TFRegistryExtension is the catch-all extension to prepare TF registry markdown to be
// parsed.
//
// It should always be used when parsing TF markdown in the bridge.
var TFRegistryExtension goldmark.Extender = tfRegistryExtension{}

type tfRegistryExtension struct{}

func (s tfRegistryExtension) Extend(md goldmark.Markdown) {
	extension.GFM.Extend(md)     // GitHub Flavored Markdown
	section.Extension.Extend(md) // AST defined sections
	meta.Extension.Extend(md)    // Support for YAML metadata blocks
	md.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(recognizeHeaderAfterHTML{}, 902),
	))
}

// recognizeHeaderAfterHTML allows us to work around a difference in how TF's registry parses
// markdown vs goldmark's CommonMark parser.
//
// Goldmark correctly (for CommonMark) parses the following as a single HTML Block:
//
//	<div>
//	 content
//	</div>
//	## Header
//
// This is a common pattern in GCP, and we need to parse it as a HTML block, then a header
// block. This AST transformation makes the desired change.
type recognizeHeaderAfterHTML struct{}

func (recognizeHeaderAfterHTML) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	WalkNode(node, func(node *ast.HTMLBlock) {
		if node.Lines().Len() == 0 {
			return
		}

		last := node.Lines().At(node.Lines().Len() - 1)
		if bytes.HasPrefix(last.Value(reader.Source()), []byte("## ")) {
			node.Lines().SetSliced(0, node.Lines().Len()-1)
			heading := ast.NewHeading(2)
			heading.Lines().Append(last)
			node.Parent().InsertAfter(node.Parent(), node, heading)
		}
	})
}

func WalkNode[T ast.Node](node ast.Node, f func(T)) {
	err := ast.Walk(node, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		n, ok := node.(T)
		if ok && entering {
			f(n)
		}
		return ast.WalkContinue, nil
	})
	contract.AssertNoErrorf(err, "impossible: ast.Walk never returns an error")
}

func RenderMarkdown() renderer.Renderer {
	// [markdown.NewRenderer] does not produce a renderer that can render [ast.String],
	// so we augment the [renderer.Renderer] with that type.
	r := renderMarkdown{markdown.NewRenderer()}
	writeString := func(
		writer util.BufWriter, _ []byte, n ast.Node, entering bool,
	) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		_, err := writer.Write(n.(*ast.String).Value)
		return ast.WalkContinue, err
	}
	r.Register(ast.KindString, writeString)
	return r
}

type renderMarkdown struct{ *markdown.Renderer }
