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
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

// InconsistencyFilter defines how to filter inconsistencies
type InconsistencyFilter interface {
	// ShouldIgnoreAttribute returns true if inconsistencies for this attribute should be ignored
	ShouldIgnoreAttribute(resourceType, attrName string) bool

	// ShouldIgnoreValueChange returns true if changes between these values should be ignored
	ShouldIgnoreValueChange(resourceType, attrName string, plannedVal, actualVal interface{}) bool
}

// DefaultInconsistencyFilter implements common filtering logic
type DefaultInconsistencyFilter struct {
	// Known attributes to ignore by resource type
	// Map of resource type -> set of attribute names
	IgnoredAttributes map[string]map[string]bool

	// Provider-specific custom filters
	CustomFilters []InconsistencyFilter
}

// GetInconsistencyFilter creates a filter with common known inconsistencies and optional custom filters
func GetInconsistencyFilter(customFilter interface{}) InconsistencyFilter {
	filter := NewDefaultInconsistencyFilter()

	// Add custom filter if provided and implements the correct interface
	if customFilter != nil {
		if cf, ok := customFilter.(InconsistencyFilter); ok {
			filter.AddCustomFilter(cf)
		}
	}

	return filter
}

// NewDefaultInconsistencyFilter creates a filter with common known inconsistencies
func NewDefaultInconsistencyFilter() *DefaultInconsistencyFilter {
	filter := &DefaultInconsistencyFilter{
		IgnoredAttributes: make(map[string]map[string]bool),
		CustomFilters:     []InconsistencyFilter{},
	}

	// Common attributes that frequently show inconsistencies but aren't problematic
	commonIgnores := []string{
		"last_modified",
		"etag",
		"generation",
		"timeouts",
		"id", // Often reformatted but functionally equivalent
	}

	// Add provider-specific attributes that should be ignored
	awsIgnores := append([]string{
		"arn",            // Often returned with slight differences in format
		"creation_token", // Often has metadata added by AWS
		"tags_all",       // Computed value that often differs
	}, commonIgnores...)

	filter.AddIgnoredAttributes("aws_*", awsIgnores...)

	// GCP resources often have these computed attributes that can legitimately differ
	gcpIgnores := append([]string{
		"effective_labels",
		"terraform_labels",
		"self_link",
	}, commonIgnores...)

	filter.AddIgnoredAttributes("google_*", gcpIgnores...)

	// Azure resources often have these computed attributes
	azureIgnores := append([]string{
		"principal_id",
		"tenant_id",
		"identity",
	}, commonIgnores...)

	filter.AddIgnoredAttributes("azurerm_*", azureIgnores...)

	return filter
}

// AddIgnoredAttributes adds attributes to ignore for a resource type
// Resource type can use * as a wildcard
func (f *DefaultInconsistencyFilter) AddIgnoredAttributes(resourceType string, attributes ...string) {
	if _, exists := f.IgnoredAttributes[resourceType]; !exists {
		f.IgnoredAttributes[resourceType] = make(map[string]bool)
	}

	for _, attr := range attributes {
		f.IgnoredAttributes[resourceType][attr] = true
	}
}

// AddCustomFilter adds a custom filter to the chain
func (f *DefaultInconsistencyFilter) AddCustomFilter(filter InconsistencyFilter) {
	f.CustomFilters = append(f.CustomFilters, filter)
}

// ShouldIgnoreAttribute checks if attribute inconsistencies should be ignored
func (f *DefaultInconsistencyFilter) ShouldIgnoreAttribute(resourceType, attrName string) bool {
	// First check custom filters
	for _, filter := range f.CustomFilters {
		if filter.ShouldIgnoreAttribute(resourceType, attrName) {
			return true
		}
	}

	// Check for exact resource type match
	if attrs, exists := f.IgnoredAttributes[resourceType]; exists {
		if attrs[attrName] {
			return true
		}
	}

	// Check for wildcard matches
	for pattern, attrs := range f.IgnoredAttributes {
		if strings.Contains(pattern, "*") {
			if matched, _ := filepath.Match(pattern, resourceType); matched {
				if attrs[attrName] {
					return true
				}
			}
		}
	}

	// Special handling for common patterns

	// Skip computed terraform internal attributes
	if strings.HasPrefix(attrName, "~") {
		return true
	}

	// Skip timeouts
	if strings.HasPrefix(attrName, "timeouts.") {
		return true
	}

	// Skip status attributes
	if strings.HasSuffix(attrName, "_status") || strings.HasSuffix(attrName, ".status") {
		return true
	}

	// Skip timestamps
	if strings.Contains(attrName, "timestamp") ||
		strings.Contains(attrName, "_at") ||
		strings.Contains(attrName, "_time") {
		return true
	}

	return false
}

// ShouldIgnoreValueChange determines if specific value changes should be ignored
func (f *DefaultInconsistencyFilter) ShouldIgnoreValueChange(
	resourceType, attrName string, plannedVal, actualVal interface{},
) bool {
	// First check custom filters
	for _, filter := range f.CustomFilters {
		if filter.ShouldIgnoreValueChange(resourceType, attrName, plannedVal, actualVal) {
			return true
		}
	}

	// Common case: nil/null vs empty string/array
	// Many providers inconsistently use null vs empty values
	if isNilOrEmpty(plannedVal) && isNilOrEmpty(actualVal) {
		return true
	}

	// Ignore formatting differences in ID fields
	if strings.HasSuffix(attrName, "id") || attrName == "id" {
		strPlanned, okP := coerceToString(plannedVal)
		strActual, okA := coerceToString(actualVal)

		if okP && okA {
			// If IDs differ only in case or separators, ignore
			normalizedP := normalizeID(strPlanned)
			normalizedA := normalizeID(strActual)
			if normalizedP == normalizedA {
				return true
			}
		}
	}

	// Compare values with type conversion for numbers
	// Common case: "5" vs 5 should be considered equal
	if isNumeric(plannedVal) && isNumeric(actualVal) {
		// Try to convert both to float64 for comparison
		p, pOk := toFloat64(plannedVal)
		a, aOk := toFloat64(actualVal)
		if pOk && aOk && p == a {
			return true
		}
	}

	// Ignore boolean representation differences (e.g., "true" vs true)
	if isBoolean(plannedVal) && isBoolean(actualVal) {
		p, pOk := toBool(plannedVal)
		a, aOk := toBool(actualVal)
		if pOk && aOk && p == a {
			return true
		}
	}

	return false
}

// Helper functions for filtering

// isNilOrEmpty returns true if the value is nil, null, empty string, empty list, etc.
func isNilOrEmpty(v interface{}) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case string:
		return val == ""
	case []interface{}:
		return len(val) == 0
	case map[string]interface{}:
		return len(val) == 0
	case *string:
		return val == nil || *val == ""
	case *[]interface{}:
		return val == nil || len(*val) == 0
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Ptr, reflect.Interface:
			if rv.IsNil() {
				return true
			}
			return isNilOrEmpty(rv.Elem().Interface())
		case reflect.Map, reflect.Slice, reflect.Array:
			return rv.Len() == 0
		}
		return false
	}
}

// coerceToString attempts to convert a value to string
func coerceToString(v interface{}) (string, bool) {
	if v == nil {
		return "", false
	}

	switch val := v.(type) {
	case string:
		return val, true
	case *string:
		if val == nil {
			return "", false
		}
		return *val, true
	case fmt.Stringer:
		return val.String(), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

// normalizeID standardizes an ID string for comparison
func normalizeID(id string) string {
	// Convert to lowercase
	id = strings.ToLower(id)

	// Replace common separators with a standard one
	separators := []string{"-", "_", ":", "/", "."}
	for _, sep := range separators {
		id = strings.ReplaceAll(id, sep, "")
	}

	// Remove any whitespace
	id = strings.Join(strings.Fields(id), "")

	return id
}

// isNumeric checks if a value is numeric (int, float, or string representation)
func isNumeric(v interface{}) bool {
	if v == nil {
		return false
	}

	switch val := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	case string:
		// Try to parse as float
		_, err := fmt.Sscanf(val, "%f", new(float64))
		return err == nil
	}
	return false
}

// toFloat64 converts a numeric value to float64
func toFloat64(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}

	switch val := v.(type) {
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case float64:
		return val, true
	case string:
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		return f, err == nil
	}
	return 0, false
}

// isBoolean checks if a value is a boolean or string representation of boolean
func isBoolean(v interface{}) bool {
	if v == nil {
		return false
	}

	switch val := v.(type) {
	case bool:
		return true
	case string:
		lower := strings.ToLower(val)
		return lower == "true" || lower == "false"
	}
	return false
}

// toBool converts a value to bool
func toBool(v interface{}) (bool, bool) {
	if v == nil {
		return false, false
	}

	switch val := v.(type) {
	case bool:
		return val, true
	case string:
		lower := strings.ToLower(val)
		if lower == "true" {
			return true, true
		}
		if lower == "false" {
			return false, true
		}
	}
	return false, false
}
