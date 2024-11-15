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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func ShimSchemaOnlyProvider(ctx context.Context, provider pfprovider.Provider) shim.Provider {
	resources, err := pfutils.GatherResources(ctx, provider, NewSchemaMap)
	if err != nil {
		panic(err)
	}
	dataSources, err := pfutils.GatherDatasources(ctx, provider, NewSchemaMap)
	if err != nil {
		panic(err)
	}
	resourceMap := newSchemaOnlyResourceMap(resources)
	dataSourceMap := newSchemaOnlyDataSourceMap(dataSources)
	return &SchemaOnlyProvider{
		ctx:           ctx,
		tf:            provider,
		resourceMap:   resourceMap,
		dataSourceMap: dataSourceMap,
	}
}
