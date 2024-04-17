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

package tokens

import (
	"fmt"
	"reflect"
	"sort"
	"unicode"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

const (
	defaultMinimumModuleSize    = 5
	defaultMinimumSubmoduleSize = 5
)

// A function that joins a module and name into a pulumi type token.
//
// For example:
//
//	func(module, name string) (string, error) {
//	 return fmt.Sprintf("pkgName:%s:%s", module, name), nil
//	}
type Make func(module, name string) (string, error)

// Convert a Terraform token to a Pulumi token with the standard mapping.
//
// The mapping is
//
//	(pkg, module, name) => pkg:module/lowerFirst(name):name
func MakeStandard(pkgName string) Make {
	return func(module, name string) (string, error) {
		lowerName := string(unicode.ToLower(rune(name[0]))) + name[1:]
		return fmt.Sprintf("%s:%s/%s:%s", pkgName, module, lowerName, name), nil
	}
}

func upperCamelCase(s string) string { return cgstrings.UppercaseFirst(camelCase(s)) }

func camelCase(s string) string {
	return cgstrings.ModifyStringAroundDelimeter(s, "_", cgstrings.UppercaseFirst)
}

func checkedApply[T comparable](dst *T, src T) {
	if dst == nil {
		panic("checkedApply cannot be applied to nil dst")
	}
	var empty T
	if (*dst) == empty {
		*dst = src
	}
}

// Describe the mapping from TF resource and datasource tokens to Pulumi resources and
// datasources.
type Strategy = info.Strategy

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
func ComputeTokens(info *info.Provider, opts Strategy) error {
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

type (
	ResourceStrategy   = info.ResourceStrategy
	DataSourceStrategy = info.DataSourceStrategy
)

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

func computeDefaultResources(p *info.Provider, strategy ResourceStrategy, ignored map[string]bool) error {
	if strategy == nil {
		return nil
	}
	if p.Resources == nil {
		p.Resources = map[string]*info.Resource{}
	}
	return applyComputedTokens(p.P.ResourcesMap(), p.Resources, strategy, ignored)
}

func computeDefaultDataSources(p *info.Provider, strategy DataSourceStrategy, ignored map[string]bool) error {
	if strategy == nil {
		return nil
	}
	if p.DataSources == nil {
		p.DataSources = map[string]*info.DataSource{}
	}
	return applyComputedTokens(p.P.DataSourcesMap(), p.DataSources, strategy, ignored)
}

// For each key in the info map not present in the result map, compute a result and store
// it in the result map.
func applyComputedTokens[T info.Resource | info.DataSource](
	infoMap shim.ResourceMap, resultMap map[string]*T, tks info.ElementStrategy[T],
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
