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
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/convert"
)

// Configure configures the resource provider with "globals" that control its behavior.
func (p *provider) ConfigureWithContext(ctx context.Context, inputs resource.PropertyMap) error {
	ctx = p.initLogging(ctx, p.logSink, "")

	config, err := convert.EncodePropertyMapToDynamic(p.configEncoder, p.configType, inputs)
	if err != nil {
		return fmt.Errorf("cannot encode provider configuration to call ConfigureProvider: %w", err)
	}

	req := &tfprotov6.ConfigureProviderRequest{
		Config:           config,
		TerraformVersion: "pulumi-terraform-bridge",
	}

	resp, err := p.tfServer.ConfigureProvider(ctx, req)
	if err != nil {
		return fmt.Errorf("error calling ConfigureProvider: %w", err)
	}

	return p.processDiagnostics(resp.Diagnostics)
}
