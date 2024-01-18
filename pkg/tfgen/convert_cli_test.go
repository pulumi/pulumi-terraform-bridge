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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"encoding/json"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	bridgetesting "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testing"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
					Schema: map[string]*schema.Schema{},
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
				// Suppress warnings about ID mapping errors.
				ComputeID: func(
					ctx context.Context, state resource.PropertyMap,
				) (resource.ID, error) {
					return resource.ID("ID"), nil
				},
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
		}, []struct {
			name string
			info tfbridge.ProviderInfo
		}{{info: p, name: "simple"}})

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

		g, err := NewGenerator(GeneratorOptions{
			Package:      info.Name,
			Version:      info.Version,
			Language:     Schema,
			ProviderInfo: info,
			Root:         fs,
			Sink: diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
				Color: colors.Never,
			}),
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
	})
}
