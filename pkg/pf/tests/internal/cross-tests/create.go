// Copyright 2016-2025, Pulumi Corporation.
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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl/hclwrite"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func Create(t *testing.T, res pb.Resource, tfConfig map[string]cty.Value, options ...CreateOption) {
	var opts createOpts
	for _, f := range options {
		f(&opts)
	}

	type result struct {
		plan   tftypes.Value
		config tftypes.Value
		wasSet bool
	}

	makeResource := func(writeTo *result) pb.Resource {
		return pb.Resource{
			Name:           res.Name,
			ResourceSchema: res.ResourceSchema,
			ModifyPlanFunc: res.ModifyPlanFunc,
			CreateFunc: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
				res.CreateFunc(ctx, req, resp)
				*writeTo = result{req.Plan.Raw, req.Config.Raw, true}
			},
		}
	}

	var tfResult, puResult result

	tfProvider := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{makeResource(&tfResult)},
	})

	var hcl bytes.Buffer

	sch := hclSchemaPFResource(res.ResourceSchema)
	err := hclwrite.WriteResource(&hcl, sch, "testprovider_test", "res", tfConfig,
		hclwrite.WithCreateBeforeDestroy(true))
	require.NoError(t, err)
	t.Logf("HCL:\n%s", hcl.String())
	tfwd := t.TempDir()
	runTFPlanApply(t, tfProvider, tfwd, hcl.String())
	require.True(t, tfResult.wasSet, "terraform result result was not set")

	// Create a new provider with the pulumi result
	tfProvider = pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{makeResource(&puResult)},
	})
	shimProvider := tfbridge.ShimProvider(tfProvider)

	puConfig := crosstestsimpl.InferPulumiValue(t,
		shimProvider.ResourcesMap().Get("testprovider_test").Schema(),
		opts.resourceInfo,
		cty.ObjectVal(tfConfig),
	)
	pulumiYaml := yamlResource(t, puConfig)
	bytes, err := yaml.Marshal(pulumiYaml)
	require.NoError(t, err)
	t.Logf("Pulumi.yaml:\n%s", string(bytes))

	pt, err := pulcheck.PulCheck(t, tfProvider.ToProviderInfo(), string(bytes))
	require.NoError(t, err)
	pt.Up(t)

	require.True(t, puResult.wasSet, "pulumi result was not set")

	// Compare the result
	if assert.True(t, tfResult.wasSet) && assert.True(t, puResult.wasSet) {
		assert.Equal(t, tfResult.plan, puResult.plan, "plan")
		assert.Equal(t, tfResult.config, puResult.config, "config")
	}
}

type CreateOption func(*createOpts)

type createOpts struct {
	resourceInfo map[string]*info.Schema
}

func CreateResourceInfo(info map[string]*info.Schema) CreateOption {
	return func(o *createOpts) {
		o.resourceInfo = info
	}
}
