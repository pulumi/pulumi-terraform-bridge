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

package section_test

import (
	"bytes"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse/section"
)

type walkTransformer func(node ast.Node, entering bool) (ast.WalkStatus, error)

func (w walkTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	err := ast.Walk(node, (func(node ast.Node, entering bool) (ast.WalkStatus, error))(w))
	if err != nil {
		panic(err)
	}
}

func TestSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		walk     func(src []byte, node ast.Node, entering bool) (ast.WalkStatus, error)
		expected autogold.Value
	}{
		{
			input: `

Hi

## 1

content *foo*

content

## 2

content (again)
`,
			walk: func(src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
				s, ok := node.(*section.Section)
				if !ok || !entering {
					return ast.WalkContinue, nil
				}
				if string(s.FirstChild().(*ast.Heading).Text(src)) == "1" {
					s.Parent().RemoveChild(s.Parent(), s)
				}
				return ast.WalkContinue, nil
			},
			expected: autogold.Expect(`Hi
## 2

content (again)
`),
		},
		{
			input: `# I am a provider

foo

### Additional Logging
 This section should be skipped`,
			walk: func(src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
				if !entering {
					return ast.WalkContinue, nil
				}
				s, ok := node.(*section.Section)
				if !ok || !entering {
					return ast.WalkContinue, nil
				}

				if string(s.FirstChild().(*ast.Heading).Text(src)) == "Additional Logging" {
					s.Parent().RemoveChild(s.Parent(), s)
				}
				return ast.WalkContinue, nil
			},
			expected: autogold.Expect(`# I am a provider

foo
`),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			src := []byte(tt.input)
			walk := func(node ast.Node, entering bool) (ast.WalkStatus, error) {
				return tt.walk(src, node, entering)
			}
			var b bytes.Buffer
			gm := goldmark.New(
				goldmark.WithExtensions(section.Extension(499)),
				goldmark.WithParserOptions(
					parser.WithASTTransformers(
						util.Prioritized(walkTransformer(walk), 500),
					),
				),
				goldmark.WithRenderer(markdown.NewRenderer()),
			)
			require.NoError(t, gm.Convert(src, &b))
			tt.expected.Equal(t, b.String())
		})
	}
}
