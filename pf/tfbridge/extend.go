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

func SchemaOnlyPluginFrameworkProvider(ctx context.Context, p pfprovider.Provider) shim.Provider {
	return schemashim.ShimSchemaOnlyProvider(ctx, p)
}

// AugmentShimWithPF augments an existing shim with a PF provider.Provider.
func AugmentShimWithPF(ctx context.Context, shim shim.Provider, p pfprovider.Provider) shim.Provider {
	return muxer.AugmentShimWithPF(ctx, shim, p)
}
