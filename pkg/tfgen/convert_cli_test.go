// Copyright 2016-2023, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hexops/autogold/v2"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	bridgetesting "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestConvertViaPulumiCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Currently there is a test issue in CI/test setup:
		//
		// convertViaPulumiCLI: failed to clean up temp bridge-examples.json file: The
		// process cannot access the file because it is being used by another process..
		t.Skipf("Skipping on Windows due to a test setup issue")
	}
	t.Setenv("PULUMI_CONVERT", "1")

	simpleResourceTF := `
resource "simple_resource" "a_resource" {
    input_one = "hello"
    input_two = true
}

output "some_output" {
    value = simple_resource.a_resource.result
}`

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
				Docs: &tfbridge.DocInfo{
					Markdown: []byte(fmt.Sprintf(
						"Sample resource.\n## Example Usage\n\n"+
							"```hcl\n%s\n```\n\n##Extras\n\n",
						simpleResourceTF,
					)),
				},
			},
		},
		DataSources: map[string]*tfbridge.DataSourceInfo{
			"simple_data_source": {
				Tok: "simple:index:dataSource",
			},
		},
	}

	simpleDataSourceTF := `
data "simple_data_source" "a_data_source" {
    input_one = "hello"
    input_two = true
}

output "some_output" {
    value = data.simple_data_source.a_data_source.result
}`

	simpleResourceExpectPCL := `resource "aResource" "simple:index:resource" {
  __logicalName = "a_resource"
  renamedInput1 = "hello"
  inputTwo      = true
}

output "someOutput" {
  value = aResource.result
}
`

	simpleDataSourceExpectPCL := `aDataSource = invoke("simple:index:dataSource", {
  inputOne = "hello"
  inputTwo = true
})

output "someOutput" {
  value = aDataSource.result
}
`

	t.Run("convertViaPulumiCLI", func(t *testing.T) {
		cc := &cliConverter{}
		out, err := cc.convertViaPulumiCLI(map[string]string{
			"example1": simpleResourceTF,
			"example2": simpleDataSourceTF,
		}, []tfbridge.ProviderInfo{p})

		require.NoError(t, err)
		assert.Equal(t, 2, len(out))

		assert.Equal(t, simpleResourceExpectPCL, out["example1"].PCL)
		assert.Equal(t, simpleDataSourceExpectPCL, out["example2"].PCL)

		assert.Empty(t, out["example1"].Diagnostics)
		assert.Empty(t, out["example2"].Diagnostics)
	})

	t.Run("GenerateSchema", func(t *testing.T) {
		info := p
		tempdir := t.TempDir()
		fs := afero.NewBasePathFs(afero.NewOsFs(), tempdir)

		ct := newCoverageTracker(info.Name, info.Version)

		g, err := NewGenerator(GeneratorOptions{
			Package:      info.Name,
			Version:      info.Version,
			Language:     Schema,
			PluginHost:   &testPluginHost{},
			ProviderInfo: info,
			Root:         fs,
			Sink: diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
				Color: colors.Never,
			}),
			CoverageTracker: ct,
		})
		assert.NoError(t, err)

		err = g.Generate()
		assert.NoError(t, err)

		d, err := os.ReadFile(filepath.Join(tempdir, "schema.json"))
		assert.NoError(t, err)

		var schema pschema.PackageSpec
		err = json.Unmarshal(d, &schema)
		assert.NoError(t, err)

		bridgetesting.AssertEqualsJSONFile(t,
			"test_data/TestConvertViaPulumiCLI/schema.json", schema)

		autogold.Expect(`
Provider:     simple
Success rate: 100.00% (6/6)

Converted 100.00% of csharp examples (1/1)
Converted 100.00% of go examples (1/1)
Converted 100.00% of java examples (1/1)
Converted 100.00% of python examples (1/1)
Converted 100.00% of typescript examples (1/1)
Converted 100.00% of yaml examples (1/1)
`).Equal(t, ct.getShortResultSummary())

		require.Equalf(t, 1, len(ct.EncounteredPages), "expected 1 page")
		var page *DocumentationPage
		for _, p := range ct.EncounteredPages {
			page = p
		}
		require.Equal(t, 1, len(page.Examples), "expected 1 example")
	})

	t.Run("mappingsFile", func(t *testing.T) {
		c := &cliConverter{}
		aws := tfbridge.ProviderInfo{Name: "aws"}
		assert.Equal(t, filepath.Join(".", "aws.json"), c.mappingsFile(".", aws))
		withPrefix := tfbridge.ProviderInfo{Name: "p", ResourcePrefix: "prov"}
		assert.Equal(t, filepath.Join(".", "prov.json"), c.mappingsFile(".", withPrefix))
	})

	// Taken from https://github.com/pulumi/pulumi-azure/issues/1698
	//
	// Emulate a case where pulumi name does not match the TF provider prefix, and one of the
	// examples is referencing an unknown resource.
	//
	// Before the fix this would panic reaching out to PluginHost.ResolvePlugin.
	t.Run("unknownResource", func(t *testing.T) {
		md := []byte(strings.ReplaceAll(`
# azurerm_web_pubsub_custom_certificate

Manages an Azure Web PubSub Custom Certificate.

## Example Usage

%%%hcl

resource "azurerm_web_pubsub_service" "example" {
  name = "example-webpubsub"
}

resource "azurerm_web_pubsub_custom_certificate" "test" {
  name = "example-cert"
}

%%%`, "%%%", "```"))
		p := &schema.Provider{
			ResourcesMap: map[string]*schema.Resource{
				"azurerm_web_pubsub_custom_certificate": {
					Schema: map[string]*schema.Schema{"name": {
						Type:     schema.TypeString,
						Optional: true,
					}},
				},
			},
		}
		pi := tfbridge.ProviderInfo{
			P:       shimv2.NewProvider(p),
			Name:    "azurerm",
			Version: "0.0.1",
			Resources: map[string]*tfbridge.ResourceInfo{
				"azurerm_web_pubsub_custom_certificate": {
					Tok:  "azure:webpubsub/customCertificate:CustomCertificate",
					Docs: &tfbridge.DocInfo{Markdown: md},
				},
			},
		}
		g, err := NewGenerator(GeneratorOptions{
			Package:      "azure",
			Version:      "0.0.1",
			PluginHost:   &testPluginHost{},
			Language:     Schema,
			ProviderInfo: pi,
			Root:         afero.NewBasePathFs(afero.NewOsFs(), t.TempDir()),
			Sink: diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
				Color: colors.Never,
			}),
		})
		require.NoError(t, err)

		err = g.Generate()
		require.NoError(t, err)
	})
}

type testPluginHost struct{}

func (*testPluginHost) ServerAddr() string { panic("Unexpected call") }

func (*testPluginHost) Log(diag.Severity, resource.URN, string, int32) {
	panic("Unexpected call")
}

func (*testPluginHost) LogStatus(diag.Severity, resource.URN, string, int32) {
	panic("Unexpected call")
}

func (*testPluginHost) Analyzer(tokens.QName) (plugin.Analyzer, error) { panic("Unexpected call") }

func (*testPluginHost) PolicyAnalyzer(
	tokens.QName, string, *plugin.PolicyAnalyzerOptions,
) (plugin.Analyzer, error) {
	panic("Unexpected call")
}

func (*testPluginHost) ListAnalyzers() []plugin.Analyzer { panic("Unexpected call") }

func (*testPluginHost) Provider(tokens.Package, *semver.Version) (plugin.Provider, error) {
	panic("Unexpected call")
}

func (*testPluginHost) CloseProvider(plugin.Provider) error { panic("Unexpected call") }

func (*testPluginHost) LanguageRuntime(string, plugin.ProgramInfo) (plugin.LanguageRuntime, error) {
	panic("Unexpected call")
}

func (*testPluginHost) EnsurePlugins([]workspace.PluginSpec, plugin.Flags) error {
	panic("Unexpected call")
}

func (*testPluginHost) ResolvePlugin(
	workspace.PluginKind, string, *semver.Version,
) (*workspace.PluginInfo, error) {
	panic("Unexpected call")
}

func (*testPluginHost) GetProjectPlugins() []workspace.ProjectPlugin { panic("Unexpected call") }
func (*testPluginHost) SignalCancellation() error                    { panic("Unexpected call") }
func (*testPluginHost) Close() error                                 { return nil }

var _ plugin.Host = (*testPluginHost)(nil)
