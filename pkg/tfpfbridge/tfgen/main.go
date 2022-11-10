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

package tfgen

import (
	"context"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/schemashim"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

// Used to implement main() in programs such as pulumi-tfgen-random.
//
// The resulting binary is able to generate Pulumi Package Schema and derived package sources in
// various programming languages supported by Pulumi.
func Main(provider, version string, info info.ProviderInfo) {
	ctx := context.Background()
	shimInfo := schemashim.ShimSchemaOnlyProviderInfo(ctx, info)
	tfgen.Main(provider, version, shimInfo)
}
