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

package tfbridge

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A generic remapping strategy.
type ElementStrategy[T ResourceInfo | DataSourceInfo] func(tfToken string, elem *T) error

// Describe the mapping from TF resource and datasource tokens to Pulumi resources and
// datasources.
type Strategy struct {
	Resource   ResourceStrategy
	DataSource DataSourceStrategy
}

// A strategy for generating missing resources.
type ResourceStrategy = ElementStrategy[ResourceInfo]

// A strategy for generating missing datasources.
type DataSourceStrategy = ElementStrategy[DataSourceInfo]

// Assigns resource and data source tokens unless already specified by the user.
//
// Context: bridging a provider requires every TF resource and data source have a matching
// entry in [Resources] and [DataSources]. Each entry needs to minimally specifying the
// Pulumi token in [ResourceInfo.Tok]. The mapping entries can be done by hand for smaller
// provides but become a chore for providers with 1000s of entries.
//
// ComputeTokens scans TF resources and datasources to populate missing entries as needed
// with automatically computed Pulumi tokens.
//
// ComputeTokens always respects and does not modify pre-existing entires. The user can
// therefore manually override the token decision by providing [ResourceInfo] or
// [DataSourceInfo] entry prior to calling ComputeTokens.
//
// ComputeTokens respects [ProviderInfo.IgnoreMappings]. It will not create a mapping for
// any token in [ProviderInfo.IgnoreMappings].
func (info *ProviderInfo) ComputeTokens(opts Strategy) error {
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

// Add mapped resources and datasources according to the given strategies.
//
// Panics if ApplyStrategy would return an error.
func (info *ProviderInfo) MustComputeTokens(opts Strategy) {
	err := info.ComputeTokens(opts)
	contract.AssertNoErrorf(err, "Failed to apply default tokens")
}

func ignoredTokens(info *ProviderInfo) map[string]bool {
	ignored := map[string]bool{}
	if info == nil {
		return ignored
	}
	for _, tk := range info.IgnoreMappings {
		ignored[tk] = true
	}
	return ignored
}

func computeDefaultResources(info *ProviderInfo, strategy ResourceStrategy, ignored map[string]bool) error {
	if strategy == nil {
		return nil
	}
	if info.Resources == nil {
		info.Resources = map[string]*ResourceInfo{}
	}
	return applyComputedTokens(info.P.ResourcesMap(), info.Resources, strategy, ignored)
}

func computeDefaultDataSources(info *ProviderInfo, strategy DataSourceStrategy, ignored map[string]bool) error {
	if strategy == nil {
		return nil
	}
	if info.DataSources == nil {
		info.DataSources = map[string]*DataSourceInfo{}
	}
	return applyComputedTokens(info.P.DataSourcesMap(), info.DataSources, strategy, ignored)
}

// For each key in the info map not present in the result map, compute a result and store
// it in the result map.
func applyComputedTokens[T ResourceInfo | DataSourceInfo](
	infoMap shim.ResourceMap, resultMap map[string]*T, tks ElementStrategy[T],
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
		if ignoredMappings[k] {
			// Skipping, since there is already a non-nil resource there.
			continue
		}
		v := resultMap[k]
		var newT bool
		if v == nil {
			v = new(T)
			newT = true
		}
		err := tks(k, v)
		if err != nil {
			errs.Errors = append(errs.Errors, err)
			continue
		}

		// We only add a value to the map if it wasn't there before *and* it is
		// non-zero.
		if newT && !reflect.ValueOf(*v).IsZero() {
			resultMap[k] = v
		}
	}
	return errs.ErrorOrNil()
}

func (ts Strategy) Ignore(substring string) Strategy {
	ts.DataSource = ts.DataSource.Ignore(substring)
	ts.Resource = ts.Resource.Ignore(substring)
	return ts
}

// Instruct the strategy not to apply to a token that contains substring.
func (ts ElementStrategy[T]) Ignore(substring string) ElementStrategy[T] {
	return func(tfToken string, elem *T) error {
		if strings.Contains(tfToken, substring) {
			return nil
		}
		return ts(tfToken, elem)
	}
}
