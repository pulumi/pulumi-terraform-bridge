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

package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/stretchr/testify/require"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/providerbuilder"
)

// Add coverage for pf.NewMuxProvider.
func TestNewMuxProvider(t *testing.T) {
	t.Parallel()
	createCallCount := 0

	r := pb.NewResource(pb.NewResourceArgs{
		Name: "r",
		CreateFunc: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
			t.Logf("CREATE called: %v", req.Plan.Raw.String())
			diags := resp.State.SetAttribute(ctx, path.Root("id"), "id0")
			resp.Diagnostics = append(resp.Diagnostics, diags...)
			createCallCount++
		},
		ResourceSchema: schema.Schema{
			Attributes: map[string]schema.Attribute{
				"p": schema.StringAttribute{
					Optional: true,
				},
			},
		},
	})

	p := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{r},
	})

	// Providers using newPulumiTest utilize NewMuxProvider and MuxWith to connect up a PF provider.
	pt := newPulumiTest(t, p, `
		name: test-program
		runtime: yaml
		resources:
		  my-res:
		    type: testprovider:index:R
		    properties:
		      p: "FOO"
		`)

	pt.Up(t)

	require.Equal(t, 1, createCallCount)
}
