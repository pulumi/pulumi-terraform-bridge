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

	"github.com/Masterminds/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	b "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	md "github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
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
//
// Deprecated: This item has been moved to
// [github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens.Make]
type MakeToken func(module, name string) (string, error)

// A strategy that assigns all tokens to the same module.
//
// For example:
//
//	rStrat, dStrat := TokensSingleModule("pkgName_", "index", finalize)
//
// The above example would transform "pkgName_foo" into "pkgName:index:Foo".
//
// Deprecated: This item has been moved to
// [github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens.SingleModule]
func TokensSingleModule(
	tfPackagePrefix, moduleName string, finalize MakeToken,
) DefaultStrategy {
	return TokensKnownModules(tfPackagePrefix, moduleName, nil, finalize)
}

func tokensKnownModules[T b.ResourceInfo | b.DataSourceInfo](
	prefix, defaultModule string, modules []string,
	new func(string, string) (*T, error),
	moduleTransform func(string) string,
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
		return new(moduleTransform(mod), upperCamelCase(strings.TrimPrefix(tk, mod)))
	}
}

// A strategy for assigning tokens to a hand generated set of modules.
//
// If defaultModule is "", then the returned strategies will error on not encountering a matching module.
//
// Deprecated: This item has been moved to
// [github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens.KnownModules]
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
			}, camelCase),
		DataSource: tokensKnownModules(tfPackagePrefix, defaultModule, modules,
			func(mod, tk string) (*b.DataSourceInfo, error) {
				tk, err := finalize(mod, "get"+tk)
				if err != nil {
					return nil, err
				}
				return &b.DataSourceInfo{Tok: tokens.ModuleMember(tk)}, nil
			}, camelCase),
	}
}

// A strategy for assigning tokens to a hand generated set of modules with an arbitrary
// mapping from TF modules to Pulumi modules.
//
// If defaultModule is "", then the returned strategies will error on not encountering a matching module.
//
// Deprecated: This item has been moved to
// [github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens.MappedModules]
func TokensMappedModules(
	tfPackagePrefix, defaultModule string, modules map[string]string, finalize MakeToken,
) DefaultStrategy {
	mods := make([]string, 0, len(modules))
	for k := range modules {
		mods = append(mods, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(mods)))

	transform := func(tf string) string {
		s, ok := modules[tf]
		if !ok && tf == defaultModule {
			// We pass through the default module as is, so it might not be in
			// `modules`. We need to catch that and return as is.
			return tf
		}
		assert := "Because any mod selected must be from mods, it is guaranteed to be in modules, got %#v"
		contract.Assertf(ok, assert, tf)
		return s
	}

	return DefaultStrategy{
		Resource: tokensKnownModules(tfPackagePrefix, defaultModule, mods,
			func(mod, tk string) (*b.ResourceInfo, error) {
				tk, err := finalize(mod, tk)
				if err != nil {
					return nil, err
				}
				return &b.ResourceInfo{Tok: tokens.Type(tk)}, nil
			}, transform),
		DataSource: tokensKnownModules(tfPackagePrefix, defaultModule, mods,
			func(mod, tk string) (*b.DataSourceInfo, error) {
				tk, err := finalize(mod, "get"+tk)
				if err != nil {
					return nil, err
				}
				return &b.DataSourceInfo{Tok: tokens.ModuleMember(tk)}, nil
			}, transform),
	}
}

// Convert a Terraform token to a Pulumi token with the standard mapping.
//
// The mapping is
//
//	(pkg, module, name) => pkg:module/lowerFirst(name):name
//
// Deprecated: This item has been moved to
// [github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens.MakeStandard]
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

// Deprecated: This item has been moved to
// [github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens.InferredModulesOpts]
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
// Deprecated: This item has been moved to
// [github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens.InferredModules]
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

	MajorVersion int                      `json:"majorVersion,omitempty"`
	Fields       map[string]*fieldHistory `json:"fields,omitempty"`
}

type alias[T ~string] struct {
	Name         T    `json:"name"`         // The previous token.
	InCodegen    bool `json:"inCodegen"`    // If the alias is a fully generated resource, or just a schema alias.
	MajorVersion int  `json:"majorVersion"` // The provider's major version when Name was introduced.
}

type aliasHistory struct {
	Resources   map[string]*tokenHistory[tokens.Type]         `json:"resources,omitempty"`
	DataSources map[string]*tokenHistory[tokens.ModuleMember] `json:"datasources,omitempty"`
}

type fieldHistory struct {
	MaxItemsOne *bool `json:"maxItemsOne,omitempty"`

	Fields map[string]*fieldHistory `json:"fields,omitempty"`
	Elem   *fieldHistory            `json:"elem,omitempty"`
}

// Deprecated: This item has been moved to
// [github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge.ProviderInfo.ApplyAutoAliases]
func AutoAliasing(providerInfo *b.ProviderInfo, artifact b.ProviderMetadata) error {
	hist, err := getHistory(artifact)
	if err != nil {
		return err
	}

	var currentVersion int
	// If version is missing, we assume the current version is the most recent major
	// version in mentioned in history.
	if providerInfo.Version != "" {
		v, err := semver.NewVersion(providerInfo.Version)
		if err != nil {
			return err
		}
		currentVersion = int(v.Major())
	} else {
		for _, r := range hist.Resources {
			for _, p := range r.Past {
				if p.MajorVersion > currentVersion {
					currentVersion = p.MajorVersion
				}
			}
		}
		for _, d := range hist.DataSources {
			for _, p := range d.Past {
				if p.MajorVersion > currentVersion {
					currentVersion = p.MajorVersion
				}
			}
		}
	}

	rMap := providerInfo.P.ResourcesMap()
	dMap := providerInfo.P.DataSourcesMap()

	// Applying resource aliases adds new resources to providerInfo.Resources. To keep
	// this process deterministic, we don't apply resource aliases until all resources
	// have been examined.
	applyResourceAliases := []func(){}

	for tfToken, computed := range providerInfo.Resources {
		r, _ := rMap.GetOk(tfToken)
		aliasResource(providerInfo, r, &applyResourceAliases, hist.Resources,
			computed, tfToken, currentVersion)
	}
	for _, f := range applyResourceAliases {
		f()
	}

	for tfToken, computed := range providerInfo.DataSources {
		ds, _ := dMap.GetOk(tfToken)
		aliasDataSource(providerInfo, ds, hist.DataSources, computed, tfToken, currentVersion)
	}

	if err := md.Set(artifact, aliasMetadataKey, hist); err != nil {
		// Set fails only when `hist` is not serializable. Because `hist` is
		// composed of marshallable, non-cyclic types, this is impossible.
		contract.AssertNoErrorf(err, "History failed to serialize")
	}

	return nil
}

const aliasMetadataKey = "auto-aliasing"

func getHistory(artifact b.ProviderMetadata) (aliasHistory, error) {
	hist, ok, err := md.Get[aliasHistory](artifact, aliasMetadataKey)
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

func aliasResource(
	p *b.ProviderInfo, res shim.Resource,
	applyResourceAliases *[]func(),
	hist map[string]*tokenHistory[tokens.Type], computed *b.ResourceInfo,
	tfToken string, version int,
) {
	prev, hasPrev := hist[tfToken]
	if !hasPrev {
		// It's not in the history, so it must be new. Stick it in the history for
		// next time.
		hist[tfToken] = &tokenHistory[tokens.Type]{
			Current: computed.Tok,
		}
	} else if prev.Current != computed.Tok {
		// It's in history, but something has changed. Update the history to reflect
		// the new reality, then add aliases.

		// We don't do this eagerly because aliasResource is called while
		// iterating over p.Resources which aliasOrRenameResource mutates.
		*applyResourceAliases = append(*applyResourceAliases,
			func() { aliasOrRenameResource(p, computed, tfToken, prev, version) })
	}

	// Apply Aliasing to MaxItemOne by traversing the field tree and applying the
	// stored value.
	//
	// Note: If the user explicitly sets a MaxItemOne value, that value is respected
	// and overwrites the current history.'

	if res == nil {
		return
	}

	// If we are behind the major version, reset the fields and the major version.
	if hist[tfToken].MajorVersion < version {
		hist[tfToken].MajorVersion = version
		hist[tfToken].Fields = nil
	}

	applyResourceMaxItemsOneAliasing(res, &hist[tfToken].Fields, &computed.Fields)
}

// applyResourceMaxItemsOneAliasing traverses a shim.Resource, applying walk to each field in the resource.
func applyResourceMaxItemsOneAliasing(
	r shim.Resource, hist *map[string]*fieldHistory, info *map[string]*b.SchemaInfo,
) (bool, bool) {
	if r == nil {
		return hist != nil, info != nil
	}
	m := r.Schema()
	if m == nil {
		return hist != nil, info != nil
	}

	var rHasH, rHasI bool

	m.Range(func(k string, v shim.Schema) bool {
		h, hasH := getNonNil(hist, k)
		i, hasI := getNonNil(info, k)
		fieldHasHist, fieldHasInfo := applyMaxItemsOneAliasing(v, h, i)

		hasH = hasH || fieldHasHist
		hasI = hasI || fieldHasInfo

		if !hasH {
			delete(*hist, k)
		}
		if !hasI {
			delete(*info, k)
		}

		rHasH = rHasH || hasH
		rHasI = rHasI || hasI

		return true
	})

	return rHasH, rHasI
}

// When walking the schema tree for a resource, we create mirroring trees in
// *fieldHistory and *b.SchemaInfo. To avoid polluting either tree (and
// interfering with other actions such as SetAutonaming), we clean up the paths
// that we created but did not store any information into.
//
// For example, consider the schema for a field of type `Object{ Key1:
// List[String] }`.  The schema tree for this field looks like this:
//
//	Object:
//	  Fields:
//	    Key1:
//	      List:
//	        Elem:
//	          String
//
// When we walk the tree, we create an almost identical history tree:
//
//	Object:
//	  Fields:
//	    Key1:
//	      List:
//	        MaxItemsOne: false
//	        Elem:
//	          String
//
// We stored the additional piece of information `MaxItemsOne: false`. We need to
// keep enough of the tree to maintain that information, but no more. We can
// discard the unnecessary `Elem: String`.
//
// This keeps the tree as clean as possible for other processes which expect a
// `nil` element when making changes. Since other processes (like SetAutonaming)
// act on edge nodes (like our String), this allows us to inter-operate with them
// without interference.
//
// applyMaxItemsOneAliasing traverses a generic shim.Schema recursively, applying fieldHistory to
// SchemaInfo and vise versa as necessary to avoid breaking changes in the
// resulting sdk.
func applyMaxItemsOneAliasing(schema shim.Schema, h *fieldHistory, info *b.SchemaInfo) (hasH bool, hasI bool) {
	//revive:disable-next-line:empty-block
	if schema == nil || (schema.Type() != shim.TypeList && schema.Type() != shim.TypeSet) {
		// MaxItemsOne does not apply, so do nothing
	} else if info.MaxItemsOne != nil {
		// The user has overwritten the value, so we will just record that.
		h.MaxItemsOne = info.MaxItemsOne
		hasH = true
	} else if h.MaxItemsOne != nil {
		// If we have a previous value in the history, we keep it as is.
		info.MaxItemsOne = h.MaxItemsOne
		hasI = true
	} else {
		// There is no history for this value, so we bake it into the
		// alias history.
		h.MaxItemsOne = b.BoolRef(b.IsMaxItemsOne(schema, info))
		hasH = true
	}

	// Ensure that the h.Elem and info.Elem fields are non-nil so they can be
	// safely recursed on.
	//
	// If the .Elem existed before this function, we mark it as unsafe to cleanup.
	var hasElemH, hasElemI bool
	populateElem := func() {
		if h.Elem == nil {
			h.Elem = &fieldHistory{}
		} else {
			hasElemH = true
		}
		if info.Elem == nil {
			info.Elem = &b.SchemaInfo{}
		} else {
			hasElemI = true
		}
	}
	// Cleanup after we have walked a .Elem value.
	//
	// If the .Elem field was created in populateElem and the field was not
	// changed, we then delete the field.
	cleanupElem := func(elemHist, elemInfo bool) {
		hasElemH = hasElemH || elemHist
		hasElemI = hasElemI || elemInfo
		if !hasElemH {
			h.Elem = nil
		}
		if !hasElemI {
			info.Elem = nil
		}
	}

	e := schema.Elem()
	switch e := e.(type) {
	case shim.Resource:
		populateElem()
		eHasH, eHasI := applyResourceMaxItemsOneAliasing(e, &h.Elem.Fields, &info.Elem.Fields)
		cleanupElem(eHasH, eHasI)
	case shim.Schema:
		populateElem()
		eHasH, eHasI := applyMaxItemsOneAliasing(e, h.Elem, info.Elem)
		cleanupElem(eHasH, eHasI)
	}

	return hasH || hasElemH, hasI || hasElemI
}

func getNonNil[K comparable, V any](m *map[K]*V, key K) (_ *V, alreadyThere bool) {
	contract.Assertf(m != nil, "Cannot restore map if ptr is nil")
	if *m == nil {
		*m = map[K]*V{}
	}
	v := (*m)[key]

	if v == nil {
		var new V
		v = &new
		(*m)[key] = v
	} else {
		alreadyThere = true
	}
	return v, alreadyThere
}

func aliasOrRenameResource(
	p *b.ProviderInfo,
	res *b.ResourceInfo, tfToken string,
	hist *tokenHistory[tokens.Type], currentVersion int,
) {
	var alreadyPresent bool
	for _, a := range hist.Past {
		if a.Name == hist.Current {
			alreadyPresent = true
			break
		}
	}
	if !alreadyPresent {
		hist.Past = append(hist.Past, alias[tokens.Type]{
			Name:         hist.Current,
			InCodegen:    true,
			MajorVersion: currentVersion,
		})
	}
	for _, a := range hist.Past {
		legacy := a.Name
		// Only respect hard aliases introduced in the same major version
		if a.InCodegen && a.MajorVersion == currentVersion {
			p.RenameResourceWithAlias(tfToken, legacy,
				res.Tok, legacy.Module().Name().String(),
				res.Tok.Module().Name().String(), res)
		} else {
			res.Aliases = append(res.Aliases,
				b.AliasInfo{Type: (*string)(&legacy)})
		}
	}
}

func aliasDataSource(
	p *b.ProviderInfo,
	ds shim.Resource,
	hist map[string]*tokenHistory[tokens.ModuleMember],
	computed *b.DataSourceInfo,
	tfToken string,
	version int,
) {
	prev, hasPrev := hist[tfToken]
	if !hasPrev {
		// It's not in the history, so it must be new. Stick it in the history for
		// next time.
		hist[tfToken] = &tokenHistory[tokens.ModuleMember]{
			Current:      computed.Tok,
			MajorVersion: version,
		}
	} else if prev.Current != computed.Tok {
		aliasOrRenameDataSource(p, tfToken, prev, version)
	}

	if ds == nil {
		return
	}

	// If we are behind the major version, reset the fields and the major version.
	if hist[tfToken].MajorVersion < version {
		hist[tfToken].MajorVersion = version
		hist[tfToken].Fields = nil
	}

	applyResourceMaxItemsOneAliasing(ds, &hist[tfToken].Fields, &computed.Fields)
}

func aliasOrRenameDataSource(
	p *b.ProviderInfo, tfToken string,
	prev *tokenHistory[tokens.ModuleMember],
	currentVersion int,
) {
	// re-fetch the resource, to make sure we have the right pointer.
	computed, ok := p.DataSources[tfToken]
	if !ok {
		// The DataSource to alias has been removed. There
		// is nothing to alias anymore.
		return
	}
	alias := alias[tokens.ModuleMember]{
		Name:         prev.Current,
		MajorVersion: currentVersion,
	}
	prev.Past = append(prev.Past, alias)
	for _, a := range prev.Past {
		if a.MajorVersion != currentVersion {
			continue
		}
		legacy := a.Name
		p.RenameDataSource(tfToken, legacy,
			computed.Tok, legacy.Module().Name().String(),
			computed.Tok.Module().Name().String(), computed)
	}
}
