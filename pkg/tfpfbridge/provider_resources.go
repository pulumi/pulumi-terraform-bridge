// Copyright 2016-2022, Pulumi Corporation.
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

package tfbridge

import (
	"context"

	pfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	pulumiresource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/pfutils"
)

func (p *Provider) resources(ctx context.Context) (res pfutils.Resources, err error) {

	p.resourcesOnce.Do(func() {
		// Somehow this GetProviderSchema call needs to happen
		// at least once to avoid Resource Type Not Found in
		// the tfServer, to init it properly to remember
		// provider name and compute correct resource names
		// like random_integer instead of _integer (unknown
		// provider name).
		if _, e := p.tfServer.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{}); e != nil {
			err = e
		}
		p.resourcesCache, err = pfutils.GatherResources(ctx, p.tfProvider)
	})

	return p.resourcesCache, err
}

func (p Provider) resourceHandle(ctx context.Context, urn pulumiresource.URN) (resourceHandle, error) {
	resources, err := p.resources(ctx)
	if err != nil {
		return resourceHandle{}, err
	}

	typeName, err := p.terraformResourceName(urn.Type())
	if err != nil {
		return resourceHandle{}, err
	}

	n := pfutils.TypeName(typeName)
	schema := resources.Schema(n)

	result := resourceHandle{
		makeResource: func() pfresource.Resource {
			return resources.Resource(n)
		},
		terraformResourceName: typeName,
		schema:                schema,
	}

	if info, ok := p.info.Resources[typeName]; ok {
		result.pulumiResourceInfo = info
	}

	return result, nil
}

type resourceHandle struct {
	makeResource          func() pfresource.Resource
	terraformResourceName string
	schema                tfsdk.Schema
	pulumiResourceInfo    *info.ResourceInfo // optional
}
