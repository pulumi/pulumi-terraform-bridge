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
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/convert"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Invoke dynamically executes a built-in function in the provider.
func (p *Provider) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
	ctx := context.TODO()

	handle, err := p.datasourceHandle(ctx, tok)
	if err != nil {
		return nil, nil, err
	}

	typ := handle.schema.Type().TerraformType(ctx).(tftypes.Object)

	config, err := convert.EncodePropertyMapToDynamic(handle.encoder, typ, args)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot encode config to call ReadDataSource for %q: %w",
			handle.terraformDataSourceName, err)
	}

	if err := p.validateDataResourceConfig(ctx, handle, config); err != nil {
		return nil, nil, err
	}

	outputs, err := p.readDataSource(ctx, handle, config)
	return outputs, nil, err
}

func (p *Provider) validateDataResourceConfig(ctx context.Context, handle datasourceHandle,
	config *tfprotov6.DynamicValue) error {
	req := &tfprotov6.ValidateDataResourceConfigRequest{
		TypeName: handle.terraformDataSourceName,
		Config:   config,
	}
	resp, err := p.tfServer.ValidateDataResourceConfig(ctx, req)
	if err != nil {
		return fmt.Errorf("error calling ValidateDataResourceConfig: %w", err)
	}
	// TODO try to map resp.Diagnostics into CheckFailure instead of just error messages.
	return p.processDiagnostics(resp.Diagnostics)
}

func (p *Provider) readDataSource(ctx context.Context, handle datasourceHandle,
	config *tfprotov6.DynamicValue) (resource.PropertyMap, error) {
	typ := handle.schema.Type().TerraformType(ctx).(tftypes.Object)

	req := &tfprotov6.ReadDataSourceRequest{
		Config:   config,
		TypeName: handle.terraformDataSourceName,
		// TODO ProviderMeta
	}

	resp, err := p.tfServer.ReadDataSource(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error calling ReadDataSource: %w", err)
	}

	if err := p.processDiagnostics(resp.Diagnostics); err != nil {
		return nil, err
	}

	propertyMap, err := convert.DecodePropertyMapFromDynamic(handle.decoder, typ, resp.State)
	if err != nil {
		return nil, fmt.Errorf("cannot decode state from a call to ReadDataSource for %q: %w",
			handle.terraformDataSourceName, err)
	}

	return propertyMap, nil
}
