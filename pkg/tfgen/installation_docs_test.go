package tfgen

import (
	"bytes"
	"regexp"
	"runtime"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestPlainDocsParser(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name     string
		docFile  DocFile
		expected []byte
		edits    editRules
	}
	// Mock provider for test conversion
	p := tfbridge.ProviderInfo{
		Name: "simple",
		P: sdkv2.NewProvider(&schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"simple_resource": {
					Schema: map[string]*schema.Schema{
						"input_one": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"input_two": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		}),
	}
	pclsMap := make(map[string]translatedExample)

	tests := []testCase{
		{
			name: "Converts index.md file into Pulumi installation file",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/convert-index-file/input.md")),
			},
			expected: []byte(readfile(t, "test_data/convert-index-file/expected.md")),
			edits:    defaultEditRules(),
		},
		{
			// Discovered while generating docs for Libvirt - the test case has an incorrect ```hcl
			// on what should be a shell script. The provider's edit rule removes this.
			name: "Applies provider supplied edit rules",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/convert-index-file-edit-rules/input.md")),
			},
			expected: []byte(readfile(t, "test_data/convert-index-file-edit-rules/expected.md")),
			edits: append(
				defaultEditRules(),
				tfbridge.DocsEdit{
					Edit: func(_ string, content []byte) ([]byte, error) {
						return bytes.ReplaceAll(
							content,
							[]byte("shell environment variable.\n\n```hcl"),
							[]byte("shell environment variable.\n\n```"),
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
			g := &Generator{
				sink: mockSink{t},
				info: tfbridge.ProviderInfo{
					Golang: &tfbridge.GolangInfo{
						ImportBasePath: "github.com/pulumi/pulumi-libvirt/sdk/go/libvirt",
					},
					Name: "libvirt",
				},
				cliConverterState: &cliConverter{
					info: p,
					pcls: pclsMap,
				},
				editRules: tt.edits,
				language:  RegistryDocs,
			}
			actual, err := plainDocsParser(&tt.docFile, g)
			require.NoError(t, err)
			assertEqualHTML(t, string(tt.expected), string(actual))
		})
	}
}

func TestTrimFrontmatter(t *testing.T) {
	t.Parallel()
	type testCase struct {
		// The name of the test case.
		name     string
		input    string
		expected string
	}

	tests := []testCase{
		{
			name:     "Strips Upstream Frontmatter",
			input:    readfile(t, "test_data/strip-front-matter/openstack-input.md"),
			expected: readfile(t, "test_data/strip-front-matter/openstack-expected.md"),
		},
		{
			name:     "Returns Body If No Frontmatter",
			input:    readfile(t, "test_data/strip-front-matter/artifactory-input.md"),
			expected: readfile(t, "test_data/strip-front-matter/artifactory-expected.md"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skipf("Skipping on Windows due to a test setup issue")
			}
			t.Parallel()
			actual := trimFrontMatter([]byte(tt.input))
			assertEqualHTML(t, tt.expected, string(actual))
		})
	}
}

func TestRemoveTitle(t *testing.T) {
	t.Parallel()
	type testCase struct {
		// The name of the test case.
		name     string
		input    string
		expected string
	}

	tests := []testCase{
		{
			name:     "Strips Title Placed Anywhere",
			input:    readfile(t, "test_data/remove-title/openstack-input.md"),
			expected: readfile(t, "test_data/remove-title/openstack-expected.md"),
		},
		{
			name:     "Strips Title On Top",
			input:    readfile(t, "test_data/remove-title/artifactory-input.md"),
			expected: readfile(t, "test_data/remove-title/artifactory-expected.md"),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skipf("Skipping on Windows due to a test setup issue")
			}
			t.Parallel()
			actual, err := removeTitle([]byte(tt.input))
			assert.NoError(t, err)
			assertEqualHTML(t, tt.expected, string(actual))
		})
	}
}

//nolint:lll
func TestWriteInstallationInstructions(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name             string
		goImportBasePath string
		packageName      string
		expected         string
	}

	tc := testCase{
		name: "Generates Install Information From Package Name",
		expected: "## Installation\n\n" +
			"The testcase provider is available as a package in all Pulumi languages:\n\n" +
			"* JavaScript/TypeScript: [`@pulumi/testcase`](https://www.npmjs.com/package/@pulumi/testcase)\n" +
			"* Python: [`pulumi-testcase`](https://pypi.org/project/pulumi-testcase/)\n" +
			"* Go: [`github.com/pulumi/pulumi-testcase/sdk/v3/go/pulumi-testcase`](https://github.com/pulumi/pulumi-testcase)\n" +
			"* .NET: [`Pulumi.Testcase`](https://www.nuget.org/packages/Pulumi.Testcase)\n" +
			"* Java: [`com.pulumi/testcase`](https://central.sonatype.com/artifact/com.pulumi/testcase)\n\n",
		goImportBasePath: "github.com/pulumi/pulumi-testcase/sdk/v3/go/pulumi-testcase",
		packageName:      "testcase",
	}

	t.Run(tc.name, func(t *testing.T) {
		t.Parallel()
		actual := writeInstallationInstructions(tc.goImportBasePath, tc.packageName)
		require.Equal(t, tc.expected, actual)
	})
}

func TestWriteOverviewHeader(t *testing.T) {
	t.Parallel()
	type testCase struct {
		// The name of the test case.
		name     string
		input    string
		expected string
	}

	testCases := []testCase{
		{
			name:     "Writes When Content Exists",
			input:    readTestFile(t, "write-overview-header/with-overview-text.md"),
			expected: "## Overview\n\n",
		},
		{
			name:     "Does Not Write For Empty Overview",
			input:    readTestFile(t, "write-overview-header/empty-overview.md"),
			expected: "",
		},
		{
			name:     "Does Not Write For Empty Content",
			input:    "",
			expected: "",
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := getOverviewHeader([]byte(tt.input))
			assert.Equal(t, tt.expected, actual)
		})
	}

	// The following mirrors the way that the result of `writeOverviewHeader` gets applied to our installation doc.
	contextTest := testCase{
		name:     "Does Not Write For Empty Overview With Context",
		input:    readTestFile(t, "write-overview-header/empty-overview-with-context-input.md"),
		expected: readTestFile(t, "write-overview-header/empty-overview-with-context-expected.md"),
	}
	t.Run(contextTest.name, func(t *testing.T) {
		t.Parallel()
		text, err := removeTitle([]byte(contextTest.input))
		require.NoError(t, err)
		header := getOverviewHeader(text)
		actual := header + string(text)
		assertEqualHTML(t, contextTest.expected, actual)
	})
}

func TestWriteFrontMatter(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name         string
		providerName string
		expected     string
	}

	tc := testCase{
		name:         "Generates Front Matter for installation-configuration.md",
		providerName: "test",
		expected: delimiter +
			"# *** WARNING: This file was auto-generated. " +
			"Do not edit by hand unless you're certain you know what you are doing! ***\n" +
			"title: Test Provider\n" +
			"meta_desc: Provides an overview on how to configure the Pulumi Test provider.\n" +
			"layout: package\n" +
			delimiter +
			"\n",
	}

	t.Run(tc.name, func(t *testing.T) {
		t.Parallel()
		actual := writeFrontMatter(tc.providerName)
		require.Equal(t, tc.expected, actual)
	})
}

func TestTranslateCodeBlocks(t *testing.T) {
	type testCase struct {
		// The name of the test case.
		name       string
		contentStr string
		g          *Generator
		expected   string
	}
	p := tfbridge.ProviderInfo{
		Name: "simple",
		P: sdkv2.NewProvider(&schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"simple_resource": {
					Schema: map[string]*schema.Schema{
						"input_one": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"input_two": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			DataSourcesMap: map[string]*schema.Resource{
				"simple_data_source": {
					Schema: map[string]*schema.Schema{},
				},
			},
		}),
		Resources: map[string]*tfbridge.ResourceInfo{
			"simple_resource": {
				Tok: "simple:index:resource",
				Fields: map[string]*tfbridge.SchemaInfo{
					"input_one": {
						Name: "renamedInput1",
					},
				},
			},
		},
		DataSources: map[string]*tfbridge.DataSourceInfo{
			"simple_data_source": {
				Tok: "simple:index:dataSource",
			},
		},
	}
	pclsMap := make(map[string]translatedExample)

	tc := testCase{
		name:       "Translates HCL from examples ",
		contentStr: readfile(t, "test_data/installation-docs/configuration.md"),
		expected:   readfile(t, "test_data/installation-docs/configuration-expected.md"),
		g: &Generator{
			sink: mockSink{},
			cliConverterState: &cliConverter{
				info: p,
				pcls: pclsMap,
			},
			language: RegistryDocs,
		},
	}
	t.Run(tc.name, func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// Currently there is a test issue in CI/test setup:
			//
			// convertViaPulumiCLI: failed to clean up temp bridge-examples.json file: The
			// process cannot access the file because it is being used by another process.
			t.Skipf("Skipping on Windows due to a test setup issue")
		}
		t.Setenv("PULUMI_CONVERT", "1")
		actual, err := translateCodeBlocks(tc.contentStr, tc.g)
		require.NoError(t, err)
		require.Equal(t, tc.expected, actual)
	})
}

func TestSkipSectionHeadersByContent(t *testing.T) {
	t.Parallel()
	type testCase struct {
		// The name of the test case.
		name          string
		headersToSkip []string
		input         string
		expected      string
	}

	tc := testCase{
		name:          "Skips Sections With Unwanted Headers",
		headersToSkip: []string{"Debugging Provider Output Using Logs", "Testing and Development"},
		input:         readTestFile(t, "skip-sections-by-header/input.md"),
		expected:      readTestFile(t, "skip-sections-by-header/expected.md"),
	}

	t.Run(tc.name, func(t *testing.T) {
		t.Parallel()
		actual, err := SkipSectionByHeaderContent([]byte(tc.input), func(headerText string) bool {
			for _, header := range tc.headersToSkip {
				if headerText == header {
					return true
				}
			}
			return false
		})
		require.NoError(t, err)
		assertEqualHTML(t, tc.expected, string(actual))
	})
}

func TestSkipDefaultSectionHeaders(t *testing.T) {
	t.Parallel()
	type testCase struct {
		// The name of the test case.
		name          string
		headersToSkip []*regexp.Regexp
		input         string
		expected      string
	}

	testCases := []testCase{
		{
			name:          "Skips Sections Mentioning Logging",
			headersToSkip: getDefaultHeadersToSkip(),
			input:         "## Logging",
			expected:      "",
		},
		{
			name:          "Skips Sections Mentioning Logs",
			headersToSkip: getDefaultHeadersToSkip(),
			input:         "## This section talks about logs",
			expected:      "",
		},
		{
			name:          "Skips Sections About Testing",
			headersToSkip: getDefaultHeadersToSkip(),
			input:         "## Testing",
			expected:      "",
		},
		{
			name:          "Skips Sections About Development",
			headersToSkip: getDefaultHeadersToSkip(),
			input:         "## Development",
			expected:      "",
		},
		{
			name:          "Skips Sections About Debugging",
			headersToSkip: getDefaultHeadersToSkip(),
			input:         "## Debugging",
			expected:      "",
		},
		{
			name:          "Skips Sections Talking About Terraform CLI",
			headersToSkip: getDefaultHeadersToSkip(),
			input:         "## Terraform CLI",
			expected:      "",
		},
		{
			name:          "Skips Sections Talking About terraform cloud",
			headersToSkip: getDefaultHeadersToSkip(),
			input:         "### terraform cloud",
			expected:      "",
		},
		{
			name:          "Skips Sections About Delete Protection",
			headersToSkip: getDefaultHeadersToSkip(),
			input:         "### Delete Protection",
			expected:      "",
		},
		{
			name:          "Skips Sections About Contributing",
			headersToSkip: getDefaultHeadersToSkip(),
			input:         "## Contributing",
			expected:      "",
		},
		{
			name:          "Does Not Skip Sections About Unicorns",
			headersToSkip: getDefaultHeadersToSkip(),
			input:         "## Unicorns",
			expected:      "## Unicorns",
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual, err := SkipSectionByHeaderContent([]byte(tt.input), func(headerText string) bool {
				for _, header := range tt.headersToSkip {
					if header.Match([]byte(headerText)) {
						return true
					}
				}
				return false
			})
			require.NoError(t, err)
			assertEqualHTML(t, tt.expected, string(actual))
		})
	}
}

// Helper func to determine if the HTML rendering is equal.
// This helps in cases where the processed Markdown is slightly different from the expected Markdown
// due to goldmark making some (insignificant to the final HTML) changes when parsing and rendering.
// We convert the expected Markdown and the actual test Markdown output to HTML and verify if they are equal.
func assertEqualHTML(t *testing.T, expected, actual string) bool {
	mdRenderer := goldmark.New()
	var expectedBuf bytes.Buffer
	err := mdRenderer.Convert([]byte(expected), &expectedBuf)
	if err != nil {
		panic(err)
	}
	var outputBuf bytes.Buffer
	err = mdRenderer.Convert([]byte(actual), &outputBuf)
	if err != nil {
		panic(err)
	}
	return assert.Equal(t, expectedBuf.String(), outputBuf.String())
}
