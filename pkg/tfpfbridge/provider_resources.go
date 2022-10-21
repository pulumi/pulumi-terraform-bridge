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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	pulumiresource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
)

func (p *Provider) resources(ctx context.Context) (res resources, err error) {

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
		p.resourcesCache, err = gatherResources(ctx, p.tfProvider)
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

	schema := resources.schemaByTypeName[typeName]

	result := resourceHandle{
		makeResource:          resources.resourceByTypeName[typeName],
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

type resources struct {
	schemaByTypeName   map[string]tfsdk.Schema
	resourceByTypeName map[string]func() pfresource.Resource
	diagnostics        diag.Diagnostics
}

func gatherResources(ctx context.Context, prov provider.Provider) (resources, error) {
	provMetadata := provider.MetadataResponse{}

	if provWithMeta, ok := prov.(provider.ProviderWithMetadata); ok {
		provWithMeta.Metadata(ctx, provider.MetadataRequest{}, &provMetadata)
	}

	rs := resources{
		schemaByTypeName:   map[string]tfsdk.Schema{},
		resourceByTypeName: map[string]func() pfresource.Resource{},
		diagnostics:        diag.Diagnostics{},
	}

	for _, makeResource := range prov.Resources(ctx) {
		res := makeResource()

		resMeta := pfresource.MetadataResponse{}

		res.Metadata(ctx, pfresource.MetadataRequest{
			ProviderTypeName: provMetadata.TypeName,
		}, &resMeta)

		resSchema, diag := res.GetSchema(ctx)

		if diag.HasError() {
			errs := diag.Errors()
			err := fmt.Errorf(
				"Resource %s GetSchema() found %d errors. First error: %s",
				resMeta.TypeName,
				diag.ErrorsCount(),
				errs[0].Summary(),
			)
			return resources{}, err
		}

		rs.diagnostics.Append(diag...)

		rs.schemaByTypeName[resMeta.TypeName] = resSchema
		rs.resourceByTypeName[resMeta.TypeName] = makeResource
	}

	return rs, nil
}
