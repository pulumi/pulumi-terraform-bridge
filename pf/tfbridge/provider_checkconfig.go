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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/defaults"
)

// CheckConfig validates the configuration for this resource provider.
func (p *provider) CheckConfigWithContext(ctx context.Context, urn resource.URN,
	olds, news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {
	ctx = p.initLogging(ctx, p.logSink, urn)

	// Transform checkedInputs to apply Pulumi-level defaults.
	newsWithDefaults := defaults.ApplyDefaultInfoValues(ctx, defaults.ApplyDefaultInfoValuesArgs{
		TopSchemaMap:   p.schemaOnlyProvider.Schema(),
		TopFieldInfos:  p.info.Config,
		PropertyMap:    news,
		ProviderConfig: news,
	})

	p.lastKnownProviderConfig = newsWithDefaults

	// TODO[pulumi/pulumi-terraform-bridge#821] validate provider config
	return newsWithDefaults, []plugin.CheckFailure{}, nil
}
