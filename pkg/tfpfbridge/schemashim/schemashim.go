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

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func ShimSchemaOnlyProvider(ctx context.Context, provider pfprovider.Provider) shim.Provider {
	return &schemaOnlyProvider{
		ctx: ctx,
		tf:  provider,
	}
}

func ShimSchemaOnlyProviderInfo(ctx context.Context, provider info.ProviderInfo) tfbridge.ProviderInfo {
	shimProvider := ShimSchemaOnlyProvider(ctx, provider.P())
	return tfbridge.ProviderInfo{
		P:         shimProvider,
		Name:      provider.Name,
		Resources: convertResourceMetadata(provider.Resources),
	}
}

func convertResourceMetadata(inputs map[string]*info.ResourceInfo) map[string]*tfbridge.ResourceInfo {
	result := map[string]*tfbridge.ResourceInfo{}
	for k, v := range inputs {
		result[k] = &tfbridge.ResourceInfo{
			Tok: v.Tok,
		}
	}
	return result
}
