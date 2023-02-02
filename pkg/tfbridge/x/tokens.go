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

package x

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	b "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A function that joins a module and name into a pulumi type token.
//
// For example:
//
//	func(module, name string) (string, error) {
//	 return fmt.Sprintf("pkgName:%s:%s", module, name), nil
//	}
type MakeToken func(module, name string) (string, error)

// A strategy that assigns all tokens to the same module.
//
// For example:
//
//	rStrat, dStrat := TokensSingleModule("pkgName_", "index", finalize)
//
// The above example would transform "pkgName_foo" into "pkgName:index:Foo".
func TokensSingleModule(
	tfPackagePrefix, moduleName string, finalize MakeToken,
) DefaultStrategy {
	return TokensKnownModules(tfPackagePrefix, moduleName, nil, finalize)
}

func tokensKnownModules[T b.ResourceInfo | b.DataSourceInfo](
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
func TokensKnownModules(
	tfPackagePrefix, defaultModule string, modules []string, finalize MakeToken,
) DefaultStrategy {
	// NOTE: We could turn this from a sort + linear lookup into a radix tree to recover
	// O(log(n)) performance (current is O(n*m)) where n = number of modules and m =
	// number of mappings.
	sort.Sort(sort.Reverse(sort.StringSlice(modules)))

	return DefaultStrategy{
		Resource: tokensKnownModules(tfPackagePrefix, defaultModule, modules,
			func(mod, tk string) (*b.ResourceInfo, error) {
				tk, err := finalize(mod, tk)
				if err != nil {
					return nil, err
				}
				return &b.ResourceInfo{Tok: tokens.Type(tk)}, nil
			}),
		DataSource: tokensKnownModules(tfPackagePrefix, defaultModule, modules,
			func(mod, tk string) (*b.DataSourceInfo, error) {
				tk, err := finalize(mod, "get"+tk)
				if err != nil {
					return nil, err
				}
				return &b.DataSourceInfo{Tok: tokens.ModuleMember(tk)}, nil
			}),
	}
}

// Convert a Terraform token to a Pulumi token with the standard mapping.
//
// The mapping is
//
//	(pkg, module, name) => pkg:module/lowerFirst(name):name
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

type tokenHistory[T ~string] struct {
	Current T          `json:"current"`        // the current Pulumi token for the resource
	Past    []alias[T] `json:"past,omitempty"` // Previous tokens
}

type alias[T ~string] struct {
	Name      T    `json:"name"`      // The previous token.
	InCodegen bool `json:"inCodegen"` // If the alias is a fully generated resource, or just a schema alias.
}

type aliasHistory struct {
	Resources   map[string]*tokenHistory[tokens.Type]         `json:"resources"`
	DataSources map[string]*tokenHistory[tokens.ModuleMember] `json:"datasources"`
}

// Make a default strategy aliasing, so it is safe for the inner strategy to make breaking
// changes.
//
// artifact is the byte sequence used to store history. The next artifact is returned by
// calling the returned callback. The returned callback must be called on the provider
// that utilized the returned strategy, after ComputeDefaults was called.
//
// artifact should be considered an opaque blob.
func Aliasing(artifact []byte, defaults DefaultStrategy) (DefaultStrategy, func(*ProviderInfo) []byte, error) {
	var hist aliasHistory
	if artifact == nil {
		hist = aliasHistory{
			Resources:   map[string]*tokenHistory[tokens.Type]{},
			DataSources: map[string]*tokenHistory[tokens.ModuleMember]{},
		}
	} else {
		err := json.Unmarshal(artifact, &hist)
		if err != nil {
			return DefaultStrategy{}, nil, fmt.Errorf("parsing artifact: %w", err)
		}
	}
	remaps := &[]func(*ProviderInfo){}

	serialize := func(p *ProviderInfo) []byte {
		for _, r := range *remaps {
			r(p)
		}
		bytes, err := json.MarshalIndent(hist, "", "    ")
		contract.AssertNoError(err)
		return bytes
	}

	return aliasing(hist, defaults, remaps), serialize, nil
}

func aliasing(hist aliasHistory, defaults DefaultStrategy, remaps *[]func(*ProviderInfo)) DefaultStrategy {
	return DefaultStrategy{
		Resource:   aliasResources(hist.Resources, defaults.Resource, remaps),
		DataSource: aliasDataSources(hist.DataSources, defaults.DataSource, remaps),
	}
}

func aliasResources(
	hist map[string]*tokenHistory[tokens.Type],
	strategy ResourceStrategy, remaps *[]func(*ProviderInfo),
) ResourceStrategy {
	return func(tfToken string) (*ResourceInfo, error) {
		computed, err := strategy(tfToken)
		if err != nil {
			return nil, err
		}

		prev, hasPrev := hist[tfToken]
		if !hasPrev {
			// It's not in the history, so it must be new. Stick it in the history for
			// next time.
			*remaps = append(*remaps, func(*ProviderInfo) {
				hist[tfToken] = &tokenHistory[tokens.Type]{
					Current: computed.Tok,
				}
			})
		} else if prev.Current != computed.Tok {
			// It's in history, but something has changed. Update the history to reflect
			// the new reality, then add aliases.
			*remaps = append(*remaps, func(p *ProviderInfo) {
				// re-fetch the resource, to make sure we have the right pointer.
				computed, ok := p.Resources[tfToken]
				contract.Assertf(ok, "Resource %s decided but not present", tfToken)

				var alreadyPresent bool
				for _, a := range prev.Past {
					if a.Name == prev.Current {
						alreadyPresent = true
						break
					}
				}
				if !alreadyPresent {
					prev.Past = append(prev.Past, alias[tokens.Type]{
						Name:      prev.Current,
						InCodegen: true,
					})
				}
				for _, a := range prev.Past {
					legacy := a.Name
					if a.InCodegen {
						p.RenameResourceWithAlias(tfToken, legacy,
							computed.Tok, legacy.Module().Name().String(),
							computed.Tok.Module().Name().String(), computed)
					} else {
						computed.Aliases = append(computed.Aliases, AliasInfo{Type: (*string)(&legacy)})
					}
				}
			})

		}

		return computed, nil
	}
}

func aliasDataSources(
	hist map[string]*tokenHistory[tokens.ModuleMember],
	strategy DataSourceStrategy, remaps *[]func(*ProviderInfo),
) DataSourceStrategy {
	return func(tfToken string) (*DataSourceInfo, error) {
		computed, err := strategy(tfToken)
		if err != nil {
			return nil, err
		}

		prev, hasPrev := hist[tfToken]
		if !hasPrev {
			// It's not in the history, so it must be new. Stick it in the history for
			// next time.
			hist[tfToken] = &tokenHistory[tokens.ModuleMember]{
				Current: computed.Tok,
			}
		} else if prev.Current != computed.Tok {
			// It's in history, but something has changed. Update the history to reflect
			// the new reality, then add aliases.
			*remaps = append(*remaps, func(p *ProviderInfo) {
				// re-fetch the resource, to make sure we have the right pointer.
				computed, ok := p.DataSources[tfToken]
				contract.Assertf(ok, "DataSource %s decided but not present", tfToken)
				alias := alias[tokens.ModuleMember]{
					Name: prev.Current,
				}
				prev.Past = append(prev.Past, alias)
				for _, a := range prev.Past {
					legacy := a.Name
					p.RenameDataSource(tfToken, legacy,
						computed.Tok, legacy.Module().Name().String(),
						computed.Tok.Module().Name().String(), computed)
				}
			})

		}

		return computed, nil
	}
}
