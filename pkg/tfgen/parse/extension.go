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
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extensionast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

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
	extension.GFM.Extend(md)
	section.Extension.Extend(md)
	md.Parser().AddOptions(
		parser.WithASTTransformers(
			util.Prioritized(recognizeHeaderAfterHTML{}, 902),
		))
	md.Renderer().AddOptions(renderer.WithNodeRenderers(
		// The markdown renderer we use does not support rendering out tables.[^1].
		//
		// Without intervention, tables are rendered out as HTML. This works fine
		// for the registry (since it renders HTML). It works poorly for SDK docs,
		// since the HTML content is shown as-is.
		//
		// [^1]: https://github.com/teekennedy/goldmark-markdown/issues/19
		util.Prioritized(tableRenderer{
			renderer:  markdown.NewRenderer(),
			rows:      make([][]string, 0),
			headerRow: make([]string, 0),
		}, 499),
		// The markdown renderer we use does not support rendering raw
		// [ast.String] nodes.[^2] We just render them out as-is.
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

func (renderType renderType) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(renderType.kind, renderType.f)
}

var _ renderer.NodeRenderer = (*tableRenderer)(nil)

type tableRenderer struct {
	renderer    *markdown.Renderer
	headerRow   []string
	rows        [][]string
	tableWriter *tablewriter.Table
	inHeader    bool
	tableWidth  int
}

func (tableRenderer tableRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(extensionast.KindTable, tableRenderer.renderTable)
	r.Register(extensionast.KindTableHeader, tableRenderer.renderHeader)
	r.Register(extensionast.KindTableRow, tableRenderer.renderRow)
	r.Register(extensionast.KindTableCell, tableRenderer.renderCell)
}

func (tableRenderer *tableRenderer) renderTable(
	writer util.BufWriter,
	source []byte,
	n ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if entering {
		tableRenderer.tableWidth = len(n.(*extensionast.Table).Alignments)
		tableRenderer.headerRow = make([]string, 0, tableRenderer.tableWidth)
		tableRenderer.rows = [][]string{}
		tableRenderer.tableWriter = tablewriter.NewWriter(writer)
	} else {
		_, err := writer.WriteRune('\n')
		contract.AssertNoErrorf(err, "impossible")
		tableRenderer.tableWriter.SetHeader(tableRenderer.headerRow)
		tableRenderer.tableWriter.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
		tableRenderer.tableWriter.SetCenterSeparator("|")
		tableRenderer.tableWriter.SetAutoFormatHeaders(false)
		tableRenderer.tableWriter.SetAutoMergeCells(false)
		tableRenderer.tableWriter.SetAutoWrapText(false)
		tableRenderer.tableWriter.SetReflowDuringAutoWrap(false)
		tableRenderer.tableWriter.AppendBulk(tableRenderer.rows)
		tableRenderer.tableWriter.Render()
	}
	return ast.WalkContinue, nil
}

func (tableRenderer *tableRenderer) renderHeader(
	writer util.BufWriter,
	source []byte,
	n ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	tableRenderer.inHeader = entering
	return ast.WalkContinue, nil
}

func (tableRenderer *tableRenderer) renderRow(
	writer util.BufWriter,
	source []byte,
	n ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if entering {
		tableRenderer.rows = append(tableRenderer.rows, make([]string, 0, tableRenderer.tableWidth))
	}

	return ast.WalkContinue, nil
}

func (tableRenderer *tableRenderer) renderCell(
	writer util.BufWriter,
	source []byte,
	n ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if entering {
		var cell bytes.Buffer
		textBlock := ast.NewTextBlock()
		child := n.FirstChild()
		for child != nil {

			next := child.NextSibling()
			textBlock.AppendChild(textBlock, child)
			child = next
		}
		err := (*tableRenderer.renderer).Render(&cell, source, textBlock)
		if err != nil {
			return ast.WalkStop, err
		}
		content := strings.TrimSpace(cell.String())
		if tableRenderer.inHeader {
			tableRenderer.headerRow = append(tableRenderer.headerRow, content)
		} else {
			tableRenderer.rows[len(tableRenderer.rows)-1] = append(tableRenderer.rows[len(tableRenderer.rows)-1], content)
		}
	}
	return ast.WalkSkipChildren, nil
}
