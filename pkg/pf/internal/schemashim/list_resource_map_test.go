// Copyright 2026, Pulumi Corporation.
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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/stretchr/testify/assert"
)

func TestShimSchemaOnlyProviderWithoutListResources(t *testing.T) {
	t.Parallel()

	shimmed := ShimSchemaOnlyProvider(context.Background(), providerWithoutListResources{}).(*SchemaOnlyProvider)

	assert.Equal(t, 0, shimmed.ListResourcesMap().Len())
}

type providerWithoutListResources struct{}

func (providerWithoutListResources) Metadata(
	context.Context, provider.MetadataRequest, *provider.MetadataResponse,
) {
}

func (providerWithoutListResources) Schema(
	context.Context, provider.SchemaRequest, *provider.SchemaResponse,
) {
}

func (providerWithoutListResources) Configure(
	context.Context, provider.ConfigureRequest, *provider.ConfigureResponse,
) {
}

func (providerWithoutListResources) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

func (providerWithoutListResources) Resources(context.Context) []func() resource.Resource {
	return nil
}
