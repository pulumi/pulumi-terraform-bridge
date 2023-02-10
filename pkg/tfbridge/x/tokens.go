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

// NOTE: Experimental; We are still iterating on the design of this type, and it is
// subject to change without warning.
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
//
// NOTE: Experimental; We are still iterating on the design of this type, and it is
// subject to change without warning.
func TokensInferredModules(info *ProviderInfo, finalize MakeToken, opts *InferredModulesOpts) (DefaultStrategy, error) {
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
		Resource: tokenFromMap(tokenMap, finalize, func(tk string) *ResourceInfo {
			return &ResourceInfo{Tok: tokens.Type(tk)}
		}),
		DataSource: tokenFromMap(tokenMap, finalize, func(tk string) *DataSourceInfo {
			return &DataSourceInfo{Tok: tokens.ModuleMember(tk)}
		}),
	}, nil
}

func (opts *InferredModulesOpts) ensurePrefix(info *ProviderInfo) error {
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

func (n *node) len() int {
	if n == nil {
		return 0
	}
	i := 0
	if n.tfToken != "" {
		i += 1
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
// great-grandparet, ect. parent panics when no node is available.
//
// dfs will pick up nodes inserted up the hierarchy during traversal, but only if they
// were inserted with unique names.
func (n *node) dfs(iter func(parent func(int) *node, node *node)) {
	parent_stack := []*node{n}
	for _, c := range n.children {
		c.dfs_inner(&parent_stack, iter)
	}
}

func (n *node) dfs_inner(parent_stack *[]*node, iter func(parent func(int) *node, node *node)) {
	// Pop this node onto the parent stack so children can access it
	*parent_stack = append(*parent_stack, n)
	// Iterate over children by key, making sure that newly added keys are iterated over
	seen := map[string]bool{}
	for done := false; !done; {
		done = true
		for k, v := range n.children {
			if seen[k] {
				continue
			}
			seen[k] = true
			done = false

			v.dfs_inner(parent_stack, iter)
		}
	}

	// Pop the node off afterwards
	*parent_stack = (*parent_stack)[:len(*parent_stack)-1]

	iter(func(i int) *node { return (*parent_stack)[len(*parent_stack)-1-i] }, n)
}

func (opts *InferredModulesOpts) computeTokens(info *ProviderInfo) map[string]tokenInfo {
	contract.Assertf(opts.TfPkgPrefix != "", "TF package prefix not provided or computed")
	tree := &node{segment: opts.TfPkgPrefix}

	// build segment tree
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

	// collapse segment tree
	tree.dfs(func(parent func(int) *node, n *node) {
		if parent(0) == tree {
			// Inject each path as a node
			if n.len() < opts.MinimumModuleSize {
				// Node segment is not big enough for its own module, so inject each token
				// into the main module
				for _, child := range n.children {
					contract.Assertf(child.tfToken != "", "child: %#v", child)
					output[child.tfToken] = tokenInfo{
						mod:  opts.MainModule,
						name: n.segment + "_" + child.segment,
					}
				}
				if n.tfToken != "" {
					contract.Assertf(n.tfToken != "", "leaf: %#v", n)
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
					contract.Assertf(n.tfToken != "", "leaf: %#v", n)
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
					segment := n.segment + "_" + child.segment
					parent(0).children[segment] = &node{
						segment: segment,
						tfToken: child.tfToken,
					}
				}
				if n.tfToken == "" {
					// If this is only a leaf node, we can cut it
					delete(parent(0).children, n.segment)
				} else {
					// This also holds an actual item, so just clear the children, since
					// they have been moved into the grandparent.
					n.children = nil
				}
			} else {
				// Inject the node into the grand-parent, putting it next to the parent
				// and remove it as a child of parent.
				delete(parent(0).children, n.segment)
				n.segment = parent(0).segment + "_" + n.segment
				parent(1).children[n.segment] = n
			}
		}
	})

	return output
}

func mapProviderItems(info *ProviderInfo, each func(string, shim.Resource) bool) {
	ignored := info.ignoredTokens()
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

func tokenFromMap[T ResourceInfo | DataSourceInfo](tokenMap map[string]tokenInfo, finalize MakeToken, new func(tk string) *T) Strategy[T] {
	return func(tfToken string) (*T, error) {
		info, ok := tokenMap[tfToken]
		if !ok {
			existing := []string{}
			for k := range tokenMap {
				existing = append(existing, k)
			}
			return nil, fmt.Errorf("TF token '%s' not present when prefix computed, found %#v", tfToken, existing)
		}
		tk, err := finalize(info.mod, info.name)
		if err != nil {
			return nil, err
		}
		return new(tk), nil
	}
}
