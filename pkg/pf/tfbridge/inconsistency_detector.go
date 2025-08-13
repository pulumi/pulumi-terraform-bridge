// Copyright 2016-2024, Pulumi Corporation.
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
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// detectPFInconsistentApply compares planned and actual states for Plugin Framework providers
func detectPFInconsistentApply(
	ctx context.Context,
	resourceType string,
	plannedState *tfprotov6.DynamicValue,
	actualState *tfprotov6.DynamicValue,
	tfType tftypes.Type,
	filter tfbridge.InconsistencyFilter,
) ([]*tfbridge.InconsistencyDetail, error) {
	// Fast path: check if inconsistency detection is enabled for this resource
	if !tfbridge.ShouldDetectInconsistentApply(resourceType) {
		return nil, nil
	}

	// Skip detection for nil states
	if plannedState == nil || actualState == nil {
		return nil, nil
	}

	// Fast path: identical byte content means identical values
	if plannedState.MsgPack != nil && actualState.MsgPack != nil {
		if bytes.Equal(plannedState.MsgPack, actualState.MsgPack) {
			return nil, nil
		}
	} else if plannedState.JSON != nil && actualState.JSON != nil {
		if bytes.Equal(plannedState.JSON, actualState.JSON) {
			return nil, nil
		}
	}

	// Get configuration
	config := tfbridge.GetInconsistencyConfig()

	// For very detailed inspection in trace mode, check byte differences
	if config.DetailLevel == "trace" {
		logger := tfbridge.GetLogger(ctx)

		if plannedState.MsgPack != nil && actualState.MsgPack != nil {
			// Log messagepack bytes to help advanced debugging
			logger.Debug(fmt.Sprintf("Planned MsgPack: %x", plannedState.MsgPack))
			logger.Debug(fmt.Sprintf("Actual MsgPack: %x", actualState.MsgPack))
		}
	}

	// Unmarshal the planned state
	plannedValue, err := plannedState.Unmarshal(tfType)
	if err != nil {
		return nil, err
	}

	// Unmarshal the actual state
	actualValue, err := actualState.Unmarshal(tfType)
	if err != nil {
		return nil, err
	}

	// Fast path: if both values are null they're equal
	if plannedValue.IsNull() && actualValue.IsNull() {
		return nil, nil
	}

	// Perform comparison
	inconsistencies := []*tfbridge.InconsistencyDetail{}

	// Call recursive comparison function with max depth limit
	// to avoid excessive processing for large/complex objects
	const maxCompareDepth = 5
	compareTFValues("", resourceType, plannedValue, actualValue, filter, &inconsistencies, 0, maxCompareDepth)

	return inconsistencies, nil
}

// compareTFValues recursively compares two tftypes.Value objects
func compareTFValues(
	path string,
	resourceType string,
	planned, actual tftypes.Value,
	filter tfbridge.InconsistencyFilter,
	inconsistencies *[]*tfbridge.InconsistencyDetail,
	currentDepth int,
	maxDepth int,
) {
	// Limit recursion depth to avoid excessive CPU/memory usage
	if currentDepth > maxDepth {
		return
	}

	// Skip if path should be ignored
	if filter.ShouldIgnoreAttribute(resourceType, path) {
		return
	}

	// Limit the number of inconsistencies to report
	if len(*inconsistencies) >= tfbridge.MaxReportedDifferences {
		return
	}

	// Handle null/unknown values
	if planned.IsNull() && actual.IsNull() {
		return // Both null, no inconsistency
	}

	// Check for unknown values
	var isPlannedUnknown bool
	tftypes.Walk(planned, func(path *tftypes.AttributePath, val tftypes.Value) (bool, error) {
		if !val.IsFullyKnown() {
			isPlannedUnknown = true
			return false, nil
		}
		return true, nil
	})
	
	if isPlannedUnknown {
		// Planned unknown, actual could be any value
		// This is normal for computed values, so don't flag as inconsistency
		return
	}

	// Different types
	if !planned.Type().Equal(actual.Type()) {
		// Add to inconsistencies unless filtered
		if !filter.ShouldIgnoreValueChange(resourceType, path, planned, actual) {
			*inconsistencies = append(*inconsistencies, &tfbridge.InconsistencyDetail{
				Path:         path,
				PlannedValue: planned,
				ActualValue:  actual,
				Description:  fmt.Sprintf("type changed from %s to %s", planned.Type(), actual.Type()),
			})
		}
		return
	}

	// Handle by type
	typeKind := fmt.Sprintf("%T", planned.Type())

	switch {
	case typeKind == "tftypes.Bool", typeKind == "tftypes.Number", typeKind == "tftypes.String":
		// For primitives, direct comparison
		var plannedVal, actualVal interface{}

		errPlanned := planned.As(&plannedVal)
		errActual := actual.As(&actualVal)

		if errPlanned == nil && errActual == nil && !reflect.DeepEqual(plannedVal, actualVal) {
			// Check if this difference should be ignored
			if !filter.ShouldIgnoreValueChange(resourceType, path, plannedVal, actualVal) {
				*inconsistencies = append(*inconsistencies, &tfbridge.InconsistencyDetail{
					Path:         path,
					PlannedValue: plannedVal,
					ActualValue:  actualVal,
					Description:  fmt.Sprintf("was %v, but now %v", plannedVal, actualVal),
				})
			}
		}

	case typeKind == "tftypes.List", typeKind == "tftypes.Set", typeKind == "tftypes.Tuple":
		// For lists and sets, compare elements
		var plannedElems []tftypes.Value
		var actualElems []tftypes.Value

		errPlanned := planned.As(&plannedElems)
		errActual := actual.As(&actualElems)

		if errPlanned != nil || errActual != nil {
			return
		}

		// Check length first
		if len(plannedElems) != len(actualElems) {
			// Check if this difference should be ignored
			if !filter.ShouldIgnoreValueChange(resourceType, path, plannedElems, actualElems) {
				*inconsistencies = append(*inconsistencies, &tfbridge.InconsistencyDetail{
					Path:         path,
					PlannedValue: plannedElems,
					ActualValue:  actualElems,
					Description:  fmt.Sprintf("length changed from %d to %d", len(plannedElems), len(actualElems)),
				})
			}
			return
		}

		// For large arrays, just check a sample to avoid excessive processing
		maxSamples := 5
		if len(plannedElems) > maxSamples && currentDepth > 2 {
			// Check first, last, and a few samples in the middle
			samplesToCheck := []int{0, len(plannedElems) / 4, len(plannedElems) / 2,
				(3 * len(plannedElems)) / 4, len(plannedElems) - 1}

			for _, i := range samplesToCheck {
				if i < len(plannedElems) {
					elementPath := path
					if path == "" {
						elementPath = fmt.Sprintf("[%d]", i)
					} else {
						elementPath = fmt.Sprintf("%s[%d]", path, i)
					}

					compareTFValues(elementPath, resourceType, plannedElems[i], actualElems[i],
						filter, inconsistencies, currentDepth+1, maxDepth)
				}
			}
		} else {
			// Small list - check all elements
			for i := range plannedElems {
				elementPath := path
				if path == "" {
					elementPath = fmt.Sprintf("[%d]", i)
				} else {
					elementPath = fmt.Sprintf("%s[%d]", path, i)
				}

				compareTFValues(elementPath, resourceType, plannedElems[i], actualElems[i],
					filter, inconsistencies, currentDepth+1, maxDepth)
			}
		}

	case typeKind == "tftypes.Map", typeKind == "tftypes.Object":
		// For maps and objects, compare keys and values
		var plannedMap map[string]tftypes.Value
		var actualMap map[string]tftypes.Value

		errPlanned := planned.As(&plannedMap)
		errActual := actual.As(&actualMap)

		if errPlanned != nil || errActual != nil {
			return
		}

		// For large maps at deep nesting levels, only check a subset of keys
		isLargeMap := len(plannedMap) > 10 && currentDepth > 2
		keyCheckCount := 0
		maxKeyChecks := 10

		// Check each key in planned
		for k, plannedVal := range plannedMap {
			// Limit checks for large maps to avoid excessive processing
			if isLargeMap {
				keyCheckCount++
				if keyCheckCount > maxKeyChecks {
					break
				}
			}

			attrPath := path
			if path == "" {
				attrPath = k
			} else {
				attrPath = fmt.Sprintf("%s.%s", path, k)
			}

			// Skip if this attribute should be ignored
			if filter.ShouldIgnoreAttribute(resourceType, attrPath) {
				continue
			}

			actualVal, exists := actualMap[k]
			if !exists {
				// Attribute missing in actual
				// Check if this should be ignored
				if !filter.ShouldIgnoreValueChange(resourceType, attrPath, plannedVal, nil) {
					*inconsistencies = append(*inconsistencies, &tfbridge.InconsistencyDetail{
						Path:         attrPath,
						PlannedValue: plannedVal,
						ActualValue:  nil,
						Description:  "was present, but now absent",
					})
				}
				continue
			}

			// Recursively compare values
			compareTFValues(attrPath, resourceType, plannedVal, actualVal, filter,
				inconsistencies, currentDepth+1, maxDepth)
		}

		// Check for keys in actual that aren't in planned
		keyCheckCount = 0
		for k, actualVal := range actualMap {
			// Limit checks for large maps
			if isLargeMap {
				keyCheckCount++
				if keyCheckCount > maxKeyChecks {
					break
				}
			}

			attrPath := path
			if path == "" {
				attrPath = k
			} else {
				attrPath = fmt.Sprintf("%s.%s", path, k)
			}

			// Skip if this attribute should be ignored
			if filter.ShouldIgnoreAttribute(resourceType, attrPath) {
				continue
			}

			if _, exists := plannedMap[k]; !exists {
				// Attribute present in actual but not in planned
				// Check if this should be ignored
				if !filter.ShouldIgnoreValueChange(resourceType, attrPath, nil, actualVal) {
					*inconsistencies = append(*inconsistencies, &tfbridge.InconsistencyDetail{
						Path:         attrPath,
						PlannedValue: nil,
						ActualValue:  actualVal,
						Description:  "was absent, but now present",
					})
				}
			}
		}
	}
}

// formatPFInconsistencyMessage formats the inconsistency message for PF providers
func formatPFInconsistencyMessage(resourceType string, inconsistencies []*tfbridge.InconsistencyDetail) string {
	if len(inconsistencies) == 0 {
		return ""
	}

	// Sort inconsistencies for stable output
	sort.Slice(inconsistencies, func(i, j int) bool {
		return inconsistencies[i].Path < inconsistencies[j].Path
	})

	var lines []string
	for _, inconsistency := range inconsistencies {
		lines = append(lines, fmt.Sprintf(".%s: %s",
			inconsistency.Path, inconsistency.Description))
	}

	return fmt.Sprintf(
		"Provider produced inconsistent result after apply for %s:\n%s\n\n"+
			"This is a bug in the provider, which should be reported in the provider's own issue tracker.",
		resourceType,
		strings.Join(lines, "\n"),
	)
}

// detectAndReportPFInconsistencies detects and reports inconsistencies between planned and actual states
// This helper function abstracts the common inconsistency detection logic used in both Create and Update
// Returns an error if the inconsistency detection process itself fails
func detectAndReportPFInconsistencies(
	ctx context.Context,
	resourceType string,
	plannedState *tfprotov6.DynamicValue,
	actualState *tfprotov6.DynamicValue,
	tfType tftypes.Type,
) error {
	// Skip if inconsistency detection is disabled for this resource
	if !tfbridge.ShouldDetectInconsistentApply(resourceType) {
		return nil
	}

	// Compare planned state with actual state
	if plannedState != nil && actualState != nil {
		// Create default filter
		filter := tfbridge.GetInconsistencyFilter(nil)

		// Check for inconsistencies
		inconsistencies, err := detectPFInconsistentApply(
			ctx,
			resourceType,
			plannedState,
			actualState,
			tfType,
			filter,
		)

		if err != nil {
			return fmt.Errorf("error detecting provider inconsistencies: %w", err)
		}

		if len(inconsistencies) > 0 {
			message := formatPFInconsistencyMessage(resourceType, inconsistencies)
			logger := tfbridge.GetLogger(ctx)
			logger.Warn(message)
		}
	}
	
	return nil
}