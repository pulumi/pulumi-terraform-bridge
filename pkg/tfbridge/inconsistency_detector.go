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
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// InconsistencyDetail describes a single detected inconsistency
type InconsistencyDetail struct {
	// Path is the attribute path where the inconsistency was found
	Path string
	
	// PlannedValue is what was expected from the plan
	PlannedValue interface{}
	
	// ActualValue is what was actually returned after apply
	ActualValue interface{}
	
	// Description provides a human-readable explanation
	Description string
}

// InconsistencyResult contains the detected inconsistencies
type InconsistencyResult struct {
	// Detected indicates if inconsistencies were found
	Detected bool
	
	// ResourceType is the type of resource checked
	ResourceType string
	
	// Inconsistencies holds the list of detected inconsistencies
	Inconsistencies []InconsistencyDetail
	
	// RawPlanned holds the raw planned state (for debug log levels)
	RawPlanned interface{}
	
	// RawActual holds the raw actual state (for debug log levels)
	RawActual interface{}
}

// We don't declare interfaces here, as we'll use the ones from tfshim package

// detectInconsistentApply compares the planned state from diff with the actual state returned
// after apply to detect inconsistencies in the provider's implementation.
func detectInconsistentApply(
	ctx context.Context,
	resourceType string,
	diff shim.InstanceDiff,
	plannedState shim.InstanceState,
	actualState shim.InstanceState,
	filter InconsistencyFilter,
) InconsistencyResult {
	// Skip detection if feature is disabled
	if !ShouldDetectInconsistentApply(resourceType) {
		return InconsistencyResult{Detected: false, ResourceType: resourceType}
	}

	// Skip detection for nil states
	if plannedState == nil || actualState == nil {
		return InconsistencyResult{Detected: false, ResourceType: resourceType}
	}
	
	// Get configuration
	config := GetInconsistencyConfig()
	
	// Skip complex resources if detail level is lower
	if config.DetailLevel == "normal" && isComplexResource(resourceType) {
		// For resources known to be large/complex, only do detection at higher detail levels
		return InconsistencyResult{Detected: false, ResourceType: resourceType}
	}
	
	result := InconsistencyResult{
		ResourceType:     resourceType,
		Inconsistencies:  []InconsistencyDetail{},
	}
	
	// Get attributes from state objects
	plannedObj, err := plannedState.Object(nil)
	if err != nil {
		// If we can't get the planned object, log and return empty result
		GetLogger(ctx).Debug(fmt.Sprintf("Error getting planned state object: %v", err))
		return InconsistencyResult{Detected: false, ResourceType: resourceType}
	}
	
	actualObj, err := actualState.Object(nil)
	if err != nil {
		// If we can't get the actual object, log and return empty result
		GetLogger(ctx).Debug(fmt.Sprintf("Error getting actual state object: %v", err))
		return InconsistencyResult{Detected: false, ResourceType: resourceType}
	}
	
	// Convert to string map for comparison
	plannedAttrs := make(map[string]string)
	for k, v := range plannedObj {
		if v != nil {
			plannedAttrs[k] = fmt.Sprintf("%v", v)
		}
	}
	
	actualAttrs := make(map[string]string)
	for k, v := range actualObj {
		if v != nil {
			actualAttrs[k] = fmt.Sprintf("%v", v)
		}
	}
	
	// Initialize raw values for debugging
	if config.DetailLevel == "debug" || config.DetailLevel == "trace" {
		result.RawPlanned = plannedAttrs
		result.RawActual = actualAttrs
	}
	
	// Track how many inconsistencies we've reported to limit log size
	reportedCount := 0
	
	// Check for attributes present in planned but missing or different in actual
	for k, plannedVal := range plannedAttrs {
		// Skip attributes that should be ignored
		if filter.ShouldIgnoreAttribute(resourceType, k) {
			continue
		}
		
		// Skip computed values that were unknown in plan
		if attr := diff.Attribute(k); attr != nil && attr.NewComputed {
			continue
		}
		
		actualVal, exists := actualAttrs[k]
		
		// If the value doesn't exist or is different in the actual state
		if !exists || plannedVal != actualVal {
			// Check if we should ignore this specific value change
			if filter.ShouldIgnoreValueChange(resourceType, k, plannedVal, actualVal) {
				continue
			}
			
			// Format the inconsistency
			var description string
			if !exists {
				description = fmt.Sprintf("was %q, but now absent", formatValue(plannedVal))
			} else {
				description = fmt.Sprintf("was %q, but now %q", 
					formatValue(plannedVal), formatValue(actualVal))
			}
			
			result.Inconsistencies = append(result.Inconsistencies, InconsistencyDetail{
				Path:         k,
				PlannedValue: plannedVal,
				ActualValue:  actualVal,
				Description:  description,
			})

			// Limit the number of reported differences
			reportedCount++
			if reportedCount >= MaxReportedDifferences {
				// Add a single entry indicating more differences exist
				result.Inconsistencies = append(result.Inconsistencies, InconsistencyDetail{
					Path:        "...",
					Description: fmt.Sprintf("and %d more differences (truncated)", 
						len(plannedAttrs) + len(actualAttrs) - MaxReportedDifferences),
				})
				break
			}
		}
	}
	
	// Only check for attributes present in actual but missing in planned if we haven't hit the limit
	if reportedCount < MaxReportedDifferences {
		for k, actualVal := range actualAttrs {
			// Skip attributes that should be ignored
			if filter.ShouldIgnoreAttribute(resourceType, k) {
				continue
			}
			
			if _, exists := plannedAttrs[k]; !exists {
				// Check if we should ignore this specific value change
				if filter.ShouldIgnoreValueChange(resourceType, k, nil, actualVal) {
					continue
				}
				
				// This is an attribute that wasn't in the plan
				result.Inconsistencies = append(result.Inconsistencies, InconsistencyDetail{
					Path:         k,
					PlannedValue: nil,
					ActualValue:  actualVal,
					Description:  fmt.Sprintf("was absent, but now %q", formatValue(actualVal)),
				})

				// Limit the number of reported differences
				reportedCount++
				if reportedCount >= MaxReportedDifferences {
					// Add a single entry indicating more differences exist
					result.Inconsistencies = append(result.Inconsistencies, InconsistencyDetail{
						Path:        "...",
						Description: fmt.Sprintf("and %d more differences (truncated)", 
							len(plannedAttrs) + len(actualAttrs) - MaxReportedDifferences),
					})
					break
				}
			}
		}
	}
	
	result.Detected = len(result.Inconsistencies) > 0
	return result
}

// formatValue converts a value to a readable string format
func formatValue(val interface{}) string {
	if val == nil {
		return "null"
	}
	
	switch v := val.(type) {
	case string:
		if v == "" {
			return "empty string"
		}
		return v
	default:
		return fmt.Sprintf("%v", val)
	}
}

// formatInconsistencyMessage formats a message about detected inconsistencies
func formatInconsistencyMessage(result InconsistencyResult) string {
	if !result.Detected {
		return ""
	}
	
	// Sort inconsistencies for stable output
	sort.Slice(result.Inconsistencies, func(i, j int) bool {
		return result.Inconsistencies[i].Path < result.Inconsistencies[j].Path
	})
	
	// Build the message
	var lines []string
	for _, inconsistency := range result.Inconsistencies {
		lines = append(lines, fmt.Sprintf(".%s: %s", 
			inconsistency.Path, inconsistency.Description))
	}
	
	// Get configuration to determine detail level
	config := GetInconsistencyConfig()
	
	message := fmt.Sprintf(
		"Provider produced inconsistent result after apply for %s:\n%s",
		result.ResourceType, 
		strings.Join(lines, "\n"),
	)
	
	// Add extra debug info if configured
	if config.DetailLevel == "debug" || config.DetailLevel == "trace" {
		message += fmt.Sprintf("\n\nPlanned state: %+v\nActual state: %+v", 
			result.RawPlanned, result.RawActual)
	}
	
	message += "\n\nThis is a bug in the provider, which should be reported in the provider's own issue tracker."
	
	return message
}