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
)

// Update updates an existing resource with new values.
func (p *Provider) Update(
	urn resource.URN,
	id resource.ID,
	priorState resource.PropertyMap,
	checkedInputs resource.PropertyMap,
	timeout float64,
	ignoreChanges []string,
	preview bool,
) (resource.PropertyMap, resource.Status, error) {
	ctx := context.TODO()

	rh, err := p.resourceHandle(ctx, urn)
	if err != nil {
		return nil, 0, err
	}

	tfType := rh.schema.Type().TerraformType(ctx).(tftypes.Object)

	priorStateValue, err := ConvertPropertyMapToTFValue(tfType)(priorState)
	if err != nil {
		return nil, 0, err
	}

	checkedInputsValue, err := ConvertPropertyMapToTFValue(tfType)(checkedInputs)
	if err != nil {
		return nil, 0, err
	}

	planResp, err := p.plan(ctx, rh.terraformResourceName, priorStateValue, checkedInputsValue)
	if err != nil {
		return nil, 0, err
	}

	// TODO clarify what to do here, how to handle preview Update properly.
	if preview {
		plannedStatePropertyMap, err := ConvertDynamicValueToPropertyMap(tfType)(*planResp.PlannedState)
		if err != nil {
			return nil, 0, err
		}
		return plannedStatePropertyMap, resource.StatusOK, nil
	}

	priorStateDV, checkedInputsDV, err := makeDynamicValues2(priorStateValue, checkedInputsValue)
	if err != nil {
		return nil, 0, err
	}

	req := tfprotov6.ApplyResourceChangeRequest{
		TypeName:     rh.terraformResourceName,
		Config:       &checkedInputsDV,
		PriorState:   &priorStateDV,
		PlannedState: planResp.PlannedState,
	}

	resp, err := p.tfServer.ApplyResourceChange(ctx, &req)
	if err != nil {
		return nil, 0, err
	}

	// TODO handle resp.Diagnostics more than just detecting the first error; handle warnings, process multiple
	// errors.
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			prefix := ""
			if d.Attribute != nil {
				prefix = fmt.Sprintf("[%s] ", d.Attribute.String())
			}
			return nil, 0, fmt.Errorf("%s%s: %s", prefix, d.Summary, d.Detail)
		}
	}

	// TODO handle resp.Private
	updatedState, err := ConvertDynamicValueToPropertyMap(tfType)(*resp.NewState)
	if err != nil {
		return nil, 0, err
	}

	return updatedState, resource.StatusOK, nil
}
