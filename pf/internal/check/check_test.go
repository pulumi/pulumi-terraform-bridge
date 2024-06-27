// Copyright 2016-2024, Pulumi Corporation.
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

package check

import (
	"bytes"
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	property "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"

	pfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func TestMissingIDProperty(t *testing.T) {
	stderr, err := test(t, tfbridge.ProviderInfo{
		P: pfbridge.ShimProvider(testProvider{}),
	})

	assert.Equal(t, "error: Resource test_res has a problem: no \"id\" attribute. "+
		"To map this resource consider specifying ResourceInfo.ComputeID\n", stderr)

	assert.ErrorContains(t, err, "There were 1 unresolved ID mapping errors")
}

func TestIDWithOverride(t *testing.T) {
	stderr, err := test(t, tfbridge.ProviderInfo{
		P: pfbridge.ShimProvider(testProvider{}),
		Resources: map[string]*tfbridge.ResourceInfo{
			"test_res": {ComputeID: func(context.Context, property.PropertyMap) (property.ID, error) {
				panic("ComputeID")
			}},
		},
	})

	assert.Empty(t, stderr)
	assert.NoError(t, err)
}

func TestMuxedProvider(t *testing.T) {
	stderr, err := test(t, tfbridge.ProviderInfo{
		P: pfbridge.MuxShimWithPF(context.Background(),
			sdkv2.NewProvider(testSDKProvider()),
			testProvider{}),
		Resources: map[string]*tfbridge.ResourceInfo{
			"test_res": {ComputeID: func(context.Context, property.PropertyMap) (property.ID, error) {
				panic("ComputeID")
			}},
		},
	})

	assert.Empty(t, stderr)
	assert.NoError(t, err)
}

func test(t *testing.T, info tfbridge.ProviderInfo) (string, error) {
	var stdout, stderr bytes.Buffer
	sink := diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{
		Color: colors.Never,
	})

	err := Provider(sink, info)

	// We should not write diags to stdout
	assert.Empty(t, stdout.String())

	return stderr.String(), err
}

type (
	testProvider struct{ provider.Provider }
	testResource struct{ resource.Resource }
)

func (testProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource { return testResource{} },
	}
}

func (testProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

func (testProvider) Metadata(_ context.Context, _ provider.MetadataRequest, req *provider.MetadataResponse) {
	req.TypeName = "test"
}

func (testResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{}
}

func (testResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_res"
}

func testSDKProvider() *sdkschema.Provider {
	return &sdkschema.Provider{
		ResourcesMap: map[string]*sdkschema.Resource{
			"test_sdk": {},
		},
		DataSourcesMap: map[string]*sdkschema.Resource{},
	}
}
