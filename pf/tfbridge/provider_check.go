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
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/defaults"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// Check validates the given resource inputs from the user program and computes checked inputs that fill out default
// values. The checked inputs are then passed to subsequent, Diff, Create, or Update.
func (p *provider) CheckWithContext(
	ctx context.Context,
	urn resource.URN,
	priorState resource.PropertyMap,
	inputs resource.PropertyMap,
	allowUnknowns bool,
	randomSeed []byte,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	ctx = p.initLogging(ctx, p.logSink, urn)

	checkedInputs := inputs.Copy()

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return checkedInputs, []plugin.CheckFailure{}, err
	}

	if info := rh.pulumiResourceInfo; info != nil {
		if check := info.PreCheckCallback; check != nil {
			var err error
			checkedInputs, err = check(ctx, checkedInputs, p.lastKnownProviderConfig.Copy())
			if err != nil {
				return checkedInputs, []plugin.CheckFailure{}, err
			}
		}
	}

	// Transform checkedInputs to apply Pulumi-level defaults.
	news := defaults.ApplyDefaultInfoValues(ctx, defaults.ApplyDefaultInfoValuesArgs{
		SchemaMap:   rh.schemaOnlyShimResource.Schema(),
		SchemaInfos: rh.pulumiResourceInfo.Fields,
		ComputeDefaultOptions: tfbridge.ComputeDefaultOptions{
			URN:        urn,
			Properties: checkedInputs,
			Seed:       randomSeed,
			PriorState: priorState,
		},
		PropertyMap:    checkedInputs,
		ProviderConfig: p.lastKnownProviderConfig,
	})

	checkFailures, err := p.validateResourceConfig(ctx, urn, rh, news)

	schemaMap := rh.schemaOnlyShimResource.Schema()
	schemaInfos := rh.pulumiResourceInfo.GetFields()
	news = tfbridge.MarkSchemaSecrets(ctx, schemaMap, schemaInfos, resource.NewObjectProperty(news)).ObjectValue()

	if err != nil {
		return news, checkFailures, err
	}

	return news, checkFailures, nil
}

func (p *provider) validateResourceConfig(
	ctx context.Context,
	urn resource.URN,
	rh resourceHandle,
	inputs resource.PropertyMap,
) ([]plugin.CheckFailure, error) {

	tfType := rh.schema.Type().TerraformType(ctx).(tftypes.Object)

	encodedInputs, err := convert.EncodePropertyMapToDynamic(rh.encoder, tfType, inputs)
	if err != nil {
		return nil, fmt.Errorf("cannot encode resource inputs to call ValidateResourceConfig: %w", err)
	}

	req := tfprotov6.ValidateResourceConfigRequest{
		TypeName: rh.terraformResourceName,
		Config:   encodedInputs,
	}

	resp, err := p.tfServer.ValidateResourceConfig(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("error calling ValidateResourceConfig: %w", err)
	}

	schemaMap := rh.schemaOnlyShimResource.Schema()
	schemaInfos := rh.pulumiResourceInfo.GetFields()

	checkFailures := []plugin.CheckFailure{}
	remainingDiagnostics := []*tfprotov6.Diagnostic{}
	for _, diag := range resp.Diagnostics {
		if k := p.detectCheckFailure(ctx, urn, false /*isProvider*/, schemaMap, schemaInfos, diag); k != nil {
			checkFailures = append(checkFailures, *k)
			continue
		}
		remainingDiagnostics = append(remainingDiagnostics, diag)
	}

	if err := p.processDiagnostics(remainingDiagnostics); err != nil {
		return nil, err
	}

	return checkFailures, nil
}
