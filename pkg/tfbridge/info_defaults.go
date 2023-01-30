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
	"unicode"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	multierror "github.com/hashicorp/go-multierror"
)

// SetDefaults populates mappings such as the Resources mapping by computing sensible defaults in code. This is
// especially useful for larger providers that have thousands of entries in Resources. Manually specified mappings
// always take precedence: SetDefaults will never override an entry in the Resources map, only add new entries.
func (info *ProviderInfo) SetDefaults(defaults ProviderInfoDefaults) error {
	var errs multierror.Error
	if defaults.Resources != nil {
		if info.Resources == nil {
			info.Resources = map[string]*ResourceInfo{}
		}

		if err := setDefaults(info.P.ResourcesMap(), info.Resources, defaults.Resources); err != nil {
			errs.Errors = append(errs.Errors, fmt.Errorf("resources:\n%w", err))
		}
	}
	if defaults.DataSources != nil {
		if info.DataSources == nil {
			info.DataSources = map[string]*DataSourceInfo{}
		}
		if err := setDefaults(info.P.DataSourcesMap(), info.DataSources, defaults.DataSources); err != nil {
			errs.Errors = append(errs.Errors, fmt.Errorf("datasources:\n%w", err))
		}
	}
	return errs.ErrorOrNil()
}

type ProviderInfoDefaults struct {
	Resources   func(resourceToken string) (*ResourceInfo, error)
	DataSources func(dataSourceToken string) (*DataSourceInfo, error)
}

func (d ProviderInfoDefaults) Unmappable(substring string) ProviderInfoDefaults {
	return ProviderInfoDefaults{
		Resources:   ensureUnmappable(substring, d.Resources),
		DataSources: ensureUnmappable(substring, d.DataSources),
	}
}

func ensureUnmappable[T any](
	substring string,
	f func(t string) (*T, error),
) func(t string) (*T, error) {
	return func(t string) (*T, error) {
		if strings.Contains(t, substring) {
			return nil, fmt.Errorf("token %q matches a substring %q that was specified as Unmappable",
				t, substring)
		}
		return f(t)
	}
}

type TFToken struct {
	Module string
	Name   string
}

type TFTokenParser func(tfToken string) (*TFToken, error)

func SingleModule(prefix, defaultModule string) TFTokenParser {
	return KnownModules(prefix, defaultModule, nil)
}

func KnownModules(prefix, defaultModule string, modules []string) TFTokenParser {
	// NOTE: We could turn this from a sort + linear lookup into a radix tree to recover
	// O(log(n)) performance (current is O(n*m)) where n = number of modules and m =
	// number of mappings.
	sort.Sort(sort.Reverse(sort.StringSlice(modules)))

	return func(tfToken string) (*TFToken, error) {
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
		return &TFToken{
			Module: mod,
			Name:   strings.TrimPrefix(tk, mod),
		}, nil
	}
}

func SuggestTokens(pkg string, tfTokenParser TFTokenParser) ProviderInfoDefaults {
	return ProviderInfoDefaults{
		Resources: func(resourceToken string) (*ResourceInfo, error) {
			tfTok, err := tfTokenParser(resourceToken)
			if err != nil {
				return nil, err
			}
			token := SuggestResourceToken(pkg, tfTok)
			return &ResourceInfo{Tok: token}, nil
		},
		DataSources: func(dataSourceToken string) (*DataSourceInfo, error) {
			tfTok, err := tfTokenParser(dataSourceToken)
			if err != nil {
				return nil, err
			}
			token := SuggestDataSourceToken(pkg, tfTok)
			return &DataSourceInfo{Tok: token}, nil
		},
	}
}

func SuggestResourceToken(pkg string, token *TFToken) tokens.Type {
	mod := camelCase(token.Module)
	res := upperCamelCase(token.Name)
	return MakeResource(pkg, mod, res)
}

func SuggestDataSourceToken(pkg string, token *TFToken) tokens.ModuleMember {
	mod := camelCase(token.Module)
	fn := upperCamelCase(token.Name)
	submod := string(unicode.ToLower(rune(fn[0]))) + fn[1:]
	return MakeMember(pkg, mod+"/"+submod, "get"+fn)
}

func setDefaults[T ResourceInfo | DataSourceInfo](
	infoMap shim.ResourceMap,
	resultMap map[string]*T,
	defaultT func(tfToken string) (*T, error),
) error {
	keys := make([]string, 0, infoMap.Len())
	infoMap.Range(func(key string, _ shim.Resource) bool {
		keys = append(keys, key)
		return true
	})
	sort.Strings(keys)

	var errs multierror.Error
	for _, k := range keys {
		if _, alreadySet := resultMap[k]; alreadySet {
			// Skipping, since there is already a non-nil resource there.
			continue
		}
		v, err := defaultT(k)
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
