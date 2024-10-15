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
	"fmt"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
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
	md.SetRenderer(markdown.NewRenderer())
	extension.GFM.Extend(md)     // GitHub Flavored Markdown
	section.Extension.Extend(md) // AST defined sections
	meta.Extension.Extend(md)    // Support for YAML metadata blocks
	md.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(recognizeHeaderAfterHTML{}, 902),
	))
	md.Renderer().AddOptions(renderer.WithNodeRenderers(
		// The markdown renderer we use does not support rendering out tables.[^1].
		//
		// Without intervention, tables are rendered out as HTML. This works fine
		// for the registry (since it renders HTML). It works poorly for SDK docs,
		// since the HTML content is shown as is.
		//
		// [^1]: https://github.com/teekennedy/goldmark-markdown/issues/19
		util.Prioritized(tableRenderer{md.Renderer()}, 499),
		// The markdown renderer we use does not support rendering raw
		// [ast.String] nodes.[^2] We just render them out as is.
		//
		//nolint:lll
		//
		// [^2]: https://github.com/teekennedy/goldmark-markdown/blob/0cdef017688474073914d6db7e293a028150c0cb/renderer.go#L95-L96
		util.Prioritized(renderType{
			kind: ast.KindString,
			f: func(
				writer util.BufWriter,
				_ []byte, n ast.Node, entering bool,
			) (ast.WalkStatus, error) {
				if !entering {
					return ast.WalkContinue, nil
				}
				_, err := writer.Write(n.(*ast.String).Value)
				return ast.WalkContinue, err
			},
		}, 100),
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

type renderType struct {
	kind ast.NodeKind
	f    renderer.NodeRendererFunc
}

func (t renderType) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(t.kind, t.f)
}

func panicOnRender(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	contract.Failf("The renderer for %s should not have been called", n.Kind())
	return ast.WalkStop, nil
}

var _ renderer.NodeRenderer = (*tableRenderer)(nil)

type tableRenderer struct {
	// A reference the the renderer so that we can render the contents of inner nodes.
	r renderer.Renderer
}

func (t tableRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(extast.KindTable, t.render)
	r.Register(extast.KindTableHeader, panicOnRender)
	r.Register(extast.KindTableRow, panicOnRender)
	r.Register(extast.KindTableCell,
		func(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
			return ast.WalkContinue, nil
		})
}

func (t tableRenderer) render(
	writer util.BufWriter, source []byte, n ast.Node, entering bool,
) (ast.WalkStatus, error) {
	_, err := writer.WriteRune('\n')
	contract.AssertNoErrorf(err, "impossible")
	var inHeader bool
	header := make([]string, 0, len(n.(*extast.Table).Alignments))
	var rows [][]string
	err = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch n := n.(type) {
		case *extast.Table:
			return ast.WalkContinue, nil
		case *extast.TableHeader:
			inHeader = entering
			return ast.WalkContinue, nil
		case *extast.TableRow:
			if entering {
				rows = append(rows, make([]string, 0, len(n.Alignments)))
			}
			return ast.WalkContinue, nil
		case *extast.TableCell:
			if entering {
				var cell bytes.Buffer
				b := ast.NewTextBlock()
				for c := n.FirstChild(); c != nil; c = c.NextSibling() {
					b.AppendChild(b, c)
				}

				err := t.r.Render(&cell, source, b)
				if err != nil {
					return ast.WalkStop, err
				}

				content := strings.TrimSpace(cell.String())

				if inHeader {
					header = append(header, content)
				} else {
					rows[len(rows)-1] = append(rows[len(rows)-1], content)
				}
			}
			return ast.WalkSkipChildren, nil
		default:
			return ast.WalkStop, fmt.Errorf("unexpected node in a table: %s", n.Kind().String())
		}
	})
	table := tablewriter.NewWriter(writer)
	table.SetHeader(header)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.SetAutoFormatHeaders(false)
	table.SetAutoMergeCells(false)
	table.SetAutoWrapText(false)
	table.SetReflowDuringAutoWrap(false)
	table.AppendBulk(rows)
	table.Render()

	return ast.WalkSkipChildren, err
}
