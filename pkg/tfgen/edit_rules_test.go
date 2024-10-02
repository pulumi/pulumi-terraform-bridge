package tfgen

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func TestApplyEditRules(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name     string
		docFile  DocFile
		expected []byte
		phase    info.EditPhase
	}

	tests := []testCase{
		{
			name: "Replaces t/Terraform plan With pulumi preview",
			docFile: DocFile{
				Content: []byte("Any mention of Terraform plan or terraform plan will be Pulumi preview or pulumi preview"),
			},
			expected: []byte("Any mention of pulumi preview or pulumi preview will be Pulumi preview or pulumi preview"),
		},
		{
			name: "Strips Hashicorp links correctly",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/replace-links/input.md")),
			},
			expected: []byte(readfile(t, "test_data/replace-links/expected.md")),
		},
		{
			name: "Strips Terraform links correctly",
			docFile: DocFile{
				Content: []byte("This provider requires at least [Terraform 1.0](https://www.terraform.io/downloads.html)."),
			},
			expected: []byte("This provider requires at least Terraform 1.0."),
		},
		{
			name: "Replaces h/Hashicorp With p/Pulumi",
			docFile: DocFile{
				Content: []byte("Any mention of Hashicorp or hashicorp will be Pulumi or pulumi"),
			},
			expected: []byte("Any mention of Pulumi or pulumi will be Pulumi or pulumi"),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Replaces t/Terraform With p/Pulumi",
			docFile: DocFile{
				Content: []byte("Any mention of Terraform or terraform will be Pulumi or pulumi"),
			},
			expected: []byte("Any mention of Pulumi or pulumi will be Pulumi or pulumi"),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Replaces argument headers with input headers pattern 1",
			docFile: DocFile{
				Content: []byte("# Argument Reference\n" +
					"The following arguments are supported:\n* `some_argument`\n\n" +
					"block contains the following arguments"),
			},
			expected: []byte("# Configuration Reference\n" +
				"The following configuration inputs are supported:\n* `some_argument`\n\n" +
				"input has the following nested fields"),
			phase: info.PostCodeTranslation,
		},
		{
			name: "Replaces argument headers with input headers pattern 2",
			docFile: DocFile{
				Content: []byte("## Arguments\n" +
					"The provider supports the following arguments:"),
			},
			expected: []byte("## Configuration Reference\n" +
				"The following configuration inputs are supported:"),
			phase: info.PostCodeTranslation,
		},
		{
			name: "Skips sections about logging by default",
			docFile: DocFile{
				Content:  []byte("# I am a provider\n\n### Additional Logging\n This section should be skipped"),
				FileName: "filename",
			},
			expected: []byte("# I am a provider\n"),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Strips mentions of Terraform version pattern 1",
			docFile: DocFile{
				Content: []byte("This is a provider. It requires terraform 0.12 or later."),
			},
			expected: []byte("This is a provider."),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Strips mentions of Terraform version pattern 2",
			docFile: DocFile{
				Content: []byte("This is a provider. It requires terraform v0.12 or later."),
			},
			expected: []byte("This is a provider."),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Strips mentions of Terraform version pattern 3",
			docFile: DocFile{
				Content: []byte("This is a provider with an example. For Terraform v1.5 and later:\n Use this code."),
			},
			expected: []byte("This is a provider with an example.\nUse this code."),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Strips mentions of Terraform version pattern 4",
			docFile: DocFile{
				Content: []byte("This is a provider with an example. Terraform 1.5 and later:\n Use this code."),
			},
			expected: []byte("This is a provider with an example.\nUse this code."),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Strips mentions of Terraform version pattern 5",
			docFile: DocFile{
				Content: []byte("This is a provider with an example. Terraform 1.5 and earlier:\n Use this code."),
			},
			expected: []byte("This is a provider with an example.\nUse this code."),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Strips mentions of Terraform version pattern 6",
			docFile: DocFile{
				Content: []byte("This provider requires at least Terraform 1.0."),
			},
			expected: []byte(""),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Strips mentions of Terraform version pattern 7",
			docFile: DocFile{
				Content: []byte("This provider requires Terraform 1.0."),
			},
			expected: []byte(""),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Strips mentions of Terraform version pattern 8",
			docFile: DocFile{
				Content: []byte("A minimum of Terraform 1.4.0 is recommended."),
			},
			expected: []byte(""),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Strips mentions of Terraform version With Surrounding Text",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/replace-terraform-version/input.md")),
			},
			expected: []byte(readfile(t, "test_data/replace-terraform-version/expected.md")),
			phase:    info.PostCodeTranslation,
		},
		{
			// Found in linode
			name: "Rewrites providers.tf to Pulumi.yaml",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/rewrite-providers-tf-to-pulumi-yaml/input.md")),
			},
			expected: []byte(readfile(t, "test_data/rewrite-providers-tf-to-pulumi-yaml/expected.md")),
			phase:    info.PostCodeTranslation,
		},
		{
			name: "Rewrites terraform init to pulumi up",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/rewrite-tf-init-to-pulumi-up/input.md")),
			},
			expected: []byte(readfile(t, "test_data/rewrite-tf-init-to-pulumi-up/expected.md")),
			phase:    info.PostCodeTranslation,
		},
		{
			// Found in linode
			name: "Replaces provider block with provider configuration",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/replace-provider-block/input.md")),
			},
			expected: []byte(readfile(t, "test_data/replace-provider-block/expected.md")),
			phase:    info.PostCodeTranslation,
		},
		{
			// Found in scm
			name: "Replaces `provider` block with provider configuration",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/replace-provider-block/input-backtick.md")),
			},
			expected: []byte(readfile(t, "test_data/replace-provider-block/expected-backtick.md")),
			phase:    info.PostCodeTranslation,
		},
		{
			// Found in scm
			name: "Replaces 'D/data source(s)' with 'F/function(s)",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/replace-data-source/input.md")),
			},
			expected: []byte(readfile(t, "test_data/replace-data-source/expected.md")),
			phase:    info.PostCodeTranslation,
		},
	}
	edits := defaultEditRules()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS == "windows" {
				t.Skipf("Skipping on Windows due to a newline handling issue")
			}
			actual, err := edits.apply("*", tt.docFile.Content, tt.phase)
			require.NoError(t, err)
			assertEqualHTML(t, string(tt.expected), string(actual))
		})
	}
}

func TestApplyCustomEditRules(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name     string
		docFile  DocFile
		expected []byte
		edits    editRules
	}

	tests := []testCase{
		{
			name: "Replaces specified text pre code translation",
			docFile: DocFile{
				Content: []byte("This provider has a hroffic unreadable typo"),
			},
			expected: []byte("This provider has a horrific unreadable typo, which is now fixed"),
			edits: append(
				defaultEditRules(),
				tfbridge.DocsEdit{
					Path: "testfile.md",
					Edit: func(_ string, content []byte) ([]byte, error) {
						return bytes.ReplaceAll(
							content,
							[]byte("hroffic unreadable typo"),
							[]byte("horrific unreadable typo, which is now fixed"),
						), nil
					},
				},
			),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS == "windows" {
				t.Skipf("Skipping on Windows due to a newline handling issue")
			}
			actual, err := tt.edits.apply("testfile.md", tt.docFile.Content, info.PreCodeTranslation)
			require.NoError(t, err)
			assertEqualHTML(t, string(tt.expected), string(actual))
		})
	}

}
