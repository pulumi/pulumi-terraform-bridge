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
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/pulumi/providertest/providers"
	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/assume"
	crosstests "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/cross-tests"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"
)

// MakeConfigure returns a [testing] subtest of [Configure].
//
//	func TestMyProperty(t *testing.T) {
//		t.Run("my-subtest", crosstests.MakeConfigure(schema, tfConfig, puConfig))
//	}
//
// For details on the test itself, see [Configure].
func MakeConfigure(
	schema schema.Schema, tfConfig map[string]cty.Value, puConfig resource.PropertyMap,
	options ...ConfigureOption,
) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		Configure(t, schema, tfConfig, puConfig, options...)
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
func Configure(
	t TestingT, schema schema.Schema, tfConfig map[string]cty.Value, puConfig resource.PropertyMap,
	options ...ConfigureOption,
) {
	assume.TerraformCLI(t)

	var opts configureOptions
	for _, o := range options {
		o(&opts)
	}

	const providerName = "test"

	prov := func(config *tfsdk.Config) *pb.Provider {
		return pb.NewProvider(pb.NewProviderArgs{
			TypeName:       providerName,
			ProviderSchema: schema,
			ConfigureFunc: func(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
				*config = req.Config
			},
			AllResources: []pb.Resource{{
				Name: "res",
			}},
		})
	}

	var tfOutput, puOutput tfsdk.Config
	var runOnFail []func(t TestingT)

	logf := func(msg string, a ...any) { runOnFail = append(runOnFail, func(t TestingT) { t.Logf(msg, a...) }) }

	withAugmentedT(t, func(t *augmentedT) { // --- Run Terraform Provider ---
		var hcl bytes.Buffer
		err := crosstests.WritePF(&hcl).Provider(schema, providerName, tfConfig)
		require.NoError(t, err)
		// TF does not configure providers unless they are involved with creating
		// a resource or datasource, so we create "res" to give the TF provider a
		// reason to be configured.
		hcl.WriteString(`
resource "` + providerName + `_res" "res" {}
`)

		prov := prov(&tfOutput)
		driver := tfcheck.NewTfDriver(t, t.TempDir(), prov.TypeName, prov)

		driver.Write(t, hcl.String())
		plan, err := driver.Plan(t)
		require.NoError(t, err, "failed to generate TF plan")
		err = driver.Apply(t, plan)
		require.NoError(t, err)
	})

	withAugmentedT(t, func(t *augmentedT) { // --- Run Pulumi Provider ---
		dir := t.TempDir()

		pulumiYaml := map[string]any{
			"name":    "project",
			"runtime": "yaml",
			"backend": map[string]any{
				"url": "file://./data",
			},
			"resources": map[string]any{
				"p": map[string]any{
					"type":       "pulumi:providers:" + providerName,
					"properties": convertResourceValue(t, puConfig),
				},
			},
		}

		bytes, err := yaml.Marshal(pulumiYaml)
		require.NoError(t, err)
		logf("Pulumi.yaml:\n%s", string(bytes))
		err = os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), bytes, 0600)
		require.NoError(t, err)

		makeProvider := func(providers.PulumiTest) (pulumirpc.ResourceProviderServer, error) {
			ctx, sink := context.Background(), testLogSink{t}
			p := info.Provider{
				Name:             providerName,
				P:                tfbridge.ShimProvider(prov(&puOutput)),
				Version:          "0.1.0-dev",
				UpstreamRepoPath: ".",
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
	})

	// --- Compare results -----------------------------
	if opts.testEqual != nil {
		opts.testEqual(t, tfOutput, puOutput)
	} else {
		assert.Equal(t, tfOutput, puOutput)
	}

	if t.Failed() {
		for _, f := range runOnFail {
			f(t)
		}
	}
}

// An option for configuring [Configure] or [MakeConfigure].
//
// Existing options are:
// - [WithConfigureEquals]
type ConfigureOption func(*configureOptions)

type configureOptions struct {
	testEqual func(t TestingT, tfOutput, puOutput tfsdk.Config)
}

// WithConfigureEqual defines a comparison function for the cross-test.
//
// This function is called after both the Terraform and Pulumi portions have run, and is
// responsible for asserting that the results match.
//
// Here are 2 examples:
//
//	// Assert that both Terraform and Pulumi ran, but do not assert anything about their behavior.
//	WithConfigureEqual(func(t TestingT, tfOutput, puOutput tfsdk.Config) {})
//
//	// Assert that the underlying provider witnessed saw could not distinguish between
//	// the direct and bridged call (the default behavior).
//	WithConfigureEqual(func(t TestingT, tfOutput, puOutput tfsdk.Config) {
//		assert.Equal(t, tfOutput, puOutput)
//	})
//
// WithConfigureEqual should be used only when the direct and bridged providers don't
// agree, to limit the scope of the test so it can be checked in. In general, usage should
// be accompanied by a bridge issue to track the discrepancy.
func WithConfigureEqual(equal func(t TestingT, tfOutput, puOutput tfsdk.Config)) ConfigureOption {
	return func(opts *configureOptions) { opts.testEqual = equal }
}
