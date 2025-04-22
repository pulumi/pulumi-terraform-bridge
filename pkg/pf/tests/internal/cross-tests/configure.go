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

package crosstests

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests"
	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl/hclwrite"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
)

// MakeConfigure returns a [testing] subtest of [Configure].
//
//	func TestMyProperty(t *testing.T) {
//		t.Run("my-subtest", crosstests.MakeConfigure(schema, tfConfig, puConfig))
//	}
//
// For details on the test itself, see [Configure].
func MakeConfigure(schema pschema.Schema, tfConfig map[string]cty.Value, options ...ConfigureOption) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		Configure(t, schema, tfConfig, options...)
	}
}

// Configure will assert that a provider who's configuration is described by
// schema will observe the same inputs when configured in via HCL with the inputs
// tfInputs and when bridged and configured with Pulumi and puInputs.
//
// The idea is that the "Configured Provider" should not be able to tell if it was configured
// via HCL or Pulumi YAML:
//
//	+--------------------+                      +---------------------+
//	| Terraform Provider |--------------------->| Configure(tfInputs) |
//	+--------------------+                      +---------------------+
//	          |                                                        \
//	          |                                                         \
//	          |                                                          \
//	          |                                                      +---------------------+
//	          | tfbridge.ShimProvider                                | Configured Provider |
//	          |                                                      +---------------------+
//	          |                                                          /
//	          |                                                         /
//	          V                                                        /
//	+--------------------+                      +---------------------+
//	|   Pulumi Provider  |--------------------->| Configure(puInputs) |
//	+--------------------+                      +---------------------+
//
// Configure should be safe to run in parallel.
func Configure(t T, schema pschema.Schema, tfConfig map[string]cty.Value, options ...ConfigureOption) {
	SkipUnlessLinux(t)

	var opts configureOpts
	for _, f := range options {
		f(&opts)
	}

	const providerName = "test"

	provbuilder := func(config *tfsdk.Config) *pb.Provider {
		return pb.NewProvider(pb.NewProviderArgs{
			TypeName:       providerName,
			ProviderSchema: schema,
			ConfigureFunc: func(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
				*config = req.Config
			},
			AllResources: []pb.Resource{
				pb.NewResource(pb.NewResourceArgs{
					Name: "res",
				}),
			},
		})
	}

	var tfOutput, puOutput tfsdk.Config
	// Run the TF part
	{
		var hcl bytes.Buffer
		err := hclwrite.WriteProvider(&hcl, hclSchemaPFProvider(schema), providerName, tfConfig)
		require.NoError(t, err)
		// TF does not configure providers unless they are involved with creating
		// a resource or datasource, so we create "res" to give the TF provider a
		// reason to be configured.
		hcl.WriteString(`
resource "` + providerName + `_res" "res" {}
`)

		prov := provbuilder(&tfOutput)
		driver := tfcheck.NewTfDriver(t, t.TempDir(), prov.TypeName, tfcheck.NewTFDriverOpts{
			V6Provider: prov,
		})

		driver.Write(t, hcl.String())
		plan, err := driver.Plan(t)
		require.NoError(t, err)
		err = driver.ApplyPlan(t, plan)
		require.NoError(t, err)
	}

	// Run the Pulumi part
	{
		dir := t.TempDir()

		var puConfig resource.PropertyMap
		if opts.puConfig != nil {
			puConfig = *opts.puConfig
		} else {
			puConfig = crosstestsimpl.InferPulumiValue(t,
				tfbridge.ShimProvider(provbuilder(nil)).Schema(),
				opts.resourceInfo,
				cty.ObjectVal(tfConfig),
			)
		}

		pulumiYaml := map[string]any{
			"name":    "project",
			"runtime": "yaml",
			"backend": map[string]any{
				"url": "file://./data",
			},
			"resources": map[string]any{
				"p": map[string]any{
					"type":       "pulumi:providers:" + providerName,
					"properties": crosstests.ConvertResourceValue(t, puConfig),
				},
				"_": map[string]any{
					"type": providerName + ":Res",
					"options": map[string]any{
						"provider": "${p}",
					},
				},
			},
		}

		bytes, err := yaml.Marshal(pulumiYaml)
		require.NoError(t, err)
		t.Logf("Pulumi.yaml:\n%s", string(bytes))
		err = os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), bytes, 0o600)
		require.NoError(t, err)

		makeProvider := func(providers.PulumiTest) (pulumirpc.ResourceProviderServer, error) {
			ctx, sink := context.Background(), logging.NewTestingSink(t)

			p := info.Provider{
				Name:             providerName,
				P:                tfbridge.ShimProvider(provbuilder(&puOutput)),
				Version:          "0.1.0-dev",
				UpstreamRepoPath: ".",
				Config:           opts.resourceInfo,
			}
			p.MustComputeTokens(tokens.SingleModule(providerName, "index", tokens.MakeStandard(providerName)))

			for _, v := range p.DataSources {
				v.Docs = &info.Doc{Markdown: []byte{' '} /* don't warn the user that docs cannot be found */}
			}
			for _, v := range p.Resources {
				v.Docs = &info.Doc{Markdown: []byte{' '} /* don't warn the user that docs cannot be found */}
			}
			schema, err := tfgen.GenerateSchema(ctx, tfgen.GenerateSchemaOptions{
				ProviderInfo: p,
			})
			if err != nil {
				return nil, err
			}

			p.MetadataInfo = &info.Metadata{Path: "non-empty"}
			return tfbridge.NewProviderServer(ctx, sink, p, tfbridge.ProviderMetadata{
				PackageSchema: schema.ProviderMetadata.PackageSchema,
			})
		}

		test := pulumitest.NewPulumiTest(t, dir,
			opttest.AttachProviderServer(providerName, makeProvider),
			opttest.SkipInstall(),
			opttest.Env("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true"),
		)
		contract.Ignore(test.Preview(t)) // Assert that the preview succeeded, but not the result.
		contract.Ignore(test.Up(t))      // Assert that the update succeeded, but not the result.
	}

	assert.Equal(t, tfOutput, puOutput)
}

type configureOpts struct {
	resourceInfo map[string]*info.Schema
	puConfig     *resource.PropertyMap
}

type ConfigureOption func(*configureOpts)

// CreateResourceInfo specifies a map of [info.Schema] to apply to the provider under test.
func ConfigureProviderInfo(info map[string]*info.Schema) ConfigureOption {
	return func(o *configureOpts) { o.resourceInfo = info }
}

// ConfigurePulumiConfig specifies an explicit pulumi value for the configure call.
func ConfigurePulumiConfig(config resource.PropertyMap) ConfigureOption {
	return func(o *configureOpts) { o.puConfig = &config }
}
