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

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	md "github.com/pulumi/pulumi-terraform-bridge/v3/internal/metadata"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/metadata"
	b "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const (
	defaultMinimumModuleSize    = 5
	defaultMinimumSubmoduleSize = 5
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

type InferredModulesOpts struct {
	// The TF prefix of the package.
	TfPkgPrefix string
	// The name of the main module. Defaults to "index".
	MainModule string
	// The minimum number of shared items for a prefix before it becomes a module.
	//
	// < 0 -> don't bin into modules.
	// = 0 -> apply the default value.
	// > 0 -> set the value.
	MinimumModuleSize int
	// The number of items in a longer prefix needed to break out into it's own prefix.
	//
	// For example, with the tokens `pkg_mod_sub1_a`, `pkg_mod_sub2_b`, `pkg_mod_sub2_c`,
	// `pkg_mod_sub3_d`:
	//
	// MinimumSubmoduleSize = 3 will result in:
	//
	//	pkg:mod:Sub1A, pkg:mod:Sub2B, pkg:mod:Sub2C, pkg:mod:Sub3D
	//
	// MinimumSubmoduleSize = 2 will result in:
	//
	//	pkg:mod:Sub1A, pkg:modSub2:B, pkg:modSub2C, pkg:mod:Sub3D
	//
	// < 0 -> don't bin into submodules. Only the most common prefix will be used.
	// = 0 -> apply the default value.
	// > 0 -> set the value.
	MimimumSubmoduleSize int
}

// A strategy to infer module placement from global analysis of all items (Resources & DataSources).
func TokensInferredModules(
	info *b.ProviderInfo, finalize MakeToken, opts *InferredModulesOpts,
) (DefaultStrategy, error) {
	if opts == nil {
		opts = &InferredModulesOpts{}
	}
	err := opts.ensurePrefix(info)
	if err != nil {
		return DefaultStrategy{}, fmt.Errorf("inferring pkg prefix: %w", err)
	}
	contract.Assertf(opts.MinimumModuleSize >= 0, "Cannot have a minimum modules size less then zero")
	if opts.MinimumModuleSize == 0 {
		opts.MinimumModuleSize = defaultMinimumModuleSize
	}
	if opts.MimimumSubmoduleSize == 0 {
		opts.MimimumSubmoduleSize = defaultMinimumSubmoduleSize
	}
	if opts.MainModule == "" {
		opts.MainModule = "index"
	}

	tokenMap := opts.computeTokens(info)

	return DefaultStrategy{
		Resource: tokenFromMap(tokenMap, finalize, func(tk string) *b.ResourceInfo {
			return &b.ResourceInfo{Tok: tokens.Type(tk)}
		}),
		DataSource: tokenFromMap(tokenMap, finalize, func(tk string) *b.DataSourceInfo {
			return &b.DataSourceInfo{Tok: tokens.ModuleMember(tk)}
		}),
	}, nil
}

func (opts *InferredModulesOpts) ensurePrefix(info *b.ProviderInfo) error {
	prefix := opts.TfPkgPrefix
	var noCommonality bool
	findPrefix := func(key string, _ shim.Resource) bool {
		if noCommonality {
			return false
		}
		if prefix == "" {
			prefix = key
			return true
		}

		prefix = sharedPrefix(key, prefix)
		if prefix == "" {
			noCommonality = true
		}

		return true
	}
	mapProviderItems(info, findPrefix)
	if noCommonality {
		return fmt.Errorf("no common prefix detected")
	}
	if prefix == "" {
		return fmt.Errorf("no items found")
	}
	opts.TfPkgPrefix = prefix
	return nil
}

type node struct {
	segment  string
	children map[string]*node
	// tfToken is only non-empty if the node represents a literal tf token
	tfToken string
}

func (n *node) child(segment string) *node {
	if n.children == nil {
		n.children = map[string]*node{}
	}
	v, ok := n.children[segment]
	if ok {
		return v
	}
	child := &node{segment: segment}
	n.children[segment] = child
	return child
}

func (n *node) insert(child *node) {
	if n.children == nil {
		n.children = map[string]*node{}
	}
	_, ok := n.children[child.segment]
	contract.Assertf(!ok, "duplicate segment in child: %q", child.segment)
	n.children[child.segment] = child
}

func (n *node) len() int {
	i := 0
	if n.tfToken != "" {
		i++
	}
	for _, child := range n.children {
		i += child.len()
	}
	return i
}

// A depth first search of child nodes.
//
// parent is a function that returns parent nodes, with the immediate parent starting at 0
// and each increment increasing the indirection. 1 yields the grandparent, 2 the
// great-grandparent, etc. parent panics when no node is available.
//
// dfs will pick up nodes inserted up the hierarchy during traversal, but only if they
// were inserted with unique names.
func (n *node) dfs(iter func(parent func(int) *node, node *node)) {
	parentStack := []*node{n}
	fullIter(n.children, func(_ string, child *node) {
		child.dfsInner(&parentStack, iter)
	})
}

// Iterate over a map in any order, ensuring that all keys in the map are iterated over,
// even if they were added during the iteration.
//
// There is no guarantee of the order of the iteration.
func fullIter[K comparable, V any](m map[K]V, f func(K, V)) {
	seen := map[K]bool{}
	for done := false; !done; {
		done = true
		for k, v := range m {
			if seen[k] {
				continue
			}
			seen[k] = true
			done = false

			f(k, v)
		}
	}
}

func (n *node) dfsInner(parentStack *[]*node, iter func(parent func(int) *node, node *node)) {
	// Pop this node onto the parent stack so children can access it
	*parentStack = append(*parentStack, n)
	// Iterate over children by key, making sure that newly added keys are iterated over
	fullIter(n.children, func(k string, v *node) {
		v.dfsInner(parentStack, iter)
	})

	// Pop the node off afterwards
	*parentStack = (*parentStack)[:len(*parentStack)-1]

	iter(func(i int) *node { return (*parentStack)[len(*parentStack)-1-i] }, n)
}

// Precompute the mapping from tf tokens to pulumi modules.
//
// The resulting map is complete for all TF resources and datasources in info.P.
func (opts *InferredModulesOpts) computeTokens(info *b.ProviderInfo) map[string]tokenInfo {
	contract.Assertf(opts.TfPkgPrefix != "", "TF package prefix not provided or computed")
	tree := &node{segment: opts.TfPkgPrefix}

	// Build segment tree:
	//
	// Expand each item (resource | datasource) into it's segments (divided by "_"), then
	// insert each token into the tree structure. The tree is defined by segments, where
	// each node represents a segment and each path a token.
	mapProviderItems(info, func(s string, _ shim.Resource) bool {
		segments := strings.Split(strings.TrimPrefix(s, opts.TfPkgPrefix), "_")
		contract.Assertf(len(segments) > 0, "No segments found")
		contract.Assertf(segments[0] != "", "Empty segment from splitting %q with prefix %q", s, opts.TfPkgPrefix)
		node := tree
		for _, segment := range segments {
			node = node.child(segment)
		}
		node.tfToken = s
		return true
	})

	contract.Assertf(tree.tfToken == "", "We don't expect a resource called '%s'", opts.TfPkgPrefix)
	output := map[string]tokenInfo{}
	// Collapse the segment tree via a depth first traversal.
	tree.dfs(func(parent func(int) *node, n *node) {
		if parent(0) == tree {
			// Inject each path as a node
			if n.len() < opts.MinimumModuleSize {
				// Node segment is not big enough for its own module, so inject each token
				// into the main module
				for _, child := range n.children {
					output[child.tfToken] = tokenInfo{
						mod:  opts.MainModule,
						name: n.segment + "_" + child.segment,
					}
				}
				if n.tfToken != "" {
					output[n.tfToken] = tokenInfo{
						mod:  opts.MainModule,
						name: n.segment,
					}
				}
			} else {
				// Node segment will form its own modules, so inject each token as a
				// module member of `n.segment`.
				for _, child := range n.children {
					contract.Assertf(child.tfToken != "", "child of %q: %#v", n.segment, child)
					output[child.tfToken] = tokenInfo{
						mod:  n.segment,
						name: child.segment,
					}
				}
				// If the node is both a module and a item, put the item in the module
				if n.tfToken != "" {
					output[n.tfToken] = tokenInfo{
						mod:  n.segment,
						name: n.segment,
					}
				}
			}
		} else {
			// flatten the tree by injecting children into the parent node.
			if n.len() < opts.MimimumSubmoduleSize {
				for _, child := range n.children {
					contract.Assertf(child.children == nil, "module already flattened")
					parent(0).insert(&node{
						segment: n.segment + "_" + child.segment,
						tfToken: child.tfToken,
					})
				}
				// Clear the children, since they have been moved to the parent
				n.children = nil
				if n.tfToken == "" {
					// If this is only a leaf node, we can cut it
					delete(parent(0).children, n.segment)
				}
			} else {
				// Inject the node into the grand-parent, putting it next to the parent
				// and remove it as a child of parent.
				delete(parent(0).children, n.segment)
				parent(1).insert(&node{
					segment:  parent(0).segment + "_" + n.segment,
					tfToken:  n.tfToken,
					children: n.children,
				})
			}
		}
	})

	return output
}

func mapProviderItems(info *b.ProviderInfo, each func(string, shim.Resource) bool) {
	ignored := ignoredTokens(info)
	info.P.ResourcesMap().Range(func(key string, value shim.Resource) bool {
		if ignored[key] {
			return true
		}
		return each(key, value)
	})
	info.P.DataSourcesMap().Range(func(key string, value shim.Resource) bool {
		if ignored[key] {
			return true
		}
		return each(key, value)
	})
}

func sharedPrefix(s1, s2 string) string {
	// Shorten the longer string so it is only as long as the shortest string
	if len(s1) < len(s2) {
		s2 = s2[:len(s1)]
	} else if len(s1) > len(s2) {
		s1 = s1[:len(s2)]
	}

	for i := range s1 {
		if s1[i] != s2[i] {
			return s1[:i]
		}
	}
	return s1
}

type tokenInfo struct{ mod, name string }

func tokenFromMap[T b.ResourceInfo | b.DataSourceInfo](
	tokenMap map[string]tokenInfo, finalize MakeToken, new func(tk string) *T,
) Strategy[T] {
	return func(tfToken string) (*T, error) {
		info, ok := tokenMap[tfToken]
		if !ok {
			existing := []string{}
			for k := range tokenMap {
				existing = append(existing, k)
			}
			return nil, fmt.Errorf("TF token '%s' not present when prefix computed, found %#v", tfToken, existing)
		}
		tk, err := finalize(camelCase(info.mod), upperCamelCase(info.name))
		if err != nil {
			return nil, err
		}
		return new(tk), nil
	}
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

// Make a default strategy aliasing, so it is safe for the inner strategy to make breaking
// changes.
//
// artifact is the byte sequence used to store history. The next artifact is returned by
// calling the returned callback. The returned callback must be called on the provider
// that utilized the returned strategy, after ComputeDefaults was called.
//
// artifact should be considered an opaque blob.
func Aliasing(artifact metadata.Provider, defaults DefaultStrategy) (DefaultStrategy, FinishAlias, error) {
	const artifactKey = "auto-aliasing"
	hist, ok, err := md.Get[aliasHistory](artifact, artifactKey)
	if !ok {
		hist = aliasHistory{
			Resources:   map[string]*tokenHistory[tokens.Type]{},
			DataSources: map[string]*tokenHistory[tokens.ModuleMember]{},
		}
	}
	if err != nil {
		return DefaultStrategy{}, nil, err
	}
	remaps := &[]func(*b.ProviderInfo){}

	serialize := func(p *b.ProviderInfo) {
		for _, r := range *remaps {
			r(p)
		}
		err := md.Set(artifact, artifactKey, hist)
		contract.AssertNoError(err)
	}

	return aliasing(hist, defaults, remaps), serialize, nil
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
		computed, err := strategy(tfToken)
		if err != nil {
			return nil, err
		}

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
						computed.Aliases = append(computed.Aliases, b.AliasInfo{Type: (*string)(&legacy)})
					}
				}
			})

		}

		return computed, nil
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
			*remaps = append(*remaps, func(p *b.ProviderInfo) {
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
