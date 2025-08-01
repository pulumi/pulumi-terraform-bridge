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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hashicorp/go-multierror"
	pkgerrors "github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	schemaTools "github.com/pulumi/schema-tools/pkg"
	"github.com/spf13/afero"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/paths"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

const (
	tfgen         = "the Pulumi Terraform Bridge (tfgen) Tool"
	defaultOutDir = "sdk/"
	maxWidth      = 120 // the ideal maximum width of the generated file.
)

type Generator struct {
	pkg             tokens.Package        // the Pulumi package name (e.g. `gcp`)
	version         string                // the package version.
	language        Language              // the language runtime to generate.
	info            tfbridge.ProviderInfo // the provider info for customizing code generation
	root            afero.Fs              // the output virtual filesystem.
	providerShim    *inmemoryProvider     // a provider shim to hold the provider schema during example conversion.
	pluginHost      plugin.Host           // the plugin host for tf2pulumi.
	packageCache    *pcl.PackageCache     // the package cache for tf2pulumi.
	infoSource      il.ProviderInfoSource // the provider info source for tf2pulumi.
	sink            diag.Sink
	skipDocs        bool
	skipExamples    bool
	coverageTracker *CoverageTracker
	editRules       editRules

	convertedCode map[string][]byte

	// Set if we can't find the docs repo and we have already printed a warning
	// message.
	noDocsRepo bool

	cliConverterState *cliConverter

	examplesCache *examplesCache
}

type Language string

const (
	Golang Language = "go"
	NodeJS Language = "nodejs"
	Python Language = "python"
	CSharp Language = "dotnet"
	Schema Language = "schema"
	PCL    Language = "pulumi"
	// RegistryDocs
	// Setting RegistryDocs as a separate bridge "language" in the bridge allows us to create custom logic specific to
	// transforming and emitting upstream installation docs.
	// When we generate registry docs, we want to:
	//- be able to generate them via a separate command so we can enable it on a per-provider basis
	//- be able to pass a separate output location from the schema location (in this case, `docs/`)
	//- convert examples into all Pulumi-supported languages
	RegistryDocs Language = "registry-docs"
)

func (l Language) shouldConvertExamples() bool {
	switch l {
	case Golang, NodeJS, Python, CSharp, Schema, PCL:
		return true
	}
	return false
}

func dirToBytesMap(fs afero.Fs, dir string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	err := afero.Walk(fs, dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		content, err := afero.ReadFile(fs, path)
		if err != nil {
			return err
		}
		result[relPath] = content
		return nil
	})
	return result, err
}

func writeBytesMapToDir(fs afero.Fs, dir string, files map[string][]byte) error {
	err := fs.MkdirAll(dir, 0o755)
	if err != nil {
		return pkgerrors.Wrap(err, "failed to create dir")
	}
	for name, content := range files {
		srcDir := filepath.Dir(name)
		if srcDir != "." {
			err = fs.MkdirAll(filepath.Join(dir, srcDir), 0o755)
			if err != nil {
				return pkgerrors.Wrap(err, "failed to create dir")
			}
		}
		err := afero.WriteFile(fs, filepath.Join(dir, name), content, 0o600)
		if err != nil {
			return pkgerrors.Wrap(err, "failed to write file")
		}
	}
	return nil
}

func runPulumiPackageGenSDK(
	l Language, pkg *pschema.Package, extraFiles map[string][]byte,
) (map[string][]byte, error) {
	fs := afero.NewOsFs()
	outDir, err := afero.TempDir(fs, "", "pulumi-package-gen-sdk")
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to create temp dir")
	}

	args := []string{"package", "gen-sdk", "--language", string(l), "--out", outDir}

	if len(extraFiles) > 0 {
		overlayDir, err := afero.TempDir(fs, "", "pulumi-package-gen-sdk-overlays")
		if err != nil {
			return nil, pkgerrors.Wrap(err, "failed to create temp dir")
		}
		dest := filepath.Join(overlayDir, string(l))
		err = writeBytesMapToDir(fs, dest, extraFiles)
		if err != nil {
			return nil, pkgerrors.Wrap(err, "failed to write overlay files")
		}

		args = append(args, "--overlays", overlayDir)
	}

	schemaDir, err := afero.TempDir(fs, "", "schema")
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to create temp dir")
	}
	schemaFile := filepath.Join(schemaDir, "schema.json")
	schemaBytes, err := pkg.MarshalJSON()
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to marshal schema")
	}
	err = afero.WriteFile(fs, schemaFile, schemaBytes, 0o600)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to write schema")
	}

	args = append(args, schemaFile)

	cmd := exec.Command("pulumi", args...)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return nil, pkgerrors.New(cmd.String() + "\n" + string(out) + "\n" + stderr + "\n" + err.Error())
	}

	return dirToBytesMap(fs, filepath.Join(outDir, string(l)))
}

func (l Language) emitSDK(pkg *pschema.Package, info tfbridge.ProviderInfo, root afero.Fs,
) (map[string][]byte, error) {
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

		// codegen does not support overlays for go
		m, err := runPulumiPackageGenSDK(l, pkg, nil)
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
		return runPulumiPackageGenSDK(l, pkg, extraFiles)
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
		return runPulumiPackageGenSDK(l, pkg, extraFiles)
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
		return runPulumiPackageGenSDK(l, pkg, extraFiles)
	default:
		return nil, fmt.Errorf("%v does not support SDK generation", l)
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
		contract.Assertf(existing.Name() != name, "unexpected duplicate module member %q in %q",
			name, m.name.String())
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
	kindInvalid typeKind = iota
	kindBool
	kindInt
	kindFloat
	kindString
	kindList
	kindMap
	kindSet
	kindObject
)

// propertyType represents a non-resource, non-datasource type.
//
// Using nil for *propertyType implies catch-all any type (aka "pulumi.json#/Any" in Package Schema).
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

	typeName *string
}

func (g *Generator) Sink() diag.Sink {
	return g.sink
}

func (g *Generator) makePropertyType(typePath paths.TypePath,
	objectName string, sch shim.Schema, info *tfbridge.SchemaInfo, out bool,
	entityDocs entityDocs,
) (*propertyType, error) {
	t := &propertyType{}
	if info != nil {
		t.typeName = info.TypeName
	}

	var elemInfo *tfbridge.SchemaInfo
	if info != nil {
		t.typ = info.Type
		t.nestedType = info.NestedType
		t.altTypes = info.AltTypes
		t.asset = info.Asset
		elemInfo = info.Elem
	}

	if sch == nil {
		contract.Assertf(info != nil, "missing info when sch is nil on type: %s", typePath.String())
		return t, nil
	}

	// We should carry across any of the deprecation messages, to Pulumi, as per Terraform schema
	if sch.Deprecated() != "" && elemInfo != nil {
		elemInfo.DeprecationMessage = sch.Deprecated()
	}

	// Perform case analysis on Elem() and Type(). See shim.Schema.Elem() doc for reference. Start with scalars.
	switch sch.Type() {
	case shim.TypeBool:
		t.kind = kindBool
		return t, nil
	case shim.TypeInt:
		t.kind = kindInt
		return t, nil
	case shim.TypeFloat:
		t.kind = kindFloat
		return t, nil
	case shim.TypeString:
		t.kind = kindString
		return t, nil
	}

	// Handle single-nested blocks next.
	if blockType, ok := sch.Elem().(shim.Resource); ok && sch.Type() == shim.TypeMap {
		return g.makeObjectPropertyType(typePath, blockType, elemInfo, out, entityDocs)
	}

	// IsMaxItemOne lists and sets are flattened, transforming List[T] or Set[T] to T. Detect if this is the case.
	flatten := tfbridge.IsMaxItemsOne(sch, info)

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
		var err error
		element, err = g.makePropertyType(elemPath, objectName, elem, elemInfo, out, entityDocs)
		if err != nil {
			return nil, err
		}
	case shim.Resource:
		var err error
		element, err = g.makeObjectPropertyType(elemPath, elem, elemInfo, out, entityDocs)
		if err != nil {
			return nil, err
		}
	}

	if flatten {
		return element, nil
	}

	switch sch.Type() {
	case shim.TypeMap:
		t.kind = kindMap
		// TF treats maps without a specified type as map[string]string, so we do the same.
		if element == nil || element.kind == kindInvalid {
			element = &propertyType{kind: kindString}
		}
	case shim.TypeList:
		t.kind = kindList
	case shim.TypeSet:
		t.kind = kindSet
	case shim.TypeDynamic:
		return nil, nil // nil of type *propertyType represents the special <any> type
	default:
		contract.Failf(
			"impossible: sch.Type() should be one of TypeMap, TypeList, TypeSet, TypeDynamic at this point path: "+
				"%s, type: %s",
			typePath.String(), sch.Type())
	}
	t.element = element
	return t, nil
}

func getDocsFromSchemaMap(key string, schemaMap shim.SchemaMap) string {
	subSchema := schemaMap.Get(key)
	return subSchema.Description()
}

func (g *Generator) makeObjectPropertyType(typePath paths.TypePath,
	res shim.Resource, info *tfbridge.SchemaInfo,
	out bool, entityDocs entityDocs,
) (*propertyType, error) {
	// If the user supplied an explicit Type token override, omit generating types and short-circuit.
	if info != nil && info.OmitType {
		if info.Type == "" {
			return nil, fmt.Errorf("Cannot set info.OmitType without also setting info.Type")
		}
		return &propertyType{typ: info.Type}, nil
	}

	t := &propertyType{
		kind: kindObject,
	}

	if info != nil {
		t.typeName = info.TypeName
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

	// Look up the parent path and prepend it to the docs path, to allow for precise lookup in the entityDocs.
	fullDocsPath := ""
	currentPath := typePath
	for {
		if p, ok := currentPath.(*paths.PropertyPath); ok {
			fullDocsPath = p.PropertyName.Key + "." + fullDocsPath
		}
		if currentPath.Parent() != nil {
			currentPath = currentPath.Parent()
		} else {
			break
		}

		fullDocsPath = strings.TrimSuffix(fullDocsPath, ".")
	}

	objPath := docsPath(fullDocsPath)

	for _, key := range stableSchemas(res.Schema()) {
		propertySchema := res.Schema()

		// TODO: Figure out why counting whether this description came from the attributes seems wrong.
		// With AWS, counting this takes the takes number of arg descriptions from attribs from about 170 to about 1400.
		// This seems wrong, so we ignore the second return value here for now.
		doc, _ := getNestedDescriptionFromParsedDocs(entityDocs, objPath.join(key))

		// If we have no result from entityDocs, we look up the TF schema Description.
		if doc == "" {
			doc = getDocsFromSchemaMap(key, propertySchema)
			// Since these docs have not been parsed via entityDocs, they still need to be reformatted.
			docsInfoCtx := infoContext{
				language: g.language,
				pkg:      g.pkg,
				info:     g.info,
			}
			// Description fields have no footers, so we pass in an empty map
			fakeFooterLinks := map[string]string{}
			doc, _ = reformatText(docsInfoCtx, doc, fakeFooterLinks)
		}
		// If we still have no docs for this type, we use our final strategy to look up any docs
		// that are parsed from entity (markdown) docs and have a unique path leaf.
		if doc == "" {
			doc = getUniqueLeafDocsDescriptions(entityDocs.Arguments, objPath.join(key))
		}

		v, err := g.propertyVariable(typePath, key,
			propertySchema, propertyInfos, doc, "", out, entityDocs)
		if err != nil {
			return nil, err
		}
		if v != nil {
			t.properties = append(t.properties, v)
		}
	}

	return t, nil
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
	switch {
	case t.typeName != nil && other.typeName == nil:
		return false
	case t.typeName == nil && other.typeName != nil:
		return false
	case t.typeName != nil && other.typeName != nil &&
		*t.typeName != *other.typeName:
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
	if v.info != nil && v.info.DeprecationMessage != "" {
		return v.info.DeprecationMessage
	}

	if v.schema != nil && v.schema.Deprecated() != "" {
		return v.schema.Deprecated()
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
func (v *variable) optional() bool { return v.opt || isOptional(v.info, v.schema, v.out, v.config) }

func isOptional(info *tfbridge.SchemaInfo, schema shim.Schema, out bool, config bool) bool {
	// if we have an explicit marked as optional then let's return that
	if info != nil && info.MarkAsOptional != nil {
		return *info.MarkAsOptional
	}

	// If we're checking a property used in an output position, it isn't optional if it's computed.
	//
	// Note that config values with custom defaults are _not_ considered optional unless they are marked as such.
	customDefault := !config && info != nil && info.HasDefault()
	if out {
		return schema != nil && schema.Optional() && !schema.Computed() && !customDefault
	}
	return (schema != nil && schema.Optional() || schema.Computed()) || customDefault
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
	isProvider bool,
) *resourceType {
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
	XInMemoryDocs   bool
}

type GenerateSchemaResult struct {
	PackageSpec pschema.PackageSpec
}

func GenerateSchemaWithOptions(opts GenerateSchemaOptions) (*GenerateSchemaResult, error) {
	ctx := context.Background()
	info := opts.ProviderInfo
	sink := opts.DiagnosticsSink
	g, err := NewGenerator(GeneratorOptions{
		Package:       info.Name,
		Version:       info.Version,
		Language:      Schema,
		ProviderInfo:  info,
		Root:          afero.NewMemMapFs(),
		Sink:          sink,
		XInMemoryDocs: opts.XInMemoryDocs,
	})
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to create generator")
	}

	return g.generateSchemaResult(ctx)
}

type GeneratorOptions struct {
	Package            string
	Version            string
	Language           Language
	ProviderInfo       tfbridge.ProviderInfo
	Root               afero.Fs
	ProviderInfoSource il.ProviderInfoSource
	PluginHost         plugin.Host
	Sink               diag.Sink
	Debug              bool
	SkipDocs           bool
	SkipExamples       bool
	CoverageTracker    *CoverageTracker
	// XInMemoryDocs instructs the generator not to attempt to find a repository to
	// draw docs from, relying only on TF schema level docs.
	//
	// XInMemoryDocs is an experimental feature, and does not have any backwards
	// compatibility guarantees.
	XInMemoryDocs bool
}

// NewGenerator returns a code-generator for the given language runtime and package info.
func NewGenerator(opts GeneratorOptions) (*Generator, error) {
	pkgName, version, lang, info, root := opts.Package, opts.Version, opts.Language, opts.ProviderInfo, opts.Root
	pkg := tokens.NewPackageToken(tokens.PackageName(tokens.IntoQName(pkgName)))

	// Ensure the language is valid.
	switch lang {
	case Golang, NodeJS, Python, CSharp, Schema, PCL, RegistryDocs:
		// OK
	default:
		return nil, fmt.Errorf("unrecognized language runtime: %s", lang)
	}

	// If root is nil, default to sdk/<language>/ in the pwd.
	if root == nil {
		p, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		p = filepath.Join(p, defaultOutDir, string(lang))
		if err = os.MkdirAll(p, 0o700); err != nil {
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

		ctx := context.Background()
		pluginContext, err := plugin.NewContext(ctx, sink, sink, nil, nil, cwd, nil, false, nil)
		if err != nil {
			return nil, err
		}
		pluginHost = pluginContext.Host
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
		pkg:             pkg,
		version:         version,
		language:        lang,
		info:            info,
		root:            root,
		providerShim:    providerShim,
		pluginHost:      newCachingProviderHost(host),
		packageCache:    pcl.NewPackageCache(),
		infoSource:      host,
		sink:            sink,
		skipDocs:        opts.SkipDocs,
		skipExamples:    opts.SkipExamples,
		coverageTracker: opts.CoverageTracker,
		editRules:       getEditRules(info.DocRules),
		noDocsRepo:      opts.XInMemoryDocs,
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
func (g *Generator) Generate() (*GenerateSchemaResult, error) {
	genSchemaResult, err := g.generateSchemaResult(context.Background())
	if err != nil {
		return nil, err
	}

	// Now push the schema through the rest of the generator.
	return g.UnstableGenerateFromSchema(genSchemaResult)
}

func (g *Generator) generateSchemaResult(ctx context.Context) (*GenerateSchemaResult, error) {
	err := g.info.Validate(ctx)
	if err != nil {
		return nil, err
	}
	// First gather up the entire package contents. This structure is complete and sufficient to hand off to the
	// language-specific generators to create the full output.
	pack, err := g.gatherPackage()
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to gather package metadata")
	}

	// Convert the package to a Pulumi schema.
	pulumiPackageSpec, err := genPulumiSchema(pack, g.pkg, g.version, g.info, g.sink)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to create Pulumi schema")
	}
	// Apply schema post-processing if defined in the provider.
	if g.info.SchemaPostProcessor != nil {
		g.info.SchemaPostProcessor(&pulumiPackageSpec)
	}

	return &GenerateSchemaResult{PackageSpec: pulumiPackageSpec}, nil
}

// GenerateFromSchema creates Pulumi packages from a pulumi schema and the information the
// generator was initialized with.
//
// This is an unstable API. We have exposed it so other packages within
// pulumi-terraform-bridge can consume it. We do not recommend other packages consume this
// API.
func (g *Generator) UnstableGenerateFromSchema(genSchemaResult *GenerateSchemaResult) (*GenerateSchemaResult, error) {
	pulumiPackageSpec := genSchemaResult.PackageSpec
	schemaStats = schemaTools.CountStats(pulumiPackageSpec)

	// Serialize the schema and attach it to the provider shim.
	var err error
	g.providerShim.schema, err = json.Marshal(pulumiPackageSpec)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to marshal intermediate schema")
	}

	// Add any supplemental examples:
	err = addExtraHclExamplesToResources(g.info.ExtraResourceHclExamples, &pulumiPackageSpec)
	if err != nil {
		return nil, err
	}

	err = addExtraHclExamplesToFunctions(g.info.ExtraFunctionHclExamples, &pulumiPackageSpec)
	if err != nil {
		return nil, err
	}
	// Convert examples.
	if !g.skipExamples {
		pulumiPackageSpec = g.convertExamplesInSchema(pulumiPackageSpec)
	}

	// Go ahead and let the language generator do its thing. If we're emitting the schema, just go ahead and serialize
	// it out.
	files := make(map[string][]byte)

	switch g.language {
	case RegistryDocs:
		source := NewGitRepoDocsSource(g)
		installationFile, err := source.getInstallation(nil)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to obtain an index.md file for this provider")
		}
		content, err := plainDocsParser(installationFile, g)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to parse installation docs")
		}
		files["_index.md"] = content
	case Schema:
		// Omit the version so that the spec is stable if the version is e.g. derived from the current Git commit hash.
		pulumiPackageSpec.Version = ""

		bytes, err := json.MarshalIndent(pulumiPackageSpec, "", "    ")
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to marshal schema")
		}
		files = map[string][]byte{"schema.json": bytes}

		if info := g.info.MetadataInfo; info != nil {
			files[info.Path] = (*metadata.Data)(info.Data).MarshalIndent()
			if g.info.GenerateRuntimeMetadata {
				runtimeInfo := tfbridge.ExtractRuntimeMetadata(info)
				files[runtimeInfo.Path] = (*metadata.Data)(runtimeInfo.Data).Marshal()
			}
		}
	case PCL:
		if g.skipExamples {
			return nil, fmt.Errorf("Cannot set skipExamples and get PCL")
		}
		files = map[string][]byte{}
		for path, code := range g.convertedCode {
			path = strings.TrimPrefix(path, "#/") + ".pp"
			files[path] = code
		}
	default:
		allowDanglingRefernces := true
		if g.info.NoDanglingReferences {
			allowDanglingRefernces = false
		}
		pulumiPackage, diags, err := pschema.BindSpec(
			pulumiPackageSpec, nil, pschema.ValidationOptions{
				AllowDanglingReferences: allowDanglingRefernces,
			},
		)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to import Pulumi schema")
		}
		if diags.HasErrors() {
			return nil, err
		}
		if files, err = g.language.emitSDK(pulumiPackage, g.info, g.root); err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to generate package")
		}
	}

	// Write the result to disk. Do not overwrite the root-level README.md if any
	// exists.
	emit := emitFile
	// For the Schema language in particular, we only write files if the write
	// would cause a change. This allows build systems like Make to correctly
	// include bridge-metadata.json and schema.json as build dependencies
	// without always trying to rebuild.
	//
	// We don't do this for other languages because reading every file before writing
	// it is expensive.
	if g.language == Schema {
		emit = emitFileIfChanged
	}
	for f, contents := range files {
		if f == "README.md" {
			if _, err := g.root.Stat(f); err == nil {
				continue
			}
		}
		if err := emit(g.root, f, contents); err != nil {
			return nil, pkgerrors.Wrapf(err, "emitting file %v", f)
		}
	}

	// Emit the Pulumi project information.
	if g.language != RegistryDocs {
		if err = g.emitProjectMetadata(g.pkg, g.language); err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to create project file")
		}
	}

	// Close the plugin host.
	g.pluginHost.Close()

	return &GenerateSchemaResult{PackageSpec: pulumiPackageSpec}, nil
}

// gatherPackage creates a package plus module structure for the entire set of members of this package.
func (g *Generator) gatherPackage() (*pkg, error) {
	// First, gather up the entire package/module structure.  This includes gathering config entries, resources,
	// data sources, and any supporting type information, and placing them into modules.
	pack := newPkg(g.pkg, g.version, g.language, g.root)

	// Place all configuration variables into a single config module.
	cfg, err := g.gatherConfig()
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		pack.addModule(cfg)
	}

	// Gather the provider type for this package.
	provider, err := g.gatherProvider()
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "problem gathering the provider type")
	}
	pack.provider = provider

	// Gather up all resource modules and merge them into the current set.
	resmods, err := g.gatherResources()
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "problem gathering resources")
	} else if resmods != nil {
		pack.addModuleMap(resmods)
	}

	// Gather up all data sources into their respective modules and merge them in.
	dsmods, err := g.gatherDataSources()
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "problem gathering data sources")
	} else if dsmods != nil {
		pack.addModuleMap(dsmods)
	}

	// Now go ahead and merge in any overlays into the modules if there are any.
	olaymods, err := g.gatherOverlays()
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "problem gathering overlays")
	} else if olaymods != nil {
		pack.addModuleMap(olaymods)
	}

	return pack, nil
}

// gatherConfig returns the configuration module for this package.
func (g *Generator) gatherConfig() (*module, error) {
	// If there's no config, skip creating the module.
	cfg := g.provider().Schema()
	if cfg.Len() == 0 {
		return nil, nil
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
		// Reformat the upstream Description if necessary
		docsInfoCtx := infoContext{
			language: g.language,
			pkg:      g.pkg,
			info:     g.info,
		}
		fakeFooterLinks := map[string]string{}
		rawdoc, _ := reformatText(docsInfoCtx, sch.Description(), fakeFooterLinks)
		prop, err := g.propertyVariable(cfgPath,
			key, cfg, custom, "", rawdoc, true /*out*/, entityDocs{})
		if err != nil {
			return nil, err
		}

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
		extraConfigMap[key] = val.Schema
	}
	for key := range g.info.ExtraConfig {
		prop, err := g.propertyVariable(cfgPath,
			key, extraConfigMap, extraConfigInfo, "", "", true /*out*/, entityDocs{})
		if err != nil {
			return nil, err
		}
		if prop != nil {
			prop.config = true
			config.addMember(prop)
		}
	}
	return config, nil
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

	skipFailBuildOnMissingMapError := cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_MISSING_MAPPING_ERROR")) ||
		cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_PROVIDER_MAP_ERROR"))
	skipFailBuildOnExtraMapError := cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_EXTRA_MAPPING_ERROR"))

	// let's keep a list of TF mapping errors that we can present to the user
	var resourceMappingErrors error

	// For each resource, create its own dedicated type and module export.
	var reserr error
	seen := make(map[string]bool)
	for _, r := range stableResources(resources) {
		info := g.info.Resources[r]
		if info == nil {
			if sliceContains(g.info.IgnoreMappings, r) {
				g.debug("TF resource %q not found in provider map", r)
				continue
			}

			if !sliceContains(g.info.IgnoreMappings, r) && !skipFailBuildOnMissingMapError {
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
			reserr = multierror.Append(reserr, fmt.Errorf("%s: %w", r, err))
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
						"resource found. Remove the mapping and try again",
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
	schema shim.Resource, info *tfbridge.ResourceInfo, isProvider bool,
) (*resourceType, error) {
	// Get the resource's module and name.
	name, moduleName := resourceName(g.info.Name, rawname, info, isProvider)
	mod := tokens.NewModuleToken(g.pkg, moduleName)

	resourceToken := tokens.NewTypeToken(mod, name)
	resourcePath := paths.NewResourcePath(rawname, resourceToken, isProvider)

	// Collect documentation information
	var entityDocs entityDocs
	if !isProvider {
		// If g.noDocsRepo is set, we have established that it's pointless to get
		// docs from the repo, so we don't try.
		if !g.noDocsRepo {
			source := NewGitRepoDocsSource(g)
			pulumiDocs, err := getDocsForResource(g, source, ResourceDocs, rawname, info)
			if err == nil {
				entityDocs = pulumiDocs
			} else if !g.checkNoDocsError(err) {
				return nil, err
			}
		}
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
		rawdoc, elided := reformatText(infoContext{
			language: g.language,
			pkg:      g.pkg,
			info:     g.info,
		}, propschema.Description(), nil)
		if elided {
			rawdoc = ""
		}

		propinfo := info.Fields[key]
		// If we are generating a provider, we do not emit output property definitions as provider outputs are not
		// yet implemented.
		if !isProvider {
			// For all properties, generate the output property metadata. Note that this may differ slightly
			// from the input in that the types may differ.
			outprop, err := g.propertyVariable(resourcePath.Outputs(), key, schema.Schema(),
				info.Fields, doc, rawdoc, true /*out*/, entityDocs)
			if err != nil {
				return nil, err
			}
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

			inprop, err := g.propertyVariable(resourcePath.Inputs(),
				key, schema.Schema(), info.Fields, doc, rawdoc, false /*out*/, entityDocs)
			if err != nil {
				return nil, err
			}
			if inprop != nil {
				res.inprops = append(res.inprops, inprop)
				if !inprop.optional() {
					res.reqprops[name.String()] = true
				}
			}
		}

		// Make a state variable.  This is always optional and simply lets callers perform lookups.
		stateVar, err := g.propertyVariable(resourcePath.State(), key, schema.Schema(), info.Fields,
			doc, rawdoc, false /*out*/, entityDocs)
		if err != nil {
			return nil, err
		}
		if stateVar != nil {
			stateVar.opt = true
			stateVars = append(stateVars, stateVar)
		}
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

	return res, nil
}

func (g *Generator) gatherDataSources() (moduleMap, error) {
	// If there aren't any data sources, skip this altogether.
	sources := g.provider().DataSourcesMap()
	if sources.Len() == 0 {
		return nil, nil
	}
	modules := make(moduleMap)

	skipFailBuildOnMissingMapError := cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_MISSING_MAPPING_ERROR")) ||
		cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_PROVIDER_MAP_ERROR"))
	skipFailBuildOnExtraMapError := cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_EXTRA_MAPPING_ERROR"))

	// let's keep a list of TF mapping errors that we can present to the user
	var dataSourceMappingErrors error

	// For each data source, create its own dedicated function and module export.
	var dserr error
	seen := make(map[string]bool)
	for _, ds := range stableResources(sources) {
		dsinfo := g.info.DataSources[ds]
		if dsinfo == nil {
			if sliceContains(g.info.IgnoreMappings, ds) {
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
			if !skipFailBuildOnExtraMapError {
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
	ds shim.Resource, info *tfbridge.DataSourceInfo,
) (*resourceFunc, error) {
	// Generate the name and module for this data source.
	name, moduleName := dataSourceName(g.info.Name, rawname, info)
	mod := tokens.NewModuleToken(g.pkg, moduleName)
	dataSourcePath := paths.NewDataSourcePath(rawname, tokens.NewModuleMemberToken(mod, name))

	// Collect documentation information for this data source.
	source := NewGitRepoDocsSource(g)
	entityDocs, err := getDocsForResource(g, source, DataSourceDocs, rawname, info)
	if err != nil && !g.checkNoDocsError(err) {
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

			argvar, err := g.propertyVariable(dataSourcePath.Args(),
				arg, ds.Schema(), info.Fields, doc, "", false /*out*/, entityDocs)
			if err != nil {
				return nil, err
			}
			if argvar != nil {
				fun.args = append(fun.args, argvar)
				if !argvar.optional() {
					fun.reqargs[argvar.name] = true
				}
			}
		}

		// Also remember properties for the resulting return data structure.
		// Emit documentation for the property if available
		p, err := g.propertyVariable(dataSourcePath.Results(), arg, ds.Schema(), info.Fields,
			entityDocs.Attributes[arg], "", true /*out*/, entityDocs)
		if err != nil {
			return nil, err
		}
		if p != nil {
			fun.rets = append(fun.rets, p)
		}
	}

	// If the data source's schema doesn't expose an id property, make one up since we'd like to expose it for data
	// sources.
	if id, has := ds.Schema().GetOk("id"); !has || id.Removed() != "" {
		cust := map[string]*tfbridge.SchemaInfo{"id": {}}
		rawdoc := "The provider-assigned unique ID for this managed resource."
		idSchema := schema.SchemaMap(map[string]shim.Schema{
			"id": (&schema.Schema{Type: shim.TypeString, Computed: true}).Shim(),
		})
		p, err := g.propertyVariable(dataSourcePath.Results(), "id", idSchema, cust, "",
			rawdoc, true /*out*/, entityDocs)
		if err != nil {
			return nil, err
		}
		if p != nil {
			fun.rets = append(fun.rets, p)
		}
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
	case Schema, PCL, RegistryDocs:
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
					fmt.Errorf("overlay module %s is >1 level deep, which is not supported", name)
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

func (g *Generator) checkNoDocsError(err error) bool {
	var e GetRepoPathErr
	if !errors.As(err, &e) {
		// Not the right kind of error
		return false
	}

	// If we have already warned, we can just discard the message
	if !g.noDocsRepo {
		g.logMissingRepoPath(e)
	}
	g.noDocsRepo = true
	return true
}

func (g *Generator) logMissingRepoPath(err GetRepoPathErr) {
	msg := `Unable to find the upstream provider's documentation:
The upstream repository is expected to be at %q.
%s
The original error is: %s`

	var correction string
	if g.info.UpstreamRepoPath != "" {
		correction = fmt.Sprintf(`
The upstream repository path has been overridden, but the specified path is invalid.
You should check the value of:
tfbridge.ProviderInfo{
	UpstreamRepoPath: %q,
}`, g.info.UpstreamRepoPath)
	} else {
		correction = fmt.Sprintf(`
If the expected path is not correct, you should check the values of these fields (current values shown):
tfbridge.ProviderInfo{
	GitHubHost:              %q,
	GitHubOrg:               %q,
	Name:                    %q,
	TFProviderModuleVersion: %q,
}`, g.info.GetGitHubHost(), g.info.GetGitHubOrg(), g.info.Name, g.info.GetProviderModuleVersion())
	}

	g.sink.Warningf(&diag.Diag{Message: msg}, err.Expected, correction, err.Underlying)
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
		(info == nil || info.MarkAsComputedOnly == nil || !*info.MarkAsComputedOnly)
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
	schemaMap shim.SchemaMap, info map[string]*tfbridge.SchemaInfo,
	doc string, rawdoc string, out bool, entityDocs entityDocs,
) (*variable, error) {
	if name := propertyName(key, schemaMap, info); name != "" {
		propName := paths.PropertyName{Key: key, Name: tokens.Name(name)}
		typePath := paths.NewProperyPath(parentPath, propName)

		var shimSchema shim.Schema
		if schemaMap != nil {
			shimSchema = schemaMap.Get(key)
		}
		// Suppress write-only attributes via SchemaInfo.Omit
		// TODO[pulumi/pulumi-terraform-bridge#2938] remove when the bridge fully supports write-only fields.

		if shimSchema.WriteOnly() {
			if info == nil {
				info = make(map[string]*tfbridge.SchemaInfo)
			}
			if val, ok := info[key]; ok {
				val.Omit = true
			} else {
				info[key] = &tfbridge.SchemaInfo{
					Omit: true,
				}
			}
		}

		var varInfo *tfbridge.SchemaInfo
		if info != nil {
			varInfo = info[key]
		}

		// If a variable is marked as omitted, omit it.
		//
		// Because the recursive traversal into the fields used by this type are
		// from g.makePropertyType below, this has the effect of omitting all
		// types generated by the omitted type, which is what we want.
		if varInfo != nil && varInfo.Omit {
			if !(isOptional(varInfo, shimSchema, false, false /* config */)) {
				err := fmt.Errorf("required property %q (@ %s) may not be omitted from binding generation",
					propName, typePath)
				return nil, err
			}
			return nil, nil
		}
		typ, err := g.makePropertyType(typePath, strings.ToLower(key), shimSchema, varInfo, out, entityDocs)
		if err != nil {
			return nil, err
		}

		return &variable{
			name:         name,
			out:          out,
			doc:          doc,
			rawdoc:       rawdoc,
			schema:       shimSchema,
			info:         varInfo,
			typ:          typ,
			parentPath:   parentPath,
			propertyName: propName,
		}, nil
	}
	return nil, nil
}

// dataSourceName translates a Terraform name into its Pulumi name equivalent.
func dataSourceName(provider string, rawname string,
	info *tfbridge.DataSourceInfo,
) (tokens.ModuleMemberName, tokens.ModuleName) {
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
	info *tfbridge.ResourceInfo, isProvider bool,
) (tokens.TypeName, tokens.ModuleName) {
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
	fs afero.Fs, srcRoot, dir string, files map[string][]byte,
) error {
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

func emitFileIfChanged(vfs afero.Fs, relPath string, contents []byte) error {
	existing, err := vfs.Open(relPath)
	if errors.Is(err, fs.ErrNotExist) {
		return emitFile(vfs, relPath, contents)
	} else if err != nil {
		// We return if there was an un-expected error.
		return pkgerrors.Wrapf(err, "unable to detect if %q exists", relPath)
	}
	defer existing.Close()
	existingBytes, err := io.ReadAll(existing)
	if err != nil {
		return pkgerrors.Wrapf(err, "unable to read %q", relPath)
	}
	if bytes.Equal(existingBytes, contents) {
		return nil // No action needed
	}
	return emitFile(vfs, relPath, contents)
}

func emitFile(fs afero.Fs, relPath string, contents []byte) error {
	if err := fs.MkdirAll(path.Dir(relPath), 0o700); err != nil {
		return pkgerrors.Wrap(err, "creating directory")
	}

	f, err := fs.Create(relPath)
	if err != nil {
		return pkgerrors.Wrap(err, "creating file")
	}
	defer contract.IgnoreClose(f)

	_, err = f.Write(contents)
	return err
}

// getUniqueDocsDescriptions looks for any leaf path arguments and checks if the leaf key is unique in the argument
// docs map. If it is a unique leaf path, the function returns that argument doc's Description, else it returns "".
func getUniqueLeafDocsDescriptions(arguments map[docsPath]*argumentDocs, path docsPath) string {
	leaf := path.leaf()

	var leafDoc *argumentDocs
	// Counter for leaf fields with the same key
	occurrences := 0
	for argKey, argDoc := range arguments {
		if argKey.leaf() == leaf {
			// we found a leaf doc. It may or may not be unique.
			leafDoc = argDoc
			occurrences++
		}
		if occurrences > 1 {
			// if we have more than one key match to the leaf name, the key is not unique. Return "".
			return ""
		}
	}
	if leafDoc == nil {
		return ""
	}
	return leafDoc.description
}

// getDescriptionFromParsedDocs extracts the argument description for the given arg, or the
// attribute description if there is none.
// If the description is taken from an attribute, the second return value is true.
func getDescriptionFromParsedDocs(entityDocs entityDocs, arg string) (string, bool) {
	return getNestedDescriptionFromParsedDocs(entityDocs, docsPath(arg))
}

// getNestedDescriptionFromParsedDocs extracts the nested argument description for the given arg, or the
// top-level argument description or attribute description if there is none.
// If the description is taken from an attribute, the second return value is true.
func getNestedDescriptionFromParsedDocs(entityDocs entityDocs, path docsPath) (string, bool) {
	// Check if we have any path that matches, removing the root segment after each failed attempt.
	//
	// For example: ruleset.rules.type will check against:
	//
	// 1. ruleset.rules.type
	// 2. rules.type
	// 3. type

	for p := path; p != ""; {
		// See if we have an appropriately nested argument:
		v, ok := entityDocs.Arguments[p]
		if ok {
			return v.description, false
		}
		p = p.withOutRoot()
	}

	for attrPath := path; attrPath != ""; {
		// We return a description in the upstream attributes if none is found  in the upstream arguments. This condition
		// may be met for one of the following reasons:
		// 1. The upstream schema is incorrect and the item in question should not be an input (e.g. tags_all in AWS).
		// 2. The upstream schema is correct, but the docs are incorrect in that they have the item in question documented
		//    as an attribute, and this behavior is intentional (with the intent of being forgiving about mistakes in the
		//    upstream docs).
		//
		// (There may be other, unknown, reasons why this behavior exists.)
		//
		// Additionally, a lot of nested type descriptions are listed under the "Attributes Reference" section and as such
		// will have been parsed into entityDocs.Attributes. In the AWS provider, this behavior is responsible for many
		// type property descriptions.
		//
		// In case #1 above, we are generating an incorrect schema because the upstream schema is incorrect, and we would
		// arguably be better off not having any description in our docs. In case #2 above, this is fairly risky fallback
		// behavior with may result in incorrect docs, per pulumi-terraform-bridge#550.
		//
		// We should work to minimize the number of times this fallback behavior is triggered (and possibly eliminate it
		// altogether) due to the difficulty in determining whether the correct description is actually found.
		if description, ok := entityDocs.Attributes[string(attrPath)]; ok {
			return description, true
		}
		attrPath = attrPath.withOutRoot()
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

func sliceContains[T comparable](slice []T, target T) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}
