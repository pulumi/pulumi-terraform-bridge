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

package tfbridge

import (
	"context"

	pfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/muxer"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// Wrap a PF Provider in a shim.Provider.
//
// Deprecated: This function has been renamed ShimProviderWithContext.
func SchemaOnlyPluginFrameworkProvider(ctx context.Context, p pfprovider.Provider) shim.Provider {
	return schemashim.ShimSchemaOnlyProvider(ctx, p)
}

// MuxShimWithPF initializes a shim.Provider that will server resources from both shim and p.
//
// If shim and p both define the same token, then the value from shim will be used.
//
// To create a muxed provider, ProviderInfo.P must be the result of this function.
func MuxShimWithPF(ctx context.Context, shim shim.Provider, p pfprovider.Provider) shim.Provider {
	return muxer.AugmentShimWithPF(ctx, shim, p)
}

// MuxShimWithDisjointgPF initializes a shim.Provider that will server resources from both shim and p.
//
// This function will panic if shim and p both define the same token.
//
// To create a muxed provider, ProviderInfo.P must be the result of this function.
func MuxShimWithDisjointgPF(ctx context.Context, shim shim.Provider, p pfprovider.Provider) shim.Provider {
	return muxer.AugmentShimWithDisjointPF(ctx, shim, p)
}
