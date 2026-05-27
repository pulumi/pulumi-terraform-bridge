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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/defaults"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
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
	autonaming *info.ComputeDefaultAutonamingOptions,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	ctx = p.initLogging(ctx, p.logSink, urn)

	checkedInputs := inputs.Copy()

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return checkedInputs, []plugin.CheckFailure{}, err
	}

	priorState, err = transformFromState(ctx, rh, priorState)
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
			Autonaming: autonaming,
		},
		PropertyMap:    checkedInputs,
		ProviderConfig: p.lastKnownProviderConfig,
	})

	schemaMap := rh.schemaOnlyShimResource.Schema()
	schemaInfos := rh.pulumiResourceInfo.GetFields()

	// Identify keys auto-defaulted by ApplyDefaultInfoValues (via AutoName) so we can
	// roll them back if they trigger upstream ConflictsWith validation. The bridge
	// cannot read ConflictsWith from the protocol-level schema, so the only way to
	// learn about a conflict is to call ValidateResourceConfig.
	//
	// Compare against checkedInputs (post-PreCheckCallback), not raw inputs, so a
	// value supplied by a PreCheckCallback isn't mistaken for an AutoName default
	// and silently stripped when it triggers ConflictsWith.
	autoDefaulted := autoDefaultedKeys(checkedInputs, news, schemaMap, schemaInfos)

	news, checkFailures, err := p.validateResourceConfig(ctx, urn, rh, news, autoDefaulted)

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
	autoDefaulted map[resource.PropertyKey]bool,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	tfType := rh.schema.Type(ctx).(tftypes.Object)

	encodedInputs, err := convert.EncodePropertyMapToDynamic(rh.encoder, tfType, inputs)
	if err != nil {
		return inputs, nil, fmt.Errorf("cannot encode resource inputs to call ValidateResourceConfig: %w", err)
	}

	req := tfprotov6.ValidateResourceConfigRequest{
		TypeName: rh.terraformResourceName,
		Config:   encodedInputs,
		ClientCapabilities: &tfprotov6.ValidateResourceConfigClientCapabilities{
			WriteOnlyAttributesAllowed: true,
		},
	}

	resp, err := p.tfServer.ValidateResourceConfig(ctx, &req)
	if err != nil {
		return inputs, nil, fmt.Errorf("error calling ValidateResourceConfig: %w", err)
	}

	schemaMap := rh.schemaOnlyShimResource.Schema()
	schemaInfos := rh.pulumiResourceInfo.GetFields()

	// If any ConflictsWith failure targets a property we auto-defaulted (e.g. an
	// AutoName injected `name`), drop it from the inputs and re-validate once.
	// The bridge cannot see ConflictsWith metadata in the protocol-level schema,
	// so this is the only way to honour it for dynamic providers.
	if len(autoDefaulted) > 0 {
		stripped := stripConflictingAutoDefaults(inputs, autoDefaulted, resp.Diagnostics, schemaMap, schemaInfos)
		if stripped != nil {
			return p.validateResourceConfig(ctx, urn, rh, stripped, nil)
		}
	}

	checkFailures := []plugin.CheckFailure{}
	remainingDiagnostics := []*tfprotov6.Diagnostic{}
	for _, diag := range resp.Diagnostics {
		if k := p.detectCheckFailure(ctx, urn, false /*isProvider*/, schemaMap, schemaInfos, diag); k != nil {
			checkFailures = append(checkFailures, *k)
			continue
		}
		remainingDiagnostics = append(remainingDiagnostics, diag)
	}

	sc := &schemaContext{schemaMap: schemaMap, schemaInfos: schemaInfos}
	if err := p.processDiagnosticsWithContext(ctx, remainingDiagnostics, sc); err != nil {
		return inputs, nil, err
	}

	return inputs, checkFailures, nil
}

// autoDefaultedKeys returns the property keys that ApplyDefaultInfoValues added to
// news via an AutoName default (the user did not provide them in inputs).
func autoDefaultedKeys(
	inputs, news resource.PropertyMap,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
) map[resource.PropertyKey]bool {
	out := map[resource.PropertyKey]bool{}
	for tfKey, info := range schemaInfos {
		if info == nil || info.Default == nil || !info.Default.AutoNamed {
			continue
		}
		pk := resource.PropertyKey(tfbridge.TerraformToPulumiNameV2(tfKey, schemaMap, schemaInfos))
		if _, inUser := inputs[pk]; inUser {
			continue
		}
		if _, inNews := news[pk]; inNews {
			out[pk] = true
		}
	}
	return out
}

// stripConflictingAutoDefaults inspects ValidateResourceConfig diagnostics for
// "Conflicting configuration arguments" errors. If the conflict points at an
// auto-defaulted attribute, the attribute is removed from the returned inputs.
// Returns nil if no auto-defaulted keys were dropped.
func stripConflictingAutoDefaults(
	inputs resource.PropertyMap,
	autoDefaulted map[resource.PropertyKey]bool,
	diags []*tfprotov6.Diagnostic,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
) resource.PropertyMap {
	stripped := inputs.Copy()
	dropped := false
	for _, diag := range diags {
		if diag == nil || diag.Severity > tfprotov6.DiagnosticSeverityError {
			continue
		}
		if diag.Summary != "Conflicting configuration arguments" {
			continue
		}
		if diag.Attribute == nil {
			continue
		}
		steps := diag.Attribute.Steps()
		if len(steps) != 1 {
			continue
		}
		name, ok := steps[0].(tftypes.AttributeName)
		if !ok {
			continue
		}
		pulumiName := tfbridge.TerraformToPulumiNameV2(string(name), schemaMap, schemaInfos)
		pk := resource.PropertyKey(pulumiName)
		if !autoDefaulted[pk] {
			continue
		}
		delete(stripped, pk)
		dropped = true
	}
	if !dropped {
		return nil
	}
	return stripped
}
