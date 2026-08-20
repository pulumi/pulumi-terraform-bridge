// Copyright 2016-2025, Pulumi Corporation.
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

// TestParseImportCode pins down importCodePattern. The import section of a bridged provider's
// docs is only as good as this regex: a line it fails to recognize is emitted verbatim, which
// leaves a `terraform import` command on a Pulumi docs page.
func TestParseImportCode(t *testing.T) {
	t.Parallel()

	type expect struct {
		typ  string
		name string
		id   string
	}
	tests := []struct {
		name string
		code string
		// want is nil when the line is expected not to parse as an import example.
		want *expect
	}{
		// --- Shapes that already worked, pinned against regression. ---
		{
			name: "bare terraform import",
			code: "terraform import snowflake_api_integration.example name",
			want: &expect{typ: "snowflake_api_integration", name: "example", id: "name"},
		},
		{
			name: "dollar prompt",
			code: "$ terraform import snowflake_api_integration.example name",
			want: &expect{typ: "snowflake_api_integration", name: "example", id: "name"},
		},
		{
			name: "percent prompt",
			code: "% terraform import aws_accessanalyzer_analyzer.example exampleID",
			want: &expect{typ: "aws_accessanalyzer_analyzer", name: "example", id: "exampleID"},
		},
		{
			name: "pulumi import is recognized too",
			code: "$ pulumi import aws_accessanalyzer_analyzer.example exampleID",
			want: &expect{typ: "aws_accessanalyzer_analyzer", name: "example", id: "exampleID"},
		},
		{
			name: "leading indentation",
			code: "    terraform import aws_lb.bar my-load-balancer",
			want: &expect{typ: "aws_lb", name: "bar", id: "my-load-balancer"},
		},
		{
			name: "trailing whitespace",
			code: "terraform import aws_lb.bar my-load-balancer   ",
			want: &expect{typ: "aws_lb", name: "bar", id: "my-load-balancer"},
		},
		{
			name: "line continuations",
			code: "$ terraform import \\\n      some_resource.name \\\n      some-ID",
			want: &expect{typ: "some_resource", name: "name", id: "some-ID"},
		},
		{
			name: "id containing dots and slashes",
			code: "% terraform import aws_lb.bar " +
				"arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/app/my-load-balancer/50dc6c495c0c9188",
			want: &expect{
				typ:  "aws_lb",
				name: "bar",
				id: "arn:aws:elasticloadbalancing:us-west-2:123456789012:" +
					"loadbalancer/app/my-load-balancer/50dc6c495c0c9188",
			},
		},
		{
			name: "single quoted id without spaces keeps its quotes",
			code: "terraform import snowflake_account_grant.example 'accountName|||USAGE|true'",
			want: &expect{typ: "snowflake_account_grant", name: "example", id: "'accountName|||USAGE|true'"},
		},
		{
			name: "double quoted id without spaces keeps its quotes",
			code: `terraform import auth0_pages.my_pages "22f4f21b-017a-319d-92e7-2291c1ca36c4"`,
			want: &expect{
				typ:  "auth0_pages",
				name: "my_pages",
				id:   `"22f4f21b-017a-319d-92e7-2291c1ca36c4"`,
			},
		},
		{
			name: "angle bracket placeholder id",
			code: "$ terraform import some_resource.name <some-ID>",
			want: &expect{typ: "some_resource", name: "name", id: "<some-ID>"},
		},
		{
			name: "curly brace placeholder id",
			code: "terraform import google_project_iam_policy.default {{project_id}}",
			want: &expect{typ: "google_project_iam_policy", name: "default", id: "{{project_id}}"},
		},
		{
			name: "resource name containing dots",
			code: "terraform import aws_instance.my.instance i-abc123",
			want: &expect{typ: "aws_instance", name: "my.instance", id: "i-abc123"},
		},

		// --- Space-delimited, quoted IDs: the bug in #3584. ---
		{
			name: "double quoted id with spaces (IAM member)",
			code: `terraform import google_project_iam_member.default "{{project_id}} roles/viewer user:foo@example.com"`,
			want: &expect{
				typ:  "google_project_iam_member",
				name: "default",
				id:   `"{{project_id}} roles/viewer user:foo@example.com"`,
			},
		},
		{
			name: "double quoted id with spaces (IAM binding)",
			code: `terraform import google_project_iam_binding.default "{{project_id}} roles/viewer"`,
			want: &expect{
				typ:  "google_project_iam_binding",
				name: "default",
				id:   `"{{project_id}} roles/viewer"`,
			},
		},
		{
			name: "double quoted id with spaces and a prompt",
			code: `$ terraform import google_storage_bucket_iam_member.editor ` +
				`"b/{{bucket}} roles/storage.objectViewer user:jane@example.com"`,
			want: &expect{
				typ:  "google_storage_bucket_iam_member",
				name: "editor",
				id:   `"b/{{bucket}} roles/storage.objectViewer user:jane@example.com"`,
			},
		},
		{
			name: "single quoted id with spaces",
			code: `terraform import google_project_iam_binding.default '{{project_id}} roles/viewer'`,
			want: &expect{
				typ:  "google_project_iam_binding",
				name: "default",
				id:   `'{{project_id}} roles/viewer'`,
			},
		},
		{
			name: "quoted id with spaces across line continuations",
			code: "$ terraform import \\\n    google_project_iam_member.default \\\n" +
				`    "{{project_id}} roles/viewer user:foo@example.com"`,
			want: &expect{
				typ:  "google_project_iam_member",
				name: "default",
				id:   `"{{project_id}} roles/viewer user:foo@example.com"`,
			},
		},
		{
			name: "quoted id with trailing whitespace",
			code: `terraform import google_project_iam_binding.default "{{project_id}} roles/viewer"  `,
			want: &expect{
				typ:  "google_project_iam_binding",
				name: "default",
				id:   `"{{project_id}} roles/viewer"`,
			},
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
			name: "trailing comment after a quoted id",
			code: `terraform import google_project_iam_member.default "a b" # do this`,
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
			assert.Equal(t, tt.want.typ, got.Type, "terraform resource type")
			assert.Equal(t, tt.want.name, got.Name, "resource name")
			assert.Equal(t, tt.want.id, got.ID, "import ID")
		})
	}
}

// gcpProjectIAMResources is the subset of pulumi-gcp's resource map that the upstream
// google_project_iam docs page covers. Upstream documents the whole IAM family on one page,
// so generating any one of these resources has to reach for its siblings' tokens.
var gcpProjectIAMResources = map[string]*tfbridge.ResourceInfo{
	"google_project_iam_member":       {Tok: "gcp:projects/iAMMember:IAMMember"},
	"google_project_iam_binding":      {Tok: "gcp:projects/iAMBinding:IAMBinding"},
	"google_project_iam_policy":       {Tok: "gcp:projects/iAMPolicy:IAMPolicy"},
	"google_project_iam_audit_config": {Tok: "gcp:projects/iAMAuditConfig:IAMAuditConfig"},
}

// TestParseImports_SpaceDelimitedIDs covers https://github.com/pulumi/pulumi-terraform-bridge/issues/3584
// against the real upstream google_project_iam docs page.
func TestParseImports_SpaceDelimitedIDs(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows - test case needs to be made robust to newline handling")
	}

	input := readfile(t, "test_data/parse-imports/gcp-project-iam.md")

	parser := tfMarkdownParser{
		info:    &mockResource{token: "gcp:projects/iAMMember:IAMMember"},
		rawname: "google_project_iam_member",
		infoCtx: infoContext{
			pkg:      "gcp",
			language: "nodejs",
			info: tfbridge.ProviderInfo{
				Name:      "gcp",
				Resources: gcpProjectIAMResources,
			},
		},
	}
	parser.parseImports(input)
	actual := parser.ret.Import

	// Every example on the page is rewritten, each stamped with the token of the resource
	// it actually imports rather than the token of the page being generated.
	for _, want := range []string{
		"$ pulumi import gcp:projects/iAMMember:IAMMember default " +
			`"{{project_id}} roles/viewer user:foo@example.com"`,
		`$ pulumi import gcp:projects/iAMBinding:IAMBinding default "{{project_id}} roles/viewer"`,
		"$ pulumi import gcp:projects/iAMPolicy:IAMPolicy default {{project_id}}",
		"$ pulumi import gcp:projects/iAMAuditConfig:IAMAuditConfig default " +
			`"{{project_id}} foo.googleapis.com"`,
	} {
		assert.Containsf(t, actual, want, "expected import example for %q", want)
	}

	// The `terraform import` example upstream embeds in prose, outside any code fence, is
	// rewritten as well.
	assert.Contains(t, actual,
		"`pulumi import gcp:projects/iAMBinding:IAMBinding my_project "+
			`"{{your-project-id}} roles/{{role_id}} condition-title"`+"`")

	// No import command may refer the reader to the Terraform CLI, and none may name an
	// upstream resource instead of a Pulumi token.
	assert.NotContains(t, actual, "terraform import")

	// Nor may it carry Terraform-only `import { ... }` blocks; see issue #3585.
	assert.NotContains(t, actual, "import {")
	assert.NotContains(t, actual, "```tf")

	for _, line := range strings.Split(actual, "\n") {
		if !strings.Contains(line, "pulumi import") {
			continue
		}
		assert.NotContainsf(t, line, "google_", "import command names an upstream resource")
	}
}

// TestParseImports_DropsHCLImportBlocks covers
// https://github.com/pulumi/pulumi-terraform-bridge/issues/3585.
//
// A Terraform `import { ... }` block is Terraform-only config syntax with no Pulumi analogue,
// so it has to be dropped from the Import section. Upstream tags these fences ```tf far more
// often than ```terraform, and leaving one in does more than render stray HCL: isHCL treats
// tf/hcl as convertible, the conversion cannot succeed, and convertExamples then strips the
// entire enclosing subsection - heading, prose and sibling examples included.
func TestParseImports_DropsHCLImportBlocks(t *testing.T) {
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
				"### Importing IAM members",
				"",
				"An `import` block (Terraform v1.5.0 and later) can be used:",
				"",
				"```" + fence,
				"import {",
				`  id = "{{project_id}} roles/viewer user:foo@example.com"`,
				"  to = google_project_iam_member.default",
				"}",
				"```",
				"",
				"The command can also be used:",
				"",
				"```sh",
				`terraform import google_project_iam_member.default ` +
					`"{{project_id}} roles/viewer user:foo@example.com"`,
				"```",
				"",
			}, "\n")

			parser := tfMarkdownParser{
				info:    &mockResource{token: "gcp:projects/iAMMember:IAMMember"},
				rawname: "google_project_iam_member",
				infoCtx: infoContext{
					pkg:  "gcp",
					info: tfbridge.ProviderInfo{Name: "gcp", Resources: gcpProjectIAMResources},
				},
			}
			parser.parseImports(input)
			actual := parser.ret.Import

			// The Terraform-only import block is gone, fence and all.
			assert.NotContains(t, actual, "import {")
			assert.NotContains(t, actual, "to = google_project_iam_member.default")
			assert.NotContains(t, actual, "```"+fence)

			// Everything around it survives: the heading, the prose, and the shell
			// example, which is still rewritten to a pulumi import command.
			assert.Contains(t, actual, "### Importing IAM members")
			assert.Contains(t, actual, "The command can also be used:")
			assert.Contains(t, actual,
				"$ pulumi import gcp:projects/iAMMember:IAMMember default "+
					`"{{project_id}} roles/viewer user:foo@example.com"`)
		})
	}
}

// TestParseImports_UnmappedResourceFallsBackToPageToken pins the fallback: when an example
// names a Terraform resource this provider does not bridge, we keep stamping the token of the
// page being generated rather than dropping the example.
func TestParseImports_UnmappedResourceFallsBackToPageToken(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on windows - test case needs to be made robust to newline handling")
	}

	input := strings.Join([]string{
		"",
		"```sh",
		`terraform import google_some_unmapped_resource.default "a b"`,
		"```",
		"",
	}, "\n")

	parser := tfMarkdownParser{
		info:    &mockResource{token: "gcp:projects/iAMMember:IAMMember"},
		rawname: "google_project_iam_member",
		infoCtx: infoContext{
			pkg:  "gcp",
			info: tfbridge.ProviderInfo{Name: "gcp", Resources: gcpProjectIAMResources},
		},
	}
	parser.parseImports(input)

	assert.Contains(t, parser.ret.Import,
		`$ pulumi import gcp:projects/iAMMember:IAMMember default "a b"`)
}
