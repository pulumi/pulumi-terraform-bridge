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

package tfbridgetests

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
)

// Take a simple PF-based provider resource through a basic life-cycle: create, update, replace, delete.
func Test_BasicLifecycle(t *testing.T) {
	t.Parallel()

	r := pb.NewResource(pb.NewResourceArgs{
		Name: "r",
		CreateFunc: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
			diags := resp.State.SetAttribute(ctx, path.Root("id"), "id0")
			resp.Diagnostics = append(resp.Diagnostics, diags...)

			var p string
			diags = req.Plan.GetAttribute(ctx, path.Root("p"), &p)
			resp.Diagnostics = append(resp.Diagnostics, diags...)

			var re *string
			diags = req.Plan.GetAttribute(ctx, path.Root("re"), &re)
			resp.Diagnostics = append(resp.Diagnostics, diags...)

			diags = resp.State.SetAttribute(ctx, path.Root("p"), p)
			resp.Diagnostics = append(resp.Diagnostics, diags...)

			if re != nil {
				diags = resp.State.SetAttribute(ctx, path.Root("re"), re)
				resp.Diagnostics = append(resp.Diagnostics, diags...)
			}
		},
		UpdateFunc: func(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
			var p string
			diags := req.Plan.GetAttribute(ctx, path.Root("p"), &p)
			resp.Diagnostics = append(resp.Diagnostics, diags...)

			var re *string
			diags = req.Plan.GetAttribute(ctx, path.Root("re"), &re)
			resp.Diagnostics = append(resp.Diagnostics, diags...)

			diags = resp.State.SetAttribute(ctx, path.Root("p"), p)
			resp.Diagnostics = append(resp.Diagnostics, diags...)

			if re != nil {
				diags = resp.State.SetAttribute(ctx, path.Root("re"), *re)
				resp.Diagnostics = append(resp.Diagnostics, diags...)
			}
		},
		ResourceSchema: rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				"id": rschema.StringAttribute{
					Computed: true,
					PlanModifiers: []planmodifier.String{
						stringplanmodifier.UseStateForUnknown(),
					},
				},
				"p": rschema.StringAttribute{Optional: true},
				"re": rschema.StringAttribute{
					Optional: true,
					PlanModifiers: []planmodifier.String{
						stringplanmodifier.RequiresReplace(),
					},
				},
			},
		},
	})

	provider := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{r},
	})

	programYAML := `
        name: test-program
        runtime: yaml
        resources:
          my-res:
            type: testprovider:index:R
            properties:
              p: "FOO"
        `

	test := newPulumiTest(t, provider, programYAML)

	previewAndUpdate := func(name, prog string, expectPreviewChanges, expectChanges autogold.Value) {
		test.WritePulumiYaml(t, prog)

		previewResult := test.Preview(t, optpreview.Diff())
		t.Logf("%s preview: %s", name, previewResult.StdOut+previewResult.StdErr)
		expectPreviewChanges.Equal(t, previewResult.ChangeSummary)

		upResult := test.Up(t)
		t.Logf("%s up: %s", name, upResult.StdOut+upResult.StdErr)
		expectChanges.Equal(t, upResult.Summary.ResourceChanges)

		t.Logf("STATE: %s", test.ExportStack(t).Deployment)
	}

	previewAndUpdate(
		"initial",
		programYAML,
		autogold.Expect(map[apitype.OpType]int{apitype.OpType("create"): 2}),
		autogold.Expect(&map[string]int{"create": 2}),
	)

	previewAndUpdate(
		"empty",
		programYAML,
		autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 2}),
		autogold.Expect(&map[string]int{"same": 2}),
	)

	programYAML2 := `
        name: test-program
        runtime: yaml
        resources:
          my-res:
            type: testprovider:index:R
            properties:
              p: "BAR"
        `

	previewAndUpdate(
		"update",
		programYAML2,
		autogold.Expect(map[apitype.OpType]int{apitype.OpType("same"): 1, apitype.OpType("update"): 1}),
		autogold.Expect(&map[string]int{"same": 1, "update": 1}),
	)

	programYAML3 := `
        name: test-program
        runtime: yaml
        resources:
          my-res:
            type: testprovider:index:R
            properties:
              p: "BAR"
              re: "replace-me"
        `

	previewAndUpdate(
		"replace",
		programYAML3,
		autogold.Expect(map[apitype.OpType]int{
			apitype.OpType("replace"): 1,
			apitype.OpType("same"):    1,
		}),
		autogold.Expect(&map[string]int{"replace": 1, "same": 1}),
	)
}
