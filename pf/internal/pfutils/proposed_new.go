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

package pfutils

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Computes the ProposedNewState from priorState and config.
//
// Likely the canonical implementation is ProposedNew objchange.go:
//
// https://github.com/hashicorp/terraform/blob/v1.3.6/internal/plans/objchange/objchange.go#L27-#L27
//
// Terraform core does this as a utility to providers to make their code easier; therefore the bridge has to do it also,
// as it imitates Terraform core.
//
// Quote from TF docs serves as a spec:
//
//	The ProposedNewState merges any non-null values in the configuration with any computed attributes in PriorState
//	as a utility to help providers avoid needing to implement such merging functionality themselves.
//
//	The state is represented as a tftypes.Object, with each attribute and nested block getting its own key and
//	value.
//
//	The ProposedNewState will be null when planning a delete operation.
//
// When Pulumi programs retract attributes, config (checkedInputs) will have no entry for these, while priorState might
// have an entry. ProposedNewState must have a Null entry in this case for Diff to work properly and recognize an
// attribute deletion.
func ProposedNew(ctx context.Context, schema Schema, priorState, config tftypes.Value) (tftypes.Value, error) {
	// If the config and prior are both null, return early here before populating the prior block. The prevents
	// non-null blocks from appearing the proposed state value.
	if config.IsNull() && priorState.IsNull() {
		return priorState, nil
	}
	objectType := schema.Type().TerraformType(ctx).(tftypes.Object)
	if priorState.IsNull() {
		priorState = newObjectWithDefaults(objectType, func(t tftypes.Type) tftypes.Value {
			return tftypes.NewValue(t, nil)
		})
	}

	joinOpts := JoinOptions{
		SetElementEqual: NonComputedEq(schema),
	}

	joinOpts.Reconcile = func(diff Diff) (*tftypes.Value, error) {
		priorStateValue := diff.Value1
		configValue := diff.Value2
		pathType, err := getNearestEnclosingPathType(schema, diff.Path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", diff.Path, err)
		}
		switch pathType {
		case pathToReadOnlyAttribute:
			return priorStateValue, nil
		case pathToNonComputedAttribute, pathToBlock:
			return configValue, nil
		case pathToRoot:
			if configValue != nil && !configValue.IsNull() && configValue.IsKnown() {
				// Here priorStateValue must be unknown. If it was an object, Reconcile would not be
				// called. If it was null, it got substituted via newObjectWithDefaults.
				contract.Assert(!priorStateValue.IsKnown())
				v, err := rewriteNullComputedAsUnknown(schema, diff.Path, *configValue)
				return &v, err
			}
			return priorStateValue, nil
		case pathToComputedOptionalAttribute:
			if configValue != nil {
				if !configValue.IsNull() && configValue.IsKnown() {
					if priorStateValue != nil && !priorStateValue.IsKnown() {
						v, err := rewriteNullComputedAsUnknown(schema, diff.Path, *configValue)
						return &v, err
					}
					return configValue, nil
				}
				if !configValue.IsKnown() {
					return configValue, nil
				}
			}
			return priorStateValue, nil
		default:
			return nil, fmt.Errorf("impossible")
		}
	}

	joined, err := Join(joinOpts, tftypes.NewAttributePath(), &priorState, &config)
	if err != nil {
		return tftypes.Value{}, err
	}
	if joined == nil {
		return tftypes.Value{}, fmt.Errorf("unexpected nil when computing ProposedNewState")
	}
	return *joined, nil
}

func rewriteNullComputedAsUnknown(schema Schema,
	offset *tftypes.AttributePath, val tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(val, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		pt, err := getNearestEnclosingPathType(schema, joinPaths(offset, p))
		if err != nil {
			return v, err
		}
		switch pt {
		case pathToComputedOptionalAttribute, pathToReadOnlyAttribute:
			if v.IsNull() {
				return tftypes.NewValue(v.Type(), tftypes.UnknownValue), nil
			}
		}
		return v, nil
	})
}

// Constructs an empty object of the given type where all fields are initialized to default values.
func newObjectWithDefaults(typ tftypes.Object, defaultValue func(tftypes.Type) tftypes.Value) tftypes.Value {
	attrs := map[string]tftypes.Value{}
	for attrName, attrTy := range typ.AttributeTypes {
		attrs[attrName] = defaultValue(attrTy)
	}
	return tftypes.NewValue(typ, attrs)
}

type pathType uint16

const (
	pathUnknown                     pathType = 0
	pathToRoot                      pathType = 1
	pathToBlock                     pathType = 2
	pathToNonComputedAttribute      pathType = 3
	pathToReadOnlyAttribute         pathType = 4
	pathToComputedOptionalAttribute pathType = 5
	pathToNestedObject              pathType = 6
)

func getPathType(schema Schema, path *tftypes.AttributePath) (pathType, error) {
	if len(path.Steps()) == 0 {
		return pathToRoot, nil
	}
	lookupResult, err := LookupTerraformPath(schema, path)
	if err != nil {
		return pathUnknown, err
	}
	attr := lookupResult.Attr
	switch {
	case lookupResult.IsNestedObject:
		return pathToNestedObject, nil
	case lookupResult.IsBlock:
		return pathToBlock, nil
	case !attr.IsComputed():
		return pathToNonComputedAttribute, nil
	case !attr.IsOptional() && !attr.IsRequired():
		contract.Assert(attr.IsComputed())
		return pathToReadOnlyAttribute, nil
	default:
		contract.Assertf(attr.IsOptional(), "Remaining case should be optional")
		contract.Assertf(!attr.IsRequired(), "Remaining case should not be required")
		contract.Assertf(attr.IsComputed(), "Remaining case should be computed")
		return pathToComputedOptionalAttribute, nil
	}
}

// Searches for non-erroring getPathType starting from path and upward (..).
func getNearestEnclosingPathType(schema Schema, path *tftypes.AttributePath) (pathType, error) {
	for {
		ty, err := getPathType(schema, path)
		if err != nil {
			return pathUnknown, err
		}
		switch {
		case ty == pathToNestedObject:
			path = path.WithoutLastStep()
		default:
			return ty, nil
		}
	}
}
