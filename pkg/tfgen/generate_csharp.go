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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
// nolint: goconst
package tfgen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/tools"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfbridge"
)

// newCSharpGenerator returns a language generator that understands how to produce .NET packages.
func newCSharpGenerator(pkg, version string, info tfbridge.ProviderInfo, overlaysDir, outDir string) langGenerator {
	return &csharpGenerator{
		pkg:         pkg,
		version:     version,
		info:        info,
		overlaysDir: overlaysDir,
		outDir:      outDir,
	}
}

type csharpGenerator struct {
	pkg         string
	version     string
	info        tfbridge.ProviderInfo
	overlaysDir string
	outDir      string
}

type declarer interface {
	Name() string
}

type csharpNestedType struct {
	declarer declarer
	isInput  bool
	isPlain  bool
	typ      *propertyType
}

type csharpNestedTypes struct {
	nameToType  map[string]*csharpNestedType
	perDeclarer map[declarer][]*csharpNestedType
}

func gatherNestedTypesForModule(mod *module) *csharpNestedTypes {
	nt := &csharpNestedTypes{
		nameToType:  make(map[string]*csharpNestedType),
		perDeclarer: make(map[declarer][]*csharpNestedType),
	}

	for _, member := range mod.members {
		switch member := member.(type) {
		case *resourceType:
			nt.gatherFromProperties(member, member.name, "Args", member.inprops, true, false)
			nt.gatherFromProperties(member, member.name, "", member.outprops, false, false)
			if !member.IsProvider() {
				nt.gatherFromProperties(member, member.name, "GetArgs", member.statet.properties, true, false)
			}
		case *resourceFunc:
			name := strings.Title(member.name)
			nt.gatherFromProperties(member, name, "Args", member.args, true, true)
			nt.gatherFromProperties(member, name, "Result", member.rets, false, false)
		case *variable:
			typ, name := member.typ, csharpPropertyName(member)
			nt.gatherFromPropertyType(member, name, "", "", typ, false, true)
		}
	}

	return nt
}

func (nt *csharpNestedTypes) getDeclaredTypes(declarer declarer) []*csharpNestedType {
	return nt.perDeclarer[declarer]
}

func (nt *csharpNestedTypes) declareType(
	declarer declarer, namePrefix, name, nameSuffix string, typ *propertyType, isInput, isPlain bool) string {

	// Generate a name for this nested type.
	baseName := namePrefix + strings.Title(name)

	// Override the nested type name, if necessary.
	if typ.nestedType.Name().String() != "" {
		baseName = typ.nestedType.Name().String()
	}

	typeName := baseName + strings.Title(nameSuffix)
	typ.name = typeName

	if existing := nt.nameToType[typeName]; existing != nil {
		contract.Failf("Nested type %q declared by %s was already declared by %s",
			typeName, existing.declarer.Name(), declarer.Name())
	}

	t := &csharpNestedType{
		declarer: declarer,
		isInput:  isInput,
		isPlain:  isPlain,
		typ:      typ,
	}

	nt.nameToType[typeName] = t
	nt.perDeclarer[declarer] = append(nt.perDeclarer[declarer], t)

	return baseName
}

func (nt *csharpNestedTypes) gatherFromProperties(
	declarer declarer, namePrefix, nameSuffix string, ps []*variable, isInput, isPlain bool) {

	for _, p := range ps {
		nt.gatherFromPropertyType(declarer, namePrefix, p.name, nameSuffix, p.typ, isInput, isPlain)
	}
}

func (nt *csharpNestedTypes) gatherFromPropertyType(
	declarer declarer, namePrefix, name, nameSuffix string, typ *propertyType, isInput, isPlain bool) {

	switch typ.kind {
	case kindList, kindSet, kindMap:
		if typ.element != nil {
			nt.gatherFromPropertyType(declarer, namePrefix, name, nameSuffix, typ.element, isInput, isPlain)
		}
	case kindObject:
		baseName := nt.declareType(declarer, namePrefix, name, nameSuffix, typ, isInput, isPlain)
		nt.gatherFromProperties(declarer, baseName, nameSuffix, typ.properties, isInput, isPlain)
	}
}

// commentChars returns the comment characters to use for single-line comments.
func (g *csharpGenerator) commentChars() string {
	return "//"
}

// moduleDir returns the directory for the given module.
func (g *csharpGenerator) moduleDir(mod *module) string {
	dir := g.outDir
	if mod.name != "" {
		dir = filepath.Join(dir, strings.Title(mod.name))
	}
	return dir
}

// assemblyName returns the assembly name for the package.
func (g *csharpGenerator) assemblyName() string {
	return "Pulumi." + strings.Title(g.pkg)
}

// moduleNamespace returns the C# namespace for the given module.
func (g *csharpGenerator) moduleNamespace(mod *module) string {
	pkgNamespace := g.assemblyName()
	if mod.root() {
		return pkgNamespace
	}
	return pkgNamespace + "." + strings.Title(mod.name)
}

// openWriter opens a writer for the given module and file name, emitting the standard header automatically.
func (g *csharpGenerator) openWriter(mod *module, name string) (
	*tools.GenWriter, string, error) {

	dir := g.moduleDir(mod)
	file := filepath.Join(dir, name)
	w, err := tools.NewGenWriter(tfgen, file)
	if err != nil {
		return nil, "", err
	}

	// Emit a standard warning header ("do not edit", etc).
	w.EmitHeaderWarning(g.commentChars())

	return w, file, nil
}

// emitPackage emits an entire package pack into the configured output directory with the configured settings.
func (g *csharpGenerator) emitPackage(pack *pkg) error {
	// Ensure that we have a root module.
	root := pack.modules.ensureModule("")
	if pack.provider != nil {
		root.members = append(root.members, pack.provider)
	}

	// Generate the individual modules and their contents.
	_, err := g.emitModules(pack)
	if err != nil {
		return err
	}

	return g.emitProjectFile()
}

// emitProjectFile emits a C# project file into the configured output directory.
func (g *csharpGenerator) emitProjectFile() error {
	assemblyName := g.assemblyName()

	w, err := tools.NewGenWriter(tfgen, filepath.Join(g.outDir, assemblyName+".csproj"))
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	version, err := semver.ParseTolerant(g.info.Version)
	if err != nil {
		return errors.Wrap(err, "could not parse version when emitting project file")
	}

	// Omit build metadata for .NET.
	version.Build = nil

	var buf bytes.Buffer
	err = csharpProjectFileTemplate.Execute(&buf, csharpProjectFileTemplateContext{
		XMLDoc:  fmt.Sprintf(`.\%s.xml`, assemblyName),
		Info:    g.info,
		Version: version.String(),
	})
	if err != nil {
		return err
	}
	w.WriteString(buf.String())
	return nil
}

// emitModules emits all modules in the given package. It returns a full list of files and any error that occurred.
func (g *csharpGenerator) emitModules(pack *pkg) ([]string, error) {
	var allFiles []string
	for _, mod := range pack.modules.values() {
		files, err := g.emitModule(mod)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, files...)
	}
	return allFiles, nil
}

// emitModule emits a module.  This module ends up having many possible ES6 sub-modules which are then re-exported
// at the top level.  This is to make it convenient for overlays to import files within the same module without
// causing problematic cycles.  For example, imagine a module m with many members; the result is:
//
//     m/
//         README.md
//         index.ts
//         member1.ts
//         member<etc>.ts
//         memberN.ts
//
// The one special case is the configuration module, which yields a vars.ts file containing all exported variables.
//
// Note that the special module "" represents the top-most package module and won't be placed in a sub-directory.
//
// The return values are the full list of files generated, the index file, and any error that occurred, respectively.
func (g *csharpGenerator) emitModule(mod *module) ([]string, error) {
	glog.V(3).Infof("emitModule(%s)", mod.name)

	// Ensure that the target module directory exists.
	dir := g.moduleDir(mod)
	if err := tools.EnsureDir(dir); err != nil {
		return nil, errors.Wrapf(err, "creating module directory")
	}

	// Ensure that the target module directory contains a README.md file.
	if err := g.ensureReadme(dir); err != nil {
		return nil, errors.Wrapf(err, "creating module README file")
	}

	// Gather the nested types for this module.
	nested := gatherNestedTypesForModule(mod)

	// Now, enumerate each module member, in the order presented to us, and do the right thing.
	var files []string
	for _, member := range mod.members {
		file, err := g.emitModuleMember(mod, member, nested)
		if err != nil {
			return nil, errors.Wrapf(err, "emitting module %s member %s", mod.name, member.Name())
		} else if file != "" {
			files = append(files, file)
		}
	}

	// If this is a config module, we need to emit the configuration variables.
	if mod.config() {
		file, err := g.emitConfigVariables(mod, nested)
		if err != nil {
			return nil, errors.Wrapf(err, "emitting config module variables")
		}
		files = append(files, file)
	}

	// Lastly, if this is the root module, we need to emit a file containing utility functions consumed by other
	// modules.
	if mod.root() {
		utils, err := g.emitUtilities(mod)
		if err != nil {
			return nil, errors.Wrapf(err, "emitting utility file for root module")
		}
		files = append(files, utils)
	}

	return files, nil
}

// ensureReadme writes out a stock README.md file, provided one doesn't already exist.
func (g *csharpGenerator) ensureReadme(dir string) error {
	rf := filepath.Join(dir, "README.md")
	_, err := os.Stat(rf)
	if err == nil {
		return nil // file already exists, exit right away.
	} else if !os.IsNotExist(err) {
		return err // legitimate error, propagate it.
	}

	// If we got here, the README.md doesn't already exist -- write out a stock one.
	w, err := tools.NewGenWriter(tfgen, rf)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	downstreamLicense := g.info.GetTFProviderLicense()
	licenseTypeURL := getLicenseTypeURL(downstreamLicense)
	w.Writefmtln(standardDocReadme, g.pkg, g.info.Name, g.info.GetGitHubOrg(), downstreamLicense, licenseTypeURL)
	return nil
}

// emitUtilities emits a utilities file for submodules to consume.
func (g *csharpGenerator) emitUtilities(mod *module) (string, error) {
	contract.Require(mod.root(), "mod.root()")

	// Open the utilities.ts file for this module and ready it for writing.
	w, utilities, err := g.openWriter(mod, "Utilities.cs")
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	// Strip any 'v' off of the version.
	version := g.info.Version
	if len(version) > 0 && version[0] == 'v' {
		version = version[1:]
	}

	var buf bytes.Buffer
	err = csharpUtilitiesTemplate.Execute(&buf, csharpUtilitiesTemplateContext{
		Namespace: g.moduleNamespace(mod),
		ClassName: "Utilities",
		Version:   version,
	})
	if err != nil {
		return "", err
	}

	w.WriteString(buf.String())
	return utilities, nil
}

// emitModuleMember emits the given member, and returns the module file that it was emitted into (if any).
func (g *csharpGenerator) emitModuleMember(
	mod *module, member moduleMember, nested *csharpNestedTypes) (string, error) {

	glog.V(3).Infof("emitModuleMember(%s, %s)", mod, member.Name())

	switch t := member.(type) {
	case *resourceType:
		return g.emitResourceType(mod, t, nested)
	case *resourceFunc:
		return g.emitResourceFunc(mod, t, nested)
	case *variable:
		contract.Assertf(mod.config(),
			"only expected top-level variables in config module (%s is not one)", mod.name)
		// skip the variable, we will process it later.
		return "", nil
	case *overlayFile:
		return g.emitOverlay(mod, t)
	default:
		contract.Failf("unexpected member type: %v", reflect.TypeOf(member))
		return "", nil
	}
}

func (g *csharpGenerator) emitConfigVariableType(w *tools.GenWriter, typ *propertyType) {
	contract.Assert(typ.kind == kindObject)

	w.Writefmtln("")

	// Open the class.
	w.Writefmtln("    public class %s", typ.name)
	w.Writefmtln("    {")

	// Generate each output field.
	for _, prop := range typ.properties {
		propertyName := csharpPropertyName(prop)
		propertyType := csharpPropertyType(prop, false /*wrapInput*/)

		initializer := ""
		if !prop.optional() {
			initializer = " = null!;"
		}

		emitCSharpPropertyDocComment(w, prop, "        ")
		w.Writefmtln("        public %s %s { get; set; }%s", propertyType, propertyName, initializer)
	}

	// Close the class.
	w.Writefmtln("    }")
}

// emitConfigVariables emits all config vaiables in the given module, returning the resulting file.
func (g *csharpGenerator) emitConfigVariables(mod *module, nestedTypes *csharpNestedTypes) (string, error) {
	// Create a Variables.cs file into which all configuration variables will go.
	w, config, err := g.openWriter(mod, "Variables.cs")
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	// Open the namespace.
	w.Writefmtln("using System.Collections.Immutable;")
	w.Writefmtln("")
	w.Writefmtln("namespace %s", g.moduleNamespace(mod))
	w.Writefmtln("{")

	// Open the config class.
	w.Writefmtln("    public static class Config")
	w.Writefmtln("    {")

	// Create a config bag for the variables to pull from.
	w.Writefmtln("        private static readonly Pulumi.Config __config = new Pulumi.Config(\"%v\");", g.pkg)
	w.Writefmtln("")

	// Emit an entry for all config variables.
	var nested []*csharpNestedType
	for _, member := range mod.members {
		if v, ok := member.(*variable); ok {
			g.emitConfigVariable(w, v)
			nested = append(nested, nestedTypes.getDeclaredTypes(v)...)
		}
	}

	// Close the config class.
	w.Writefmtln("    }")

	// Emit any nested types.
	w.Writefmtln("    namespace ConfigTypes")
	w.Writefmtln("    {")

	// Write each input type.
	for _, nested := range nested {
		g.emitConfigVariableType(w, nested.typ)
	}

	// Close the namespace
	w.Writefmtln("    }")

	// Close the namespace.
	w.Writefmtln("}")

	return config, nil
}

func (g *csharpGenerator) emitConfigVariable(w *tools.GenWriter, v *variable) {
	propertyType := qualifiedCSharpType(v.typ, "ConfigTypes", false /*wrapInputs*/)

	var getFunc string
	nullableSigil := "?"
	switch v.typ.kind {
	case kindString:
		getFunc = "Get"
	case kindBool:
		getFunc = "GetBoolean"
	case kindInt:
		getFunc = "GetInt32"
	case kindFloat:
		getFunc = "GetDouble"
	case kindSet, kindList:
		getFunc = "GetObject<" + propertyType + ">"
		nullableSigil = ""
	default:
		getFunc = "GetObject<" + propertyType + ">"
	}

	propertyName := strings.Title(v.name)

	initializer := fmt.Sprintf("__config.%s(\"%s\")", getFunc, v.name)
	if defaultValue := csharpDefaultValue(v); defaultValue != "null" {
		initializer += " ?? " + defaultValue
	}

	emitCSharpPropertyDocComment(w, v, "        ")
	w.Writefmtln("        public static %s%s %s { get; set; } = %s;",
		propertyType, nullableSigil, propertyName, initializer)
	w.Writefmtln("")
}

var csharpDocCommentEscaper = strings.NewReplacer(
	`&`, "&amp;",
	`<`, "&lt;",
	`>`, "&gt;",
)

func sanitizeForCSharpDocComment(str string) string {
	return csharpDocCommentEscaper.Replace(str)
}

func hasDocComment(v *variable) bool {
	return v.doc != "" && v.doc != elidedDocComment || v.rawdoc != ""
}

func emitCSharpPropertyDocComment(w *tools.GenWriter, v *variable, prefix string) {
	if v.doc != "" && v.doc != elidedDocComment {
		emitCSharpDocComment(w, v.doc, v.docURL, prefix)
	} else if v.rawdoc != "" {
		emitCSharpRawDocComment(w, v.rawdoc, prefix)
	}
}

func emitCSharpDocComment(w *tools.GenWriter, comment, docURL, prefix string) {
	var lines []string
	if comment != elidedDocComment {
		lines = strings.Split(comment, "\n")
	}

	if docURL != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("> This content is derived from %s.", docURL))
	}

	if len(lines) > 0 {
		w.Writefmtln("%s/// <summary>", prefix)
		for i, docLine := range lines {
			// Break if we get to the last line and it's empty
			if i == len(lines)-1 && strings.TrimSpace(docLine) == "" {
				break
			}

			// Print the line of documentation
			w.Writefmtln("%v/// %s", prefix, sanitizeForCSharpDocComment(docLine))
		}
		w.Writefmtln("%s/// </summary>", prefix)

	}
}

func emitCSharpRawDocComment(w *tools.GenWriter, comment, prefix string) {
	if comment == "" {
		return
	}

	width := 0
	w.Writefmtln("%s/// <summary>", prefix)
	for i, word := range strings.Fields(comment) {
		word = sanitizeForCSharpDocComment(word)
		switch {
		case i == 0:
			word = fmt.Sprintf("%v/// %s", prefix, word)
		case width+len(word)+1 > maxWidth:
			w.WriteString("\n")
			word = fmt.Sprintf("%v/// %s", prefix, word)
			width = 0
		default:
			word = " " + word
		}

		w.WriteString(word)
		width += len(word)
	}
	w.Writefmtln("")
	w.Writefmtln("%s/// </summary>", prefix)
}

type csharpResourceGenerator struct {
	g *csharpGenerator

	mod    *module
	res    *resourceType
	fun    *resourceFunc
	nested []*csharpNestedType

	name string
	w    *tools.GenWriter
}

func newCSharpResourceGenerator(g *csharpGenerator, mod *module, res *resourceType,
	nested *csharpNestedTypes) *csharpResourceGenerator {

	return &csharpResourceGenerator{
		g:      g,
		mod:    mod,
		res:    res,
		nested: nested.getDeclaredTypes(res),
		name:   res.name,
	}
}

func newCSharpDatasourceGenerator(g *csharpGenerator, mod *module, fun *resourceFunc,
	nested *csharpNestedTypes) *csharpResourceGenerator {

	return &csharpResourceGenerator{
		g:      g,
		mod:    mod,
		fun:    fun,
		nested: nested.getDeclaredTypes(fun),
		name:   strings.Title(fun.name),
	}
}

func (rg *csharpResourceGenerator) emit() (string, error) {
	// Create a resource source file into which all of this resource's types will go.
	// We need to check if the resource is called "utilities", as this may overlap with the utilities.cs file.
	filename := rg.name
	if filename == "Utilities" {
		filename = fmt.Sprintf("%s_", filename)
	}

	w, file, err := rg.g.openWriter(rg.mod, filename+".cs")
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)
	rg.w = w

	rg.openNamespace()
	if rg.res != nil {
		rg.generateResourceClass()
		rg.generateResourceArgs()
		rg.generateResourceState()
	} else {
		contract.Assert(rg.fun != nil)
		rg.generateDatasourceFunc()
		rg.generateDatasourceArgs()
		rg.generateDatasourceResult()
	}
	rg.generateNestedTypes()
	rg.closeNamespace()

	return file, nil
}

func (rg *csharpResourceGenerator) openNamespace() {
	rg.w.Writefmtln("using System.Collections.Immutable;")
	rg.w.Writefmtln("using System.Threading.Tasks;")
	rg.w.Writefmtln("using Pulumi.Serialization;")
	rg.w.Writefmtln("")
	rg.w.Writefmtln("namespace %s", rg.g.moduleNamespace(rg.mod))
	rg.w.Writefmtln("{")
}

func (rg *csharpResourceGenerator) closeNamespace() {
	rg.w.Writefmtln("}")
}

func (rg *csharpResourceGenerator) generateResourceClass() {
	// Write the doc comment, if any.
	if rg.res.doc != "" {
		emitCSharpDocComment(rg.w, rg.res.doc, rg.res.docURL, "    ")
	}

	// Open the class.
	className := rg.name
	baseType := "Pulumi.CustomResource"
	if rg.res.IsProvider() {
		baseType = "Pulumi.ProviderResource"
	}
	rg.w.Writefmtln("    public partial class %s : %s", className, baseType)
	rg.w.Writefmtln("    {")

	// Emit all output properties.
	for _, prop := range rg.res.outprops {
		// Write the property attribute
		wireName := prop.name
		propertyName := csharpPropertyName(prop)
		propertyType := qualifiedCSharpPropertyType(prop, "Outputs", false /*wrapInput*/)

		// Workaround the fact that provider inputs come back as strings.
		if rg.res.IsProvider() && !isPrimitiveKind(prop.typ.kind) {
			propertyType = "string"
			if prop.optional() {
				propertyType += "?"
			}
		}

		emitCSharpPropertyDocComment(rg.w, prop, "        ")
		rg.w.Writefmtln("        [Output(\"%s\")]", wireName)
		rg.w.Writefmtln("        public Output<%s> %s { get; private set; } = null!;", propertyType, propertyName)
		rg.w.Writefmtln("")
	}
	rg.w.Writefmtln("")

	// Emit the class constructor.
	argsType := rg.res.argst.name

	var argsDefault string
	if len(rg.res.reqprops) == 0 {
		// If the number of required input properties was zero, we can make the args object optional.
		argsDefault = " = null"
		argsType += "?"
	}

	optionsType := "CustomResourceOptions"
	if rg.res.IsProvider() {
		optionsType = "ResourceOptions"
	}

	// Write a comment prior to the constructor.
	rg.w.Writefmtln("        /// <summary>")
	rg.w.Writefmtln("        /// Create a %s resource with the given unique name, arguments, and options.", className)
	rg.w.Writefmtln("        /// </summary>")
	rg.w.Writefmtln("        ///")
	rg.w.Writefmtln("        /// <param name=\"name\">The unique name of the resource</param>")
	rg.w.Writefmtln("        /// <param name=\"args\">The arguments used to populate this resource's properties</param>")
	rg.w.Writefmtln("        /// <param name=\"options\">A bag of options that control this resource's behavior</param>")

	rg.w.Writefmtln("        public %s(string name, %s args%s, %s? options = null)",
		className, argsType, argsDefault, optionsType)
	rg.w.Writefmtln("            : base(\"%s\", name, args, MakeResourceOptions(options, \"\"))", rg.res.info.Tok)
	rg.w.Writefmtln("        {")
	rg.w.Writefmtln("        }")

	// Write a private constructor for the use of `Get`.
	if !rg.res.IsProvider() {
		rg.w.Writefmtln("")
		rg.w.Writefmtln("        private %s(string name, Input<string> id, %s? state = null, %s? options = null)",
			className, rg.res.statet.name, optionsType)
		rg.w.Writefmtln("            : base(\"%s\", name, state, MakeResourceOptions(options, id))", rg.res.info.Tok)
		rg.w.Writefmtln("        {")
		rg.w.Writefmtln("        }")
	}

	// Write the method that will calculate the resource options.
	rg.w.Writefmtln("")
	rg.w.Writefmtln("        private static %[1]s MakeResourceOptions(%[1]s? options, Input<string>? id)", optionsType)
	rg.w.Writefmtln("        {")
	rg.w.Writefmtln("            var defaultOptions = new %s", optionsType)
	rg.w.Writefmtln("            {")
	rg.w.Writefmtln("                Id = id,")
	rg.w.Writefmt("                Version = Utilities.Version,")

	switch len(rg.res.info.Aliases) {
	case 0:
		rg.w.Writefmtln("")
	case 1:
		rg.w.Writefmt("                Aliases = { ")
		rg.generateAlias(rg.res.info.Aliases[0])
		rg.w.Writefmtln(" },")
	default:
		rg.w.Writefmtln("                Aliases =")
		rg.w.Writefmtln("                {")
		for _, alias := range rg.res.info.Aliases {
			rg.w.Writefmt("                    ")
			rg.generateAlias(alias)
			rg.w.Writefmtln(",")
		}
		rg.w.Writefmtln("                },")
	}

	rg.w.Writefmtln("            };")
	rg.w.Writefmtln("            return (%s)ResourceOptions.Merge(defaultOptions, options);", optionsType)
	rg.w.Writefmtln("        }")

	// Write the `Get` method for reading instances of this resource unless this is a provider resource.
	if !rg.res.IsProvider() {
		rg.w.Writefmtln("        /// <summary>")
		rg.w.Writefmtln("        /// Get an existing %s resource's state with the given name, ID, and optional extra",
			className)
		rg.w.Writefmtln("        /// properties used to qualify the lookup.")
		rg.w.Writefmtln("        ///")
		rg.w.Writefmtln("        /// <param name=\"name\">The unique name of the resulting resource.</param>")
		rg.w.Writefmtln("        /// <param name=\"id\">The unique provider ID of the resource to lookup.</param>")
		rg.w.Writefmtln("        /// <param name=\"state\">Any extra arguments used during the lookup.</param>")
		rg.w.Writefmtln("        /// <param name=\"options\">A bag of options that control this resource's behavior</param>")
		rg.w.Writefmtln("        /// </summary>")
		rg.w.Writefmtln("        public static %s Get(string name, Input<string> id, %s? state = null, %s? options = null)",
			className, rg.res.statet.name, optionsType)
		rg.w.Writefmtln("        {")
		rg.w.Writefmtln("            return new %s(name, id, state, options);", className)
		rg.w.Writefmtln("        }")
	}

	// Close the class.
	rg.w.Writefmtln("    }")
}

func (rg *csharpResourceGenerator) generateAlias(alias tfbridge.AliasInfo) {
	rg.w.WriteString("new Alias { ")
	var parts []string
	if alias.Name != nil {
		parts = append(parts, fmt.Sprintf("Name = \"%s\"", *alias.Name))
	}
	if alias.Project != nil {
		parts = append(parts, fmt.Sprintf("Project = \"%s\"", *alias.Project))
	}
	if alias.Type != nil {
		parts = append(parts, fmt.Sprintf("Type = \"%s\"", *alias.Type))
	}
	rg.w.WriteString(strings.Join(parts, ", "))
	rg.w.WriteString(" }")
}

func (rg *csharpResourceGenerator) generateResourceArgs() {
	rg.generateInputType(rg.res.argst, false /*nested*/)
}

func (rg *csharpResourceGenerator) generateResourceState() {
	if !rg.res.IsProvider() {
		rg.generateInputType(rg.res.statet, false /*nested*/)
	}
}

func (rg *csharpResourceGenerator) generateDatasourceFunc() {
	// Open the partial class we'll use for datasources.
	// TODO(pdg): this needs a better name that is guaranteed to be unique.
	rg.w.Writefmtln("    public static partial class Invokes")
	rg.w.Writefmtln("    {")

	methodName := strings.Title(rg.fun.name)

	var typeParameter string
	if rg.fun.retst != nil {
		typeParameter = fmt.Sprintf("<%s>", rg.fun.retst.name)
	}

	var argsParamDef string
	argsParamRef := "null"
	if rg.fun.argst != nil {
		argsType := rg.fun.argst.name

		var argsDefault string
		if len(rg.fun.reqargs) == 0 {
			// If the number of required input properties was zero, we can make the args object optional.
			argsDefault = " = null"
			argsType += "?"
		}

		argsParamDef = fmt.Sprintf("%s args%s, ", argsType, argsDefault)
		argsParamRef = "args"
	}

	// Emit the doc comment, if any.
	if rg.fun.doc != "" {
		emitCSharpDocComment(rg.w, rg.fun.doc, rg.fun.docURL, "        ")
	}

	// Emit the datasource method.
	rg.w.Writefmt("        public static Task%s %s(%sInvokeOptions? options = null)",
		typeParameter, methodName, argsParamDef)
	rg.w.Writefmtln(" => return Pulumi.Deployment.Instance.InvokeAsync%s(\"%s\", %s, options.WithVersion());",
		typeParameter, rg.fun.info.Tok, argsParamRef)

	// Close the class.
	rg.w.Writefmtln("    }")
}

func (rg *csharpResourceGenerator) generateDatasourceArgs() {
	if rg.fun.argst != nil {
		// TODO(pdg): this should pass true for plainType
		rg.generateInputType(rg.fun.argst, false /*nested*/)
	}
}

func (rg *csharpResourceGenerator) generateDatasourceResult() {
	if rg.fun.retst != nil {
		rg.generateOutputType(rg.fun.retst, false /*nested*/)
	}
}

func (rg *csharpResourceGenerator) generateInputProperty(prop *variable, nested bool) {
	qualifier := ""
	if !nested {
		qualifier = "Inputs"
	}

	wireName := prop.name
	propertyName := csharpPropertyName(prop)
	propertyType := qualifiedCSharpPropertyType(prop, qualifier, true /*wrapInput*/)

	// First generate the input attribute.
	attributeArgs := ""
	if !prop.optional() {
		attributeArgs = ", required: true"
	}
	if rg.res != nil && rg.res.IsProvider() && prop.typ.kind != kindString {
		attributeArgs += ", json: true"
	}

	// Next generate the input property itself. The way this is generated depends on the type of the property:
	// complex types like lists and maps need a backing field.
	switch prop.typ.kind {
	case kindList, kindSet, kindMap:
		backingFieldName := "_" + prop.name
		backingFieldType := qualifiedCSharpType(prop.typ, qualifier, true /*wrapInput*/)

		rg.w.Writefmtln("        [Input(\"%s\"%s)]", wireName, attributeArgs)
		rg.w.Writefmtln("        private %s? %s;", backingFieldType, backingFieldName)

		if hasDocComment(prop) {
			rg.w.Writefmtln("")
		}
		emitCSharpPropertyDocComment(rg.w, prop, "        ")

		rg.w.Writefmtln("        public %s %s", propertyType, propertyName)
		rg.w.Writefmtln("        {")
		rg.w.Writefmtln("            get => %s ?? (%s = new %s());", backingFieldName, backingFieldName, backingFieldType)
		rg.w.Writefmtln("            set => %s = value;", backingFieldName)
		rg.w.Writefmtln("        }")
	default:
		initializer := ""
		if !prop.optional() {
			initializer = " = null!;"
		}

		emitCSharpPropertyDocComment(rg.w, prop, "        ")
		rg.w.Writefmtln("        [Input(\"%s\"%s)]", wireName, attributeArgs)
		rg.w.Writefmtln("        public %s %s { get; set; }%s", propertyType, propertyName, initializer)
	}
}

func (rg *csharpResourceGenerator) generateInputType(typ *propertyType, nested bool) {
	contract.Assert(typ.kind == kindObject)

	rg.w.Writefmtln("")

	// Open the class.
	rg.w.Writefmtln("    public sealed class %s : Pulumi.ResourceArgs", typ.name)
	rg.w.Writefmtln("    {")

	// Declare each input property.
	for _, p := range typ.properties {
		rg.generateInputProperty(p, nested)
		rg.w.Writefmtln("")
	}

	// Generate a constructor that will set default values.
	rg.w.Writefmtln("        public %s()", typ.name)
	rg.w.Writefmtln("        {")
	for _, prop := range typ.properties {
		if defaultValue := csharpDefaultValue(prop); defaultValue != "null" {
			propertyName := csharpPropertyName(prop)
			rg.w.Writefmtln("            %s = %s;", propertyName, defaultValue)
		}
	}
	rg.w.Writefmtln("        }")

	// Close the class.
	rg.w.Writefmtln("    }")
}

func (rg *csharpResourceGenerator) generateOutputType(typ *propertyType, nested bool) {
	contract.Assert(typ.kind == kindObject)

	qualifier := ""
	if !nested {
		qualifier = "Outputs"
	}

	rg.w.Writefmtln("")

	// Open the class and attribute it appropriately.
	rg.w.Writefmtln("    [OutputType]")
	rg.w.Writefmtln("    public sealed class %s", typ.name)
	rg.w.Writefmtln("    {")

	// Generate each output field.
	for _, prop := range typ.properties {
		fieldName := csharpPropertyName(prop)
		fieldType := qualifiedCSharpPropertyType(prop, qualifier, false /*wrapInput*/)
		emitCSharpPropertyDocComment(rg.w, prop, "        ")
		rg.w.Writefmtln("        public readonly %s %s;", fieldType, fieldName)
	}
	if len(typ.properties) > 0 {
		rg.w.Writefmtln("")
	}

	// Generate an appropriately-attributed constructor that will set this types' fields.
	rg.w.Writefmtln("        [OutputConstructor]")
	rg.w.Writefmt("        private %s(", typ.name)

	// Generate the constructor parameters.
	for i, prop := range typ.properties {
		paramName := csharpIdentifier(prop.name)
		paramType := qualifiedCSharpPropertyType(prop, qualifier, false /*wrapInput*/)

		terminator := ""
		if i != len(typ.properties)-1 {
			terminator = ","
		}

		paramDef := fmt.Sprintf("%s %s%s", paramType, paramName, terminator)
		if len(typ.properties) > 1 {
			paramDef = "\n            " + paramDef
		}
		rg.w.WriteString(paramDef)
	}
	rg.w.Writefmtln(")")

	// Generate the constructor body.
	rg.w.Writefmtln("        {")
	for _, prop := range typ.properties {
		paramName := csharpIdentifier(prop.name)
		fieldName := csharpPropertyName(prop)
		rg.w.Writefmtln("            %s = %s;", fieldName, paramName)
	}
	rg.w.Writefmtln("        }")

	// Close the class.
	rg.w.Writefmtln("    }")
}

func (rg *csharpResourceGenerator) generateInputTypes(nts []*csharpNestedType) {
	// Open the Inputs namespace.
	rg.w.Writefmtln("")
	rg.w.Writefmtln("    namespace Inputs")
	rg.w.Writefmtln("    {")

	// Write each input type.
	for _, nt := range nts {
		contract.Assert(nt.isInput)
		rg.generateInputType(nt.typ, true /*nested*/)
	}

	// Close the namespace
	rg.w.Writefmtln("    }")
}

func (rg *csharpResourceGenerator) generateOutputTypes(nts []*csharpNestedType) {
	// Open the Outputs namespace.
	rg.w.Writefmtln("")
	rg.w.Writefmtln("    namespace Outputs")
	rg.w.Writefmtln("    {")

	// Write each output type.
	for _, nt := range nts {
		contract.Assert(!nt.isInput)
		rg.generateOutputType(nt.typ, true /*nested*/)
	}

	// Close the namespace
	rg.w.Writefmtln("    }")
}

func (rg *csharpResourceGenerator) generateNestedTypes() {
	sort.Slice(rg.nested, func(i, j int) bool {
		a, b := rg.nested[i], rg.nested[j]
		return a.typ.name < b.typ.name
	})

	var inputTypes []*csharpNestedType
	var outputTypes []*csharpNestedType
	for _, nt := range rg.nested {
		contract.Assert(nt.declarer == rg.res || nt.declarer == rg.fun)

		if nt.isInput {
			inputTypes = append(inputTypes, nt)
		} else {
			outputTypes = append(outputTypes, nt)
		}
	}

	rg.generateInputTypes(inputTypes)
	rg.generateOutputTypes(outputTypes)
}

//nolint:lll
func (g *csharpGenerator) emitResourceType(mod *module, res *resourceType, nested *csharpNestedTypes) (string, error) {
	rg := newCSharpResourceGenerator(g, mod, res, nested)
	return rg.emit()
}

func (g *csharpGenerator) emitResourceFunc(mod *module, fun *resourceFunc, nested *csharpNestedTypes) (string, error) {
	rg := newCSharpDatasourceGenerator(g, mod, fun, nested)
	return rg.emit()
}

// emitOverlay copies an overlay from its source to the target, and returns the resulting file to be exported.
func (g *csharpGenerator) emitOverlay(mod *module, overlay *overlayFile) (string, error) {
	// Copy the file from the overlays directory to the destination.
	dir := g.moduleDir(mod)
	dst := filepath.Join(dir, overlay.name)
	if overlay.Copy() {
		if err := copyFile(overlay.src, dst); err != nil {
			return "", err
		}
	} else {
		if _, err := os.Stat(dst); err != nil {
			return "", err
		}
	}

	// And then export the overlay's contents from the index.
	return dst, nil
}

func csharpIdentifier(s string) string {
	switch s {
	case "abstract", "as", "base", "bool",
		"break", "byte", "case", "catch",
		"char", "checked", "class", "const",
		"continue", "decimal", "default", "delegate",
		"do", "double", "else", "enum",
		"event", "explicit", "extern", "false",
		"finally", "fixed", "float", "for",
		"foreach", "goto", "if", "implicit",
		"in", "int", "interface", "internal",
		"is", "lock", "long", "namespace",
		"new", "null", "object", "operator",
		"out", "override", "params", "private",
		"protected", "public", "readonly", "ref",
		"return", "sbyte", "sealed", "short",
		"sizeof", "stackalloc", "static", "string",
		"struct", "switch", "this", "throw",
		"true", "try", "typeof", "uint",
		"ulong", "unchecked", "unsafe", "ushort",
		"using", "virtual", "void", "volatile", "while":
		return "@" + s

	default:
		return s
	}
}

func csharpPropertyName(v *variable) string {
	if v.info != nil && v.info.CSharpName != "" {
		return v.info.CSharpName
	}
	return strings.Title(v.name)
}

// csharpPropertyType returns the C# type name for a given schema property.
//
// wrapInput can be set to true to cause the generated type to be deeply wrapped with `Input<T>`.
func csharpPropertyType(v *variable, wrapInput bool) string {
	return qualifiedCSharpPropertyType(v, "", wrapInput)
}

func isImmutableArrayType(typ *propertyType, wrapInput bool) bool {
	return (typ.kind == kindSet || typ.kind == kindList) && !wrapInput
}

func qualifiedCSharpPropertyType(v *variable, qualifier string, wrapInput bool) string {
	t := qualifiedCSharpType(v.typ, qualifier, wrapInput)
	if v.optional() && !isImmutableArrayType(v.typ, wrapInput) {
		t += "?"
	}
	return t
}

// qualifiedCsharpType returns the C# type name for a given schema value type and element kind.
func qualifiedCSharpType(typ *propertyType, qualifier string, wrapInput bool) string {
	// Prefer overrides over the underlying type.
	var t string
	switch {
	case typ == nil:
		return "object"
	case typ.asset != nil:
		if typ.asset.IsArchive() {
			t = typ.asset.Type()
		} else {
			t = "AssetOrArchive"
		}
	default:
		// First figure out the raw type.
		switch typ.kind {
		case kindBool:
			t = "bool"
		case kindInt:
			t = "int"
		case kindFloat:
			t = "double"
		case kindString:
			t = "string"
		case kindSet, kindList:
			elemType := qualifiedCSharpType(typ.element, qualifier, false)
			if wrapInput {
				return fmt.Sprintf("InputList<%v>", elemType)
			}
			return fmt.Sprintf("ImmutableArray<%v>", elemType)
		case kindMap:
			elemType := qualifiedCSharpType(typ.element, qualifier, false)
			if wrapInput {
				return fmt.Sprintf("InputMap<%v>", elemType)
			}
			return fmt.Sprintf("ImmutableDictionary<string, %v>", elemType)
		case kindObject:
			t = typ.name
			if qualifier != "" {
				t = qualifier + "." + t
			}
		default:
			contract.Failf("Unrecognized schema type: %v", typ.kind)
		}
	}

	if wrapInput {
		t = fmt.Sprintf("Input<%v>", t)
	}

	return t
}

func csharpPrimitiveValue(value interface{}) (string, error) {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			return "true", nil
		}
		return "false", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64), nil
	case reflect.String:
		return fmt.Sprintf("%q", v.String()), nil
	default:
		return "", errors.Errorf("unsupported default value of type %T", value)
	}
}

func csharpDefaultValue(prop *variable) string {
	defaultValue := "null"
	if prop.info == nil || prop.info.Default == nil {
		return defaultValue
	}
	defaults := prop.info.Default

	if defaults.Value != nil {
		dv, err := csharpPrimitiveValue(defaults.Value)
		if err != nil {
			cmdutil.Diag().Warningf(diag.Message("", err.Error()))
			return defaultValue
		}
		defaultValue = dv
	}

	if len(defaults.EnvVars) != 0 {
		getType := ""
		switch prop.typ.kind {
		case kindBool:
			getType = "Boolean"
		case kindInt:
			getType = "Int32"
		case kindFloat:
			getType = "Double"
		}

		envVars := fmt.Sprintf("%q", defaults.EnvVars[0])
		for _, e := range defaults.EnvVars[1:] {
			envVars += fmt.Sprintf(", %q", e)
		}

		getEnv := fmt.Sprintf("Utilities.GetEnv%s(%s)", getType, envVars)
		if defaultValue != "null" {
			defaultValue = fmt.Sprintf("(%s ?? %s)", getEnv, defaultValue)
		} else {
			defaultValue = getEnv
		}
	}

	return defaultValue
}
