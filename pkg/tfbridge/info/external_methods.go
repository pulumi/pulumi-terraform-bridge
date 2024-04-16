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

package info

// This file maintains backward comparability by maintaining methods on *Provider that are
// supplied by packages that import info.

import (
	"strings"
	_ "unsafe"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Materialize tfbridge.ApplyAutoAliases as a method in *Provider.
//
// This needs linkname to avoid go's package coherence rules.

//go:linkname applyAutoAliases tfbridge.ApplyAutoAliases
func applyAutoAliases(info *Provider) error

func (info *Provider) MustApplyAutoAliases() {
	err := applyAutoAliases(info)
	contract.AssertNoErrorf(err, "Failed to apply aliases")
}

func (info *Provider) ApplyAutoAliases() error { return applyAutoAliases(info) }

type Strategy struct {
	Resource   ResourceStrategy
	DataSource DataSourceStrategy
}

// A strategy for generating missing resources.
type ResourceStrategy = ElementStrategy[Resource]

// A strategy for generating missing datasources.
type DataSourceStrategy = ElementStrategy[DataSource]

// A generic remapping strategy.
type ElementStrategy[T Resource | DataSource] func(tfToken string, elem *T) error

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

//go:linkname computeTokens github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens.ComputeTokens
func computeTokens(info *Provider, opts Strategy) error

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
func (info *Provider) ComputeTokens(opts Strategy) error {
	return computeTokens(info, opts)
}

// Add mapped resources and datasources according to the given strategies.
//
// Panics if ApplyStrategy would return an error.
func (info *Provider) MustComputeTokens(opts Strategy) {
	err := computeTokens(info, opts)
	contract.AssertNoErrorf(err, "Failed to apply token mapping")
}
