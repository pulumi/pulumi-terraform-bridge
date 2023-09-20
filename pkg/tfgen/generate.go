// Copyright 2016-2022, Pulumi Corporation.
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
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	dotnetgen "github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	nodejsgen "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	pygen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/afero"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/paths"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	schemaTools "github.com/pulumi/schema-tools/pkg"
)

const (
	tfgen         = "the Pulumi Terraform Bridge (tfgen) Tool"
	defaultOutDir = "sdk/"
	maxWidth      = 120 // the ideal maximum width of the generated file.
)

type Generator struct {
	pkg              tokens.Package        // the Pulumi package name (e.g. `gcp`)
	version          string                // the package version.
	language         Language              // the language runtime to generate.
	info             tfbridge.ProviderInfo // the provider info for customizing code generation
	root             afero.Fs              // the output virtual filesystem.
	providerShim     *inmemoryProvider     // a provider shim to hold the provider schema during example conversion.
	pluginHost       plugin.Host           // the plugin host for tf2pulumi.
	packageCache     *pcl.PackageCache     // the package cache for tf2pulumi.
	infoSource       il.ProviderInfoSource // the provider info source for tf2pulumi.
	terraformVersion string                // the Terraform version to target for example codegen, if any
	sink             diag.Sink
	skipDocs         bool
	skipExamples     bool
	coverageTracker  *CoverageTracker
	renamesBuilder   *renamesBuilder
	editRules        editRules

	convertedCode map[string][]byte
}

type Language string

const (
	Golang Language = "go"
	NodeJS Language = "nodejs"
	Python Language = "python"
	CSharp Language = "dotnet"
	Schema Language = "schema"
	PCL    Language = "pulumi"
)

func (l Language) shouldConvertExamples() bool {
	switch l {
	case Golang, NodeJS, Python, CSharp, Schema, PCL:
		return true
	}
	return false
}

func (l Language) emitSDK(pkg *pschema.Package, info tfbridge.ProviderInfo, root afero.Fs) (map[string][]byte, error) {
	var extraFiles map[string][]byte
	var err error

	switch l {
	case Golang:
		if psi := info.Golang; psi != nil && psi.Overlay != nil {
			extraFiles, err = getOverlayFiles(psi.Overlay, ".go", root)
			if err != nil {
				return nil, err
			}
		}

		err = cleanDir(root, pkg.Name, nil)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		m, err := gogen.GeneratePackage(tfgen, pkg)
		if err != nil {
			return nil, err
		}
		var errs multierror.Error
		for k, v := range extraFiles {
			f := m[k]
			if f != nil {
				errs.Errors = append(errs.Errors,
					fmt.Errorf("overlay conflicts with generated file at '%s'", k))
			}
			m[k] = v
		}
		return m, errs.ErrorOrNil()
	case NodeJS:
		if psi := info.JavaScript; psi != nil && psi.Overlay != nil {
			extraFiles, err = getOverlayFiles(psi.Overlay, ".ts", root)
			if err != nil {
				return nil, err
			}
		}
		// We exclude the "tests" directory because some nodejs package dirs (e.g. pulumi-docker)
		// store tests here. We don't want to include them in the overlays because we don't want it
		// exported with the module, but we don't want them deleted in a cleanup of the directory.
		exclusions := codegen.NewStringSet("tests")

		// We don't need to add overlays to the exclusion list because they have already been read
		// into memory so deleting the files is not a problem.
		err = cleanDir(root, "", exclusions)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		return nodejsgen.GeneratePackage(tfgen, pkg, extraFiles)
	case Python:
		if psi := info.Python; psi != nil && psi.Overlay != nil {
			extraFiles, err = getOverlayFiles(psi.Overlay, ".py", root)
			if err != nil {
				return nil, err
			}
		}

		// python's outdir path follows the pattern [provider]/sdk/python/pulumi_[pkg name]
		pyOutDir := fmt.Sprintf("pulumi_%s", pkg.Name)
		err = cleanDir(root, pyOutDir, nil)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		return pygen.GeneratePackage(tfgen, pkg, extraFiles)
	case CSharp:
		if psi := info.CSharp; psi != nil && psi.Overlay != nil {
			extraFiles, err = getOverlayFiles(psi.Overlay, ".cs", root)
			if err != nil {
				return nil, err
			}
		}
		err = cleanDir(root, "", nil)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		return dotnetgen.GeneratePackage(tfgen, pkg, extraFiles)
	default:
		return nil, errors.Errorf("%v does not support SDK generation", l)
	}
}

var AllLanguages = []Language{Golang, NodeJS, Python, CSharp}

// pkg is a directory containing one or more modules.
type pkg struct {
	name     tokens.Package // the package name.
	version  string         // the package version.
	language Language       // the package's language.
	root     afero.Fs       // the root of the package.
	modules  moduleMap      // the modules inside of this package.
	provider *resourceType  // the provider type for this package.
}

func newPkg(name tokens.Package, version string, language Language, fs afero.Fs) *pkg {
	return &pkg{
		name:     name,
		version:  version,
		language: language,
		root:     fs,
		modules:  make(moduleMap),
	}
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

type moduleMap map[tokens.Module]*module

func (m moduleMap) keys() []tokens.Module {
	var ks []tokens.Module
	for k := range m {
		ks = append(ks, k)
	}
	sort.SliceStable(ks, func(i, j int) bool {
		return ks[i].String() < ks[j].String()
	})
	return ks
}

func (m moduleMap) values() []*module {
	var vs []*module
	for _, k := range m.keys() {
		vs = append(vs, m[k])
	}
	return vs
}

func (m moduleMap) ensureModule(name tokens.Module) *module {
	if _, ok := m[name]; !ok {
		m[name] = newModule(name)
	}
	return m[name]
}

// module is a single module that was generated by this tool, containing members (types and functions).
type module struct {
	name    tokens.Module  // the friendly module name.
	members []moduleMember // a list of exported members from this module (ordered in case of dependencies).
}

func newModule(name tokens.Module) *module {
	return &module{name: name}
}

// configMod is the special name used for configuration modules.
var configMod tokens.ModuleName = tokens.ModuleName("config")

// indexMod is the special name used for the root default module.
var indexMod tokens.ModuleName = tokens.ModuleName("index")

// config returns true if this is a configuration module.
func (m *module) config() bool {
	return m.name.Name() == configMod
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

func (g *Generator) makePropertyType(typePath paths.TypePath,
	objectName string, sch shim.Schema, info *tfbridge.SchemaInfo, out bool,
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
		contract.Assertf(info != nil, "missing info when sch is nil on type: "+typePath.String())
		return t
	}

	// We should carry across any of the deprecation messages, to Pulumi, as per Terraform schema
	if sch.Deprecated() != "" && elemInfo != nil {
		elemInfo.DeprecationMessage = sch.Deprecated()
	}

	// Perform case analysis on Elem() and Type(). See shim.Schema.Elem() doc for reference. Start with scalars.
	switch sch.Type() {
	case shim.TypeBool:
		t.kind = kindBool
		return t
	case shim.TypeInt:
		t.kind = kindInt
		return t
	case shim.TypeFloat:
		t.kind = kindFloat
		return t
	case shim.TypeString:
		t.kind = kindString
		return t
	}

	// Handle single-nested blocks next.
	if blockType, ok := sch.Elem().(shim.Resource); ok && sch.Type() == shim.TypeMap {
		return g.makeObjectPropertyType(typePath, objectName, blockType, elemInfo, out, entityDocs)
	}

	// IsMaxItemOne lists and sets are flattened, transforming List[T] to T. Detect if this is the case.
	flatten := false
	switch sch.Type() {
	case shim.TypeList, shim.TypeSet:
		if tfbridge.IsMaxItemsOne(sch, info) {
			flatten = true
		}
	}

	// The remaining cases are collections, List[T], Set[T] or Map[T], and recursion needs NewElementPath except for
	// flattening that stays at the current path.
	var elemPath paths.TypePath = paths.NewElementPath(typePath)
	if flatten {
		elemPath = typePath
	}

	// Recognize object types encoded as a shim.Resource and compute the element type.
	var element *propertyType
	switch elem := sch.Elem().(type) {
	case shim.Schema:
		element = g.makePropertyType(elemPath, objectName, elem, elemInfo, out, entityDocs)
	case shim.Resource:
		element = g.makeObjectPropertyType(elemPath, objectName, elem, elemInfo, out, entityDocs)
	}

	if flatten {
		return element
	}

	switch sch.Type() {
	case shim.TypeMap:
		t.kind = kindMap
	case shim.TypeList:
		t.kind = kindList
	case shim.TypeSet:
		t.kind = kindSet
	default:
		panic("impossible: sch.Type() should be one of TypeMap, TypeList, TypeSet at this point")
	}
	t.element = element
	return t
}

func (g *Generator) makeObjectPropertyType(typePath paths.TypePath,
	objectName string, res shim.Resource, info *tfbridge.SchemaInfo,
	out bool, entityDocs entityDocs) *propertyType {

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

	for _, key := range stableSchemas(res.Schema()) {
		propertySchema := res.Schema()

		// TODO: Figure out why counting whether this description came from the attributes seems wrong.
		// With AWS, counting this takes the takes number of arg descriptions from attribs from about 170 to about 1400.
		// This seems wrong, so we ignore the second return value here for now.
		doc, _ := getNestedDescriptionFromParsedDocs(entityDocs, objectName, key)

		if v := g.propertyVariable(typePath, key,
			propertySchema, propertyInfos, doc, "", out, entityDocs); v != nil {
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

	schema shim.Schema
	info   *tfbridge.SchemaInfo

	typ *propertyType

	parentPath   paths.TypePath
	propertyName paths.PropertyName
}

func (v *variable) Name() string { return v.name }
func (v *variable) Doc() string  { return v.doc }

func (v *variable) deprecationMessage() string {
	if v.schema != nil && v.schema.Deprecated() != "" {
		return v.schema.Deprecated()
	}

	if v.info != nil && v.info.DeprecationMessage != "" {
		return v.info.DeprecationMessage
	}

	return ""
}

func (v *variable) forceNew() bool {
	// Output properties don't forceNew so we can return false by default
	if v.out {
		return false
	}

	// if we have an explicit marked as ForceNew then let's return that as that overrides
	// the TF schema
	if v.info != nil && v.info.ForceNew != nil {
		return *v.info.ForceNew
	}

	return v.schema != nil && v.schema.ForceNew()
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
		return v.schema != nil && v.schema.Optional() && !v.schema.Computed() && !customDefault
	}
	return (v.schema != nil && v.schema.Optional() || v.schema.Computed()) || customDefault
}

// resourceType is a generated resource type that represents a Pulumi CustomResource definition.
type resourceType struct {
	mod        tokens.Module
	name       string
	doc        string
	isProvider bool
	inprops    []*variable
	outprops   []*variable
	reqprops   map[string]bool
	argst      *propertyType // input properties.
	statet     *propertyType // output properties (all optional).
	schema     shim.Resource
	info       *tfbridge.ResourceInfo
	entityDocs entityDocs // parsed docs.

	resourcePath *paths.ResourcePath
}

func (rt *resourceType) Name() string { return rt.name }
func (rt *resourceType) Doc() string  { return rt.doc }

// IsProvider is true if this resource is a ProviderResource.
func (rt *resourceType) IsProvider() bool { return rt.isProvider }

func (rt *resourceType) TypeToken() tokens.Type {
	return tokens.NewTypeToken(rt.mod, tokens.TypeName(rt.name))
}

func newResourceType(resourcePath *paths.ResourcePath,
	mod tokens.Module, name tokens.TypeName, entityDocs entityDocs,
	schema shim.Resource, info *tfbridge.ResourceInfo,
	isProvider bool) *resourceType {

	// We want to add the import details to the description so we can display those for the user
	description := entityDocs.Description
	if entityDocs.Import != "" {
		description = fmt.Sprintf("%s\n\n%s", description, entityDocs.Import)
	}

	return &resourceType{
		mod:          mod,
		name:         name.String(),
		doc:          description,
		isProvider:   isProvider,
		schema:       schema,
		info:         info,
		reqprops:     make(map[string]bool),
		entityDocs:   entityDocs,
		resourcePath: resourcePath,
	}
}

// resourceFunc is a generated resource function that is exposed to interact with Pulumi objects.
type resourceFunc struct {
	mod            tokens.Module
	name           string
	doc            string
	args           []*variable
	rets           []*variable
	reqargs        map[string]bool
	argst          *propertyType
	retst          *propertyType
	schema         shim.Resource
	info           *tfbridge.DataSourceInfo
	entityDocs     entityDocs
	dataSourcePath *paths.DataSourcePath
}

func (rf *resourceFunc) Name() string { return rf.name }
func (rf *resourceFunc) Doc() string  { return rf.doc }

func (rf *resourceFunc) ModuleMemberToken() tokens.ModuleMember {
	return tokens.NewModuleMemberToken(rf.mod, tokens.ModuleMemberName(rf.name))
}

// overlayFile is a file that should be added to a module "as-is" and then exported from its index.
type overlayFile struct {
	name string
	src  string
}

func (of *overlayFile) Name() string { return of.name }
func (of *overlayFile) Doc() string  { return "" }
func (of *overlayFile) Copy() bool   { return of.src != "" }

func GenerateSchema(info tfbridge.ProviderInfo, sink diag.Sink) (pschema.PackageSpec, error) {
	res, err := GenerateSchemaWithOptions(GenerateSchemaOptions{
		ProviderInfo:    info,
		DiagnosticsSink: sink,
	})
	if err != nil {
		return pschema.PackageSpec{}, err
	}
	return res.PackageSpec, nil
}

type GenerateSchemaOptions struct {
	ProviderInfo    tfbridge.ProviderInfo
	DiagnosticsSink diag.Sink
}

type GenerateSchemaResult struct {
	PackageSpec pschema.PackageSpec
	Renames     Renames
}

func GenerateSchemaWithOptions(opts GenerateSchemaOptions) (*GenerateSchemaResult, error) {
	info := opts.ProviderInfo
	sink := opts.DiagnosticsSink
	g, err := NewGenerator(GeneratorOptions{
		Package:      info.Name,
		Version:      info.Version,
		Language:     Schema,
		ProviderInfo: info,
		Root:         afero.NewMemMapFs(),
		Sink:         sink,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create generator")
	}

	// NOTE: sequence identical to(*Generator).Generate().
	pack, err := g.gatherPackage()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to gather package metadata")
	}

	s, err := genPulumiSchema(pack, g.pkg, g.version, g.info, g.renamesBuilder)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate schema")
	}

	r, err := g.Renames()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate renames")
	}

	if err := nameCheck(g.info, s, g.renamesBuilder, g.sink); err != nil {
		return nil, err
	}

	return &GenerateSchemaResult{
		PackageSpec: s,
		Renames:     r,
	}, nil
}

type GeneratorOptions struct {
	Package            string
	Version            string
	Language           Language
	ProviderInfo       tfbridge.ProviderInfo
	Root               afero.Fs
	ProviderInfoSource il.ProviderInfoSource
	PluginHost         plugin.Host
	TerraformVersion   string
	Sink               diag.Sink
	Debug              bool
	SkipDocs           bool
	SkipExamples       bool
	CoverageTracker    *CoverageTracker
}

// NewGenerator returns a code-generator for the given language runtime and package info.
func NewGenerator(opts GeneratorOptions) (*Generator, error) {
	pkgName, version, lang, info, root := opts.Package, opts.Version, opts.Language, opts.ProviderInfo, opts.Root

	pkg := tokens.NewPackageToken(tokens.PackageName(tokens.IntoQName(pkgName)))

	// Ensure the language is valid.
	switch lang {
	case Golang, NodeJS, Python, CSharp, Schema, PCL:
		// OK
	default:
		return nil, errors.Errorf("unrecognized language runtime: %s", lang)
	}

	// If root is nil, default to sdk/<language>/ in the pwd.
	if root == nil {
		p, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		p = filepath.Join(p, defaultOutDir, string(lang))
		if err = os.MkdirAll(p, 0700); err != nil {
			return nil, err
		}
		root = afero.NewBasePathFs(afero.NewOsFs(), p)
	}

	sink := opts.Sink
	if sink == nil {
		diagOpts := diag.FormatOptions{
			Color: cmdutil.GetGlobalColorization(),
			Debug: opts.Debug,
		}
		cmdutil.InitDiag(diagOpts)
		sink = cmdutil.Diag()
	}

	pluginHost := opts.PluginHost
	if pluginHost == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		ctx, err := plugin.NewContext(sink, sink, nil, nil, cwd, nil, false, nil)
		if err != nil {
			return nil, err
		}
		pluginHost = ctx.Host
	}

	infoSources := append([]il.ProviderInfoSource{}, opts.ProviderInfoSource, il.PluginProviderInfoSource)
	infoSource := il.NewCachingProviderInfoSource(il.NewMultiProviderInfoSource(infoSources...))

	providerShim := newInMemoryProvider(pkg, nil, info)
	host := &inmemoryProviderHost{
		Host:               pluginHost,
		ProviderInfoSource: infoSource,
		provider:           providerShim,
	}

	return &Generator{
		pkg:              pkg,
		version:          version,
		language:         lang,
		info:             info,
		root:             root,
		providerShim:     providerShim,
		pluginHost:       newCachingProviderHost(host),
		packageCache:     pcl.NewPackageCache(),
		infoSource:       host,
		terraformVersion: opts.TerraformVersion,
		sink:             sink,
		skipDocs:         opts.SkipDocs,
		skipExamples:     opts.SkipExamples,
		coverageTracker:  opts.CoverageTracker,
		renamesBuilder:   newRenamesBuilder(pkg, opts.ProviderInfo.GetResourcePrefix()),
		editRules:        getEditRules(info.DocRules),
	}, nil
}

func (g *Generator) error(f string, args ...interface{}) {
	g.sink.Errorf(diag.Message("", f), args...)
}

func (g *Generator) warn(f string, args ...interface{}) {
	g.sink.Warningf(diag.Message("", f), args...)
}

func (g *Generator) debug(f string, args ...interface{}) {
	g.sink.Debugf(diag.Message("", f), args...)
}

func (g *Generator) provider() shim.Provider {
	return g.info.P
}

type GenerateOptions struct {
	ModuleFormat string
}

// Generate creates Pulumi packages from the information it was initialized with.
func (g *Generator) Generate() error {

	// First gather up the entire package contents. This structure is complete and sufficient to hand off to the
	// language-specific generators to create the full output.
	pack, err := g.gatherPackage()
	if err != nil {
		return errors.Wrapf(err, "failed to gather package metadata")
	}

	// Convert the package to a Pulumi schema.
	pulumiPackageSpec, err := genPulumiSchema(pack, g.pkg, g.version, g.info, g.renamesBuilder)
	if err != nil {
		return errors.Wrapf(err, "failed to create Pulumi schema")
	}

	// Apply schema post-processing if defined in the provider.
	if g.info.SchemaPostProcessor != nil {
		g.info.SchemaPostProcessor(&pulumiPackageSpec)
	}

	// As a side-effect genPulumiSchema also populated rename tables.
	renames, err := g.Renames()
	if err != nil {
		return errors.Wrapf(err, "failed to generate renames")
	}

	if err := nameCheck(g.info, pulumiPackageSpec, g.renamesBuilder, g.sink); err != nil {
		return err
	}

	genSchemaResult := &GenerateSchemaResult{
		PackageSpec: pulumiPackageSpec,
		Renames:     renames,
	}

	// Now push the schema through the rest of the generator.
	return g.UnstableGenerateFromSchema(genSchemaResult)
}

// GenerateFromSchema creates Pulumi packages from a pulumi schema and the information the
// generator was initialized with.
//
// This is an unstable API. We have exposed it so other packages within
// pulumi-terraform-bridge can consume it. We do not recommend other packages consume this
// API.
func (g *Generator) UnstableGenerateFromSchema(genSchemaResult *GenerateSchemaResult) error {
	// MetadataInfo gets stored on disk from previous versions of the provider. Renames do not need to be
	// history-aware and they are simply re-computed from scratch as part of generating the schema.
	// If we are storing such metadata, override the previous Renames with the new Renames.
	if info := g.info.MetadataInfo; info != nil {
		err := metadata.Set(info.Data, "renames", genSchemaResult.Renames)
		if err != nil {
			return fmt.Errorf("[pkg/tfgen] failed to add renames to MetadataInfo.Data: %w", err)
		}
	}

	pulumiPackageSpec := genSchemaResult.PackageSpec
	schemaStats = schemaTools.CountStats(pulumiPackageSpec)

	// Serialize the schema and attach it to the provider shim.
	var err error
	g.providerShim.schema, err = json.Marshal(pulumiPackageSpec)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal intermediate schema")
	}

	// Add any supplemental examples:
	err = addExtraHclExamplesToResources(g.info.ExtraResourceHclExamples, &pulumiPackageSpec)
	if err != nil {
		return err
	}

	err = addExtraHclExamplesToFunctions(g.info.ExtraFunctionHclExamples, &pulumiPackageSpec)
	if err != nil {
		return err
	}

	// Convert examples.
	if !g.skipExamples {
		pulumiPackageSpec = g.convertExamplesInSchema(pulumiPackageSpec)
	}

	// Go ahead and let the language generator do its thing. If we're emitting the schema, just go ahead and serialize
	// it out.
	var files map[string][]byte
	switch g.language {
	case Schema:
		// Omit the version so that the spec is stable if the version is e.g. derived from the current Git commit hash.
		pulumiPackageSpec.Version = ""

		bytes, err := json.MarshalIndent(pulumiPackageSpec, "", "    ")
		if err != nil {
			return errors.Wrapf(err, "failed to marshal schema")
		}
		files = map[string][]byte{"schema.json": bytes}

		if info := g.info.MetadataInfo; info != nil {
			files[info.Path] = (*metadata.Data)(info.Data).Marshal()
		}
	case PCL:
		if g.skipExamples {
			return fmt.Errorf("Cannot set skipExamples and get PCL")
		}
		files = map[string][]byte{}
		for path, code := range g.convertedCode {
			path = strings.TrimPrefix(path, "#/") + ".pp"
			files[path] = code
		}
	default:
		pulumiPackage, diags, err := pschema.BindSpec(pulumiPackageSpec, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to import Pulumi schema")
		}
		if diags.HasErrors() {
			return err
		}
		if files, err = g.language.emitSDK(pulumiPackage, g.info, g.root); err != nil {
			return errors.Wrapf(err, "failed to generate package")
		}
	}

	// Write the result to disk. Do not overwrite the root-level README.md if any exists.
	for f, contents := range files {
		if f == "README.md" {
			if _, err := g.root.Stat(f); err == nil {
				continue
			}
		}
		if err := emitFile(g.root, f, contents); err != nil {
			return errors.Wrapf(err, "emitting file %v", f)
		}
	}

	// Emit the Pulumi project information.
	if err = g.emitProjectMetadata(g.pkg, g.language); err != nil {
		return errors.Wrapf(err, "failed to create project file")
	}

	// Print out some documentation stats as a summary afterwards.
	printDocStats()

	// Close the plugin host.
	g.pluginHost.Close()

	return nil
}

// Remanes can be called after a successful call to Generate to extract name mappings.
func (g *Generator) Renames() (Renames, error) {
	return g.renamesBuilder.BuildRenames()
}

// gatherPackage creates a package plus module structure for the entire set of members of this package.
func (g *Generator) gatherPackage() (*pkg, error) {
	// First, gather up the entire package/module structure.  This includes gathering config entries, resources,
	// data sources, and any supporting type information, and placing them into modules.
	pack := newPkg(g.pkg, g.version, g.language, g.root)

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
func (g *Generator) gatherConfig() *module {
	// If there's no config, skip creating the module.
	cfg := g.provider().Schema()
	if cfg.Len() == 0 {
		return nil
	}
	config := newModule(tokens.NewModuleToken(g.pkg, configMod))

	// Sort the config variables to ensure they are emitted in a deterministic order.
	custom := g.info.Config
	var cfgkeys []string
	cfg.Range(func(key string, _ shim.Schema) bool {
		cfgkeys = append(cfgkeys, key)
		return true
	})
	sort.Strings(cfgkeys)

	cfgPath := paths.NewConfigPath()

	// Add an entry for each config variable.
	for _, key := range cfgkeys {
		// Generate a name and type to use for this key.
		sch := cfg.Get(key)
		prop := g.propertyVariable(cfgPath,
			key, cfg, custom, "", sch.Description(), true /*out*/, entityDocs{})
		if prop != nil {
			prop.config = true
			config.addMember(prop)
		}
	}

	// Ensure there weren't any keys that were unrecognized.
	for key := range custom {
		if _, has := cfg.GetOk(key); !has {
			g.warn("custom config schema %s was not present in the Terraform metadata", key)
		}
	}

	// Now, if there are any extra config variables, that are Pulumi-only, add them.
	extraConfigInfo := map[string]*tfbridge.SchemaInfo{}
	extraConfigMap := schema.SchemaMap{}
	for key, val := range g.info.ExtraConfig {
		extraConfigInfo[key] = val.Info
		extraConfigMap.Set(key, val.Schema)
	}
	for key := range g.info.ExtraConfig {
		if prop := g.propertyVariable(cfgPath,
			key, extraConfigMap, extraConfigInfo, "", "", true /*out*/, entityDocs{}); prop != nil {
			prop.config = true
			config.addMember(prop)
		}
	}

	return config
}

// gatherProvider returns the provider resource for this package.
func (g *Generator) gatherProvider() (*resourceType, error) {
	cfg := g.provider().Schema()
	if cfg == nil {
		cfg = schema.SchemaMap{}
	}
	info := &tfbridge.ResourceInfo{
		Tok:    tokens.Type(g.pkg.String()),
		Fields: g.info.Config,
	}
	res, err := g.gatherResource("", (&schema.Resource{Schema: cfg}).Shim(), info, true)
	return res, err
}

// gatherResources returns all modules and their resources.
func (g *Generator) gatherResources() (moduleMap, error) {
	// If there aren't any resources, skip this altogether.
	resources := g.provider().ResourcesMap()
	if resources.Len() == 0 {
		return nil, nil
	}
	modules := make(moduleMap)

	skipFailBuildOnMissingMapError := isTruthy(os.Getenv("PULUMI_SKIP_MISSING_MAPPING_ERROR")) || isTruthy(os.Getenv(
		"PULUMI_SKIP_PROVIDER_MAP_ERROR"))
	skipFailBuildOnExtraMapError := isTruthy(os.Getenv("PULUMI_SKIP_EXTRA_MAPPING_ERROR"))

	// let's keep a list of TF mapping errors that we can present to the user
	var resourceMappingErrors error

	// For each resource, create its own dedicated type and module export.
	var reserr error
	seen := make(map[string]bool)
	for _, r := range stableResources(resources) {
		info := g.info.Resources[r]
		if info == nil {
			if ignoreMappingError(g.info.IgnoreMappings, r) {
				g.debug("TF resource %q not found in provider map", r)
				continue
			}

			if !ignoreMappingError(g.info.IgnoreMappings, r) && !skipFailBuildOnMissingMapError {
				resourceMappingErrors = multierror.Append(resourceMappingErrors,
					fmt.Errorf("TF resource %q not mapped to the Pulumi provider", r))
			} else {
				g.warn("TF resource %q not found in provider map", r)
			}
			continue
		}
		seen[r] = true

		res, err := g.gatherResource(r, resources.Get(r), info, false)
		if err != nil {
			// Keep track of the error, but keep going, so we can expose more at once.
			reserr = multierror.Append(reserr, err)
		} else {
			// Add any members returned to the specified module.
			modules.ensureModule(res.mod).addMember(res)
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
			if !skipFailBuildOnExtraMapError {
				resourceMappingErrors = multierror.Append(resourceMappingErrors,
					fmt.Errorf("Pulumi token %q is mapped to TF provider resource %q, but no such "+
						"resource found. The mapping will be ignored in the generated provider",
						g.info.Resources[name].Tok, name))
			} else {
				g.warn("Pulumi token %q is mapped to TF provider resource %q, but no such "+
					"resource found. The mapping will be ignored in the generated provider",
					g.info.Resources[name].Tok, name)
			}
		}
	}
	// let's check the unmapped Resource Errors
	if resourceMappingErrors != nil {
		return nil, resourceMappingErrors
	}

	return modules, nil
}

// gatherResource returns the module name and one or more module members to represent the given resource.
func (g *Generator) gatherResource(rawname string,
	schema shim.Resource, info *tfbridge.ResourceInfo, isProvider bool) (*resourceType, error) {

	// Get the resource's module and name.
	name, moduleName := resourceName(g.info.Name, rawname, info, isProvider)
	mod := tokens.NewModuleToken(g.pkg, moduleName)

	resourceToken := tokens.NewTypeToken(mod, name)
	resourcePath := paths.NewResourcePath(rawname, resourceToken, isProvider)

	g.renamesBuilder.registerResource(resourcePath)

	// Collect documentation information
	var entityDocs entityDocs
	if !isProvider {
		pd, err := getDocsForResource(g, g.info.GetGitHubOrg(), g.info.Name,
			g.info.GetResourcePrefix(), ResourceDocs, rawname, info, g.info.GetProviderModuleVersion(),
			g.info.GetGitHubHost())
		if err != nil {
			return nil, err
		}
		entityDocs = pd
	} else {
		entityDocs.Description = fmt.Sprintf(
			"The provider type for the %s package. By default, resources use package-wide configuration\n"+
				"settings, however an explicit `Provider` instance may be created and passed during resource\n"+
				"construction to achieve fine-grained programmatic control over provider settings. See the\n"+
				"[documentation](https://www.pulumi.com/docs/reference/programming-model/#providers) for more information.",
			g.info.Name)
	}

	// Create an empty module and associated resource type.
	res := newResourceType(resourcePath, mod, name, entityDocs, schema, info, isProvider)

	// Next, gather up all properties.
	var stateVars []*variable
	for _, key := range stableSchemas(schema.Schema()) {
		propschema := schema.Schema().Get(key)
		if propschema.Removed() != "" {
			continue
		}

		// TODO[pulumi/pulumi#397]: represent sensitive types using a Secret<T> type.
		doc, foundInAttributes := getDescriptionFromParsedDocs(entityDocs, key)
		rawdoc := propschema.Description()

		propinfo := info.Fields[key]

		// If we are generating a provider, we do not emit output property definitions as provider outputs are not
		// yet implemented.
		if !isProvider {
			// For all properties, generate the output property metadata. Note that this may differ slightly
			// from the input in that the types may differ.
			outprop := g.propertyVariable(resourcePath.Outputs(), key, schema.Schema(),
				info.Fields, doc, rawdoc, true /*out*/, entityDocs)
			if outprop != nil {
				res.outprops = append(res.outprops, outprop)
			}
		}

		// If an input, generate the input property metadata.
		if input(propschema, propinfo) {
			if foundInAttributes && !isProvider {
				argumentDescriptionsFromAttributes++
				msg := fmt.Sprintf("Argument desc from attributes: resource, rawname = '%s', property = '%s'", rawname, key)
				g.debug(msg)
			}

			inprop := g.propertyVariable(resourcePath.Inputs(),
				key, schema.Schema(), info.Fields, doc, rawdoc, false /*out*/, entityDocs)
			if inprop != nil {
				res.inprops = append(res.inprops, inprop)
				if !inprop.optional() {
					res.reqprops[name.String()] = true
				}
			}
		}

		// Make a state variable.  This is always optional and simply lets callers perform lookups.
		stateVar := g.propertyVariable(resourcePath.State(), key, schema.Schema(), info.Fields,
			doc, rawdoc, false /*out*/, entityDocs)
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
		if _, has := schema.Schema().GetOk(key); !has {
			msg := fmt.Sprintf("there is a custom mapping on resource '%s' for field '%s', but the field was not "+
				"found in the Terraform metadata and will be ignored. To fix, remove the mapping.", rawname, key)

			if isTruthy(os.Getenv("PULUMI_EXTRA_MAPPING_ERROR")) {
				return nil, fmt.Errorf(msg)
			}

			g.warn(msg)
		}
	}

	return res, nil
}

func (g *Generator) gatherDataSources() (moduleMap, error) {
	// If there aren't any data sources, skip this altogether.
	sources := g.provider().DataSourcesMap()
	if sources.Len() == 0 {
		return nil, nil
	}
	modules := make(moduleMap)

	skipFailBuildOnMissingMapError := isTruthy(os.Getenv("PULUMI_SKIP_MISSING_MAPPING_ERROR")) || isTruthy(os.Getenv(
		"PULUMI_SKIP_PROVIDER_MAP_ERROR"))
	failBuildOnExtraMapError := isTruthy(os.Getenv("PULUMI_EXTRA_MAPPING_ERROR"))

	// let's keep a list of TF mapping errors that we can present to the user
	var dataSourceMappingErrors error

	// For each data source, create its own dedicated function and module export.
	var dserr error
	seen := make(map[string]bool)
	for _, ds := range stableResources(sources) {
		dsinfo := g.info.DataSources[ds]
		if dsinfo == nil {
			if ignoreMappingError(g.info.IgnoreMappings, ds) {
				g.debug("TF data source %q not found in provider map but ignored", ds)
				continue
			}

			if !skipFailBuildOnMissingMapError {
				dataSourceMappingErrors = multierror.Append(dataSourceMappingErrors,
					fmt.Errorf("TF data source %q not mapped to the Pulumi provider", ds))
			} else {
				g.warn("TF data source %q not found in provider map", ds)
			}
			continue
		}
		seen[ds] = true

		fun, err := g.gatherDataSource(ds, sources.Get(ds), dsinfo)
		if err != nil {
			// Keep track of the error, but keep going, so we can expose more at once.
			dserr = multierror.Append(dserr, err)
		} else {
			// Add any members returned to the specified module.
			modules.ensureModule(fun.mod).addMember(fun)
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
			if failBuildOnExtraMapError {
				dataSourceMappingErrors = multierror.Append(dataSourceMappingErrors,
					fmt.Errorf("Pulumi token %q is mapped to TF provider data source %q, but no such "+
						"data source found. Remove the mapping and try again",
						g.info.DataSources[name].Tok, name))
			} else {
				g.warn("Pulumi token %q is mapped to TF provider data source %q, but no such "+
					"data source found. The mapping will be ignored in the generated provider",
					g.info.DataSources[name].Tok, name)
			}
		}
	}

	// let's check the unmapped DataSource Errors
	if dataSourceMappingErrors != nil {
		return nil, dataSourceMappingErrors
	}

	return modules, nil
}

// gatherDataSource returns the module name and members for the given data source function.
func (g *Generator) gatherDataSource(rawname string,
	ds shim.Resource, info *tfbridge.DataSourceInfo) (*resourceFunc, error) {

	// Generate the name and module for this data source.
	name, moduleName := dataSourceName(g.info.Name, rawname, info)
	mod := tokens.NewModuleToken(g.pkg, moduleName)
	dataSourcePath := paths.NewDataSourcePath(rawname, tokens.NewModuleMemberToken(mod, name))
	g.renamesBuilder.registerDataSource(dataSourcePath)

	// Collect documentation information for this data source.
	entityDocs, err := getDocsForResource(g, g.info.GetGitHubOrg(), g.info.Name,
		g.info.GetResourcePrefix(), DataSourceDocs, rawname, info, g.info.GetProviderModuleVersion(),
		g.info.GetGitHubHost())
	if err != nil {
		return nil, err
	}

	// Build up the function information.
	fun := &resourceFunc{
		mod:            mod,
		name:           name.String(),
		doc:            entityDocs.Description,
		reqargs:        make(map[string]bool),
		schema:         ds,
		info:           info,
		entityDocs:     entityDocs,
		dataSourcePath: dataSourcePath,
	}

	// See if arguments for this function are optional, and generate detailed metadata.
	for _, arg := range stableSchemas(ds.Schema()) {
		sch := ds.Schema().Get(arg)
		if sch.Removed() != "" {
			continue
		}
		cust := info.Fields[arg]

		// Remember detailed information for every input arg (we will use it below).
		if input(sch, cust) {
			doc, foundInAttributes := getDescriptionFromParsedDocs(entityDocs, arg)
			if foundInAttributes {
				argumentDescriptionsFromAttributes++
				msg := fmt.Sprintf("Argument desc taken from attributes: data source, rawname = '%s', property = '%s'",
					rawname, arg)
				g.debug(msg)
			}

			argvar := g.propertyVariable(dataSourcePath.Args(),
				arg, ds.Schema(), info.Fields, doc, "", false /*out*/, entityDocs)
			fun.args = append(fun.args, argvar)
			if !argvar.optional() {
				fun.reqargs[argvar.name] = true
			}
		}

		// Also remember properties for the resulting return data structure.
		// Emit documentation for the property if available
		fun.rets = append(fun.rets,
			g.propertyVariable(dataSourcePath.Results(),
				arg, ds.Schema(), info.Fields, entityDocs.Attributes[arg], "", true /*out*/, entityDocs))
	}

	// If the data source's schema doesn't expose an id property, make one up since we'd like to expose it for data
	// sources.
	if id, has := ds.Schema().GetOk("id"); !has || id.Removed() != "" {
		cust := map[string]*tfbridge.SchemaInfo{"id": {}}
		rawdoc := "The provider-assigned unique ID for this managed resource."
		idSchema := schema.SchemaMap(map[string]shim.Schema{
			"id": (&schema.Schema{Type: shim.TypeString, Computed: true}).Shim(),
		})
		fun.rets = append(fun.rets,
			g.propertyVariable(dataSourcePath.Results(),
				"id", idSchema, cust, "", rawdoc, true /*out*/, entityDocs))
	}

	// Produce the args/return types, if needed.
	if len(fun.args) > 0 {
		fun.argst = &propertyType{
			kind:       kindObject,
			name:       fmt.Sprintf("%sArgs", upperFirst(name.String())),
			doc:        fmt.Sprintf("A collection of arguments for invoking %s.", name),
			properties: fun.args,
		}
	}
	if len(fun.rets) > 0 {
		fun.retst = &propertyType{
			kind:       kindObject,
			name:       fmt.Sprintf("%sResult", upperFirst(name.String())),
			doc:        fmt.Sprintf("A collection of values returned by %s.", name),
			properties: fun.rets,
		}
	}

	return fun, nil
}

// gatherOverlays returns any overlay modules and their contents.
func (g *Generator) gatherOverlays() (moduleMap, error) {
	modules := make(moduleMap)

	// Pluck out the overlay info from the right structure.  This is language dependent.
	var overlay *tfbridge.OverlayInfo
	switch g.language {
	case NodeJS:
		if jsinfo := g.info.JavaScript; jsinfo != nil {
			overlay = jsinfo.Overlay
		}
	case Python:
		if pyinfo := g.info.Python; pyinfo != nil {
			overlay = pyinfo.Overlay
		}
	case Golang:
		if goinfo := g.info.Golang; goinfo != nil {
			overlay = goinfo.Overlay
		}
	case CSharp:
		if csharpinfo := g.info.CSharp; csharpinfo != nil {
			overlay = csharpinfo.Overlay
		}
	case Schema, PCL:
		// N/A
	default:
		contract.Failf("unrecognized language: %s", g.language)
	}

	if overlay != nil {
		// Add the overlays that go in the root ("index") for the enclosing package.
		for _, file := range overlay.DestFiles {
			modToken := tokens.NewModuleToken(g.pkg, indexMod)
			root := modules.ensureModule(modToken)
			root.addMember(&overlayFile{name: file})
		}

		// Now add all overlays that are modules.
		for name, modolay := range overlay.Modules {
			if len(modolay.Modules) > 0 {
				return nil,
					errors.Errorf("overlay module %s is >1 level deep, which is not supported", name)
			}
			modToken := tokens.NewModuleToken(g.pkg, tokens.ModuleName(name))
			mod := modules.ensureModule(modToken)
			for _, file := range modolay.DestFiles {
				mod.addMember(&overlayFile{name: file})
			}
		}
	}

	return modules, nil
}

// emitProjectMetadata emits the Pulumi.yaml project file into the package's root directory.
func (g *Generator) emitProjectMetadata(name tokens.Package, language Language) error {
	w, err := newGenWriter(tfgen, g.root, "Pulumi.yaml")
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)
	w.Writefmtln("name: %s", name)
	w.Writefmtln("description: A Pulumi resource provider for %s.", name)
	w.Writefmtln("language: %s", language)
	return nil
}

// input checks whether the given property is supplied by the user (versus being always computed).
func input(sch shim.Schema, info *tfbridge.SchemaInfo) bool {
	return (sch.Optional() || sch.Required()) &&
		!(info != nil && info.MarkAsComputedOnly != nil && *info.MarkAsComputedOnly)
}

// propertyName translates a Terraform underscore_cased_property_name into a JavaScript camelCasedPropertyName.
// IDEA: ideally specific languages could override this, to ensure "idiomatic naming", however then the bridge
//
//	would need to understand how to unmarshal names in a language-idiomatic way (and specifically reverse the
//	name transformation process).  This isn't impossible, but certainly complicates matters.
func propertyName(key string, sch shim.SchemaMap, custom map[string]*tfbridge.SchemaInfo) string {
	// BUGBUG: work around issue in the Elastic Transcoder where a field has a trailing ":".
	key = strings.TrimSuffix(key, ":")

	return tfbridge.TerraformToPulumiNameV2(key, sch, custom)
}

// propertyVariable creates a new property, with the Pulumi name, out of the given components.
//
// key is the Terraform property name
//
// parentPath together with key uniquely locates the property in the Terraform schema.
func (g *Generator) propertyVariable(parentPath paths.TypePath, key string,
	sch shim.SchemaMap, info map[string]*tfbridge.SchemaInfo,
	doc string, rawdoc string, out bool, entityDocs entityDocs) *variable {

	if name := propertyName(key, sch, info); name != "" {
		propName := paths.PropertyName{Key: key, Name: tokens.Name(name)}
		typePath := paths.NewProperyPath(parentPath, propName)

		if g.renamesBuilder != nil {
			g.renamesBuilder.registerProperty(parentPath, propName)
		}

		var schema shim.Schema
		if sch != nil {
			schema = sch.Get(key)
		}
		var varInfo *tfbridge.SchemaInfo
		if info != nil {
			varInfo = info[key]
		}

		return &variable{
			name:         name,
			out:          out,
			doc:          doc,
			rawdoc:       rawdoc,
			schema:       schema,
			info:         varInfo,
			typ:          g.makePropertyType(typePath, strings.ToLower(key), schema, varInfo, out, entityDocs),
			parentPath:   parentPath,
			propertyName: propName,
		}
	}
	return nil
}

// dataSourceName translates a Terraform name into its Pulumi name equivalent.
func dataSourceName(provider string, rawname string,
	info *tfbridge.DataSourceInfo) (tokens.ModuleMemberName, tokens.ModuleName) {
	if info == nil || info.Tok == "" {
		// default transformations.
		name := withoutPackageName(provider, rawname) // strip off the pkg prefix.
		name = tfbridge.TerraformToPulumiNameV2(name, nil, nil)
		return tokens.ModuleMemberName(name), tokens.ModuleName(name)
	}
	// otherwise, a custom transformation exists; use it.
	return info.Tok.Name(), info.Tok.Module().Name()
}

// resourceName translates a Terraform name into its Pulumi name equivalent, plus a module name.
func resourceName(provider string, rawname string,
	info *tfbridge.ResourceInfo, isProvider bool) (tokens.TypeName, tokens.ModuleName) {
	if isProvider {
		return "Provider", indexMod
	}
	if info == nil || info.Tok == "" {
		// default transformations.
		name := withoutPackageName(provider, rawname)              // strip off the pkg prefix.
		camel := tfbridge.TerraformToPulumiNameV2(name, nil, nil)  // camelCase the module name.
		pascal := tfbridge.TerraformToPulumiNameV2(name, nil, nil) // PascalCase the resource name.
		if pascal != "" {
			pascal = string(unicode.ToUpper(rune(pascal[0]))) + pascal[1:]
		}

		modName := tokens.ModuleName(camel)
		return tokens.TypeName(pascal), modName
	}
	// otherwise, a custom transformation exists; use it.
	return info.Tok.Name(), info.Tok.Module().Name()
}

// withoutPackageName strips off the package prefix from a raw name.
func withoutPackageName(pkg string, rawname string) string {
	// Providers almost always have function and resource names prefixed with the package name,
	// but every rule has an exception! In HTTP provider the "http" datasource intentionally
	// does not do that, as noted in:
	//
	///nolint:lll
	// https://github.com/hashicorp/terraform-provider-http/blob/0eeb9818e8114631a3c7dc61e750f11180ca987b/internal/provider/data_source_http.go#L47
	//
	// Therefore the code trims the prefix if it finds it, but leaves as-is otherwise.
	return strings.TrimPrefix(rawname, pkg+"_")
}

func stableResources(resources shim.ResourceMap) []string {
	var rs []string
	resources.Range(func(r string, _ shim.Resource) bool {
		rs = append(rs, r)
		return true
	})
	sort.Strings(rs)
	return rs
}

func stableSchemas(schemas shim.SchemaMap) []string {
	var ss []string
	schemas.Range(func(s string, _ shim.Schema) bool {
		ss = append(ss, s)
		return true
	})
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
	case tfbridge.UnlicensedLicenseType:
		return ""
	default:
		contract.Failf("Unrecognized license: %v", license)
		return ""
	}
}

func getOverlayFilesImpl(overlay *tfbridge.OverlayInfo, extension string,
	fs afero.Fs, srcRoot, dir string, files map[string][]byte) error {

	for _, f := range overlay.DestFiles {
		if path.Ext(f) == extension {
			fp := path.Join(dir, f)
			contents, err := afero.ReadFile(fs, path.Join(srcRoot, fp))
			if err != nil {
				return err
			}
			// If we are in Python (and potentially Go) then we may need to strip the leading
			// folder extension from the fp. Otherwise, when we write the overlay back
			// it will write to a double nested structure
			// eg. sdk/python/pulumi_provider/pulumi_provider/file.py
			// We need to do this *after* we read the file so that we can assemble the package
			// correctly later
			if extension == ".py" {
				fp = path.Base(fp)
			}
			files[fp] = contents
		}
	}
	for k, v := range overlay.Modules {
		if err := getOverlayFilesImpl(v, extension, fs, srcRoot, path.Join(dir, k), files); err != nil {
			return err
		}
	}

	return nil
}

func getOverlayFiles(overlay *tfbridge.OverlayInfo, extension string, root afero.Fs) (map[string][]byte, error) {
	files := map[string][]byte{}
	if err := getOverlayFilesImpl(overlay, extension, root, "", "", files); err != nil {
		return nil, err
	}
	return files, nil
}

func emitFile(fs afero.Fs, relPath string, contents []byte) error {
	if err := fs.MkdirAll(path.Dir(relPath), 0700); err != nil {
		return errors.Wrap(err, "creating directory")
	}

	f, err := fs.Create(relPath)
	if err != nil {
		return errors.Wrap(err, "creating file")
	}
	defer contract.IgnoreClose(f)

	_, err = f.Write(contents)
	return err
}

// getDescriptionFromParsedDocs extracts the argument description for the given arg, or the
// attribute description if there is none.
// If the description is taken from an attribute, the second return value is true.
func getDescriptionFromParsedDocs(entityDocs entityDocs, arg string) (string, bool) {
	return getNestedDescriptionFromParsedDocs(entityDocs, "", arg)
}

// getNestedDescriptionFromParsedDocs extracts the nested argument description for the given arg, or the
// top-level argument description or attribute description if there is none.
// If the description is taken from an attribute, the second return value is true.
func getNestedDescriptionFromParsedDocs(entityDocs entityDocs, objectName string, arg string) (string, bool) {
	if res := entityDocs.Arguments[objectName]; res != nil && res.arguments != nil && res.arguments[arg] != "" {
		return res.arguments[arg], false
	} else if res := entityDocs.Arguments[arg]; res != nil && res.description != "" {
		return res.description, false
	}

	attribute := entityDocs.Attributes[arg]

	if attribute != "" {
		// We return a description in the upstream attributes if none is found  in the upstream arguments. This condition
		// may be met for one of the following reasons:
		// 1. The upstream schema is incorrect and the item in question should not be an input (e.g. tags_all in AWS).
		// 2. The upstream schema is correct, but the docs are incorrect in that they have the item in question documented
		//    as an attribute, and this behavior is intentional (with the intent of being forgiving about mistakes in the
		//    upstream docs).
		//
		// (There may be other, unknown, reasons why this behavior exists.)
		//
		// In case #1 above, we are generating an incorrect schema because the upstream schema is incorrect, and we would
		// arguably be better off not having any description in our docs. In case #2 above, this is fairly risky fallback
		// behavior with may result in incorrect docs, per pulumi-terraform-bridge#550.
		//
		// We should work to minimize the number of times this fallback behavior is triggered (and possibly eliminate it
		// altogether) due to the difficulty in determining whether the correct description is actually found.
		return attribute, true
	}

	return "", false
}

// cleanDir removes all existing files from a directory except those in the exclusions list.
// Note: The exclusions currently don't function recursively, so you cannot exclude a single file
// in a subdirectory, only entire subdirectories. This function will need improvements to be able to
// target that use-case.
func cleanDir(fs afero.Fs, dirPath string, exclusions codegen.StringSet) error {
	if exclusions == nil {
		exclusions = codegen.NewStringSet()
	}
	subPaths, err := afero.ReadDir(fs, dirPath)
	if err != nil {
		return err
	}

	if len(subPaths) > 0 {
		for _, path := range subPaths {
			if !exclusions.Has(path.Name()) {
				err = fs.RemoveAll(filepath.Join(dirPath, path.Name()))
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func isTruthy(s string) bool {
	return s == "1" || strings.EqualFold(s, "true")
}

func ignoreMappingError(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
