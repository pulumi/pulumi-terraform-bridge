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

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Simplifies calling PlanResourceChanges in Terraform.
//
// Pulumi providers use plan in a few places, for example:
//
//     Diff(priorState, checkedInputs):
//         plannedState = plan(priorState, checkedInputs)
//         priorState.Diff(plannedState)
//
// And create is a special case with priorState=nilState:
//
//     Create(checkedInputs):
//         plannedState = plan(nilState, checkedInputs)
//         ApplyResourceChange(plannedState)
func (p *Provider) plan(
	ctx context.Context,
	typeName string,
	schema tfsdk.Schema,
	priorState tftypes.Value,
	checkedInputs tftypes.Value,
) (*tfprotov6.PlanResourceChangeResponse, error) {
	proposedNewState, err := computeProposedNewState(ctx, schema, priorState, checkedInputs)
	if err != nil {
		return nil, err
	}

	priorStateV, configV, proposedNewStateV, err := makeDynamicValues3(
		priorState, checkedInputs, proposedNewState)
	if err != nil {
		return nil, err
	}

	planReq := tfprotov6.PlanResourceChangeRequest{
		TypeName:         typeName,
		PriorState:       &priorStateV,
		ProposedNewState: &proposedNewStateV,
		Config:           &configV,

		// TODO PriorPrivate
		// TODO ProviderMeta
	}

	planResp, err := p.tfServer.PlanResourceChange(ctx, &planReq)
	if err != nil {
		return nil, err
	}

	return planResp, nil
}

// This method could possibly reuse actual TF implementation if it is link-able, since TF must do this somewhere.
//
// Quote from TF docs serves as a spec:
//
//     The ProposedNewState merges any non-null values in the configuration with any computed attributes in PriorState
//     as a utility to help providers avoid needing to implement such merging functionality themselves.
//
//     The state is represented as a tftypes.Object, with each attribute and nested block getting its own key and value.
//
//     The ProposedNewState will be null when planning a delete operation.
//
// When Pulumi programs retract attributes, config (checkedInputs) will have no entry for these, while priorState might
// have an entry. ProposedNewState must have a Null entry in this case for Diff to work properly and recognize an
// attribute deletion.
func computeProposedNewState(
	ctx context.Context,
	schema tfsdk.Schema,
	priorState, config tftypes.Value,
) (tftypes.Value, error) {
	tfType := schema.Type().TerraformType(ctx).(tftypes.Object)
	empty := newObjectWithNullValues(tfType)

	return tftypes.Transform(empty, func(p *tftypes.AttributePath, old tftypes.Value) (tftypes.Value, error) {
		if len(p.Steps()) == 0 {
			return old, nil
		}

		attr, err := schema.AttributeAtTerraformPath(ctx, p)
		if err != nil {
			return tftypes.Value{}, err
		}

		attrTy := attr.FrameworkType().TerraformType(ctx)

		if attr.IsComputed() {
			// preserve value from priorState for computed attributes
			return valueAtPathOrNull(attrTy, p, priorState)
		}

		// otherwise take the value from config; missing values in config indicate deletions and need to be
		// encoded as null here
		return valueAtPathOrNull(attrTy, p, config)
	})
}

// Constructs an empty object of the given type where all fields are initialized to Null.
func newObjectWithNullValues(typ tftypes.Object) tftypes.Value {
	attrs := map[string]tftypes.Value{}
	for attrName, attrTy := range typ.AttributeTypes {
		attrs[attrName] = tftypes.NewValue(attrTy, nil)
	}
	return tftypes.NewValue(typ, attrs)
}

// Drills down into a Value. Returns the found value and a flag indicating if it was found or not.
func valueAtPath(p *tftypes.AttributePath, root tftypes.Value) (tftypes.Value, bool, error) {
	result, _, err := tftypes.WalkAttributePath(root, p)
	if err == tftypes.ErrInvalidStep {
		return tftypes.Value{}, false, nil // not found
	}
	if err != nil {
		return tftypes.Value{}, false, err // error
	}
	resultValue, ok := result.(tftypes.Value)
	if !ok {
		return tftypes.Value{}, false, fmt.Errorf(
			"Expected a value of type tftypes.Value but got: %v", result)
	}
	return resultValue, true, nil
}

// Like valueAtPath but encodes missing values as Null. Needs to know the type of the Null.
func valueAtPathOrNull(ty tftypes.Type, p *tftypes.AttributePath, root tftypes.Value) (tftypes.Value, error) {
	v, hasV, err := valueAtPath(p, root)
	if err != nil {
		return tftypes.Value{}, err
	}
	if !hasV {
		return tftypes.NewValue(ty, nil), nil
	}
	return v, nil
}
