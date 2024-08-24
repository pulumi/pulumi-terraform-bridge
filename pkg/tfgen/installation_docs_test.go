package tfgen

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

//nolint:lll
func TestPlainDocsParser(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name     string
		docFile  DocFile
		expected []byte
	}

	tests := []testCase{
		{
			name: "Replaces Upstream Front Matter With Pulumi Front Matter",
			docFile: DocFile{
				Content: []byte("---\nlayout: \"openstack\"\npage_title: \"Provider: OpenStack\"\nsidebar_current: \"docs-openstack-index\"\ndescription: |-\n  The OpenStack provider is used to interact with the many resources supported by OpenStack. The provider needs to be configured with the proper credentials before it can be used.\n---\n\n# OpenStack Provider\n\nThe OpenStack provider is used to interact with the\nmany resources supported by OpenStack. The provider needs to be configured\nwith the proper credentials before it can be used.\n\nUse the navigation to the left to read about the available resources."),
			},
			expected: []byte("---\ntitle: OpenStack Provider Installation & Configuration\nmeta_desc: Provides an overview on how to configure the Pulumi OpenStack Provider.\nlayout: package\n---\n\nThe OpenStack provider is used to interact with the\nmany resources supported by OpenStack. The provider needs to be configured\nwith the proper credentials before it can be used.\n\nUse the navigation to the left to read about the available resources."),
		},
		{
			name: "Writes Pulumi Style Front Matter If Not Present",
			docFile: DocFile{
				Content: []byte("# Artifactory Provider\n\nThe [Artifactory](https://jfrog.com/artifactory/) provider is used to interact with the resources supported by Artifactory. The provider needs to be configured with the proper credentials before it can be used.\n\nLinks to documentation for specific resources can be found in the table of contents to the left.\n\nThis provider requires access to Artifactory APIs, which are only available in the _licensed_ pro and enterprise editions. You can determine which license you have by accessing the following the URL `${host}/artifactory/api/system/licenses/`.\n\nYou can either access it via API, or web browser - it require admin level credentials."),
			},
			expected: []byte("---\ntitle: Artifactory Provider Installation & Configuration\nmeta_desc: Provides an overview on how to configure the Pulumi Artifactory Provider.\nlayout: package\n---\n\nThe [Artifactory](https://jfrog.com/artifactory/) provider is used to interact with the resources supported by Artifactory. The provider needs to be configured with the proper credentials before it can be used.\n\nLinks to documentation for specific resources can be found in the table of contents to the left.\n\nThis provider requires access to Artifactory APIs, which are only available in the _licensed_ pro and enterprise editions. You can determine which license you have by accessing the following the URL `${host}/artifactory/api/system/licenses/`.\n\nYou can either access it via API, or web browser - it require admin level credentials."),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Skipf("this function is under development and will receive tests once all parts are completed")
			t.Parallel()
			g := &Generator{
				sink: mockSink{t},
			}
			actual, err := plainDocsParser(&tt.docFile, g)
			require.NoError(t, err)
			require.Equal(t, string(tt.expected), string(actual))
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

func TestWriteFrontMatter(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name     string
		title    string
		expected string
	}

	tc := testCase{
		name:  "Generates Front Matter for installation-configuration.md",
		title: "Testcase Provider",
		expected: delimiter +
			"title: Testcase Provider Installation & Configuration\n" +
			"meta_desc: Provides an overview on how to configure the Pulumi Testcase Provider.\n" +
			"layout: package\n" +
			delimiter +
			"\n",
	}

	t.Run(tc.name, func(t *testing.T) {
		t.Parallel()
		actual := writeFrontMatter(tc.title)
		require.Equal(t, tc.expected, actual)
	})
}

func TestWriteIndexFrontMatter(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name        string
		displayName string
		expected    string
	}

	tc := testCase{
		name:        "Generates Front Matter for _index.md",
		displayName: "Testcase",
		expected: delimiter +
			"title: Testcase\n" +
			"meta_desc: The Testcase provider for Pulumi " +
			"can be used to provision any of the cloud resources available in Testcase.\n" +
			"layout: package\n" +
			delimiter,
	}

	t.Run(tc.name, func(t *testing.T) {
		t.Parallel()
		actual := writeIndexFrontMatter(tc.displayName)
		require.Equal(t, tc.expected, actual)
	})
}

func TestApplyEditRules(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// The name of the test case.
		name     string
		docFile  DocFile
		expected []byte
	}

	tests := []testCase{
		{
			name: "Replaces h/Hashicorp With p/Pulumi",
			docFile: DocFile{
				Content: []byte("Any mention of Hashicorp or hashicorp will be Pulumi or pulumi"),
			},
			expected: []byte("Any mention of Pulumi or pulumi will be Pulumi or pulumi"),
		},
		{
			name: "Replaces t/Terraform With p/Pulumi",
			docFile: DocFile{
				Content: []byte("Any mention of Terraform or terraform will be Pulumi or pulumi"),
			},
			expected: []byte("Any mention of Pulumi or pulumi will be Pulumi or pulumi"),
		},
		{
			name: "Replaces argument headers with input headers",
			docFile: DocFile{
				Content: []byte("# Argument Reference\n" +
					"The following arguments are supported:\n* `some_argument`\n\n" +
					"block contains the following arguments"),
			},
			expected: []byte("# Configuration Reference\n" +
				"The following configuration inputs are supported:\n* `some_argument`\n\n" +
				"input has the following nested fields"),
		},
		{
			name: "Replaces terraform plan with pulumi preview",
			docFile: DocFile{
				Content: []byte("terraform plan this program"),
			},
			expected: []byte("pulumi preview this program"),
		},
		{
			name: "Skips sections about logging by default",
			docFile: DocFile{
				Content:  []byte("# I am a provider\n\n### Additional Logging\n This section should be skipped"),
				FileName: "filename",
			},
			expected: []byte("# I am a provider\n"),
		},
		{
			name: "Strips Hashicorp links correctly",
			docFile: DocFile{
				Content: []byte(readfile(t, "test_data/replace-links/input.md")),
			},
			expected: []byte(readfile(t, "test_data/replace-links/actual.md")),
		},
		{
			name: "Strips mentions of Terraform version pattern 1",
			docFile: DocFile{
				Content: []byte("This is a provider. It requires terraform 0.12 or later."),
			},
			expected: []byte("This is a provider."),
		},
		{
			name: "Strips mentions of Terraform version pattern 2",
			docFile: DocFile{
				Content: []byte("This is a provider. It requires terraform v0.12 or later."),
			},
			expected: []byte("This is a provider."),
		},
		{
			name: "Strips mentions of Terraform version pattern 3",
			docFile: DocFile{
				Content: []byte("This is a provider with an example. For Terraform v1.5 and later:\n Use this code."),
			},
			expected: []byte("This is a provider with an example.\nUse this code."),
		},
		{
			name: "Strips mentions of Terraform version pattern 4",
			docFile: DocFile{
				Content: []byte("This is a provider with an example. Terraform 1.5 and later:\n Use this code."),
			},
			expected: []byte("This is a provider with an example.\nUse this code."),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := &Generator{
				sink:      mockSink{t},
				editRules: defaultEditRules(),
			}
			actual, err := applyEditRules(tt.docFile.Content, "testfile.md", g)
			require.NoError(t, err)
			assertEqualHTML(t, string(tt.expected), string(actual))
		})
	}
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
		expected:      readTestFile(t, "skip-sections-by-header/actual.md"),
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
