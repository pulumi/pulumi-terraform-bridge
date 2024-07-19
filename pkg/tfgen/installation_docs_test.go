package tfgen

import (
	"testing"

	"github.com/stretchr/testify/require"
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
					"The following arguments are supported:\n* `some_argument`\n" +
					"block contains the following arguments"),
			},
			expected: []byte("# Configuration Reference\n" +
				"The following configuration inputs are supported:\n* `some_argument`\n" +
				"input has the following nested fields"),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual, err := applyEditRules(tt.docFile.Content, &tt.docFile)
			require.NoError(t, err)
			require.Equal(t, string(tt.expected), string(actual))
		})
	}
}
