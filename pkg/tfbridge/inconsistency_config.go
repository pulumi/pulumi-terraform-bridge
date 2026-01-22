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
	"os"
	"strings"
	"sync"
)

const (
	// Environment variables for controlling inconsistency detection

	// EnvDetectInconsistentApply controls whether to detect and report inconsistencies between
	// planned state and actual state after provider apply operations
	EnvDetectInconsistentApply = "PULUMI_DETECT_INCONSISTENT_APPLY"

	// EnvDetectInconsistentApplyDetail controls the level of detail for inconsistency reporting
	// Values: "normal" (default), "debug", "trace"
	EnvDetectInconsistentApplyDetail = "PULUMI_DETECT_INCONSISTENT_APPLY_DETAIL"

	// EnvDetectInconsistentApplyResources allows limiting inconsistency detection to specific
	// resource types (comma-separated list of resource types, empty means all)
	EnvDetectInconsistentApplyResources = "PULUMI_DETECT_INCONSISTENT_APPLY_RESOURCES"

	// MaxReportedDifferences limits the number of inconsistencies reported to avoid overwhelming logs
	MaxReportedDifferences = 10
)

// InconsistencyConfig holds configuration for inconsistency detection
type InconsistencyConfig struct {
	// Enabled indicates whether detection is enabled at all
	Enabled bool

	// DetailLevel controls how much detail to include in reports
	DetailLevel string

	// ResourceTypes contains resource types to check (empty means all)
	ResourceTypes map[string]bool
}

var (
	configMutex  sync.RWMutex
	cachedConfig *InconsistencyConfig
)

// GetInconsistencyConfig returns the configured settings for inconsistency detection.
// Configuration is cached after first call to avoid repeated environment variable reads.
func GetInconsistencyConfig() InconsistencyConfig {
	configMutex.RLock()
	if cachedConfig != nil {
		defer configMutex.RUnlock()
		return *cachedConfig
	}
	configMutex.RUnlock()

	configMutex.Lock()
	defer configMutex.Unlock()

	// Double-check after acquiring write lock
	if cachedConfig != nil {
		return *cachedConfig
	}

	config := InconsistencyConfig{
		Enabled:       isTruthy(os.Getenv(EnvDetectInconsistentApply)),
		DetailLevel:   "normal",
		ResourceTypes: make(map[string]bool),
	}

	// Parse detail level
	detail := os.Getenv(EnvDetectInconsistentApplyDetail)
	if detail != "" {
		switch strings.ToLower(detail) {
		case "debug", "trace":
			config.DetailLevel = strings.ToLower(detail)
		default:
			config.DetailLevel = "normal"
		}
	}

	// Parse resource types filter
	resources := os.Getenv(EnvDetectInconsistentApplyResources)
	if resources != "" {
		for _, r := range strings.Split(resources, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				config.ResourceTypes[r] = true
			}
		}
	}

	cachedConfig = &config
	return config
}

// resetConfigCacheForTesting resets the configuration cache.
// This is only for use in tests that need to change environment variables.
func resetConfigCacheForTesting() {
	configMutex.Lock()
	defer configMutex.Unlock()
	cachedConfig = nil
}

// isTruthy returns whether a given string represents a "true" value.
func isTruthy(s string) bool {
	switch strings.ToLower(s) {
	case "1", "y", "yes", "true", "t", "on":
		return true
	}
	return false
}

// ShouldDetectInconsistentApply determines if we should perform inconsistency detection
// for a specific resource type
func ShouldDetectInconsistentApply(resourceType string) bool {
	config := GetInconsistencyConfig()
	if !config.Enabled {
		return false
	}

	// Skip detection for non-resource changes (e.g., provider config)
	if resourceType == "" {
		return false
	}

	// Skip detection for data sources
	if strings.HasPrefix(resourceType, "data.") {
		return false
	}

	// If no specific resources are configured, check all resources
	if len(config.ResourceTypes) == 0 {
		return true
	}

	// Otherwise, only check specified resources
	return config.ResourceTypes[resourceType]
}
