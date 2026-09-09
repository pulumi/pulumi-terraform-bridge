// Copyright 2016-2026, Pulumi Corporation.
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

package tfgen

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// TestParseImportCode tests importCodePattern against the import-command shapes that appear in
// upstream docs. A line the regex does not match is emitted verbatim, which leaves a
// `terraform import` command on a Pulumi docs page.
func TestParseImportCode(t *testing.T) {
	t.Parallel()

	type expect struct {
		name string
		id   string
	}
	tests := []struct {
		name string
		code string
		// want is nil when the line is expected not to parse as an import example.
		want *expect
	}{
		// --- Shapes the pattern recognizes. ---
		{
			name: "bare terraform import",
			code: "terraform import snowflake_api_integration.example name",
			want: &expect{name: "example", id: "name"},
		},
		{
			name: "dollar prompt",
			code: "$ terraform import snowflake_api_integration.example name",
			want: &expect{name: "example", id: "name"},
		},
		{
			name: "percent prompt",
			code: "% terraform import aws_accessanalyzer_analyzer.example exampleID",
			want: &expect{name: "example", id: "exampleID"},
		},
		{
			name: "pulumi import is recognized too",
			code: "$ pulumi import aws_accessanalyzer_analyzer.example exampleID",
			want: &expect{name: "example", id: "exampleID"},
		},
		{
			name: "leading indentation",
			code: "    terraform import aws_lb.bar my-load-balancer",
			want: &expect{name: "bar", id: "my-load-balancer"},
		},
		{
			name: "trailing whitespace",
			code: "terraform import aws_lb.bar my-load-balancer   ",
			want: &expect{name: "bar", id: "my-load-balancer"},
		},
		{
			name: "line continuations",
			code: "$ terraform import \\\n      some_resource.name \\\n      some-ID",
			want: &expect{name: "name", id: "some-ID"},
		},
		{
			name: "id containing dots and slashes",
			code: "% terraform import aws_lb.bar " +
				"arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/my-load-balancer/50dc6c495c0c9188",
			want: &expect{
				name: "bar",
				id: "arn:aws:elasticloadbalancing:us-west-2:123456789012:" +
					"loadbalancer/app/my-load-balancer/50dc6c495c0c9188",
			},
		},
		{
			name: "single quoted id without spaces keeps its quotes",
			code: "terraform import snowflake_account_grant.example 'accountName|||USAGE|true'",
			want: &expect{name: "example", id: "'accountName|||USAGE|true'"},
		},
		{
			name: "double quoted id without spaces keeps its quotes",
			code: `terraform import auth0_pages.my_pages "22f4f21b-017a-319d-92e7-2291c1ca36c4"`,
			want: &expect{name: "my_pages", id: `"22f4f21b-017a-319d-92e7-2291c1ca36c4"`},
		},
		{
			name: "angle bracket placeholder id",
			code: "$ terraform import some_resource.name <some-ID>",
			want: &expect{name: "name", id: "<some-ID>"},
		},
		{
			name: "curly brace placeholder id",
			code: "terraform import google_project_iam_policy.default {{project_id}}",
			want: &expect{name: "default", id: "{{project_id}}"},
		},
		{
			name: "resource name containing dots",
			code: "terraform import aws_instance.my.instance i-abc123",
			want: &expect{name: "my.instance", id: "i-abc123"},
		},

		// --- Shapes that must keep NOT parsing, so we do not emit a bogus command. ---
		{
			name: "not an import command",
			code: "terraform plan",
		},
		{
			name: "import with no id",
			code: "terraform import aws_lb.bar",
		},
		{
			name: "unquoted id containing spaces stays ambiguous",
			code: "terraform import google_project_iam_member.default {{project_id}} roles/viewer",
		},
		{
			name: "two quoted arguments",
			code: `terraform import google_project_iam_member.default "a" "b"`,
		},
		{
			name: "unterminated quote",
			code: `terraform import google_project_iam_member.default "{{project_id}} roles/viewer`,
		},
		{
			name: "trailing comment after an id",
			code: `terraform import google_project_iam_member.default abc # do this`,
		},
		{
			name: "resource address without a dot",
			code: "terraform import aws_lb my-load-balancer",
		},
		{
			name: "hcl import block",
			code: "import {\n  to = aws_iam_role.example\n  id = \"developer_name\"\n}",
		},
		{
			name: "prose mentioning the command",
			code: "Use terraform import to bring the resource under management",
		},
		{
			name: "empty",
			code: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseImportCode(tt.code)
			if tt.want == nil {
				assert.Falsef(t, ok, "expected %q not to parse, got %+v", tt.code, got)
				return
			}
			require.Truef(t, ok, "expected %q to parse as an import example", tt.code)
			assert.Equal(t, tt.want.name, got.Name, "resource name")
			assert.Equal(t, tt.want.id, got.ID, "import ID")
		})
	}
}

// https://github.com/pulumi/pulumi-terraform-bridge/issues/3585
func TestParseImports_DropsHCLImportBlocksForAllTerraformLanguageIdentifiers(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows - test cases need to be made robust to newline handling")
	}

	for _, fence := range []string{"terraform", "tf", "hcl"} {
		fence := fence
		t.Run(fence, func(t *testing.T) {
			t.Parallel()
			input := strings.Join([]string{
				"",
				"### Importing roles",
				"",
				"An `import` block (Terraform v1.5.0 and later) can be used:",
				"",
				"```" + fence,
				"import {",
				`  id = "developer_name"`,
				"  to = aws_iam_role.example",
				"}",
				"```",
				"",
				"The command can also be used:",
				"",
				"```sh",
				"terraform import aws_iam_role.example developer_name",
				"```",
				"",
			}, "\n")

			parser := tfMarkdownParser{
				info:    &mockResource{token: "aws:iam/role:Role"},
				rawname: "aws_iam_role",
				infoCtx: infoContext{
					pkg:  "aws",
					info: tfbridge.ProviderInfo{Name: "aws"},
				},
			}
			parser.parseImports(input)
			actual := parser.ret.Import

			// The Terraform-only import block is gone, fence and all.
			assert.NotContains(t, actual, "import {")
			assert.NotContains(t, actual, "to = aws_iam_role.example")
			assert.NotContains(t, actual, "```"+fence)

			// Everything around it survives: the heading, the prose, and the shell
			// example, which is still rewritten to a pulumi import command.
			assert.Contains(t, actual, "### Importing roles")
			assert.Contains(t, actual, "The command can also be used:")
			assert.Contains(t, actual, "$ pulumi import aws:iam/role:Role example developer_name")
		})
	}
}

// Upstream embeds import commands in backticked prose as well as in fences. Left alone they
// leak the Terraform CLI and an upstream resource name onto a Pulumi docs page.
func TestParseImports_RewritesImportCommandsInProse(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows - test cases need to be made robust to newline handling")
	}

	for _, tc := range []struct {
		name   string
		line   string
		expect string
	}{
		{
			name:   "backticked command in a sentence",
			line:   "Roles can be imported with `terraform import aws_iam_role.example developer_name`.",
			expect: "Roles can be imported with `pulumi import aws:iam/role:Role example developer_name`.",
		},
		{
			name:   "command inside a note",
			line:   "-> **Note** use `terraform import aws_iam_role.my_role my-role` for existing roles.",
			expect: "-> **Note** use `pulumi import aws:iam/role:Role my_role my-role` for existing roles.",
		},
		{
			name:   "an already-pulumi command is left alone",
			line:   "Import with `pulumi import aws:iam/role:Role example developer_name`.",
			expect: "Import with `pulumi import aws:iam/role:Role example developer_name`.",
		},
		{
			name:   "prose that merely mentions the command is untouched",
			line:   "Use `terraform` to import resources, or run terraform import by hand.",
			expect: "Use `terraform` to import resources, or run terraform import by hand.",
		},
		{
			name:   "a span that is not a command is untouched",
			line:   "The `role_id` field is set on import; see terraform import docs.",
			expect: "The `role_id` field is set on import; see terraform import docs.",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parser := tfMarkdownParser{
				info:    &mockResource{token: "aws:iam/role:Role"},
				rawname: "aws_iam_role",
				infoCtx: infoContext{
					pkg:  "aws",
					info: tfbridge.ProviderInfo{Name: "aws"},
				},
			}
			// A fenced example alongside the prose: rewriteImportMarkdown only reports a
			// rewrite when it finds something to rewrite.
			input := strings.Join([]string{
				"",
				tc.line,
				"",
				"```sh",
				"terraform import aws_iam_role.example developer_name",
				"```",
				"",
			}, "\n")
			parser.parseImports(input)

			assert.Contains(t, parser.ret.Import, tc.expect)
			assert.NotContains(t, parser.ret.Import, "terraform import aws_iam_role")
		})
	}
}
