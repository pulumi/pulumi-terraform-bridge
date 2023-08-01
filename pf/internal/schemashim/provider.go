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

package schemashim

import (
	"context"

	pfprovider "github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/util"
)

type SchemaOnlyProvider struct {
	util.UnimplementedProvider
	ctx context.Context
	tf  pfprovider.Provider
}

func (p *SchemaOnlyProvider) PfProvider() pfprovider.Provider {
	return p.tf
}

var _ shim.Provider = (*SchemaOnlyProvider)(nil)

func (p *SchemaOnlyProvider) Schema() shim.SchemaMap {
	ctx := p.ctx
	schemaResp := &pfprovider.SchemaResponse{}
	p.tf.Schema(ctx, pfprovider.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		panic("Schema() returned error diags")
	}
	return newSchemaMap(pfutils.FromProviderSchema(schemaResp.Schema))
}

func (p *SchemaOnlyProvider) ResourcesMap() shim.ResourceMap {
	resources, err := pfutils.GatherResources(context.TODO(), p.tf)
	if err != nil {
		panic(err)
	}
	return &schemaOnlyResourceMap{resources}
}

func (p *SchemaOnlyProvider) DataSourcesMap() shim.ResourceMap {
	dataSources, err := pfutils.GatherDatasources(context.TODO(), p.tf)
	if err != nil {
		panic(err)
	}
	return &schemaOnlyDataSourceMap{dataSources}
}
