// Copyright 2016-2024, Pulumi Corporation.
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

package pf

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// ShimProvider is a shim.Provider that is capable of serving the
// [github.com/pulumi/pulumi-terraform-bridge/pf] aspect of the bridge.
//
// The interface for ShimProvider is considered unstable, and may change at any time.
type ShimProvider interface {
	shim.Provider

	Server(context.Context) (tfprotov6.ProviderServer, error)
	Resources(context.Context) (pfutils.Resources, error)
	DataSources(context.Context) (pfutils.DataSources, error)
	Config(context.Context) (tftypes.Object, error)
}
