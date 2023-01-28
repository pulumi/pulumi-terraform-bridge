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
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/utils"
)

// A strategy for generating missing resources.
//
// NOTE: Experimental; We are still iterating on the design of this type, and it is
// subject to change without warning.
type ResourceStrategy = Strategy[ResourceInfo]

// A strategy for generating missing datasources.
//
// NOTE: Experimental; We are still iterating on the design of this type, and it is
// subject to change without warning.
type DatasourceStrategy = Strategy[DataSourceInfo]

// A generic remapping strategy.
//
// NOTE: Experimental; We are still iterating on the design of this type, and it is
// subject to change without warning.
type Strategy[T ResourceInfo | DataSourceInfo] func(tfToken string, tfTokens []string) (*T, error)

// A function that joins a module and name into a pulumi type token.
//
// For example:
//
//	func(module, name string) (string, error) {
//	 return fmt.Sprintf("pkgName:%s:%s", module, name), nil
//	}
//
// NOTE: Experimental; We are still iterating on the design of this type, and it is
// subject to change without warning.
type MakeToken func(module, name string) (string, error)

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
//
// NOTE: Experimental; We are still iterating on the design of this function, and it is
// subject to change without warning.
func TokensSingleModule(
	tfPackagePrefix, moduleName string, finalize MakeToken,
) (ResourceStrategy, DatasourceStrategy) {
	return TokensKnownModules(tfPackagePrefix, moduleName, nil, finalize)
}

func tokensKnownModules[T ResourceInfo | DataSourceInfo](
	prefix, defaultModule string, modules *utils.SortedMap[string, string], new func(string, string) (*T, error),
) Strategy[T] {
	return func(tfToken string, _ []string) (*T, error) {
		tk := strings.TrimPrefix(tfToken, prefix)
		if len(tk) == len(tfToken) {
			return nil, fmt.Errorf("token '%s' missing package prefix '%s'", tfToken, prefix)
		}
		mod := defaultModule
		for _, m := range modules.Keys() {
			if strings.HasPrefix(tk, m) {
				mod, _ = modules.Get(m)
				break
			}
		}
		if mod == "" {
			return nil, fmt.Errorf("could not find a module that prefixes '%s' in '%#v'", tk, modules.Keys())
		}
		return new(mod, upperCamelCase(strings.TrimPrefix(tk, mod)))
	}
}

// A strategy for assigning tokens to a hand generated set of modules with a default
// mapping from TF modules to pulumi modules.
//
// If defaultModule is "", then the returned strategies will error on not encountering a matching module.
//
// NOTE: Experimental; We are still iterating on the design of this function, and it is
// subject to change without warning.
func TokensKnownModules(
	tfPackagePrefix, defaultModule string, modules []string, finalize MakeToken,
) (ResourceStrategy, DatasourceStrategy) {
	m := make(map[string]string, len(modules))
	for _, mod := range modules {
		m[mod] = camelCase(mod)
	}
	return TokensKnownModulesMap(tfPackagePrefix, defaultModule, m, finalize)
}

// A strategy for assigning tokens to a hand generated set of modules.
//
// If defaultModule is "", then the returned strategies will error on not encountering a matching module.
//
// NOTE: Experimental; We are still iterating on the design of this function, and it is
// subject to change without warning.
func TokensKnownModulesMap(
	tfPackagePrefix, defaultModule string, modules map[string]string, finalize MakeToken,
) (ResourceStrategy, DatasourceStrategy) {
	// NOTE: We could turn this from a sort + linear lookup into a radix tree to recover
	// O(log(n)) performance (current is O(n*m)) where n = number of modules and m =
	// number of mappings.
	m := utils.NewSortedMap(modules)

	return tokensKnownModules(tfPackagePrefix, defaultModule, m, func(mod, tk string) (*ResourceInfo, error) {
			tk, err := finalize(mod, tk)
			if err != nil {
				return nil, err
			}
			return &ResourceInfo{Tok: tokens.Type(tk)}, nil
		}), tokensKnownModules(tfPackagePrefix, defaultModule, m, func(mod, tk string) (*DataSourceInfo, error) {
			tk, err := finalize(mod, "get"+tk)
			if err != nil {
				return nil, err
			}
			return &DataSourceInfo{Tok: tokens.ModuleMember(tk)}, nil
		})
}

// Mark that a strategy cannot handle a sub-string.
//
// NOTE: Experimental; We are still iterating on the design of this function, and it is
// subject to change without warning.
func (ts Strategy[T]) Unmappable(substring string) Strategy[T] {
	return func(tfToken string, tfTokens []string) (*T, error) {
		if strings.Contains(tfToken, substring) {
			return nil, UnmappableError{
				TfToken: tfToken,
				Reason:  fmt.Errorf("contains unmapable sub-string '%s'", substring),
			}
		}
		return ts(tfToken, tfTokens)
	}
}

// Indicate that a token cannot be mapped.
//
// NOTE: Experimental; We are still iterating on the design of this type, and it is
// subject to change without warning.
type UnmappableError struct {
	TfToken string
	Reason  error
}

func (err UnmappableError) Error() string {
	return fmt.Sprintf("'%s' unmappable: %s", err.TfToken, err.Reason)
}

func (err UnmappableError) Unwrap() error {
	return err.Reason
}
