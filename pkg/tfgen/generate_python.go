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
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/golang/glog"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfbridge"
	pycodegen "github.com/pulumi/pulumi/pkg/codegen/python"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/tools"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// newPythonGenerator returns a language generator that understands how to produce Python packages.
func newPythonGenerator(pkg, version string, info tfbridge.ProviderInfo, overlaysDir, outDir string) langGenerator {
	return &pythonGenerator{
		pkg:                  pkg,
		version:              version,
		info:                 info,
		overlaysDir:          overlaysDir,
		outDir:               outDir,
		snakeCaseToCamelCase: make(map[string]string),
		camelCaseToSnakeCase: make(map[string]string),
	}
}

type pythonGenerator struct {
	pkg                  string
	version              string
	info                 tfbridge.ProviderInfo
	overlaysDir          string
	outDir               string
	snakeCaseToCamelCase map[string]string // property mapping from snake case to camel case
	camelCaseToSnakeCase map[string]string // property mapping from camel case to snake case
}

// commentChars returns the comment characters to use for single-line comments.
func (g *pythonGenerator) commentChars() string {
	return "#"
}

// moduleDir returns the directory for the given module.
func (g *pythonGenerator) moduleDir(mod *module) string {
	dir := filepath.Join(g.outDir, pyPack(g.pkg))
	if mod.name != "" {
		dir = filepath.Join(dir, pycodegen.PyName(mod.name))
	}
	return dir
}

// relativeRootDir returns the relative path to the root directory for the given module.
func (g *pythonGenerator) relativeRootDir(mod *module) string {
	p, err := filepath.Rel(g.moduleDir(mod), filepath.Join(g.outDir, pyPack(g.pkg)))
	contract.IgnoreError(err)
	return p
}

// openWriter opens a writer for the given module and file name, emitting the standard header automatically.
func (g *pythonGenerator) openWriter(mod *module, name string, needsSDK bool) (*tools.GenWriter, error) {
	dir := g.moduleDir(mod)
	file := filepath.Join(dir, name)
	w, err := tools.NewGenWriter(tfgen, file)
	if err != nil {
		return nil, err
	}

	// Set the encoding to UTF-8, in case the comments contain non-ASCII characters.
	w.Writefmtln("# coding=utf-8")

	// Emit a standard warning header ("do not edit", etc).
	w.EmitHeaderWarning(g.commentChars())

	// If needed, emit the standard Pulumi SDK import statement.
	if needsSDK {
		g.emitSDKImport(mod, w)
	}

	return w, nil
}

func (g *pythonGenerator) emitSDKImport(mod *module, w *tools.GenWriter) {
	w.Writefmtln("import json")
	w.Writefmtln("import warnings")
	w.Writefmtln("import pulumi")
	w.Writefmtln("import pulumi.runtime")
	w.Writefmtln("from typing import Union")
	w.Writefmtln("from %s import utilities, tables", g.relativeRootDir(mod))
	w.Writefmtln("")
}

// typeName returns a type name for a given resource type.
func (g *pythonGenerator) typeName(r *resourceType) string {
	return r.name
}

// emitPackage emits an entire package pack into the configured output directory with the configured settings.
func (g *pythonGenerator) emitPackage(pack *pkg) error {
	// First, generate the individual modules and their contents.
	submodules, err := g.emitModules(pack.modules)
	if err != nil {
		return err
	}

	// Generate a top-level index file that re-exports any child modules.
	index := pack.modules.ensureModule("")
	if pack.provider != nil {
		index.members = append(index.members, pack.provider)
	}

	if err := g.emitModule(index, submodules); err != nil {
		return err
	}

	if err := g.emitPythonTypes(pack); err != nil {
		return err
	}

	// Finally emit the package metadata (setup.py, etc).
	return g.emitPackageMetadata(pack)
}

// emitModules emits all modules in the given module map.  It returns a full list of files, a map of module to its
// associated index, and any error that occurred, if any.
func (g *pythonGenerator) emitModules(mmap moduleMap) ([]string, error) {
	var modules []string
	for _, mod := range mmap.values() {
		if mod.name == "" {
			continue // skip the root module, it is handled specially.
		}
		if err := g.emitModule(mod, nil); err != nil {
			return nil, err
		}
		modules = append(modules, mod.name)
	}
	return modules, nil
}

// emitModule emits a module as a Python package.  This package ends up having many Python sub-modules which are then
// re-exported at the top level.  This is to make it convenient for overlays to import files within the same module
// without causing problematic cycles.  For example, imagine a module m with many members; the result is:
//
//     m/
//         README.md
//         __init__.py
//         member1.py
//         member<etc>.py
//         memberN.py
//
// The one special case is the configuration module, which yields a vars.py file containing all exported variables.
////
// Note that the special module "" represents the top-most package module and won't be placed in a sub-directory.
//
// The return values are the full list of files generated, the index file, and any error that occurred, respectively.
func (g *pythonGenerator) emitModule(mod *module, submods []string) error {
	glog.V(3).Infof("emitModule(%s)", mod.name)

	// Ensure that the target module directory exists.
	dir := g.moduleDir(mod)
	if err := tools.EnsureDir(dir); err != nil {
		return errors.Wrapf(err, "creating module directory")
	}

	// Ensure that the target module directory contains a README.md file.
	if err := g.ensureReadme(dir); err != nil {
		return errors.Wrapf(err, "creating module README file")
	}

	// Keep track of any immediately exported package modules, as we will re-export them.
	var exports []string

	// Calculate our casing tables. We do this up front because our docstring generator (which is run during
	// emitModuleMember) requires them.
	for _, member := range mod.members {
		if res, ok := member.(*resourceType); ok {
			for _, prop := range res.inprops {
				g.recordProperty(prop)
			}
			for _, prop := range res.outprops {
				g.recordProperty(prop)
			}
		}
	}

	// Now, enumerate each module member, in the order presented to us, and do the right thing.
	for _, member := range mod.members {
		m, err := g.emitModuleMember(mod, member)
		if err != nil {
			return errors.Wrapf(err, "emitting module %s member %s", mod.name, member.Name())
		} else if m != "" {
			exports = append(exports, m)
		}
	}

	// If this is a config module, we need to emit the configuration variables.
	if mod.config() {
		m, err := g.emitConfigVariables(mod)
		if err != nil {
			return errors.Wrapf(err, "emitting config module variables")
		}
		exports = append(exports, m)
	}

	// If this is the root module, we need to emit the utilities.
	if mod.root() {
		if err := g.emitUtilities(mod); err != nil {
			return errors.Wrap(err, "emitting utilities")
		}

		if err := g.emitPropertyConversionTables(mod); err != nil {
			return errors.Wrap(err, "emitting conversion tables")
		}
	}

	// Lastly, generate an index file for this module.
	if err := g.emitIndex(mod, exports, submods); err != nil {
		return errors.Wrapf(err, "emitting module %s index", mod.name)
	}

	return nil
}

// ensureReadme writes out a stock README.md file, provided one doesn't already exist.
func (g *pythonGenerator) ensureReadme(dir string) error {
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

// emitIndex emits an __init__.py module, optionally re-exporting other members or submodules.
func (g *pythonGenerator) emitIndex(mod *module, exports, submods []string) error {
	// Open the index.ts file for this module, and ready it for writing.
	w, err := g.openWriter(mod, "__init__.py", false)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	// If there are subpackages, export them in the __all__ variable.
	if len(submods) > 0 {
		w.Writefmtln("import importlib")
		w.Writefmtln("# Make subpackages available:")
		w.Writefmt("__all__ = [")
		for i, sub := range submods {
			if i > 0 {
				w.Writefmt(", ")
			}
			w.Writefmt("'%s'", pycodegen.PyName(sub))
		}
		w.Writefmtln("]")
		w.Writefmtln("for pkg in __all__:")
		w.Writefmtln("    if pkg != 'config':")
		w.Writefmtln("        importlib.import_module(f'{__name__}.{pkg}')")
	}

	// Now, import anything to export flatly that is a direct export rather than sub-module.
	if len(exports) > 0 {
		if len(submods) > 0 {
			w.Writefmtln("")
		}
		w.Writefmtln("# Export this package's modules as members:")
		for _, exp := range exports {
			w.Writefmtln("from .%s import *", pycodegen.PyName(exp))
		}
	}

	return nil
}

// emitUtilities emits a utilities file for submodules to consume.
func (g *pythonGenerator) emitUtilities(mod *module) error {
	contract.Require(mod.root(), "mod.root()")

	// Open the utilities.ts file for this module and ready it for writing.
	w, err := g.openWriter(mod, "utilities.py", false)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	// TODO: use w.WriteString
	w.Writefmt(pyUtilitiesFile)
	return nil
}

// emitModuleMember emits the given member, and returns the module file that it was emitted into (if any).
func (g *pythonGenerator) emitModuleMember(mod *module, member moduleMember) (string, error) {
	glog.V(3).Infof("emitModuleMember(%s, %s)", mod, member.Name())

	switch t := member.(type) {
	case *resourceType:
		return g.emitResourceType(mod, t)
	case *resourceFunc:
		return g.emitResourceFunc(mod, t)
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

// emitConfigVariables emits all config vaiables in the given module, returning the resulting file.
func (g *pythonGenerator) emitConfigVariables(mod *module) (string, error) {
	// Create a vars.ts file into which all configuration variables will go.
	config := "vars"
	w, err := g.openWriter(mod, config+".py", true)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	// Create a config bag for the variables to pull from.
	w.Writefmtln("__config__ = pulumi.Config('%s')", g.pkg)
	w.Writefmtln("")

	// Emit an entry for all config variables.
	for _, member := range mod.members {
		if v, ok := member.(*variable); ok {
			g.emitConfigVariable(w, v)
		}
	}

	return config, nil
}

func (g *pythonGenerator) emitConfigVariable(w *tools.GenWriter, v *variable) {
	configFetch := fmt.Sprintf("__config__.get('%s')", v.name)
	if defaultValue := pyDefaultValue(v); defaultValue != "" {
		configFetch += " or " + defaultValue
	}

	w.Writefmtln("%s = %s", pycodegen.PyName(v.name), configFetch)
	if v.doc != "" && v.doc != elidedDocComment {
		g.emitDocComment(w, v.doc, v.docURL, "")
	} else if v.rawdoc != "" {
		g.emitRawDocComment(w, v.rawdoc, "")
	}
	w.Writefmtln("")
}

func (g *pythonGenerator) emitDocComment(w *tools.GenWriter, comment, docURL, prefix string) {
	if comment == elidedDocComment && docURL == "" {
		return
	}

	w.Writefmtln(`%s"""`, prefix)

	if comment != elidedDocComment {
		lines := strings.Split(comment, "\n")
		for i, docLine := range lines {
			// Break if we get to the last line and it's empty
			if i == len(lines)-1 && strings.TrimSpace(docLine) == "" {
				break
			}

			// Print the line of documentation
			w.Writefmtln("%s%s", prefix, docLine)
		}

		if docURL != "" {
			w.Writefmtln("")
		}
	}

	if docURL != "" {
		w.Writefmtln("%s> This content is derived from %s.", prefix, docURL)
	}

	w.Writefmtln(`%s"""`, prefix)
}

func (g *pythonGenerator) emitRawDocComment(w *tools.GenWriter, comment, prefix string) {
	if comment != "" {
		curr := 0
		for _, word := range strings.Fields(comment) {
			if curr > 0 {
				if curr+len(word)+1 > (maxWidth - len(prefix)) {
					curr = 0
					w.Writefmtln("")
					w.Writefmt(prefix)
				} else {
					w.Writefmt(" ")
					curr++
				}
			} else {
				w.Writefmtln(`%s"""`, prefix)
				w.Writefmt(prefix)
			}
			w.Writefmt(word)
			curr += len(word)
		}
		w.Writefmtln("")
		w.Writefmtln(`%s"""`, prefix)
	}
}

func (g *pythonGenerator) emitAwaitableType(w *tools.GenWriter, pot *propertyType) string {
	baseName := pyClassName(pot.name)

	// Produce a class definition with optional """ comment.
	w.Writefmtln("class %s:", baseName)
	if pot.doc != "" {
		g.emitDocComment(w, pot.doc, "", "    ")
	}

	// Now generate an initializer with properties for all inputs.
	w.Writefmt("    def __init__(__self__")
	for _, prop := range pot.properties {
		w.Writefmt(", %s=None", pycodegen.PyName(prop.name))
	}
	w.Writefmtln("):")
	for _, prop := range pot.properties {
		// Check that required arguments are present.  Also check that types are as expected.
		pname := pycodegen.PyName(prop.name)
		ptype := pyType(prop)
		w.Writefmtln("        if %s and not isinstance(%s, %s):", pname, pname, ptype)
		w.Writefmtln("            raise TypeError(\"Expected argument '%s' to be a %s\")", pname, ptype)

		// Now perform the assignment, and follow it with a """ doc comment if there was one found.
		w.Writefmtln("        __self__.%[1]s = %[1]s", pname)
		if prop.doc != "" && prop.doc != elidedDocComment {
			g.emitDocComment(w, prop.doc, prop.docURL, "        ")
		} else if prop.rawdoc != "" {
			g.emitRawDocComment(w, prop.rawdoc, "        ")
		}
	}

	awaitableName := "Awaitable" + baseName

	// Produce an awaitable subclass.
	w.Writefmtln("class %s(%s):", awaitableName, baseName)

	// Emit __await__ and __iter__ in order to make this type awaitable.
	//
	// Note that we need __await__ to be an iterator, but we only want it to return one value. As such, we use
	// `if False: yield` to construct this.
	//
	// We also need the result of __await__ to be a plain, non-awaitable value. We achieve this by returning a new
	// instance of the base class.
	w.Writefmtln("    # pylint: disable=using-constant-test")
	w.Writefmtln("    def __await__(self):")
	w.Writefmtln("        if False:")
	w.Writefmtln("            yield self")
	w.Writefmtln("        return %s(", baseName)
	for i, prop := range pot.properties {
		if i > 0 {
			w.Writefmtln(",")
		}
		pname := pycodegen.PyName(prop.name)
		w.Writefmt("            %s=self.%s", pname, pname)
	}
	w.Writefmtln(")")

	return awaitableName
}

func (g *pythonGenerator) emitResourceType(mod *module, res *resourceType) (string, error) {
	// Create a resource file for this resource's module.
	name := pycodegen.PyName(res.name)
	w, err := g.openWriter(mod, name+".py", true)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	baseType := "pulumi.CustomResource"
	if res.IsProvider() {
		baseType = "pulumi.ProviderResource"
	}

	if !res.IsProvider() && res.info.DeprecationMessage != "" {
		w.Writefmtln("warnings.warn(\"%s\", DeprecationWarning)", res.info.DeprecationMessage)
	}

	// Produce a class definition with optional """ comment.
	w.Writefmtln("class %s(%s):", pyClassName(res.name), baseType)
	g.emitMembers(w, mod, res)

	if res.info.DeprecationMessage != "" {
		w.Writefmtln("    warnings.warn(\"%s\", DeprecationWarning)", res.info.DeprecationMessage)
	}
	// Now generate an initializer with arguments for all input properties.
	w.Writefmt("    def __init__(__self__, resource_name, opts=None")

	// If there's an argument type, emit it.
	for _, prop := range res.inprops {
		w.Writefmt(", %s=None", pycodegen.PyName(prop.name))
	}

	// Old versions of TFGen emitted parameters named __name__ and __opts__. In order to preserve backwards
	// compatibility, we still emit them, but we don't emit documentation for them.
	w.Writefmt(", __props__=None")
	w.Writefmtln(", __name__=None, __opts__=None):")
	g.emitInitDocstring(w, mod, res)
	if res.info.DeprecationMessage != "" {
		w.Writefmtln("        pulumi.log.warn(\"%s is deprecated: %s\")", name, res.info.DeprecationMessage)
	}
	w.Writefmtln("        if __name__ is not None:")
	w.Writefmtln(`            warnings.warn("explicit use of __name__ is deprecated", DeprecationWarning)`)
	w.Writefmtln("            resource_name = __name__")
	w.Writefmtln("        if __opts__ is not None:")
	w.Writefmtln(
		`            warnings.warn("explicit use of __opts__ is deprecated, use 'opts' instead", DeprecationWarning)`)
	w.Writefmtln("            opts = __opts__")
	w.Writefmtln("        if opts is None:")
	w.Writefmtln("            opts = pulumi.ResourceOptions()")
	w.Writefmtln("        if not isinstance(opts, pulumi.ResourceOptions):")
	w.Writefmtln("            raise TypeError('Expected resource options to be a ResourceOptions instance')")
	w.Writefmtln("        if opts.version is None:")
	w.Writefmtln("            opts.version = utilities.get_version()")
	w.Writefmtln("        if opts.id is None:")
	w.Writefmtln("            if __props__ is not None:")
	w.Writefmt("                raise TypeError(")
	w.Writefmtln("'__props__ is only valid when passed in combination with a valid opts.id to get an existing resource')")
	w.Writefmtln("            __props__ = dict()")
	w.Writefmtln("")

	ins := make(map[string]bool)
	for _, prop := range res.inprops {
		pname := pycodegen.PyName(prop.name)

		// Fill in computed defaults for arguments.
		if defaultValue := pyDefaultValue(prop); defaultValue != "" {
			w.Writefmtln("            if %s is None:", pname)
			w.Writefmtln("                %s = %s", pname, defaultValue)
		}

		// Check that required arguments are present.
		if !prop.optional() {
			w.Writefmtln("            if %s is None:", pname)
			w.Writefmtln("                raise TypeError(\"Missing required property '%s'\")", pname)
		}

		// And add it to the dictionary.
		arg := pname

		// If this resource is a provider then, regardless of the schema of the underlying provider
		// type, we must project all properties as strings. For all properties that are not strings,
		// we'll marshal them to JSON and use the JSON string as a string input.
		//
		// Note the use of the `json` package here - we must import it at the top of the file so
		// that we can use it.
		if res.IsProvider() && prop.schema != nil && prop.schema.Type != schema.TypeString {
			arg = fmt.Sprintf("pulumi.Output.from_input(%s).apply(json.dumps) if %s is not None else None", arg, arg)
		}
		w.Writefmtln("            __props__['%s'] = %s", pname, arg)

		ins[prop.name] = true
	}

	for _, prop := range res.outprops {
		// Default any pure output properties to None.  This ensures they are available as properties, even if
		// they don't ever get assigned a real value, and get documentation if available.
		if !ins[prop.name] {
			w.Writefmtln("            __props__['%s'] = None", pycodegen.PyName(prop.name))
		}
	}

	if len(res.info.Aliases) > 0 {
		w.Writefmt(`        alias_opts = pulumi.ResourceOptions(aliases=[`)

		for i, alias := range res.info.Aliases {
			if i > 0 {
				w.Writefmt(", ")
			}

			g.writeAlias(w, alias)
		}

		w.Writefmtln(`])`)
		w.Writefmtln(`        opts = pulumi.ResourceOptions.merge(opts, alias_opts)`)
	}

	// Finally, chain to the base constructor, which will actually register the resource.
	w.Writefmtln("        super(%s, __self__).__init__(", res.name)
	w.Writefmtln("            '%s',", res.info.Tok)
	w.Writefmtln("            resource_name,")
	w.Writefmtln("            __props__,")
	w.Writefmtln("            opts)")
	w.Writefmtln("")

	w.Writefmtln("    @staticmethod")
	w.Writefmt("    def get(resource_name, id, opts=None")
	for _, prop := range res.outprops {
		w.Writefmt(", %[1]s=None", pycodegen.PyName(prop.name))
	}
	w.Writefmtln("):")
	g.emitGetDocstring(w, mod, res)
	w.Writefmtln(
		"        opts = pulumi.ResourceOptions.merge(opts, pulumi.ResourceOptions(id=id))")
	w.Writefmtln("")
	w.Writefmtln("        __props__ = dict()")

	for _, prop := range res.outprops {
		w.Writefmtln(`        __props__["%[1]s"] = %[1]s`, pycodegen.PyName(prop.name))
	}

	w.Writefmtln("        return %s(resource_name, opts=opts, __props__=__props__)", pyClassName(res.name))

	// Override translate_{input|output}_property on each resource to translate between snake case and
	// camel case when interacting with tfbridge.
	w.Writefmtln(
		`    def translate_output_property(self, prop):
        return tables._CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop

    def translate_input_property(self, prop):
        return tables._SNAKE_TO_CAMEL_CASE_TABLE.get(prop) or prop
`)

	return name, nil
}

func (g *pythonGenerator) writeAlias(w *tools.GenWriter, alias tfbridge.AliasInfo) {
	w.WriteString("pulumi.Alias(")
	parts := []string{}
	if alias.Name != nil {
		parts = append(parts, fmt.Sprintf("name=\"%v\"", *alias.Name))
	}
	if alias.Project != nil {
		parts = append(parts, fmt.Sprintf("project=\"%v\"", *alias.Project))
	}
	if alias.Type != nil {
		parts = append(parts, fmt.Sprintf("type_=\"%v\"", *alias.Type))
	}

	for i, part := range parts {
		if i > 0 {
			w.Writefmt(", ")
		}

		w.WriteString(part)
	}

	w.WriteString(")")
}

func (g *pythonGenerator) emitResourceFunc(mod *module, fun *resourceFunc) (string, error) {
	name := pycodegen.PyName(fun.name)
	w, err := g.openWriter(mod, name+".py", true)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	if fun.info.DeprecationMessage != "" {
		w.Writefmtln("warnings.warn(\"%s\", DeprecationWarning)", fun.info.DeprecationMessage)
	}

	// If there is a return type, emit it.
	retTypeName := ""
	if fun.retst != nil {
		retTypeName = g.emitAwaitableType(w, fun.retst)
		w.Writefmtln("")
	}

	// Write out the function signature.
	w.Writefmt("def %s(", name)
	for _, arg := range fun.args {
		w.Writefmt("%s=None,", pycodegen.PyName(arg.name))
	}
	w.Writefmt("opts=None")
	w.Writefmtln("):")

	var buf bytes.Buffer
	// If this func has documentation, write it at the top of the docstring, otherwise use a generic comment.
	if fun.doc != "" && fun.doc != elidedDocComment {
		fmt.Fprintln(&buf, fun.doc)
	} else {
		fmt.Fprintln(&buf, "Use this data source to access information about an existing resource.")
	}

	if len(fun.args) > 0 {
		fmt.Fprintln(&buf, "")
	}
	for _, arg := range fun.args {
		g.emitPropDocstring(&buf, arg, false /*wrapInput*/)
	}

	// Nested structures are typed as `dict` so we include some extra documentation for these structures.
	g.emitNestedStructuresDocstring(&buf, fun.args, false /*wrapInput*/)

	g.emitDocComment(w, buf.String(), fun.docURL, "    ")

	if fun.info.DeprecationMessage != "" {
		w.Writefmtln("    pulumi.log.warn(\"%s is deprecated: %s\")", name, fun.info.DeprecationMessage)
	}

	// Copy the function arguments into a dictionary.
	w.Writefmtln("    __args__ = dict()")
	w.Writefmtln("")
	for _, arg := range fun.args {
		// TODO: args validation.
		w.Writefmtln("    __args__['%s'] = %s", arg.name, pycodegen.PyName(arg.name))
	}

	// If the caller explicitly specified a version, use it, otherwise inject this package's version.
	w.Writefmtln("    if opts is None:")
	w.Writefmtln("        opts = pulumi.InvokeOptions()")
	w.Writefmtln("    if opts.version is None:")
	w.Writefmtln("        opts.version = utilities.get_version()")

	// Now simply invoke the runtime function with the arguments.
	w.Writefmtln("    __ret__ = pulumi.runtime.invoke('%s', __args__, opts=opts).value", fun.info.Tok)
	w.Writefmtln("")

	// And copy the results to an object, if there are indeed any expected returns.
	if fun.retst != nil {
		w.Writefmtln("    return %s(", retTypeName)
		for i, ret := range fun.rets {
			w.Writefmt("        %s=__ret__.get('%s')", pycodegen.PyName(ret.name), ret.name)
			if i == len(fun.rets)-1 {
				w.Writefmtln(")")
			} else {
				w.Writefmtln(",")
			}
		}
	}

	return name, nil
}

// emitOverlay copies an overlay from its source to the target, and returns the resulting file to be exported.
func (g *pythonGenerator) emitOverlay(mod *module, overlay *overlayFile) (string, error) {
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

var requirementRegex = regexp.MustCompile(`^>=([^,]+),<[^,]+$`)
var pep440AlphaRegex = regexp.MustCompile(`^(\d+\.\d+\.\d)+a(\d+)$`)
var pep440BetaRegex = regexp.MustCompile(`^(\d+\.\d+\.\d+)b(\d+)$`)
var pep440RCRegex = regexp.MustCompile(`^(\d+\.\d+\.\d+)rc(\d+)$`)
var pep440DevRegex = regexp.MustCompile(`^(\d+\.\d+\.\d+)\.dev(\d+)$`)

var oldestAllowedPulumi = semver.Version{
	Major: 0,
	Minor: 17,
	Patch: 28,
}

// emitPackageMetadata generates all the non-code metadata required by a Pulumi package.
func (g *pythonGenerator) emitPackageMetadata(pack *pkg) error {
	// The generator already emitted Pulumi.yaml, so that just leaves the `setup.py`.
	w, err := tools.NewGenWriter(tfgen, filepath.Join(g.outDir, "setup.py"))
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	// Emit a standard warning header ("do not edit", etc).
	w.EmitHeaderWarning(g.commentChars())

	// Now create a standard Python package from the metadata.
	w.Writefmtln("import errno")
	w.Writefmtln("from setuptools import setup, find_packages")
	w.Writefmtln("from setuptools.command.install import install")
	w.Writefmtln("from subprocess import check_call")
	w.Writefmtln("")

	// Create a command that will install the Pulumi plugin for this resource provider.
	w.Writefmtln("class InstallPluginCommand(install):")
	w.Writefmtln("    def run(self):")
	w.Writefmtln("        install.run(self)")
	w.Writefmtln("        try:")
	w.Writefmtln("            check_call(['pulumi', 'plugin', 'install', 'resource', '%s', '${PLUGIN_VERSION}'])",
		pack.name)
	w.Writefmtln("        except OSError as error:")
	w.Writefmtln("            if error.errno == errno.ENOENT:")
	w.Writefmtln("                print(\"\"\"")
	w.Writefmtln("                There was an error installing the %s resource provider plugin.", pack.name)
	w.Writefmtln("                It looks like `pulumi` is not installed on your system.")
	w.Writefmtln("                Please visit https://pulumi.com/ to install the Pulumi CLI.")
	w.Writefmtln("                You may try manually installing the plugin by running")
	w.Writefmtln("                `pulumi plugin install resource %s ${PLUGIN_VERSION}`", pack.name)
	w.Writefmtln("                \"\"\")")
	w.Writefmtln("            else:")
	w.Writefmtln("                raise")
	w.Writefmtln("")

	// Generate a readme method which will load README.rst, we use this to fill out the
	// long_description field in the setup call.
	w.Writefmtln("def readme():")
	w.Writefmtln("    with open('README.md', encoding='utf-8') as f:")
	w.Writefmtln("        return f.read()")
	w.Writefmtln("")

	// Finally, the actual setup part.
	w.Writefmtln("setup(name='%s',", pyPack(pack.name))
	w.Writefmtln("      version='${VERSION}',")
	if g.info.Description != "" {
		w.Writefmtln("      description='%s',", generateManifestDescription(g.info))
	}
	w.Writefmtln("      long_description=readme(),")
	w.Writefmtln("      long_description_content_type='text/markdown',")
	w.Writefmtln("      cmdclass={")
	w.Writefmtln("          'install': InstallPluginCommand,")
	w.Writefmtln("      },")
	if g.info.Keywords != nil {
		w.Writefmt("      keywords='")
		for i, kw := range g.info.Keywords {
			if i > 0 {
				w.Writefmt(" ")
			}
			w.Writefmt(kw)
		}
		w.Writefmtln("',")
	}
	if g.info.Homepage != "" {
		w.Writefmtln("      url='%s',", g.info.Homepage)
	}
	if g.info.Repository != "" {
		w.Writefmtln("      project_urls={")
		w.Writefmtln("          'Repository': '%s'", g.info.Repository)
		w.Writefmtln("      },")
	}
	if g.info.License != "" {
		w.Writefmtln("      license='%s',", g.info.License)
	}
	w.Writefmtln("      packages=find_packages(),")

	// Publish type metadata: PEP 561
	w.Writefmtln("      package_data={")
	w.Writefmtln("			'%s': [", pyPack(pack.name))
	w.Writefmtln("				'py.typed'")
	w.Writefmtln("			]")
	w.Writefmtln("		},")

	// Emit all requires clauses.
	var reqs map[string]string
	if g.info.Python != nil {
		reqs = g.info.Python.Requires
	} else {
		reqs = make(map[string]string)
	}

	// Ensure that the Pulumi SDK has an entry if not specified. If the SDK _is_ specified, ensure
	// that it specifies an acceptable version range.
	if pulumiReq, ok := reqs["pulumi"]; ok {
		// We expect a specific pattern of ">=version,<version" here.
		matches := requirementRegex.FindStringSubmatch(pulumiReq)
		if len(matches) != 2 {
			return errors.Errorf("invalid requirement specifier \"%s\"; expected \">=version1,<version2\"", pulumiReq)
		}

		lowerBound, err := pep440VersionToSemver(matches[1])
		if err != nil {
			return errors.Errorf("invalid version for lower bound: %v", err)
		}
		if lowerBound.LT(oldestAllowedPulumi) {
			return errors.Errorf("lower version bound must be at least %v", oldestAllowedPulumi)
		}
	} else {
		reqs["pulumi"] = ""
	}

	// Sort the entries so they are deterministic.
	reqnames := []string{
		"semver>=2.8.1",
		"parver>=0.2.1",
	}
	for req := range reqs {
		reqnames = append(reqnames, req)
	}
	sort.Strings(reqnames)

	w.Writefmtln("      install_requires=[")
	for i, req := range reqnames {
		var comma string
		if i < len(reqnames)-1 {
			comma = ","
		}
		w.Writefmtln("          '%s%s'%s", req, reqs[req], comma)
	}
	w.Writefmtln("      ],")

	w.Writefmtln("      zip_safe=False)")
	return nil
}

// emitPythonTypes generates an empty py.typed file to signal type checking in accordance with PEP 561.
func (g *pythonGenerator) emitPythonTypes(pack *pkg) error {
	fileSuffix := fmt.Sprintf("%s/py.typed", pyPack(pack.name))
	w, err := tools.NewGenWriter(tfgen, filepath.Join(g.outDir, fileSuffix))
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	return nil
}

func pep440VersionToSemver(v string) (semver.Version, error) {
	switch {
	case pep440AlphaRegex.MatchString(v):
		parts := pep440AlphaRegex.FindStringSubmatch(v)
		v = parts[1] + "-alpha." + parts[2]
	case pep440BetaRegex.MatchString(v):
		parts := pep440BetaRegex.FindStringSubmatch(v)
		v = parts[1] + "-beta." + parts[2]
	case pep440RCRegex.MatchString(v):
		parts := pep440RCRegex.FindStringSubmatch(v)
		v = parts[1] + "-rc." + parts[2]
	case pep440DevRegex.MatchString(v):
		parts := pep440DevRegex.FindStringSubmatch(v)
		v = parts[1] + "-dev." + parts[2]
	}

	return semver.ParseTolerant(v)
}

// Emits property conversion tables for all properties recorded using `recordProperty`. The two tables emitted here are
// used to convert to and from snake case and camel case.
func (g *pythonGenerator) emitPropertyConversionTables(tableModule *module) error {
	w, err := g.openWriter(tableModule, "tables.py", false)
	if err != nil {
		return err
	}

	defer contract.IgnoreClose(w)
	var allKeys []string
	for key := range g.snakeCaseToCamelCase {
		allKeys = append(allKeys, key)
	}
	sort.Strings(allKeys)

	w.Writefmtln("_SNAKE_TO_CAMEL_CASE_TABLE = {")
	for _, key := range allKeys {
		value := g.snakeCaseToCamelCase[key]
		if key != value {
			w.Writefmtln("    %q: %q,", key, value)
		}
	}
	w.Writefmtln("}")
	w.Writefmtln("\n_CAMEL_TO_SNAKE_CASE_TABLE = {")
	for _, value := range allKeys {
		key := g.snakeCaseToCamelCase[value]
		if key != value {
			w.Writefmtln("    %q: %q,", key, value)
		}
	}
	w.Writefmtln("}")
	return nil
}

// recordProperty records the given property's name and member names. For each property name contained in the given
// property, the name is converted to snake case and recorded in the snake case to camel case table.
//
// Once all resources have been emitted, the table is written out to a format usable for implementations of
// translate_input_property and translate_output_property.
func (g *pythonGenerator) recordProperty(prop *variable) {
	snakeCaseName := pycodegen.PyName(prop.name)
	g.snakeCaseToCamelCase[snakeCaseName] = prop.name
	g.camelCaseToSnakeCase[prop.name] = snakeCaseName

	// Due to bugs in earlier versions of the bride, we only recurse on properties with TypeMap and an object-typed
	// element. This is consistent with the earlier behavior.
	if prop.schema.Type == schema.TypeMap && prop.typ.kind == kindObject {
		for _, p := range prop.typ.properties {
			g.recordProperty(p)
		}
	}
}

// emitMembers emits property declarations and docstrings for all output properties of the given resource.
func (g *pythonGenerator) emitMembers(w *tools.GenWriter, mod *module, res *resourceType) {
	for _, prop := range res.outprops {
		name := pycodegen.PyName(prop.name)
		ty := pyType(prop)
		w.Writefmtln("    %s: pulumi.Output[%s]", name, ty)
		if prop.doc != "" {
			doc := prop.doc

			nested := nestedStructure(prop)
			if len(nested) > 0 {
				doc = fmt.Sprintf("%s\n\n", doc)

				var buf bytes.Buffer
				g.emitNestedStructureBullets(&buf, nested, "  ", false /*wrapInput*/)
				doc = fmt.Sprintf("%s%s", doc, buf.String())
			}

			g.emitDocComment(w, doc, prop.docURL, "    ")
		}
	}
}

// emitInitDocstring emits the docstring for the __init__ method of the given resource type.
//
// Sphinx (the documentation generator that we use to generate Python docs) does not draw a
// distinction between documentation comments on the class itself and documentation comments on the
// __init__ method of a class. The docs repo instructs Sphinx to concatenate the two together, which
// means that we don't need to emit docstrings on the class at all as long as the __init__ docstring
// is good enough.
//
// The docstring we generate here describes both the class itself and the arguments to the class's
// constructor. The format of the docstring is in "Sphinx form":
//   1. Parameters are introduced using the syntax ":param <type> <name>: <comment>". Sphinx parses this and uses it
//      to populate the list of parameters for this function.
//   2. The doc string of parameters is expected to be indented to the same indentation as the type of the parameter.
//      Sphinx will complain and make mistakes if this is not the case.
//   3. The doc string can't have random newlines in it, or Sphinx will complain.
//
// This function does the best it can to navigate these constraints and produce a docstring that
// Sphinx can make sense of.
func (g *pythonGenerator) emitInitDocstring(w *tools.GenWriter, mod *module, res *resourceType) {
	// "buf" contains the full text of the docstring, without the leading and trailing triple quotes.
	var buf bytes.Buffer

	// If this resource has documentation, write it at the top of the docstring, otherwise use a generic comment.
	if res.doc != "" && res.doc != elidedDocComment {
		fmt.Fprintln(&buf, res.doc)
	} else {
		fmt.Fprintf(&buf, "Create a %s resource with the given unique name, props, and options.\n", res.name)
	}
	fmt.Fprintln(&buf, "")

	// All resources have a resource_name parameter and opts parameter.
	fmt.Fprintln(&buf, ":param str resource_name: The name of the resource.")
	fmt.Fprintln(&buf, ":param pulumi.ResourceOptions opts: Options for the resource.")
	for _, prop := range res.inprops {
		g.emitPropDocstring(&buf, prop, true /*wrapInput*/)
	}

	// Nested structures are typed as `dict` so we include some extra documentation for these structures.
	g.emitNestedStructuresDocstring(&buf, res.inprops, true /*wrapInput*/)

	// emitDocComment handles the prefix and triple quotes.
	g.emitDocComment(w, buf.String(), res.docURL, "        ")
}

func (g *pythonGenerator) emitGetDocstring(w *tools.GenWriter, mod *module, res *resourceType) {
	// "buf" contains the full text of the docstring, without the leading and trailing triple quotes.
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "Get an existing %s resource's state with the given name, id, and optional extra\n"+
		"properties used to qualify the lookup.\n", res.name)
	fmt.Fprintln(&buf, "")

	fmt.Fprintln(&buf, ":param str resource_name: The unique name of the resulting resource.")
	fmt.Fprintln(&buf, ":param str id: The unique provider ID of the resource to lookup.")
	fmt.Fprintln(&buf, ":param pulumi.ResourceOptions opts: Options for the resource.")
	for _, prop := range res.outprops {
		g.emitPropDocstring(&buf, prop, true /*wrapInput*/)
	}

	// Nested structures are typed as `dict` so we include some extra documentation for these structures.
	g.emitNestedStructuresDocstring(&buf, res.outprops, true /*wrapInput*/)

	// emitDocComment handles the prefix and triple quotes.
	g.emitDocComment(w, buf.String(), res.docURL, "        ")
}

func (g *pythonGenerator) emitPropDocstring(buf io.Writer, prop *variable, wrapInput bool) {
	if prop.doc == "" || prop.doc == elidedDocComment {
		return
	}

	name := pycodegen.PyName(prop.name)
	ty := pyType(prop)
	if wrapInput {
		ty = fmt.Sprintf("pulumi.Input[%s]", ty)
	}

	// If this property has some documentation associated with it, we need to split it so that it is indented
	// in a way that Sphinx can understand.
	lines := strings.Split(prop.doc, "\n")
	for i, docLine := range lines {
		// Break if we get to the last line and it's empty.
		if i == len(lines)-1 && strings.TrimSpace(docLine) == "" {
			break
		}

		// If it's the first line, print the :param header.
		if i == 0 {
			fmt.Fprintf(buf, ":param %s %s: %s\n", ty, name, docLine)
		} else {
			// Otherwise, print out enough padding to align with the first char of the type.
			fmt.Fprintf(buf, "       %s\n", docLine)
		}
	}
}

func (g *pythonGenerator) emitNestedStructuresDocstring(buf io.Writer, props []*variable, wrapInput bool) {
	var names []string
	nestedMap := make(map[string][]*nestedVariable)
	for _, prop := range props {
		nested := nestedStructure(prop)
		if len(nested) > 0 {
			nestedMap[prop.Name()] = nested
			names = append(names, prop.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		nested := nestedMap[name]
		fmt.Fprintf(buf, "\nThe **%s** object supports the following:\n\n", pycodegen.PyName(name))
		g.emitNestedStructureBullets(buf, nested, "  ", wrapInput)
	}
}

func (g *pythonGenerator) emitNestedStructureBullets(buf io.Writer, nested []*nestedVariable, indent string,
	wrapInput bool) {

	if len(nested) == 0 {
		return
	}

	for i, nes := range nested {
		name := nes.prop.Name()
		if snake, ok := g.camelCaseToSnakeCase[name]; ok {
			name = snake
		}

		typ := pyType(nes.prop)
		if wrapInput {
			typ = fmt.Sprintf("pulumi.Input[%s]", typ)
		}

		docPrefix := " - "
		if nes.prop.doc == "" {
			// If there's no doc available, just write a new line.
			docPrefix = "\n"
		}
		fmt.Fprintf(buf, "%s* `%s` (`%s`)%s", indent, name, typ, docPrefix)

		// If this property has some documentation associated with it, we need to split it so that it is indented
		// in a way that Sphinx can understand.
		lines := strings.Split(nes.prop.doc, "\n")
		for j, docLine := range lines {
			// Break if we get to the last line and it's empty.
			if j == len(lines)-1 && strings.TrimSpace(docLine) == "" {
				break
			}

			// If it's the first line, print it.
			if j == 0 {
				fmt.Fprintln(buf, docLine)
			} else {
				// Otherwise, pad with a couple spaces so the text is aligned with the bullet text above.
				fmt.Fprintf(buf, "%s  %s\n", indent, docLine)
			}
		}

		if len(nes.nested) > 0 {
			fmt.Fprintln(buf, "")
			g.emitNestedStructureBullets(buf, nes.nested, indent+"  ", wrapInput)
			if i < len(nested)-1 {
				fmt.Fprintln(buf, "")
			}
		}
	}
}

type nestedVariable struct {
	prop   *variable
	nested []*nestedVariable
}

func nestedStructure(v *variable) []*nestedVariable {
	return nestedStructureFromPropertyType(v.typ)
}

func nestedStructureFromPropertyType(typ *propertyType) []*nestedVariable {
	if typ == nil {
		return nil
	}

	switch typ.kind {
	case kindSet, kindList:
		return nestedStructureFromPropertyType(typ.element)
	case kindObject:
		nested := make([]*nestedVariable, len(typ.properties))
		for i, p := range typ.properties {
			nested[i] = &nestedVariable{
				prop:   p,
				nested: nestedStructure(p),
			}
		}
		return nested
	default:
		return nil
	}
}

// pyType returns the expected runtime type for the given variable.  Of course, being a dynamic language, this
// check is not exhaustive, but it should be good enough to catch 80% of the cases early on.
func pyType(v *variable) string {
	return pyTypeFromPropertyType(v.typ)
}

func pyTypeFromPropertyType(typ *propertyType) string {
	// If this is an asset or archive type, return the proper Pulumi SDK type name.
	if typ.asset != nil {
		if typ.asset.IsArchive() {
			return "pulumi." + typ.asset.Type()
		}
		return "Union[pulumi.Asset, pulumi.Archive]"
	}

	switch typ.kind {
	case kindBool:
		return "bool"
	case kindInt, kindFloat:
		return "float"
	case kindString:
		return "str"
	case kindSet, kindList:
		return "list"
	default:
		return "dict"
	}
}

// pyPack returns the suggested package name for the given string.
func pyPack(s string) string {
	return "pulumi_" + s
}

// pyClassName turns a raw name into one that is suitable as a Python class name.
func pyClassName(name string) string {
	return pycodegen.EnsureKeywordSafe(name)
}

func pyPrimitiveValue(value interface{}) (string, error) {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			return "True", nil
		}
		return "False", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64), nil
	case reflect.String:
		return fmt.Sprintf("'%s'", v.String()), nil
	default:
		return "", errors.Errorf("unsupported default value of type %T", value)
	}
}

func pyDefaultValue(v *variable) string {
	defaultValue := ""
	if v.info == nil || v.info.Default == nil {
		return defaultValue
	}
	defaults := v.info.Default

	if defaults.Value != nil {
		dv, err := pyPrimitiveValue(defaults.Value)
		if err != nil {
			cmdutil.Diag().Warningf(diag.Message("", err.Error()))
			return defaultValue
		}
		defaultValue = dv
	}

	if len(defaults.EnvVars) > 0 {
		envFunc := "utilities.get_env"
		switch v.schema.Type {
		case schema.TypeBool:
			envFunc = "utilities.get_env_bool"
		case schema.TypeInt:
			envFunc = "utilities.get_env_int"
		case schema.TypeFloat:
			envFunc = "utilities.get_env_float"
		}

		envVars := fmt.Sprintf("'%s'", defaults.EnvVars[0])
		for _, e := range defaults.EnvVars[1:] {
			envVars += fmt.Sprintf(", '%s'", e)
		}
		if defaultValue == "" {
			defaultValue = fmt.Sprintf("%s(%s)", envFunc, envVars)
		} else {
			defaultValue = fmt.Sprintf("(%s(%s) or %s)", envFunc, envVars, defaultValue)
		}
	}

	return defaultValue
}
