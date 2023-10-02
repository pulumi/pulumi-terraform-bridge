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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestConvertViaPulumiCLI(t *testing.T) {
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
				Tok: "simple:index:resource",
				Fields: map[string]*tfbridge.SchemaInfo{
					"input_one": {
						Name: "renamedInput1",
					},
				},
			},
		},
		DataSources: map[string]*tfbridge.DataSourceInfo{},
	}

	simpleResourceTF := `
resource "simple_resource" "a_resource" {
    input_one = "hello"
    input_two = true
}

output "some_output" {
    value = simple_resource.a_resource.result
}`
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
}
