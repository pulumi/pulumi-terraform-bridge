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

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	pb "github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/providerbuilder"
	br "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
)

func TestCreateResourceWithDynamicAttribute(t *testing.T) {
	r := pb.Resource{
		Name: "myres",
		CreateFunc: func(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
			panic("!")
		},
		ResourceSchema: schema.Schema{
			Attributes: map[string]schema.Attribute{
				"manifest": schema.DynamicAttribute{},
			},
		},
	}

	p := pb.NewProvider(pb.NewProviderArgs{
		AllResources: []pb.Resource{r},
	})

	bridgedProvider := pulcheck.QuickProvider(t, "testprovider", br.ShimProvider(p))
	pulcheck.PulCheck(t, bridgedProvider, "program")
}
