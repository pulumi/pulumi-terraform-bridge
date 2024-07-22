package tfgen

import (
	"runtime"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"

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

//nolint:lll
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
		contentStr: "Use the navigation to the left to read about the available resources.\n\n## Example Usage\n\n```hcl\nresource \"simple_resource\" \"a_resource\" {\n    input_one = \"hello\"\n    input_two = true\n}\n\noutput \"some_output\" {\n    value = simple_resource.a_resource.result\n}```\n\n## Configuration Reference\n\nThe following configuration inputs are supported:",
		expected:   "Use the navigation to the left to read about the available resources.\n\n## Example Usage\n\n{{< chooser language \"typescript,python,go,csharp,java,yaml\" >}}\n{{% choosable language typescript %}}\n```yaml\n# Pulumi.yaml provider configuration file\nname: configuration-example\nruntime: nodejs\n\n```\n```typescript\nimport * as pulumi from \"@pulumi/pulumi\";\nimport * as simple from \"@pulumi/simple\";\n\nconst aResource = new simple.index.Resource(\"a_resource\", {\n    renamedInput1: \"hello\",\n    inputTwo: true,\n});\nexport const someOutput = aResource.result;\n```\n{{% /choosable %}}\n{{% choosable language python %}}\n```yaml\n# Pulumi.yaml provider configuration file\nname: configuration-example\nruntime: python\n\n```\n```python\nimport pulumi\nimport pulumi_simple as simple\n\na_resource = simple.index.Resource(\"a_resource\",\n    renamed_input1=hello,\n    input_two=True)\npulumi.export(\"someOutput\", a_resource[\"result\"])\n```\n{{% /choosable %}}\n{{% choosable language csharp %}}\n```yaml\n# Pulumi.yaml provider configuration file\nname: configuration-example\nruntime: dotnet\n\n```\n```csharp\nusing System.Collections.Generic;\nusing System.Linq;\nusing Pulumi;\nusing Simple = Pulumi.Simple;\n\nreturn await Deployment.RunAsync(() => \n{\n    var aResource = new Simple.Index.Resource(\"a_resource\", new()\n    {\n        RenamedInput1 = \"hello\",\n        InputTwo = true,\n    });\n\n    return new Dictionary<string, object?>\n    {\n        [\"someOutput\"] = aResource.Result,\n    };\n});\n```\n{{% /choosable %}}\n{{% choosable language go %}}\n```yaml\n# Pulumi.yaml provider configuration file\nname: configuration-example\nruntime: go\n\n```\n```go\npackage main\n\nimport (\n\t\"github.com/pulumi/pulumi-simple/sdk/go/simple\"\n\t\"github.com/pulumi/pulumi/sdk/v3/go/pulumi\"\n)\n\nfunc main() {\n\tpulumi.Run(func(ctx *pulumi.Context) error {\n\t\taResource, err := simple.NewResource(ctx, \"a_resource\", &simple.ResourceArgs{\n\t\t\tRenamedInput1: \"hello\",\n\t\t\tInputTwo:      true,\n\t\t})\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tctx.Export(\"someOutput\", aResource.Result)\n\t\treturn nil\n\t})\n}\n```\n{{% /choosable %}}\n{{% choosable language yaml %}}\n```yaml\n# Pulumi.yaml provider configuration file\nname: configuration-example\nruntime: yaml\n\n```\n```yaml\nresources:\n  aResource:\n    type: simple:resource\n    name: a_resource\n    properties:\n      renamedInput1: hello\n      inputTwo: true\noutputs:\n  someOutput: ${aResource.result}\n```\n{{% /choosable %}}\n{{% choosable language java %}}\n```yaml\n# Pulumi.yaml provider configuration file\nname: configuration-example\nruntime: java\n\n```\n```java\npackage generated_program;\n\nimport com.pulumi.Context;\nimport com.pulumi.Pulumi;\nimport com.pulumi.core.Output;\nimport com.pulumi.simple.resource;\nimport com.pulumi.simple.ResourceArgs;\nimport java.util.List;\nimport java.util.ArrayList;\nimport java.util.Map;\nimport java.io.File;\nimport java.nio.file.Files;\nimport java.nio.file.Paths;\n\npublic class App {\n    public static void main(String[] args) {\n        Pulumi.run(App::stack);\n    }\n\n    public static void stack(Context ctx) {\n        var aResource = new Resource(\"aResource\", ResourceArgs.builder()\n            .renamedInput1(\"hello\")\n            .inputTwo(true)\n            .build());\n\n        ctx.export(\"someOutput\", aResource.result());\n    }\n}\n```\n{{% /choosable %}}\n{{< /chooser >}}\n\n\n## Configuration Reference\n\nThe following configuration inputs are supported:",

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
			// process cannot access the file because it is being used by another process..
			t.Skipf("Skipping on Windows due to a test setup issue")
		}
		t.Setenv("PULUMI_CONVERT", "1")
		actual, err := translateCodeBlocks(tc.contentStr, tc.g)
		require.NoError(t, err)
		require.Equal(t, tc.expected, actual)
	})
}
