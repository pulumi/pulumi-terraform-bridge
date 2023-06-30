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
	"sort"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	b "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func knownModules[T b.ResourceInfo | b.DataSourceInfo](
	prefix, defaultModule string, modules []string,
	apply func(string, string, *T) error,
	moduleTransform func(string) string,
) b.ElementStrategy[T] {
	return func(tfToken string, elem *T) error {
		tk := strings.TrimPrefix(tfToken, prefix)
		if len(tk) == len(tfToken) {
			return fmt.Errorf("token '%s' missing package prefix '%s'", tfToken, prefix)
		}
		mod := defaultModule
		for _, m := range modules {
			if strings.HasPrefix(tk, m) {
				mod = m
				break
			}
		}
		if mod == "" {
			return fmt.Errorf("could not find a module that prefixes '%s' in '%#v'", tk, modules)
		}
		return apply(moduleTransform(mod), upperCamelCase(strings.TrimPrefix(tk, mod)), elem)
	}
}

// A strategy for assigning tokens to a hand generated set of modules.
//
// If defaultModule is "", then the returned strategies will error on not encountering a matching module.
func KnownModules(
	tfPackagePrefix, defaultModule string, modules []string, finalize Make,
) b.Strategy {
	// NOTE: We could turn this from a sort + linear lookup into a radix tree to recover
	// O(log(n)) performance (current is O(n*m)) where n = number of modules and m =
	// number of mappings.
	sort.Sort(sort.Reverse(sort.StringSlice(modules)))

	return b.Strategy{
		Resource: knownModules(tfPackagePrefix, defaultModule, modules,
			func(mod, tk string, r *b.ResourceInfo) error {
				tk, err := finalize(mod, tk)
				if err != nil {
					return err
				}
				checkedApply(&r.Tok, tokens.Type(tk))
				return nil
			}, camelCase),
		DataSource: knownModules(tfPackagePrefix, defaultModule, modules,
			func(mod, tk string, d *b.DataSourceInfo) error {
				tk, err := finalize(mod, "get"+tk)
				if err != nil {
					return err
				}
				checkedApply(&d.Tok, tokens.ModuleMember(tk))
				return nil
			}, camelCase),
	}
}
