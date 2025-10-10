// Copyright 2016-2023, Pulumi Corporation.
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

package x

import (
	"fmt"
	"sort"

	"github.com/hashicorp/go-multierror"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// Add mapped resources and datasources according to the given strategies.
//
// Deprecated: This item has been moved to
// [github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge.ComputeTokens]
func ComputeDefaults(info *info.Provider, opts DefaultStrategy) error {
	var errs multierror.Error

	ignored := ignoredTokens(info)

	err := computeDefaultResources(info, opts.Resource, ignored)
	if err != nil {
		errs.Errors = append(errs.Errors, fmt.Errorf("resources:\n%w", err))
	}
	err = computeDefaultDataSources(info, opts.DataSource, ignored)
	if err != nil {
		errs.Errors = append(errs.Errors, fmt.Errorf("datasources:\n%w", err))
	}
	return errs.ErrorOrNil()
}

func ignoredTokens(info *info.Provider) map[string]bool {
	ignored := map[string]bool{}
	if info == nil {
		return ignored
	}
	for _, tk := range info.IgnoreMappings {
		ignored[tk] = true
	}
	return ignored
}

func computeDefaultResources(providerInfo *info.Provider, strategy ResourceStrategy, ignored map[string]bool) error {
	if strategy == nil {
		return nil
	}
	if providerInfo.Resources == nil {
		providerInfo.Resources = map[string]*info.Resource{}
	}
	return applyComputedTokens(providerInfo.P.ResourcesMap(), providerInfo.Resources, strategy, ignored)
}

func computeDefaultDataSources(
	providerInfo *info.Provider, strategy DataSourceStrategy, ignored map[string]bool,
) error {
	if strategy == nil {
		return nil
	}
	if providerInfo.DataSources == nil {
		providerInfo.DataSources = map[string]*info.DataSource{}
	}
	return applyComputedTokens(providerInfo.P.DataSourcesMap(), providerInfo.DataSources, strategy, ignored)
}

// For each key in the info map not present in the result map, compute a result and store
// it in the result map.
func applyComputedTokens[T info.Resource | info.DataSource](
	infoMap shim.ResourceMap, resultMap map[string]*T, tks Strategy[T],
	ignoredMappings map[string]bool,
) error {
	keys := make([]string, 0, infoMap.Len())
	infoMap.Range(func(key string, _ shim.Resource) bool {
		keys = append(keys, key)
		return true
	})
	sort.Strings(keys)

	var errs multierror.Error
	for _, k := range keys {
		v := resultMap[k]
		if v != nil || ignoredMappings[k] {
			// Skipping, since there is already a non-nil resource there.
			continue
		}
		v, err := tks(k)
		if err != nil {
			errs.Errors = append(errs.Errors, err)
			continue
		}
		if v != nil {
			resultMap[k] = v
		}
	}
	return errs.ErrorOrNil()
}
