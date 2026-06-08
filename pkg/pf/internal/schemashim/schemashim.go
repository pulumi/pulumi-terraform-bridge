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

// ShimSchemaOnlyProvider gathers cheap PF metadata immediately and defers
// resource, data source, and list resource schema loading until the selected
// schema is used. Eager metadata gathering uses the caller's context. The
// stored construction context is detached from cancellation so a canceled
// startup or gather context cannot poison future lazy schema loads.
func ShimSchemaOnlyProvider(ctx context.Context, provider pfprovider.Provider) shim.Provider {
	if ctx == nil {
		ctx = context.Background()
	}
	lazyCtx := context.WithoutCancel(ctx)
	resources, err := pfutils.GatherResources(ctx, provider, NewSchemaMap)
	if err != nil {
		panic(err)
	}
	dataSources, err := pfutils.GatherDatasources(ctx, provider, NewSchemaMap)
	if err != nil {
		panic(err)
	}
	listResources, err := pfutils.GatherListResources(ctx, provider, NewSchemaMap)
	if err != nil {
		panic(err)
	}
	resourceMap := newSchemaOnlyResourceMap(resources)
	dataSourceMap := newSchemaOnlyDataSourceMap(dataSources)
	listResourceMap := newSchemaOnlyListResourceMap(listResources)
	return &SchemaOnlyProvider{
		ctx:             lazyCtx,
		tf:              provider,
		resourceMap:     resourceMap,
		dataSourceMap:   dataSourceMap,
		listResourceMap: listResourceMap,
	}
}
