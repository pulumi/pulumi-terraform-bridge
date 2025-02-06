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

// Package fallbackstrat provides strategies for generating tokens.
// The strategies here fall back to using the [tokens.InferredModules] strategy
// if the primary strategy fails to find a module for a given token.
package fallbackstrat

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
)

func tokenStrategyWithFallback(
	strategy tokens.Strategy,
	fallback tokens.Strategy,
) tokens.Strategy {
	resourceFallback := func(tfToken string, elem *info.Resource) error {
		if err := strategy.Resource(tfToken, elem); err != nil {
			return fallback.Resource(tfToken, elem)
		}
		return nil
	}
	dataSourceFallback := func(tfToken string, elem *info.DataSource) error {
		if err := strategy.DataSource(tfToken, elem); err != nil {
			return fallback.DataSource(tfToken, elem)
		}
		return nil
	}
	return tokens.Strategy{
		Resource:   resourceFallback,
		DataSource: dataSourceFallback,
	}
}

// KnownModulesWithInferredFallback returns a strategy that uses the KnownModules
// strategy for the given modules, but falls back to the inferred strategy if
// the KnownModules strategy fails to find a module for a given token.
//
// See [tokens.KnownModules] and [tokens.InferredModules] for more information.
func KnownModulesWithInferredFallback(
	p *info.Provider, tfPackagePrefix, defaultModule string, modules []string, finalize tokens.Make,
) (tokens.Strategy, error) {
	opts := &tokens.InferredModulesOpts{
		TfPkgPrefix:          tfPackagePrefix,
		MinimumModuleSize:    2,
		MimimumSubmoduleSize: 2,
	}
	inferred, err := tokens.InferredModules(p, finalize, opts)
	if err != nil {
		return tokens.Strategy{}, err
	}
	return tokenStrategyWithFallback(
		tokens.KnownModules(tfPackagePrefix, defaultModule, modules, finalize),
		inferred,
	), nil
}

// MappedModulesWithInferredFallback returns a strategy that uses the MappedModules
// strategy for the given modules, but falls back to the inferred strategy if
// the MappedModules strategy fails to find a module for a given token.
//
// See [tokens.MappedModules] and [tokens.InferredModules] for more information.
func MappedModulesWithInferredFallback(
	p *info.Provider, tfPackagePrefix, defaultModule string, modules map[string]string, finalize tokens.Make,
) (tokens.Strategy, error) {
	inferred, err := tokens.InferredModules(p, finalize, &tokens.InferredModulesOpts{
		TfPkgPrefix:          tfPackagePrefix,
		MinimumModuleSize:    2,
		MimimumSubmoduleSize: 2,
	})
	if err != nil {
		return tokens.Strategy{}, err
	}
	return tokenStrategyWithFallback(
		tokens.MappedModules(tfPackagePrefix, defaultModule, modules, finalize),
		inferred,
	), nil
}
