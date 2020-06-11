// Copyright 2016-2018, Pulumi Corporation.
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

package tfgen

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/pkg/errors"
	dotnetgen "github.com/pulumi/pulumi/pkg/v2/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v2/codegen/go"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2"
	nodejsgen "github.com/pulumi/pulumi/pkg/v2/codegen/nodejs"
	pygen "github.com/pulumi/pulumi/pkg/v2/codegen/python"
	pschema "github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tools"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/tf2pulumi/il"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
)

const (
	tfgen         = "the Pulumi Terraform Bridge (tfgen) Tool"
	defaultOutDir = "sdk/"
	maxWidth      = 120 // the ideal maximum width of the generated file.
)

type generator struct {
	pkg          string                // the Pulum package name (e.g. `gcp`)
	version      string                // the package version.
	language     language              // the language runtime to generate.
	info         tfbridge.ProviderInfo // the provider info for customizing code generation
	outDir       string                // the directory in which to generate the code.
	providerShim *inmemoryProvider     // a provider shim to hold the provider schema during example conversion.
	pluginHost   plugin.Host           // the plugin host for tf2pulumi.
	packageCache *hcl2.PackageCache    // the package cache for tf2pulumi.
	infoSource   il.ProviderInfoSource // the provider info source for tf2pulumi.
}

type language string

const (
	golang       language = "go"
	nodeJS       language = "nodejs"
	python       language = "python"
	csharp       language = "dotnet"
	pulumiSchema language = "schema"
)

func (l language) shouldConvertExamples() bool {
	switch l {
	case golang, nodeJS, python, csharp, pulumiSchema:
		return true
	}
	return false
}

func (l language) emitSDK(pkg *pschema.Package, info tfbridge.ProviderInfo, outDir string) (map[string][]byte, error) {
	var extraFiles map[string][]byte
	var err error

	switch l {
	case golang:
		return gogen.GeneratePackage(tfgen, pkg)
	case nodeJS:
		if psi := info.JavaScript; psi != nil && psi.Overlay != nil {
			extraFiles, err = getOverlayFiles(psi.Overlay, ".ts", outDir)
			if err != nil {
				return nil, err
			}
		}
		return nodejsgen.GeneratePackage(tfgen, pkg, extraFiles)
	case python:
		if psi := info.Python; psi != nil && psi.Overlay != nil {
			extraFiles, err = getOverlayFiles(psi.Overlay, ".py", outDir)
			if err != nil {
				return nil, err
			}
		}
		return pygen.GeneratePackage(tfgen, pkg, extraFiles)
	case csharp:
		if psi := info.CSharp; psi != nil && psi.Overlay != nil {
			extraFiles, err = getOverlayFiles(psi.Overlay, ".cs", outDir)
			if err != nil {
				return nil, err
			}
		}
		return dotnetgen.GeneratePackage(tfgen, pkg, extraFiles)
	default:
		return nil, errors.Errorf("%v does not support SDK generation", l)
	}
}

var allLanguages = []language{golang, nodeJS, python, csharp}

// pkg is a directory containing one or more modules.
type pkg struct {
	name     string        // the package name.
	version  string        // the package version.
	language language      // the package's language.
	path     string        // the path to the package directory on disk.
	modules  moduleMap     // the modules inside of this package.
	provider *resourceType // the provider type for this package.
}

func newPkg(name string, version string, language language, path string) *pkg {
	return &pkg{
		name:     name,
		version:  version,
		language: language,
		path:     path,
		modules:  make(moduleMap),
	}
}

func extractModuleName(name string) string {
	// TODO[pulumi/pulumi-terraform#107]: for now, while we migrate to the new structure, just ignore sub-modules.
	//     After we are sure our customers have upgraded to the new bits, we can remove this logic.  In fact, in the
	//     end we may actually want to support this structure, but probably in a different way, and not right now.
	sepix := strings.IndexRune(name, '/')
	if sepix != -1 {
		name = name[:sepix] // temporarily whack everything after the /.
	}
	if name == "index" {
		name = "" // temporarily change index to "".
	}
	return name
}

// addModule registers a new module in the given package.  If one already exists under the name, we will merge
// the entry with the existing module (where merging simply appends the members).
func (p *pkg) addModule(m *module) {
	p.modules[m.name] = p.modules[m.name].merge(m)
}

// addModuleMap registers an array of modules, using the same logic that addModule uses.
func (p *pkg) addModuleMap(m moduleMap) {
	for _, k := range m.keys() {
		p.addModule(m[k])
	}
}

type moduleMap map[string]*module

func (m moduleMap) keys() []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func (m moduleMap) values() []*module {
	var vs []*module
	for _, k := range m.keys() {
		vs = append(vs, m[k])
	}
	return vs
}

func (m moduleMap) ensureModule(name string) *module {
	name = extractModuleName(name)
	if _, ok := m[name]; !ok {
		m[name] = newModule(name)
	}
	return m[name]
}

// module is a single module that was generated by this tool, containing members (types and functions).
type module struct {
	name    string         // the friendly module name.
	members []moduleMember // a list of exported members from this module (ordered in case of dependencies).
}

func newModule(name string) *module {
	return &module{name: name}
}

// configMod is the special name used for configuration modules.
const configMod = "config"

// config returns true if this is a configuration module.
func (m *module) config() bool {
	return m.name == configMod
}

// addMember appends a new member.  This maintains ordering in case the code is sensitive to declaration order.
func (m *module) addMember(member moduleMember) {
	name := member.Name()
	for _, existing := range m.members {
		contract.Assertf(existing.Name() != name, "unexpected duplicate module member %s", name)
	}
	m.members = append(m.members, member)
}

// merge merges two separate modules into one and returns the result.
func (m *module) merge(other *module) *module {
	if m == nil {
		return other
	} else if other == nil {
		return m
	}

	contract.Assertf(m.name == other.name,
		"expected module names %s and %s to match", m.name, other.name)
	contract.Assertf(m.config() == other.config(),
		"cannot combine config and non-config modules (%s %t; %s %t)",
		m.name, m.config(), other.name, other.config())
	return &module{
		name:    m.name,
		members: append(m.members, other.members...),
	}
}

// moduleMember is an exportable type.
type moduleMember interface {
	Name() string
	Doc() string
}

type typeKind int

const (
	kindInvalid = iota
	kindBool
	kindInt
	kindFloat
	kindString
	kindList
	kindMap
	kindSet
	kindObject
)

// Avoid an unused warning from varcheck.
var _ = kindInvalid

// propertyType represents a non-resource, non-datasource type. Property types may be simple
type propertyType struct {
	name       string
	doc        string
	kind       typeKind
	element    *propertyType
	properties []*variable

	typ        tokens.Type
	nestedType tokens.Type
	altTypes   []tokens.Type
	asset      *tfbridge.AssetTranslation
}

func makePropertyType(objectName string, sch *schema.Schema, info *tfbridge.SchemaInfo, out bool,
	entityDocs entityDocs) *propertyType {

	t := &propertyType{}

	var elemInfo *tfbridge.SchemaInfo
	if info != nil {
		t.typ = info.Type
		t.nestedType = info.NestedType
		t.altTypes = info.AltTypes
		t.asset = info.Asset
		elemInfo = info.Elem
	}

	if sch == nil {
		contract.Assert(info != nil)
		return t
	}

	switch sch.Type {
	case schema.TypeBool:
		t.kind = kindBool
	case schema.TypeInt:
		t.kind = kindInt
	case schema.TypeFloat:
		t.kind = kindFloat
	case schema.TypeString:
		t.kind = kindString
	case schema.TypeList:
		t.kind = kindList
	case schema.TypeMap:
		t.kind = kindMap
	case schema.TypeSet:
		t.kind = kindSet
	}

	// We should carry across any of the deprecation messages, to Pulumi, as per Terraform schema
	if sch.Deprecated != "" && elemInfo != nil {
		elemInfo.DeprecationMessage = sch.Deprecated
	}

	switch elem := sch.Elem.(type) {
	case *schema.Schema:
		t.element = makePropertyType(objectName, elem, elemInfo, out, entityDocs)
	case *schema.Resource:
		t.element = makeObjectPropertyType(objectName, elem, elemInfo, out, entityDocs)
	}

	switch t.kind {
	case kindList, kindSet:
		if tfbridge.IsMaxItemsOne(sch, info) {
			t = t.element
		}
	case kindMap:
		// If this map has a "resource" element type, just use the generated element type. This works around a bug in
		// TF that effectively forces this behavior.
		if t.element != nil && t.element.kind == kindObject {
			t = t.element
		}
	}

	return t
}

func makeObjectPropertyType(objectName string, res *schema.Resource, info *tfbridge.SchemaInfo, out bool,
	entityDocs entityDocs) *propertyType {

	t := &propertyType{
		kind: kindObject,
	}

	if info != nil {
		t.typ = info.Type
		t.nestedType = info.NestedType
		t.altTypes = info.AltTypes
		t.asset = info.Asset
	}

	var propertyInfos map[string]*tfbridge.SchemaInfo
	if info != nil {
		propertyInfos = info.Fields
	}

	for _, key := range stableSchemas(res.Schema) {
		propertySchema := res.Schema[key]

		var propertyInfo *tfbridge.SchemaInfo
		if propertyInfos != nil {
			propertyInfo = propertyInfos[key]
		}

		doc := getNestedDescriptionFromParsedDocs(entityDocs, objectName, key)
		if v := propertyVariable(key, propertySchema, propertyInfo, doc, "", "", out, entityDocs); v != nil {
			t.properties = append(t.properties, v)
		}
	}

	return t
}

func (t *propertyType) equals(other *propertyType) bool {
	if t == nil && other == nil {
		return true
	}
	if (t != nil) != (other != nil) {
		return false
	}
	if t.name != other.name {
		return false
	}
	if t.kind != other.kind {
		return false
	}
	if !t.element.equals(other.element) {
		return false
	}
	if len(t.properties) != len(other.properties) {
		return false
	}
	for i, p := range t.properties {
		o := other.properties[i]
		if p.name != o.name {
			return false
		}
		if p.optional() != o.optional() {
			return false
		}
		if !p.typ.equals(o.typ) {
			return false
		}
	}

	if t.typ != other.typ {
		return false
	}
	if t.nestedType != other.nestedType {
		return false
	}
	if t.asset != nil && (other.asset == nil || *t.asset != *other.asset) {
		return false
	} else if other.asset != nil {
		return false
	}
	if len(t.altTypes) != len(other.altTypes) {
		return false
	}
	for i, t := range t.altTypes {
		if t != other.altTypes[i] {
			return false
		}
	}

	return true
}

// variable is a schematized variable, property, argument, or return type.
type variable struct {
	name   string
	out    bool
	opt    bool
	config bool // config is true if this variable represents a Pulumi config value.
	doc    string
	rawdoc string
	docURL string

	schema *schema.Schema
	info   *tfbridge.SchemaInfo

	typ *propertyType
}

func (v *variable) Name() string { return v.name }
func (v *variable) Doc() string  { return v.doc }

func (v *variable) deprecationMessage() string {
	if v.schema != nil && v.schema.Deprecated != "" {
		return v.schema.Deprecated
	}

	if v.info != nil && v.info.DeprecationMessage != "" {
		return v.info.DeprecationMessage
	}

	return ""
}

// optional checks whether the given property is optional, either due to Terraform or an overlay.
func (v *variable) optional() bool {
	if v.opt {
		return true
	}

	//if we have an explicit marked as optional then let's return that
	if v.info != nil && v.info.MarkAsOptional != nil {
		return *v.info.MarkAsOptional
	}

	// If we're checking a property used in an output position, it isn't optional if it's computed.
	//
	// Note that config values with custom defaults are _not_ considered optional unless they are marked as such.
	customDefault := !v.config && v.info != nil && v.info.HasDefault()
	if v.out {
		return v.schema != nil && v.schema.Optional && !v.schema.Computed && !customDefault
	}
	return (v.schema != nil && v.schema.Optional || v.schema.Computed) || customDefault
}

// resourceType is a generated resource type that represents a Pulumi CustomResource definition.
type resourceType struct {
	name       string
	doc        string
	isProvider bool
	inprops    []*variable
	outprops   []*variable
	reqprops   map[string]bool
	argst      *propertyType // input properties.
	statet     *propertyType // output properties (all optional).
	schema     *schema.Resource
	info       *tfbridge.ResourceInfo
	docURL     string
	entityDocs entityDocs // parsed docs.
}

func (rt *resourceType) Name() string { return rt.name }
func (rt *resourceType) Doc() string  { return rt.doc }

// IsProvider is true if this resource is a ProviderResource.
func (rt *resourceType) IsProvider() bool { return rt.isProvider }

func newResourceType(name string, entityDocs entityDocs, schema *schema.Resource, info *tfbridge.ResourceInfo,
	isProvider bool) *resourceType {

	return &resourceType{
		name:       name,
		doc:        entityDocs.Description,
		isProvider: isProvider,
		schema:     schema,
		info:       info,
		reqprops:   make(map[string]bool),
		docURL:     entityDocs.URL,
		entityDocs: entityDocs,
	}
}

// resourceFunc is a generated resource function that is exposed to interact with Pulumi objects.
type resourceFunc struct {
	name       string
	doc        string
	args       []*variable
	rets       []*variable
	reqargs    map[string]bool
	argst      *propertyType
	retst      *propertyType
	schema     *schema.Resource
	info       *tfbridge.DataSourceInfo
	docURL     string
	entityDocs entityDocs
}

func (rf *resourceFunc) Name() string { return rf.name }
func (rf *resourceFunc) Doc() string  { return rf.doc }

// overlayFile is a file that should be added to a module "as-is" and then exported from its index.
type overlayFile struct {
	name string
	src  string
}

func (of *overlayFile) Name() string { return of.name }
func (of *overlayFile) Doc() string  { return "" }
func (of *overlayFile) Copy() bool   { return of.src != "" }

// newGenerator returns a code-generator for the given language runtime and package info.
func newGenerator(pkg, version string, lang language, info tfbridge.ProviderInfo, outDir string) (*generator, error) {
	// Ensure the language is valid.
	switch lang {
	case golang, nodeJS, python, csharp, pulumiSchema:
		// OK
	default:
		return nil, errors.Errorf("unrecognized language runtime: %s", lang)
	}

	// If outDir is empty, default to sdk/<language>/ in the pwd.
	if outDir == "" {
		p, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		if outDir == "" {
			outDir = filepath.Join(p, defaultOutDir, string(lang))
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ctx, err := plugin.NewContext(nil, nil, nil, nil, cwd, nil, nil)
	if err != nil {
		return nil, err
	}

	providerShim := &inmemoryProvider{
		name: pkg,
		info: info,
	}
	host := &inmemoryProviderHost{
		Host:               ctx.Host,
		ProviderInfoSource: il.NewCachingProviderInfoSource(il.PluginProviderInfoSource),
		provider:           providerShim,
	}

	return &generator{
		pkg:          pkg,
		version:      version,
		language:     lang,
		info:         info,
		outDir:       outDir,
		providerShim: providerShim,
		pluginHost: &cachingProviderHost{
			Host:  host,
			cache: map[string]plugin.Provider{},
		},
		packageCache: hcl2.NewPackageCache(),
		infoSource:   host,
	}, nil
}

func (g *generator) provider() *schema.Provider {
	return g.info.P
}

// Generate creates Pulumi packages from the information it was initialized with.
func (g *generator) Generate() error {
	// First gather up the entire package contents.  This structure is complete and sufficient to hand off
	// to the language-specific generators to create the full output.
	pack, err := g.gatherPackage()
	if err != nil {
		return errors.Wrapf(err, "failed to gather package metadata")
	}

	// Ensure the target exists.
	if err = g.preparePackage(pack); err != nil {
		return errors.Wrapf(err, "failed to prepare package")
	}

	// Convert the package to a Pulumi schema.
	pulumiPackageSpec, err := genPulumiSchema(pack, g.pkg, g.version, g.info)
	if err != nil {
		return errors.Wrapf(err, "failed to create Pulumi schema")
	}

	// Serialize the schema and attach it to the provider shim.
	g.providerShim.schema, err = json.Marshal(pulumiPackageSpec)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal intermediate schema")
	}

	// Convert examples.
	pulumiPackageSpec = g.convertExamplesInSchema(pulumiPackageSpec)

	// Go ahead and let the language generator do its thing. If we're emitting the schema, just go ahead and serialize
	// it out.
	var files map[string][]byte
	if g.language == pulumiSchema {
		// Omit the version so that the spec is stable if the version is e.g. derived from the current Git commit hash.
		pulumiPackageSpec.Version = ""

		bytes, err := json.MarshalIndent(pulumiPackageSpec, "", "    ")
		if err != nil {
			return errors.Wrapf(err, "failed to marshal schema")
		}
		files = map[string][]byte{"schema.json": bytes}
	} else {
		pulumiPackage, err := pschema.ImportSpec(pulumiPackageSpec, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to import Pulumi schema")
		}
		if files, err = g.language.emitSDK(pulumiPackage, g.info, g.outDir); err != nil {
			return errors.Wrapf(err, "failed to generate package")
		}
	}

	// Write the result to disk. Do not overwrite the root-level README.md if any exists.
	for f, contents := range files {
		if f == "README.md" {
			if _, err := os.Stat(filepath.Join(g.outDir, f)); err == nil {
				continue
			}
		}
		if err := emitFile(g.outDir, f, contents); err != nil {
			return errors.Wrapf(err, "emitting file %v", f)
		}
	}

	// Emit the Pulumi project information.
	if err = g.emitProjectMetadata(pack); err != nil {
		return errors.Wrapf(err, "failed to create project file")
	}

	// Print out some documentation stats as a summary afterwards.
	printDocStats(false, false)

	// Close the plugin host.
	g.pluginHost.Close()

	return nil
}

// gatherPackage creates a package plus module structure for the entire set of members of this package.
func (g *generator) gatherPackage() (*pkg, error) {
	// First, gather up the entire package/module structure.  This includes gathering config entries, resources,
	// data sources, and any supporting type information, and placing them into modules.
	pack := newPkg(g.pkg, g.version, g.language, g.outDir)

	// Place all configuration variables into a single config module.
	if cfg := g.gatherConfig(); cfg != nil {
		pack.addModule(cfg)
	}

	// Gather the provider type for this package.
	provider, err := g.gatherProvider()
	if err != nil {
		return nil, errors.Wrapf(err, "problem gathering the provider type")
	}
	pack.provider = provider

	// Gather up all resource modules and merge them into the current set.
	resmods, err := g.gatherResources()
	if err != nil {
		return nil, errors.Wrapf(err, "problem gathering resources")
	} else if resmods != nil {
		pack.addModuleMap(resmods)
	}

	// Gather up all data sources into their respective modules and merge them in.
	dsmods, err := g.gatherDataSources()
	if err != nil {
		return nil, errors.Wrapf(err, "problem gathering data sources")
	} else if dsmods != nil {
		pack.addModuleMap(dsmods)
	}

	// Now go ahead and merge in any overlays into the modules if there are any.
	olaymods, err := g.gatherOverlays()
	if err != nil {
		return nil, errors.Wrapf(err, "problem gathering overlays")
	} else if olaymods != nil {
		pack.addModuleMap(olaymods)
	}

	return pack, nil
}

// gatherConfig returns the configuration module for this package.
func (g *generator) gatherConfig() *module {
	// If there's no config, skip creating the module.
	cfg := g.provider().Schema
	if len(cfg) == 0 {
		return nil
	}
	config := newModule(configMod)

	// Sort the config variables to ensure they are emitted in a deterministic order.
	custom := g.info.Config
	var cfgkeys []string
	for key := range cfg {
		cfgkeys = append(cfgkeys, key)
	}
	sort.Strings(cfgkeys)

	// Add an entry for each config variable.
	for _, key := range cfgkeys {
		// Generate a name and type to use for this key.
		sch := cfg[key]
		docURL := getDocsIndexURL(g.info.GetGitHubOrg(), g.info.Name)
		prop := propertyVariable(key, sch, custom[key], "", sch.Description, docURL, true /*out*/, entityDocs{})
		if prop != nil {
			prop.config = true
			config.addMember(prop)
		}
	}

	// Ensure there weren't any keys that were unrecognized.
	for key := range custom {
		if _, has := cfg[key]; !has {
			cmdutil.Diag().Warningf(
				diag.Message("", "custom config schema %s was not present in the Terraform metadata"), key)
		}
	}

	// Now, if there are any extra config variables, that are Pulumi-only, add them.
	for key, val := range g.info.ExtraConfig {
		if prop := propertyVariable(key, val.Schema, val.Info, "", "", "", true /*out*/, entityDocs{}); prop != nil {
			prop.config = true
			config.addMember(prop)
		}
	}

	return config
}

// gatherProvider returns the provider resource for this package.
func (g *generator) gatherProvider() (*resourceType, error) {
	cfg := g.provider().Schema
	if cfg == nil {
		cfg = map[string]*schema.Schema{}
	}
	info := &tfbridge.ResourceInfo{
		Tok:    tokens.Type(g.pkg),
		Fields: g.info.Config,
	}
	_, res, err := g.gatherResource("", &schema.Resource{Schema: cfg}, info, true)
	return res, err
}

// gatherResources returns all modules and their resources.
func (g *generator) gatherResources() (moduleMap, error) {
	// If there aren't any resources, skip this altogether.
	resources := g.provider().ResourcesMap
	if len(resources) == 0 {
		return nil, nil
	}
	modules := make(moduleMap)

	// For each resource, create its own dedicated type and module export.
	var reserr error
	seen := make(map[string]bool)
	for _, r := range stableResources(resources) {
		info := g.info.Resources[r]
		if info == nil {
			// if this resource was missing, issue a warning and skip it.
			cmdutil.Diag().Warningf(
				diag.Message("", "resource %s not found in provider map; skipping"), r)
			continue
		}
		seen[r] = true

		module, res, err := g.gatherResource(r, resources[r], info, false)
		if err != nil {
			// Keep track of the error, but keep going, so we can expose more at once.
			reserr = multierror.Append(reserr, err)
		} else {
			// Add any members returned to the specified module.
			modules.ensureModule(module).addMember(res)
		}
	}
	if reserr != nil {
		return nil, reserr
	}

	// Emit a warning if there is a map but some names didn't match.
	var names []string
	for name := range g.info.Resources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !seen[name] {
			cmdutil.Diag().Warningf(
				diag.Message("", "resource %s (%s) wasn't found in the Terraform module; possible name mismatch?"),
				name, g.info.Resources[name].Tok)
		}
	}

	return modules, nil
}

// gatherResource returns the module name and one or more module members to represent the given resource.
func (g *generator) gatherResource(rawname string,
	schema *schema.Resource, info *tfbridge.ResourceInfo, isProvider bool) (string, *resourceType, error) {
	// Get the resource's module and name.
	name, module := resourceName(g.info.Name, rawname, info, isProvider)

	// Collect documentation information
	var entityDocs entityDocs
	if !isProvider {
		pd, err := getDocsForProvider(g, g.info.GetGitHubOrg(), g.info.Name,
			g.info.GetResourcePrefix(), ResourceDocs, rawname, info)
		if err != nil {
			return "", nil, err
		}
		entityDocs = pd
	} else {
		entityDocs.Description = fmt.Sprintf(
			"The provider type for the %s package. By default, resources use package-wide configuration\n"+
				"settings, however an explicit `Provider` instance may be created and passed during resource\n"+
				"construction to achieve fine-grained programmatic control over provider settings. See the\n"+
				"[documentation](https://www.pulumi.com/docs/reference/programming-model/#providers) for more information.",
			g.info.Name)
		entityDocs.URL = getDocsIndexURL(g.info.GetGitHubOrg(), g.info.Name)
	}

	// Create an empty module and associated resource type.
	res := newResourceType(name, entityDocs, schema, info, isProvider)

	args := tfbridge.CleanTerraformSchema(schema.Schema)

	// Next, gather up all properties.
	var stateVars []*variable
	for _, key := range stableSchemas(args) {
		propschema := args[key]
		// TODO[pulumi/pulumi#397]: represent sensitive types using a Secret<T> type.
		doc := getDescriptionFromParsedDocs(entityDocs, key)
		rawdoc := propschema.Description

		propinfo := info.Fields[key]

		// If we are generating a provider, we do not emit output property definitions as provider outputs are not
		// yet implemented.
		if !isProvider {
			// For all properties, generate the output property metadata. Note that this may differ slightly
			// from the input in that the types may differ.
			outprop := propertyVariable(key, propschema, propinfo, doc, rawdoc, "", true /*out*/, entityDocs)
			if outprop != nil {
				res.outprops = append(res.outprops, outprop)
			}
		}

		// If an input, generate the input property metadata.
		if input(propschema, propinfo) {
			inprop := propertyVariable(key, propschema, propinfo, doc, rawdoc, "", false /*out*/, entityDocs)
			if inprop != nil {
				res.inprops = append(res.inprops, inprop)
				if !inprop.optional() {
					res.reqprops[name] = true
				}
			}
		}

		// Make a state variable.  This is always optional and simply lets callers perform lookups.
		stateVar := propertyVariable(key, propschema, propinfo, doc, rawdoc, "", false /*out*/, entityDocs)
		stateVar.opt = true
		stateVars = append(stateVars, stateVar)
	}

	className := res.name

	// Generate a state type for looking up instances of this resource.
	res.statet = &propertyType{
		kind:       kindObject,
		name:       fmt.Sprintf("%sState", className),
		doc:        fmt.Sprintf("Input properties used for looking up and filtering %s resources.", res.name),
		properties: stateVars,
	}

	// Next, generate the args interface for this class, and add it first to the list (since the res type uses it).
	res.argst = &propertyType{
		kind:       kindObject,
		name:       fmt.Sprintf("%sArgs", className),
		doc:        fmt.Sprintf("The set of arguments for constructing a %s resource.", name),
		properties: res.inprops,
	}

	// Ensure there weren't any custom fields that were unrecognized.
	for key := range info.Fields {
		if _, has := schema.Schema[key]; !has {
			cmdutil.Diag().Warningf(
				diag.Message("", "custom resource schema %s.%s was not present in the Terraform metadata"),
				name, key)
		}
	}

	return module, res, nil
}

func (g *generator) gatherDataSources() (moduleMap, error) {
	// If there aren't any data sources, skip this altogether.
	sources := g.provider().DataSourcesMap
	if len(sources) == 0 {
		return nil, nil
	}
	modules := make(moduleMap)

	// Sort and enumerate all variables in a deterministic order.
	var srckeys []string
	for key := range sources {
		srckeys = append(srckeys, key)
	}
	sort.Strings(srckeys)

	// For each data source, create its own dedicated function and module export.
	var dserr error
	seen := make(map[string]bool)
	for _, ds := range srckeys {
		dsinfo := g.info.DataSources[ds]
		if dsinfo == nil {
			// if this data source was missing, issue a warning and skip it.
			cmdutil.Diag().Warningf(
				diag.Message("", "data source %s not found in provider map; skipping"), ds)
			continue
		}
		seen[ds] = true

		module, fun, err := g.gatherDataSource(ds, sources[ds], dsinfo)
		if err != nil {
			// Keep track of the error, but keep going, so we can expose more at once.
			dserr = multierror.Append(dserr, err)
		} else {
			// Add any members returned to the specified module.
			modules.ensureModule(module).addMember(fun)
		}
	}
	if dserr != nil {
		return nil, dserr
	}

	// Emit a warning if there is a map but some names didn't match.
	var names []string
	for name := range g.info.DataSources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !seen[name] {
			cmdutil.Diag().Warningf(
				diag.Message("", "data source %s (%s) wasn't found in the Terraform module; possible name mismatch?"),
				name, g.info.DataSources[name].Tok)
		}
	}

	return modules, nil
}

// gatherDataSource returns the module name and members for the given data source function.
func (g *generator) gatherDataSource(rawname string,
	ds *schema.Resource, info *tfbridge.DataSourceInfo) (string, *resourceFunc, error) {
	// Generate the name and module for this data source.
	name, module := dataSourceName(g.info.Name, rawname, info)

	// Collect documentation information for this data source.
	entityDocs, err := getDocsForProvider(g, g.info.GetGitHubOrg(), g.info.Name,
		g.info.GetResourcePrefix(), DataSourceDocs, rawname, info)
	if err != nil {
		return "", nil, err
	}

	// Build up the function information.
	fun := &resourceFunc{
		name:       name,
		doc:        entityDocs.Description,
		reqargs:    make(map[string]bool),
		schema:     ds,
		info:       info,
		docURL:     entityDocs.URL,
		entityDocs: entityDocs,
	}

	// Sort the args and return properties so we are ready to go.
	args := tfbridge.CleanTerraformSchema(ds.Schema)
	var argkeys []string
	for arg := range args {
		argkeys = append(argkeys, arg)
	}
	sort.Strings(argkeys)

	// See if arguments for this function are optional, and generate detailed metadata.
	for _, arg := range argkeys {
		sch := args[arg]
		cust := info.Fields[arg]

		// Remember detailed information for every input arg (we will use it below).
		if input(args[arg], cust) {
			doc := getDescriptionFromParsedDocs(entityDocs, arg)
			argvar := propertyVariable(arg, sch, cust, doc, "", "", false /*out*/, entityDocs)
			fun.args = append(fun.args, argvar)
			if !argvar.optional() {
				fun.reqargs[argvar.name] = true
			}
		}

		// Also remember properties for the resulting return data structure.
		// Emit documentation for the property if available
		fun.rets = append(fun.rets,
			propertyVariable(arg, sch, cust, entityDocs.Attributes[arg], "", "", true /*out*/, entityDocs))
	}

	// If the data source's schema doesn't expose an id property, make one up since we'd like to expose it for data
	// sources.
	if _, has := args["id"]; !has {
		sch := &schema.Schema{
			Type:     schema.TypeString,
			Computed: true,
		}
		cust := &tfbridge.SchemaInfo{}
		rawdoc := "The provider-assigned unique ID for this managed resource."
		fun.rets = append(fun.rets,
			propertyVariable("id", sch, cust, "", rawdoc, "", true /*out*/, entityDocs))
	}

	// Produce the args/return types, if needed.
	if len(fun.args) > 0 {
		fun.argst = &propertyType{
			kind:       kindObject,
			name:       fmt.Sprintf("%sArgs", upperFirst(name)),
			doc:        fmt.Sprintf("A collection of arguments for invoking %s.", name),
			properties: fun.args,
		}
	}
	if len(fun.rets) > 0 {
		fun.retst = &propertyType{
			kind:       kindObject,
			name:       fmt.Sprintf("%sResult", upperFirst(name)),
			doc:        fmt.Sprintf("A collection of values returned by %s.", name),
			properties: fun.rets,
		}
	}

	return module, fun, nil
}

// gatherOverlays returns any overlay modules and their contents.
func (g *generator) gatherOverlays() (moduleMap, error) {
	modules := make(moduleMap)

	// Pluck out the overlay info from the right structure.  This is language dependent.
	var overlay *tfbridge.OverlayInfo
	switch g.language {
	case nodeJS:
		if jsinfo := g.info.JavaScript; jsinfo != nil {
			overlay = jsinfo.Overlay
		}
	case python:
		if pyinfo := g.info.Python; pyinfo != nil {
			overlay = pyinfo.Overlay
		}
	case golang:
		if goinfo := g.info.Golang; goinfo != nil {
			overlay = goinfo.Overlay
		}
	case csharp:
		// TODO: CSharp overlays
	case pulumiSchema:
		// N/A
	default:
		contract.Failf("unrecognized language: %s", g.language)
	}

	if overlay != nil {
		// Add the overlays that go in the root ("index") for the enclosing package.
		for _, file := range overlay.DestFiles {
			root := modules.ensureModule("")
			root.addMember(&overlayFile{name: file})
		}

		// Now add all overlays that are modules.
		for name, modolay := range overlay.Modules {
			if len(modolay.Modules) > 0 {
				return nil,
					errors.Errorf("overlay module %s is >1 level deep, which is not supported", name)
			}

			mod := modules.ensureModule(name)
			for _, file := range modolay.DestFiles {
				mod.addMember(&overlayFile{name: file})
			}
		}
	}

	return modules, nil
}

// preparePackage ensures the root exists and generates any metadata required by a Pulumi package.
func (g *generator) preparePackage(pack *pkg) error {
	// Ensure the output path exists.
	return tools.EnsureDir(g.outDir)
}

// emitProjectMetadata emits the Pulumi.yaml project file into the package's root directory.
func (g *generator) emitProjectMetadata(pack *pkg) error {
	w, err := tools.NewGenWriter(tfgen, filepath.Join(g.outDir, "Pulumi.yaml"))
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)
	w.Writefmtln("name: %s", pack.name)
	w.Writefmtln("description: A Pulumi resource provider for %s.", pack.name)
	w.Writefmtln("language: %s", pack.language)
	return nil
}

// input checks whether the given property is supplied by the user (versus being always computed).
func input(sch *schema.Schema, info *tfbridge.SchemaInfo) bool {
	return (sch.Optional || sch.Required) && !(info != nil && info.MarkAsComputedOnly != nil && *info.MarkAsComputedOnly)
}

// propertyName translates a Terraform underscore_cased_property_name into a JavaScript camelCasedPropertyName.
// IDEA: ideally specific languages could override this, to ensure "idiomatic naming", however then the bridge
//     would need to understand how to unmarshal names in a language-idiomatic way (and specifically reverse the
//     name transformation process).  This isn't impossible, but certainly complicates matters.
func propertyName(key string, sch *schema.Schema, custom *tfbridge.SchemaInfo) string {
	// Use the name override, if one exists, or use the standard name mangling otherwise.
	if custom != nil {
		if custom.Name != "" {
			return custom.Name
		}
	}

	// BUGBUG: work around issue in the Elastic Transcoder where a field has a trailing ":".
	if strings.HasSuffix(key, ":") {
		key = key[:len(key)-1]
	}

	return tfbridge.TerraformToPulumiName(key, sch, custom, false /*no to PascalCase; we want camelCase*/)
}

// propertyVariable creates a new property, with the Pulumi name, out of the given components.
func propertyVariable(key string, sch *schema.Schema, info *tfbridge.SchemaInfo,
	doc string, rawdoc string, docURL string, out bool, entityDocs entityDocs) *variable {
	if name := propertyName(key, sch, info); name != "" {
		return &variable{
			name:   name,
			out:    out,
			doc:    doc,
			rawdoc: rawdoc,
			schema: sch,
			info:   info,
			docURL: docURL,
			typ:    makePropertyType(strings.ToLower(key), sch, info, out, entityDocs),
		}
	}
	return nil
}

// dataSourceName translates a Terraform name into its Pulumi name equivalent.
func dataSourceName(provider string, rawname string, info *tfbridge.DataSourceInfo) (string, string) {
	if info == nil || info.Tok == "" {
		// default transformations.
		name := withoutPackageName(provider, rawname)                 // strip off the pkg prefix.
		return tfbridge.TerraformToPulumiName(name, nil, nil, false), // camelCase the data source name.
			tfbridge.TerraformToPulumiName(name, nil, nil, false) // camelCase the filename.
	}
	// otherwise, a custom transformation exists; use it.
	return string(info.Tok.Name()), string(info.Tok.Module().Name())
}

// resourceName translates a Terraform name into its Pulumi name equivalent, plus a module name.
func resourceName(provider string, rawname string, info *tfbridge.ResourceInfo, isProvider bool) (string, string) {
	if isProvider {
		return "Provider", ""
	}
	if info == nil || info.Tok == "" {
		// default transformations.
		name := withoutPackageName(provider, rawname)                // strip off the pkg prefix.
		return tfbridge.TerraformToPulumiName(name, nil, nil, true), // PascalCase the resource name.
			tfbridge.TerraformToPulumiName(name, nil, nil, false) // camelCase the filename.
	}
	// otherwise, a custom transformation exists; use it.
	return string(info.Tok.Name()), string(info.Tok.Module().Name())
}

// withoutPackageName strips off the package prefix from a raw name.
func withoutPackageName(pkg string, rawname string) string {
	contract.Assert(strings.HasPrefix(rawname, pkg+"_"))
	name := rawname[len(pkg)+1:] // strip off the pkg prefix.
	return name
}

func stableResources(resources map[string]*schema.Resource) []string {
	var rs []string
	for r := range resources {
		rs = append(rs, r)
	}
	sort.Strings(rs)
	return rs
}

func stableSchemas(schemas map[string]*schema.Schema) []string {
	var ss []string
	for s := range schemas {
		ss = append(ss, s)
	}
	sort.Strings(ss)
	return ss
}

// upperFirst returns the string with an upper-cased first character.
func upperFirst(s string) string {
	c, rest := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(c)) + s[rest:]
}

func generateManifestDescription(info tfbridge.ProviderInfo) string {
	if info.TFProviderVersion == "" {
		return info.Description
	}

	return fmt.Sprintf("%s. Based on terraform-provider-%s: version v%s", info.Description, info.Name,
		info.TFProviderVersion)
}

func getLicenseTypeURL(license tfbridge.TFProviderLicense) string {
	switch license {
	case tfbridge.MITLicenseType:
		return "https://mit-license.org/"
	case tfbridge.MPL20LicenseType:
		return "https://www.mozilla.org/en-US/MPL/2.0/"
	case tfbridge.Apache20LicenseType:
		return "https://www.apache.org/licenses/LICENSE-2.0.html"
	default:
		contract.Failf("Unrecognized license: %v", license)
		return ""
	}
}

func getOverlayFilesImpl(overlay *tfbridge.OverlayInfo, extension, srcRoot, dir string, files map[string][]byte) error {
	for _, f := range overlay.DestFiles {
		if path.Ext(f) == extension {
			fp := path.Join(dir, f)
			contents, err := ioutil.ReadFile(path.Join(srcRoot, fp))
			if err != nil {
				return err
			}
			files[fp] = contents
		}
	}
	for k, v := range overlay.Modules {
		if err := getOverlayFilesImpl(v, extension, srcRoot, path.Join(dir, k), files); err != nil {
			return err
		}
	}
	return nil

}

func getOverlayFiles(overlay *tfbridge.OverlayInfo, extension, srcRoot string) (map[string][]byte, error) {
	files := map[string][]byte{}
	if err := getOverlayFilesImpl(overlay, extension, srcRoot, "", files); err != nil {
		return nil, err
	}
	return files, nil
}

func emitFile(outDir, relPath string, contents []byte) error {
	p := path.Join(outDir, relPath)
	if err := tools.EnsureDir(path.Dir(p)); err != nil {
		return errors.Wrap(err, "creating directory")
	}

	f, err := os.Create(p)
	if err != nil {
		return errors.Wrap(err, "creating file")
	}
	defer contract.IgnoreClose(f)

	_, err = f.Write(contents)
	return err
}

// getDescriptionFromParsedDocs extracts the argument description for the given arg, or the
// attribute description if there is none.
func getDescriptionFromParsedDocs(entityDocs entityDocs, arg string) string {
	return getNestedDescriptionFromParsedDocs(entityDocs, "", arg)
}

// getNestedDescriptionFromParsedDocs extracts the nested argument description for the given arg, or the
// top-level argument description or attribute description if there is none.
func getNestedDescriptionFromParsedDocs(entityDocs entityDocs, objectName string, arg string) string {
	if res := entityDocs.Arguments[objectName]; res != nil && res.arguments != nil && res.arguments[arg] != "" {
		return res.arguments[arg]
	} else if res := entityDocs.Arguments[arg]; res != nil && res.description != "" {
		return res.description
	}
	return entityDocs.Attributes[arg]
}
