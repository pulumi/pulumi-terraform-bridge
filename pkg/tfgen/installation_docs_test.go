package tfgen

import (
	"bytes"
	"regexp"
	"runtime"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestPlainDocsParser(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name    string
		desc    string
		docFile DocFile
		edits   editRules
	}
	// Mock provider for test conversion
	p := info.Provider{
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
			name: "index",
			desc: "Converts index.md file into Pulumi installation file",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/convert-index-file/input.md")),
			},
			edits: defaultEditRules(),
		},
		{
			name: "editRules",
			// Discovered while generating docs for Libvirt - the test case has an incorrect ```hcl
			// on what should be a shell script. The provider's edit rule removes this.
			desc: "Applies provider supplied edit rules",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/convert-index-file-edit-rules/input.md")),
			},
			edits: append(
				defaultEditRules(),
				info.DocsEdit{
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
		{
			// Discovered while generating docs for SD-WAN.
			// Tests whether the custom table renderer is used correctly in docsgen overall.
			name: "table",
			desc: "Transforms table correctly",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/convert-index-file-with-table/input.md")),
			},
			edits: defaultEditRules(),
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
				info: info.Provider{
					Golang: &info.Golang{
						ImportBasePath: "github.com/pulumi/pulumi-libvirt/sdk/go/libvirt",
					},
					Repository: "https://github.com/pulumi/pulumi-libvirt",
				},
				cliConverterState: &cliConverter{
					info: p,
					pcls: pclsMap,
				},
				editRules: tt.edits,
				language:  RegistryDocs,
				pkg:       tokens.NewPackageToken("libvirt"),
			}
			actual, err := plainDocsParser(&tt.docFile, g)
			require.NoError(t, err)
			autogold.ExpectFile(t, autogold.Raw(string(actual)))
		})
	}
}

func TestDisplayNameFallback(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name        string
		displayName string
		pkgName     string
		expected    string
	}

	tests := []testCase{
		{
			name:        "Uses Display Name",
			displayName: "Unicorn",
			pkgName:     "Horse",
			expected:    "Unicorn",
		},
		{
			name:     "Defaults to pkgName as provider name",
			pkgName:  "Horse",
			expected: "Horse",
		},
		{
			name:     "Capitalizes pkgName if lower case",
			pkgName:  "shetlandpony",
			expected: "Shetlandpony",
		},
		{
			name:        "Does not alter Display Name",
			displayName: "Palo Alto Networks Cloud NGFW For AWS Provider",
			pkgName:     "cloudngfwaws",
			expected:    "Palo Alto Networks Cloud NGFW For AWS Provider",
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
				info: info.Provider{
					DisplayName: tt.displayName,
				},
				pkg: tokens.NewPackageToken(tokens.PackageName(tt.pkgName)),
			}
			actual := getProviderDisplayName(g)

			assert.Equal(t, tt.expected, actual)
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
		displayName      string
		packageName      string
		expected         autogold.Value
		ghOrg            string
		repository       string
	}

	tests := []testCase{
		{
			name: "Generates Install Information From Package Name",
			expected: autogold.Expect("## Installation\n\n" +
				"The testcase provider is available as a package in all Pulumi languages:\n\n" +
				"* JavaScript/TypeScript: [`@pulumi/testcase`](https://www.npmjs.com/package/@pulumi/testcase)\n" +
				"* Python: [`pulumi-testcase`](https://pypi.org/project/pulumi-testcase/)\n" +
				"* Go: [`github.com/pulumi/pulumi-testcase/sdk/v3/go/pulumi-testcase`](https://github.com/pulumi/pulumi-testcase)\n" +
				"* .NET: [`Pulumi.Testcase`](https://www.nuget.org/packages/Pulumi.Testcase)\n" +
				"* Java: [`com.pulumi/testcase`](https://central.sonatype.com/artifact/com.pulumi/testcase)\n\n"),
			goImportBasePath: "github.com/pulumi/pulumi-testcase/sdk/v3/go/pulumi-testcase",
			displayName:      "testcase",
			packageName:      "testcase",
			repository:       "pulumi-testcase",
			ghOrg:            "pulumi",
		},
		{
			name: "Generates Install Information From Display And Package Names",
			expected: autogold.Expect("## Installation\n\n" +
				"The Test Case provider is available as a package in all Pulumi languages:\n\n" +
				"* JavaScript/TypeScript: [`@pulumi/testcase`](https://www.npmjs.com/package/@pulumi/testcase)\n" +
				"* Python: [`pulumi-testcase`](https://pypi.org/project/pulumi-testcase/)\n" +
				"* Go: [`github.com/pulumi/pulumi-testcase/sdk/v3/go/pulumi-testcase`](https://github.com/pulumi/pulumi-testcase)\n" +
				"* .NET: [`Pulumi.Testcase`](https://www.nuget.org/packages/Pulumi.Testcase)\n" +
				"* Java: [`com.pulumi/testcase`](https://central.sonatype.com/artifact/com.pulumi/testcase)\n\n"),
			goImportBasePath: "github.com/pulumi/pulumi-testcase/sdk/v3/go/pulumi-testcase",
			displayName:      "Test Case",
			packageName:      "testcase",
			repository:       "pulumi-testcase",
			ghOrg:            "pulumi",
		},
		{
			name: "Generates Generation Instruction For Dynamically Bridged Provider Using GitHub Org And Source Repo",
			expected: autogold.Expect("## Generate Provider\n\n" +
				"The Test Case provider must be installed as a Local Package by following the " +
				"[instructions for Any Terraform Provider](https://www.pulumi.com/registry/packages/terraform-provider/):\n\n" +
				"```bash\npulumi package add terraform-provider unicorncorp/testcase\n```\n"),
			goImportBasePath: "github.com/pulumi/pulumi-testcase/sdk/v3/go/pulumi-testcase",
			displayName:      "Test Case",
			packageName:      "testcase",
			repository:       "terraform-provider-testcase",
			ghOrg:            "unicorncorp",
		},
		{
			name: "Generates Deprecation Note If Provider on Deprecated List",
			expected: autogold.Expect(
				"## Generate Provider\n\n" +
					"The Civo provider must be installed as a Local Package by following the " +
					"[instructions for Any Terraform Provider](https://www.pulumi.com/registry/packages/terraform-provider/):\n\n" +
					"```bash\npulumi package add terraform-provider civo/civo\n```\n" +
					"~> **NOTE:** This provider was previously published as @pulumi/civo.\n" +
					"However, that package is no longer being updated." +
					"Going forward, it is available as a [Local Package](https://www.pulumi.com/blog/any-terraform-provider/) " +
					"instead.\n" +
					"Please see the [provider's repository](https://github.com/pulumi/pulumi-civo) for details.\n\n"),
			goImportBasePath: "github.com/pulumi/pulumi-testcase/sdk/v3/go/pulumi-testcase",
			displayName:      "Civo",
			packageName:      "civo",
			repository:       "terraform-provider-civo",
			ghOrg:            "civo",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skipf("Skipping on Windows due to a test setup issue")
			}
			t.Parallel()
			actual := writeInstallationInstructions(tt.goImportBasePath, tt.displayName, tt.packageName, tt.ghOrg, tt.repository)
			tt.expected.Equal(t, actual)
		})
	}
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
		providerName: "Test",
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
		name       string
		desc       string
		contentStr string
		g          *Generator
	}
	providerInfo := info.Provider{
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
		Resources: map[string]*info.Resource{
			"simple_resource": {
				Tok: "simple:index:resource",
				Fields: map[string]*info.Schema{
					"input_one": {
						Name: "renamedInput1",
					},
				},
			},
		},
		DataSources: map[string]*info.DataSource{
			"simple_data_source": {
				Tok: "simple:index:dataSource",
			},
		},
	}
	generator, err := NewGenerator(GeneratorOptions{
		Language:     RegistryDocs,
		PluginHost:   &testPluginHost{},
		ProviderInfo: providerInfo,
	})
	assert.NoError(t, err)

	testCases := []testCase{
		{
			name:       "configuration",
			desc:       "Translates HCL from examples ",
			contentStr: readfile(t, "test_data/installation-docs/configuration.md"),
			g:          generator,
		},
		{
			name:       "invalid-example",
			desc:       "Does not translate an invalid example and leaves example block blank",
			contentStr: readfile(t, "test_data/installation-docs/invalid-example.md"),
			g:          generator,
		},
		{
			name:       "provider-config-only",
			desc:       "Translates standalone provider config into Pulumi config YAML",
			contentStr: readfile(t, "test_data/installation-docs/provider-config-only.md"),
			g:          generator,
		},
		{
			name:       "example-only",
			desc:       "Translates standalone example into languages",
			contentStr: readfile(t, "test_data/installation-docs/example-only.md"),
			g:          generator,
		},
	}

	for _, tt := range testCases {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				// Currently there is a test issue in CI/test setup:
				//
				// convertViaPulumiCLI: failed to clean up temp bridge-examples.json file: The
				// process cannot access the file because it is being used by another process.
				t.Skipf("Skipping on Windows due to a test setup issue")
			}
			t.Setenv("PULUMI_CONVERT", "1")
			actual, err := translateCodeBlocks(tt.contentStr, tt.g)
			require.NoError(t, err)
			autogold.ExpectFile(t, autogold.Raw(actual))
		})
	}
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

func TestRemoveEmptyExamples(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name     string
		input    string
		expected string
	}

	tc := testCase{
		name:     "An empty Example Usage section is skipped",
		input:    readTestFile(t, "skip-empty-examples/input.md"),
		expected: readTestFile(t, "skip-empty-examples/expected.md"),
	}

	t.Run(tc.name, func(t *testing.T) {
		t.Parallel()
		actual, err := removeEmptySection("Example Usage", []byte(tc.input))
		require.NoError(t, err)
		assertEqualHTML(t, tc.expected, string(actual))
	})
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
