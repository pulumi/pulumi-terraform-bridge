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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have
// on the provider.
func (p *provider) DiffConfigWithContext(
	ctx context.Context,
	urn resource.URN,
	oldInputs, state, inputs resource.PropertyMap,
	allowUnknowns bool,
	ignoreChanges []string,
) (plugin.DiffResult, error) {
	ctx = p.initLogging(ctx, p.logSink, urn)
	diffConfig := tfbridge.DiffConfig(p.info.P.Schema(), p.info.Config)
	return diffConfig(urn, oldInputs, state, inputs, allowUnknowns, ignoreChanges)
}
