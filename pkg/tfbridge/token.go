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
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// A strategy for generating missing resources.
type ResourceTokenStrategy = TokenStrategy[ResourceInfo]

// A strategy for generating missing datasources.
type DataSourceTokenStrategy = TokenStrategy[DataSourceInfo]

// A function that joins a module and name into a pulumi type token.
//
// For example:
//
//	func(module, name string) (string, error) {
//	 return fmt.Sprintf("pkgName:%s:%s", module, name), nil
//	}
type FinalizeToken func(module, name string) (string, error)

type TokenStrategy[T any] func(tfToken string, tfTokens []string) (*T, error)

func upperCamelCase(s string) string { return cgstrings.UppercaseFirst(camelCase(s)) }

func camelCase(s string) string {
	return cgstrings.ModifyStringAroundDelimeter(s, "_", cgstrings.UppercaseFirst)
}

// A strategy that assigns all tokens to the same module.
//
// For example:
//
//	rStrat, dStrat := TokensSingleModule("pkgName_", "index", finalize)
//
// The above example would transform "pkgName_foo" into "pkgName:index:Foo".
func TokensSingleModule(
	tfPackagePrefix, moduleName string, finalize FinalizeToken,
) (ResourceTokenStrategy, DataSourceTokenStrategy) {
	return TokensKnownModules(tfPackagePrefix, moduleName, nil, finalize)
}

func tokensKnownModules[T any](
	prefix, defaultModule string, modules []string, new func(string, string) (*T, error),
) TokenStrategy[T] {
	return func(tfToken string, _ []string) (*T, error) {
		tk := strings.TrimPrefix(tfToken, prefix)
		if len(tk) == len(tfToken) {
			return nil, fmt.Errorf("token '%s' missing package prefix '%s'", tfToken, prefix)
		}
		mod := defaultModule
		for _, m := range modules {
			if strings.HasPrefix(tk, m) {
				mod = m
				break
			}
		}
		if mod == "" {
			return nil, fmt.Errorf("could not find a module that prefixes '%s' in '%#v'", tk, modules)
		}
		return new(camelCase(mod), upperCamelCase(strings.TrimPrefix(tk, mod)))
	}
}

// A strategy for assigning tokens to a hand generated set of modules.
//
// If defaultModule is "", then the returned strategies will error on not encountering a matching module.
func TokensKnownModules(
	tfPackagePrefix, defaultModule string, modules []string, finalize FinalizeToken,
) (ResourceTokenStrategy, DataSourceTokenStrategy) {
	// NOTE: We could turn this from a sort + linear lookup into a radix tree to recover
	// O(log(n)) performance (current is O(n*m)) where n = number of modules and m =
	// number of mappings.
	sort.Sort(sort.Reverse(sort.StringSlice(modules)))

	return tokensKnownModules(tfPackagePrefix, defaultModule, modules, func(mod, tk string) (*ResourceInfo, error) {
			tk, err := finalize(mod, tk)
			if err != nil {
				return nil, err
			}
			return &ResourceInfo{Tok: tokens.Type(tk)}, nil
		}), tokensKnownModules(tfPackagePrefix, defaultModule, modules, func(mod, tk string) (*DataSourceInfo, error) {
			tk, err := finalize(mod, "get"+tk)
			if err != nil {
				return nil, err
			}
			return &DataSourceInfo{Tok: tokens.ModuleMember(tk)}, nil
		})
}

func (ts TokenStrategy[T]) Unmappable(substring string) TokenStrategy[T] {
	return func(tfToken string, tfTokens []string) (*T, error) {
		if strings.Contains(tfToken, substring) {
			return nil, fmt.Errorf("token '%s' contains un-map-able sub-string '%s'", tfToken, substring)
		}
		return ts(tfToken, tfTokens)
	}
}
