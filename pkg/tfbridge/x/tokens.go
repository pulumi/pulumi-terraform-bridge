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
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	md "github.com/pulumi/pulumi-terraform-bridge/v3/internal/metadata"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/metadata"
	b "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
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

// Finish an alias operation.
//
// This writes the finished operation to the passed in metadata.Provider, as well as
// updating the passed ProviderInfo passed to ComputeDefault.
type FinishAlias = func(*b.ProviderInfo)

func AutoAliasing(providerInfo *b.ProviderInfo, artifact metadata.Provider) error {
	remaps := &[]func(*b.ProviderInfo){}

	hist, err := getHistory(artifact)
	if err != nil {
		return err
	}

	for tfToken, computed := range providerInfo.Resources {
		aliasResource(hist.Resources, computed, tfToken, remaps)
	}

	for tfToken, computed := range providerInfo.DataSources {
		aliasDataSource(hist.DataSources, computed, tfToken, remaps)
	}

	for _, r := range *remaps {
		r(providerInfo)
	}

	if err := md.Set(artifact, artifactKey, hist); err != nil {
		// Set fails only when `hist` is not serializable. Because `hist` is
		// composed of marshallable, non-cyclic types, this is impossible.
		contract.AssertNoErrorf(err, "History failed to serialize")
	}

	return nil
}

const artifactKey = "auto-aliasing"

// Make a default strategy aliasing, so it is safe for the inner strategy to make breaking
// changes.
//
// artifact is the byte sequence used to store history. The next artifact is returned by
// calling the returned callback. The returned callback must be called on the provider
// that utilized the returned strategy, after ComputeDefaults was called.
//
// artifact should be considered an opaque blob.
func Aliasing(artifact metadata.Provider, defaults DefaultStrategy) (DefaultStrategy, FinishAlias, error) {
	hist, err := getHistory(artifact)
	if err != nil {
		return DefaultStrategy{}, nil, err
	}
	remaps := &[]func(*b.ProviderInfo){}

	serialize := func(p *b.ProviderInfo) {
		for _, r := range *remaps {
			r(p)
		}
		err := md.Set(artifact, artifactKey, hist)
		// Set fails only when `hist` is not serializable. Because `hist` is
		// composed of marshallable, non-cyclic types, this is impossible.
		contract.AssertNoErrorf(err, "History failed to serialize")
	}

	return aliasing(hist, defaults, remaps), serialize, nil
}

func getHistory(artifact metadata.Provider) (aliasHistory, error) {
	hist, ok, err := md.Get[aliasHistory](artifact, artifactKey)
	if err != nil {
		return aliasHistory{}, err
	}
	if !ok {
		hist = aliasHistory{
			Resources:   map[string]*tokenHistory[tokens.Type]{},
			DataSources: map[string]*tokenHistory[tokens.ModuleMember]{},
		}
	}
	return hist, nil
}

func aliasing(hist aliasHistory, defaults DefaultStrategy, remaps *[]func(*b.ProviderInfo)) DefaultStrategy {
	return DefaultStrategy{
		Resource:   aliasResources(hist.Resources, defaults.Resource, remaps),
		DataSource: aliasDataSources(hist.DataSources, defaults.DataSource, remaps),
	}
}

func aliasResources(
	hist map[string]*tokenHistory[tokens.Type],
	strategy ResourceStrategy, remaps *[]func(*b.ProviderInfo),
) ResourceStrategy {
	return func(tfToken string) (*b.ResourceInfo, error) {
		res, err := strategy(tfToken)
		if err != nil {
			return nil, err
		}
		aliasResource(hist, res, tfToken, remaps)
		return res, nil
	}
}

func aliasResource(
	hist map[string]*tokenHistory[tokens.Type],
	computed *b.ResourceInfo,
	tfToken string,
	remaps *[]func(*b.ProviderInfo),
) {
	prev, hasPrev := hist[tfToken]
	if !hasPrev {
		// It's not in the history, so it must be new. Stick it in the history for
		// next time.
		*remaps = append(*remaps, func(*b.ProviderInfo) {
			hist[tfToken] = &tokenHistory[tokens.Type]{
				Current: computed.Tok,
			}
		})
	} else if prev.Current != computed.Tok {
		// It's in history, but something has changed. Update the history to reflect
		// the new reality, then add aliases.
		*remaps = append(*remaps, func(p *b.ProviderInfo) {
			aliasOrRenameResource(p, tfToken, prev)
		})

	}
}

func aliasOrRenameResource(p *b.ProviderInfo, tfToken string, hist *tokenHistory[tokens.Type]) {
	// re-fetch the resource, to make sure we have the right pointer.
	res, ok := p.Resources[tfToken]
	if !ok {
		// The resource to be remapped has been removed
		// from the resource map. There is nothing to
		// alias anymore.
		return
	}

	var alreadyPresent bool
	for _, a := range hist.Past {
		if a.Name == hist.Current {
			alreadyPresent = true
			break
		}
	}
	if !alreadyPresent {
		hist.Past = append(hist.Past, alias[tokens.Type]{
			Name:      hist.Current,
			InCodegen: true,
		})
	}
	for _, a := range hist.Past {
		legacy := a.Name
		if a.InCodegen {
			p.RenameResourceWithAlias(tfToken, legacy,
				res.Tok, legacy.Module().Name().String(),
				res.Tok.Module().Name().String(), res)
		} else {
			res.Aliases = append(res.Aliases,
				b.AliasInfo{Type: (*string)(&legacy)})
		}
	}

}

func aliasDataSources(
	hist map[string]*tokenHistory[tokens.ModuleMember],
	strategy DataSourceStrategy, remaps *[]func(*b.ProviderInfo),
) DataSourceStrategy {
	return func(tfToken string) (*b.DataSourceInfo, error) {
		computed, err := strategy(tfToken)
		if err != nil {
			return nil, err
		}
		aliasDataSource(hist, computed, tfToken, remaps)
		return computed, nil
	}
}

func aliasDataSource(
	hist map[string]*tokenHistory[tokens.ModuleMember],
	computed *b.DataSourceInfo,
	tfToken string,
	remaps *[]func(*b.ProviderInfo),
) {
	prev, hasPrev := hist[tfToken]
	if !hasPrev {
		// It's not in the history, so it must be new. Stick it in the history for
		// next time.
		hist[tfToken] = &tokenHistory[tokens.ModuleMember]{
			Current: computed.Tok,
		}
	} else if prev.Current != computed.Tok {
		aliasOrRenameDataSource(tfToken, remaps, prev)
	}
}

func aliasOrRenameDataSource(tfToken string, remaps *[]func(*b.ProviderInfo), prev *tokenHistory[tokens.ModuleMember]) {
	// It's in history, but something has changed. Update the history to reflect
	// the new reality, then add aliases.
	*remaps = append(*remaps, func(p *b.ProviderInfo) {
		// re-fetch the resource, to make sure we have the right pointer.
		computed, ok := p.DataSources[tfToken]
		if !ok {
			// The DataSource to alias has been removed. There
			// is nothing to alias anymore.
			return
		}
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
