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
	"github.com/olekukonko/tablewriter"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse/section"
	"github.com/yuin/goldmark/extension"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extensionast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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
	r := markdown.NewRenderer()
	md.Renderer().AddOptions(renderer.WithNodeRenderers(
		// The markdown renderer we use does not support rendering out tables.[^1].
		//
		// Without intervention, tables are rendered out as HTML. This works fine
		// for the registry (since it renders HTML). It works poorly for SDK docs,
		// since the HTML content is shown as-is.
		//
		// [^1]: https://github.com/teekennedy/goldmark-markdown/issues/19
		util.Prioritized(tableRenderer{
			r:      r,
			rows:   make([][]string, 0),
			header: make([]string, 0),
		}, 499),
		//// The markdown renderer we use does not support rendering raw
		//// [ast.String] nodes.[^2] We just render them out as-is.
		////
		////nolint:lll
		////
		//// [^2]: https://github.com/teekennedy/goldmark-markdown/blob/0cdef017688474073914d6db7e293a028150c0cb/renderer.go#L95-L96
		//util.Prioritized(renderType{
		//	kind: ast.KindString,
		//	f: func(
		//		writer util.BufWriter,
		//		_ []byte, n ast.Node, entering bool,
		//	) (ast.WalkStatus, error) {
		//		if !entering {
		//			return ast.WalkContinue, nil
		//		}
		//		_, err := writer.Write(n.(*ast.String).Value)
		//		return ast.WalkContinue, err
		//	},
		//}, 100),
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

//type renderType struct {
//	kind ast.NodeKind
//	f    renderer.NodeRendererFunc
//}
//
//func (renderType renderType) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
//	r.Register(renderType.kind, renderType.f)
//}

func panicOnRender(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	contract.Failf("The renderer for %s should not have been called", n.Kind())
	return ast.WalkStop, nil
}

var _ renderer.NodeRenderer = (*tableRenderer)(nil)

type tableRenderer struct {
	r           *markdown.Renderer
	header      []string
	rows        [][]string
	tableWriter *tablewriter.Table
	inHeader    bool
	alignments  int
}

func (tableRenderer tableRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(extensionast.KindTable, tableRenderer.renderTable)
	r.Register(extensionast.KindTableHeader, tableRenderer.renderHeader)
	r.Register(extensionast.KindTableRow, tableRenderer.renderRow)
	r.Register(extensionast.KindTableCell, tableRenderer.renderCell)

	//r.Register(extensionast.KindTable, tableRenderer.render)
	//r.Register(extensionast.KindTableHeader, func(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	//	return ast.WalkContinue, nil
	//})
	//r.Register(extensionast.KindTableRow, func(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	//	return ast.WalkContinue, nil
	//})
	//r.Register(extensionast.KindTableCell,
	//	func(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	//		return ast.WalkContinue, nil
	//	})
}

func (tableRenderer *tableRenderer) renderTable(
	writer util.BufWriter,
	source []byte,
	n ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if entering {
		tableRenderer.alignments = len(n.(*extensionast.Table).Alignments)
		tableRenderer.header = make([]string, 0, tableRenderer.alignments)
		tableRenderer.rows = [][]string{}
		tableRenderer.tableWriter = tablewriter.NewWriter(writer)

	} else {
		tableRenderer.tableWriter.SetHeader(tableRenderer.header)
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
		tableRenderer.rows = append(tableRenderer.rows, make([]string, 0, tableRenderer.alignments)) //TODO: call this table width
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
		for {
			if child == nil {
				break
			}
			next := child.NextSibling()
			textBlock.AppendChild(textBlock, child)
			child = next
		}
		err := (*tableRenderer.r).Render(&cell, source, textBlock)
		if err != nil {
			return ast.WalkStop, err
		}
		content := strings.TrimSpace(cell.String())
		//content := string(textBlock.Text(source))
		//(*tableRenderer.r).

		if tableRenderer.inHeader {
			tableRenderer.header = append(tableRenderer.header, content)
		} else {
			tableRenderer.rows[len(tableRenderer.rows)-1] = append(tableRenderer.rows[len(tableRenderer.rows)-1], content)
		}
	}
	return ast.WalkSkipChildren, nil
}

func (tableRenderer tableRenderer) render(
	writer util.BufWriter, source []byte, n ast.Node, entering bool,
) (ast.WalkStatus, error) {
	//fmt.Println("before++++++++++++++++++++++++")
	//
	//n.Dump(source, 0)
	//fmt.Println("before end++++++++++++++++++++++++")

	if !entering {
		return ast.WalkContinue, nil
	}

	_, err := writer.WriteRune('\n') // this is so that we have a newline between markdown elements.
	contract.AssertNoErrorf(err, "impossible")
	var inHeader bool
	header := make([]string, 0, len(n.(*extensionast.Table).Alignments))
	var rows [][]string
	err = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch n := n.(type) {
		case *extensionast.Table:
			//tableRow := ast.NewTextBlock()
			//err = tableRenderer.Render(writer, source, tableRow)
			return ast.WalkContinue, nil
		case *extensionast.TableHeader:
			//tableRow := ast.NewTextBlock()
			//err = tableRenderer.Render(writer, source, tableRow)
			inHeader = entering
			return ast.WalkContinue, nil
		case *extensionast.TableRow:
			//tableRow := ast.NewTextBlock()
			if entering {
				//err = tableRenderer.Render(writer, source, tableRow)
				rows = append(rows, make([]string, 0, len(n.Alignments)))
			}
			return ast.WalkContinue, nil
		case *extensionast.TableCell:
			if entering {
				var cell bytes.Buffer
				textBlock := ast.NewTextBlock()
				child := n.FirstChild()
				for {
					if child == nil {
						break
					}
					next := child.NextSibling()
					textBlock.AppendChild(textBlock, child)
					child = next
				}
				err := (*tableRenderer.r).Render(&cell, source, textBlock)
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
			//tableRenderer.render(writer, source, n, entering)
			//err := tableRenderer.r.Render(writer, source, n)
			//if err != nil {
			//	panic("wtf")
			//}
			return ast.WalkContinue, nil
		default:
			return ast.WalkStop, fmt.Errorf("unexpected node in a table: %s", n.Kind())
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
	fmt.Println("after++++++++++++++++++++++++")
	n.Dump(source, 0)
	fmt.Println("++++++++++++++++++++++++")
	//err = tableRenderer.Render(writer, source, n)
	if err != nil {
		panic("wtf")
	}
	return ast.WalkSkipChildren, err
}

//
//func (tableRenderer tableRenderer) renderBlockSeparator(node ast.Node, entering bool) ast.WalkStatus {
//
//	tableRenderer.rc = newRenderContext(w, source, r.config)if entering {
//		// Add blank previous line if applicable
//		if node.PreviousSibling() != nil && node.HasBlankPreviousLines() {
//			tableRenderer.rc.writer.EndLine()
//			tableRenderer.Renderer.
//		}
//	} else {
//		// Flush line buffer to complete line written by previous block
//		r.rc.writer.FlushLine()
//	}
//	return ast.WalkContinue
//}
