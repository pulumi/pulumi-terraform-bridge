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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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
type DataSourceStrategy = Strategy[DataSourceInfo]

// A generic remapping strategy.
//
// NOTE: Experimental; We are still iterating on the design of this type, and it is
// subject to change without warning.
type Strategy[T ResourceInfo | DataSourceInfo] func(tfToken string) (*T, error)

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

// Convert a Terraform token to a Pulumi token with the standard mapping.
//
// The mapping is
//
//	(pkg, module, name) => pkg:module/lowerFirst(name):name
//
// NOTE: Experimental; We are still iterating on the design of this function, and it is
// subject to change without warning.
func MakeStandardToken(pkgName string) MakeToken {
	return func(module, name string) (string, error) {
		lowerName := string(unicode.ToLower(rune(name[0]))) + name[1:]
		return fmt.Sprintf("%s:%s/%s:%s", pkgName, module, lowerName, name), nil
	}
}

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
) DefaultStrategy {
	return TokensKnownModules(tfPackagePrefix, moduleName, nil, finalize)
}

func tokensKnownModules[T ResourceInfo | DataSourceInfo](
	prefix, defaultModule string, modules []string, new func(string, string) (*T, error),
) Strategy[T] {
	return func(tfToken string) (*T, error) {
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
//
// NOTE: Experimental; We are still iterating on the design of this function, and it is
// subject to change without warning.
func TokensKnownModules(
	tfPackagePrefix, defaultModule string, modules []string, finalize MakeToken,
) DefaultStrategy {
	// NOTE: We could turn this from a sort + linear lookup into a radix tree to recover
	// O(log(n)) performance (current is O(n*m)) where n = number of modules and m =
	// number of mappings.
	sort.Sort(sort.Reverse(sort.StringSlice(modules)))

	return DefaultStrategy{
		Resource: tokensKnownModules(tfPackagePrefix, defaultModule, modules,
			func(mod, tk string) (*ResourceInfo, error) {
				tk, err := finalize(mod, tk)
				if err != nil {
					return nil, err
				}
				return &ResourceInfo{Tok: tokens.Type(tk)}, nil
			}),
		DataSource: tokensKnownModules(tfPackagePrefix, defaultModule, modules,
			func(mod, tk string) (*DataSourceInfo, error) {
				tk, err := finalize(mod, "get"+tk)
				if err != nil {
					return nil, err
				}
				return &DataSourceInfo{Tok: tokens.ModuleMember(tk)}, nil
			}),
	}
}

func (ts DefaultStrategy) Unmappable(substring, reason string) DefaultStrategy {
	ts.DataSource = ts.DataSource.Unmappable(substring, reason)
	ts.Resource = ts.Resource.Unmappable(substring, reason)
	return ts
}

// Mark that a strategy cannot handle a sub-string.
//
// NOTE: Experimental; We are still iterating on the design of this function, and it is
// subject to change without warning.
func (ts Strategy[T]) Unmappable(substring, reason string) Strategy[T] {
	msg := fmt.Sprintf("cannot map tokens that contains '%s'", substring)
	if reason != "" {
		msg += ": " + reason
	}
	return func(tfToken string) (*T, error) {
		if strings.Contains(tfToken, substring) {
			return nil, UnmappableError{
				TfToken: tfToken,
				Reason:  fmt.Errorf(msg),
			}
		}
		return ts(tfToken)
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

// A strategy that can handle renames between runs of `make tfgen`.
//
// Renamed resources and datasources are appropriately back-aliased to their new names.
//
// NOTE: Experimental; We are still iterating on the design of this type, and it is
// subject to change without warning.
func RenameTokens(p *ProviderInfo, data []byte, strategy DefaultStrategy) (DefaultStrategy, func() []byte, error) {
	var mapping durableTokenMapping
	if data == nil {
		// No incoming data, so set up a new mapping
		mapping = durableTokenMapping{
			Resources:  map[string]*durableToken[tokens.Type]{},
			DataSource: map[string]*durableToken[tokens.ModuleMember]{},
		}
	} else {
		err := json.Unmarshal(data, &mapping)
		if err != nil {
			return DefaultStrategy{}, nil, fmt.Errorf("unmarshaling data: %w", err)
		}
	}

	serialize := func() []byte {
		bytes, err := json.Marshal(mapping)
		contract.AssertNoError(err)
		return bytes
	}

	return DefaultStrategy{
		Resource:   durableResource(p, mapping, strategy.Resource),
		DataSource: durableDataSource(p, mapping, strategy.DataSource),
	}, serialize, nil
}

func durableResource(p *ProviderInfo, mapping durableTokenMapping, strategy ResourceStrategy) ResourceStrategy {
	getToken := func(info *ResourceInfo) tokens.Type { return info.Tok }
	makeAlias := func(tfToken string, legacyTok tokens.Type, new *ResourceInfo) {
		p.RenameResourceWithAlias(tfToken,
			legacyTok, new.Tok,
			legacyTok.Module().String(), new.Tok.Module().String(),
			new)
	}
	return makeDurable(mapping.Resources, strategy, getToken, makeAlias)
}

func durableDataSource(p *ProviderInfo, mapping durableTokenMapping, strategy DataSourceStrategy) DataSourceStrategy {
	getToken := func(info *DataSourceInfo) tokens.ModuleMember { return info.Tok }
	makeAlias := func(tfToken string, legacyTok tokens.ModuleMember, new *DataSourceInfo) {
		p.RenameDataSource(tfToken,
			legacyTok, new.Tok,
			legacyTok.Module().String(), new.Tok.Module().String(),
			new)
	}
	return makeDurable(mapping.DataSource, strategy, getToken, makeAlias)
}

func makeDurable[T ~string, Info ResourceInfo | DataSourceInfo](
	mapping map[string]*durableToken[T], strategy Strategy[Info],
	getToken func(*Info) T, makeAlias func(string, T, *Info),
) Strategy[Info] {
	return func(tfToken string) (*Info, error) {
		result, err := strategy(tfToken)
		if err != nil || result == nil {
			return result, err
		}

		tk := getToken(result)
		if tk != "" {
			durable, found := mapping[tfToken]
			if !found {
				mapping[tfToken] = &durableToken[T]{
					Current: tk,
				}
			} else {
				if tk != durable.Current {
					durable.History = append(durable.History, durable.Current)
					durable.Current = tk
				}
				for _, alias := range durable.History {
					makeAlias(tfToken, alias, result)
				}
			}
		}
		return result, nil
	}
}

type durableTokenMapping struct {
	Resources  map[string]*durableToken[tokens.Type]         `json:"resources,omitempty"`
	DataSource map[string]*durableToken[tokens.ModuleMember] `json:"datasources,omitempty"`
}

type durableToken[T ~string] struct {
	Current T   `json:"current,omitempty"`
	History []T `json:"history,omitempty"`
}
