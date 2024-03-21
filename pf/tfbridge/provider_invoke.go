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
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/defaults"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Invoke dynamically executes a built-in function in the provider.
func (p *provider) InvokeWithContext(
	ctx context.Context,
	tok tokens.ModuleMember,
	args resource.PropertyMap,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	ctx = p.initLogging(ctx, p.logSink, "")

	handle, err := p.datasourceHandle(ctx, tok)
	if err != nil {
		return nil, nil, err
	}

	typ := handle.schema.Type().TerraformType(ctx).(tftypes.Object)

	// Transform args to apply Pulumi-level defaults.
	argsWithDefaults := defaults.ApplyDefaultInfoValues(ctx, defaults.ApplyDefaultInfoValuesArgs{
		SchemaMap:      handle.schemaOnlyShim.Schema(),
		SchemaInfos:    handle.pulumiDataSourceInfo.GetFields(),
		PropertyMap:    args,
		ProviderConfig: p.lastKnownProviderConfig,
	})

	config, err := convert.EncodePropertyMapToDynamic(handle.encoder, typ, argsWithDefaults)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot encode config to call ReadDataSource for %q: %w",
			handle.terraformDataSourceName, err)
	}

	if failures, err := p.validateDataResourceConfig(ctx, handle, config); err != nil || len(failures) > 0 {
		return nil, failures, err
	}

	return p.readDataSource(ctx, handle, config)
}

func (p *provider) validateDataResourceConfig(ctx context.Context, handle datasourceHandle,
	config *tfprotov6.DynamicValue) ([]plugin.CheckFailure, error) {
	req := &tfprotov6.ValidateDataResourceConfigRequest{
		TypeName: handle.terraformDataSourceName,
		Config:   config,
	}
	resp, err := p.tfServer.ValidateDataResourceConfig(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error calling ValidateDataResourceConfig: %w", err)
	}
	return p.processInvokeDiagnostics(handle, resp.Diagnostics)
}

func (p *provider) readDataSource(ctx context.Context, handle datasourceHandle,
	config *tfprotov6.DynamicValue) (resource.PropertyMap, []plugin.CheckFailure, error) {

	typ := handle.schema.Type().TerraformType(ctx).(tftypes.Object)

	req := &tfprotov6.ReadDataSourceRequest{
		Config:   config,
		TypeName: handle.terraformDataSourceName,
		// TODO[pulumi/pulumi-terraform-bridge#794] set ProviderMeta
	}

	resp, err := p.tfServer.ReadDataSource(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("error calling ReadDataSource: %w", err)
	}

	failures, err := p.processInvokeDiagnostics(handle, resp.Diagnostics)
	if err != nil || len(failures) > 0 {
		return nil, failures, err
	}

	propertyMap, err := convert.DecodePropertyMapFromDynamic(handle.decoder, typ, resp.State)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot decode state from a call to ReadDataSource for %q: %w",
			handle.terraformDataSourceName, err)
	}

	// TODO[pulumi/pulumi#12710] consuming programs (at lest in Go and YAML) are unable to accept secrets from an
	// Invoke response at the moment. Replace secrets with underlying plain values.
	for k, v := range propertyMap {
		if v.ContainsSecrets() {
			tflog.Debug(ctx, "[pf/tfbridge] Ignoring secret in Invoke result due to pulumi/pulumi#12710",
				map[string]any{
					"property": k,
					"token":    handle.token,
				})
			propertyMap[k] = propertyvalue.RemoveSecrets(v)
		} else {
			propertyMap[k] = v
		}
	}

	return propertyMap, nil, nil
}

func (p *provider) processInvokeDiagnostics(ds datasourceHandle,
	diags []*tfprotov6.Diagnostic) ([]plugin.CheckFailure, error) {
	failures, rest := p.parseInvokePropertyCheckFailures(ds, diags)
	return failures, p.processDiagnostics(rest)
}

// Some of the diagnostics pertain to an individual property and should be returned as plugin.CheckFailure for an
// optimal rendering by Pulumi CLI.
func (p *provider) parseInvokePropertyCheckFailures(ds datasourceHandle, diags []*tfprotov6.Diagnostic) (
	[]plugin.CheckFailure, []*tfprotov6.Diagnostic) {
	rest := []*tfprotov6.Diagnostic{}
	failures := []plugin.CheckFailure{}

	for _, d := range diags {
		if pk, ok := functionPropertyKey(ds, d.Attribute); ok {
			reason := strings.Join([]string{d.Summary, d.Detail}, ": ")
			failure := plugin.CheckFailure{
				Property: pk,
				Reason:   reason,
			}
			failures = append(failures, failure)
			continue
		}
		rest = append(rest, d)
	}

	return failures, rest
}
