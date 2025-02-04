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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func knownModules[T info.Resource | info.DataSource](
	prefix, defaultModule string, modules []string,
	apply func(string, string, *T, error) error,
	moduleTransform func(string) string,
) info.ElementStrategy[T] {
	return func(tfToken string, elem *T) error {
		var tk string
		if t, foundPrefix := strings.CutPrefix(tfToken, prefix); foundPrefix {
			if t == "" {
				// If the entire tfToken is the prefix, then we don't strip the prefix. It
				// will act as both prefix, module and name.
				tk = tfToken
			} else {
				tk = t
			}
		} else {
			return apply("", upperCamelCase(tk), elem,
				fmt.Errorf("token '%s' missing package prefix '%s'", tfToken, prefix))
		}

		mod := defaultModule
		for _, m := range modules {
			if strings.HasPrefix(tk, m) {
				mod = m
				break
			}
		}
		var err error
		if mod == "" {
			err = fmt.Errorf("could not find a module that prefixes '%s' in '%#v'", tk, modules)
		}
		return apply(moduleTransform(mod), upperCamelCase(strings.TrimPrefix(tk, mod)), elem, err)
	}
}

// A strategy for assigning tokens to a hand generated set of modules.
//
// If defaultModule is "", then the returned strategies will error on not encountering a matching module.
func KnownModules(
	tfPackagePrefix, defaultModule string, modules []string, finalize Make,
) Strategy {
	// NOTE: We could turn this from a sort + linear lookup into a radix tree to recover
	// O(log(n)) performance (current is O(n*m)) where n = number of modules and m =
	// number of mappings.
	sort.Sort(sort.Reverse(sort.StringSlice(modules)))

	return Strategy{
		Resource: knownModules(tfPackagePrefix, defaultModule, modules,
			knownResource(finalize), camelCase),
		DataSource: knownModules(tfPackagePrefix, defaultModule, modules,
			knownDataSource(finalize), camelCase),
	}
}

func knownResource(finalize Make) func(mod, tk string, r *info.Resource, err error) error {
	return func(mod, tk string, r *info.Resource, err error) error {
		if r.Tok != "" {
			return nil
		}
		if err != nil {
			return err
		}
		tk, err = finalize(mod, tk)
		if err != nil {
			return err
		}
		r.Tok = tokens.Type(tk)
		return nil
	}
}

func knownDataSource(finalize Make) func(mod, tk string, d *info.DataSource, err error) error {
	return func(mod, tk string, d *info.DataSource, err error) error {
		if d.Tok != "" {
			return nil
		}
		if err != nil {
			return err
		}
		tk, err = finalize(mod, "get"+tk)
		if err != nil {
			return err
		}
		d.Tok = tokens.ModuleMember(tk)
		return nil
	}
}

func KnownModulesWithInferredFallback(
	p *info.Provider, tfPackagePrefix, defaultModule string, modules []string, finalize Make,
) (Strategy, error) {
	opts := &InferredModulesOpts{
		TfPkgPrefix:          tfPackagePrefix,
		MinimumModuleSize:    2,
		MimimumSubmoduleSize: 2,
	}
	inferred, err := InferredModules(p, finalize, opts)
	if err != nil {
		return Strategy{}, err
	}
	return tokenStrategyWithFallback(
		KnownModules(tfPackagePrefix, defaultModule, modules, finalize),
		inferred,
	), nil
}
