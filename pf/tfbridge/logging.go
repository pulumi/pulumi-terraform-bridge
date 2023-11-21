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

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
)

// Configures logging. Note that urn is optional but useful to identify logs with resources.
//
// See https://developer.hashicorp.com/terraform/plugin/log/writing
func (p *provider) initLogging(ctx context.Context, sink logging.Sink, urn resource.URN) context.Context {
	return logging.InitLogging(ctx, logging.LogOptions{
		LogSink:         sink,
		URN:             urn,
		ProviderName:    p.info.Name,
		ProviderVersion: p.info.Version,
	})
}
