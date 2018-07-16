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
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/golang/glog"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/tools"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pulumi/pulumi-terraform/pkg/tfbridge"
)

// newPythonGenerator returns a language generator that understands how to produce Python packages.
func newPythonGenerator(pkg, version string, info tfbridge.ProviderInfo, overlaysDir, outDir string) langGenerator {
	return &pythonGenerator{
		pkg:         pkg,
		version:     version,
		info:        info,
		overlaysDir: overlaysDir,
		outDir:      outDir,
	}
}

type pythonGenerator struct {
	pkg         string
	version     string
	info        tfbridge.ProviderInfo
	overlaysDir string
	outDir      string
}

// commentChars returns the comment characters to use for single-line comments.
func (g *pythonGenerator) commentChars() string {
	return "#"
}

// moduleDir returns the directory for the given module.
func (g *pythonGenerator) moduleDir(mod *module) string {
	dir := filepath.Join(g.outDir, pyPack(g.pkg))
	if mod.name != "" {
		dir = filepath.Join(dir, pyName(mod.name))
	}
	return dir
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
		g.emitSDKImport(w)
	}

	return w, nil
}

func (g *pythonGenerator) emitSDKImport(w *tools.GenWriter) {
	w.Writefmtln("import pulumi")
	w.Writefmtln("import pulumi.runtime")
	w.Writefmtln("")
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
	if err := g.emitModule(index, submodules); err != nil {
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

	// Keep track of any immediately exported package modules, as we will re-export them.
	var exports []string

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

	// Lastly, generate an index file for this module.
	if err := g.emitIndex(mod, exports, submods); err != nil {
		return errors.Wrapf(err, "emitting module %s index", mod.name)
	}

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
		w.Writefmtln("# Make subpackages available:")
		w.Writefmt("__all__ = [")
		for i, sub := range submods {
			if i > 0 {
				w.Writefmt(", ")
			}
			w.Writefmt("'%s'", sub)
		}
		w.Writefmtln("]")
	}

	// Now, import anything to export flatly that is a direct export rather than sub-module.
	if len(exports) > 0 {
		if len(submods) > 0 {
			w.Writefmtln("")
		}
		w.Writefmtln("# Export this package's modules as members:")
		for _, exp := range exports {
			w.Writefmtln("from %s import *", exp)
		}
	}

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
	var getfunc string
	if v.optional() {
		getfunc = "get"
	} else {
		getfunc = "require"
	}
	w.Writefmtln("%s = __config__.%s('%s')", pyName(v.name), getfunc, v.name)
	if v.doc != "" {
		g.emitDocComment(w, v.doc, "")
	} else if v.rawdoc != "" {
		g.emitRawDocComment(w, v.rawdoc, "")
	}
	w.Writefmtln("")
}

func (g *pythonGenerator) emitDocComment(w *tools.GenWriter, comment, prefix string) {
	if comment != "" {
		var written bool
		lines := strings.Split(comment, "\n")
		for i, docLine := range lines {
			// Break if we get to the last line and it's empty
			if i == len(lines)-1 && strings.TrimSpace(docLine) == "" {
				break
			}

			// If the first line, start a doc comment.
			if i == 0 {
				w.Writefmtln(`%s"""`, prefix)
				written = true
			}

			// Print the line of documentation
			w.Writefmtln("%s%s", prefix, docLine)
		}
		if written {
			w.Writefmtln(`%s"""`, prefix)
		}
	}
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

func (g *pythonGenerator) emitPlainOldType(w *tools.GenWriter, pot *plainOldType) {
	// Produce a class definition with optional """ comment.
	w.Writefmtln("class %s(object):", pyClassName(pot.name))
	if pot.doc != "" {
		g.emitDocComment(w, pot.doc, "    ")
	}

	// Now generate an initializer with properties for all inputs.
	w.Writefmt("    def __init__(__self__")
	for _, prop := range pot.props {
		w.Writefmt(", %s=None", pyName(prop.name))
	}
	w.Writefmtln("):")
	for _, prop := range pot.props {
		// Check that required arguments are present.  Also check that types are as expected.
		pname := pyName(prop.name)
		ptype := pyType(prop)
		w.Writefmtln("        if %s and not isinstance(%s, %s):", pname, pname, ptype)
		w.Writefmtln("            raise TypeError('Expected argument %s to be a %s')", pname, ptype)

		// Now perform the assignment, and follow it with a """ doc comment if there was one found.
		w.Writefmtln("        __self__.%[1]s = %[1]s", pname)
		if prop.doc != "" {
			g.emitDocComment(w, prop.doc, "        ")
		} else if prop.rawdoc != "" {
			g.emitRawDocComment(w, prop.rawdoc, "        ")
		}
	}
}

func (g *pythonGenerator) emitResourceType(mod *module, res *resourceType) (string, error) {
	// Create a resource file for this resource's module.
	name := pyName(res.name)
	w, err := g.openWriter(mod, name+".py", true)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	// Produce a class definition with optional """ comment.
	w.Writefmtln("class %s(pulumi.CustomResource):", pyClassName(res.name))
	if res.doc != "" {
		g.emitDocComment(w, res.doc, "    ")
	}

	// Now generate an initializer with arguments for all input properties.
	w.Writefmt("    def __init__(__self__, __name__, __opts__=None")

	// If there's an argument type, emit it.
	for _, prop := range res.inprops {
		w.Writefmt(", %s=None", pyName(prop.name))
	}
	w.Writefmtln("):")

	w.Writefmtln(`        """Create a %s resource with the given unique name, props, and options."""`, res.name)
	w.Writefmtln("        if not __name__:")
	w.Writefmtln("            raise TypeError('Missing resource name argument (for URN creation)')")
	w.Writefmtln("        if not isinstance(__name__, basestring):")
	w.Writefmtln("            raise TypeError('Expected resource name to be a string')")
	w.Writefmtln("        if __opts__ and not isinstance(__opts__, pulumi.ResourceOptions):")
	w.Writefmtln("            raise TypeError('Expected resource options to be a ResourceOptions instance')")
	w.Writefmtln("")

	// Now copy all properties to a dictionary, in preparation for passing it to the base function.  Along
	// the way, we will perform argument validation.
	w.Writefmtln("        __props__ = dict()")
	w.Writefmtln("")

	ins := make(map[string]bool)
	for _, prop := range res.inprops {
		// Check that required arguments are present.  Also check that types are as expected.
		pname := pyName(prop.name)
		ptype := pyType(prop)
		if !prop.optional() {
			w.Writefmtln("        if not %s:", pname)
			w.Writefmtln("            raise TypeError('Missing required property %s')", pname)
			w.Writefmt("        elif ")
		} else {
			w.Writefmt("        if %s and ", pname)
		}
		w.Writefmtln("not isinstance(%s, %s):", pname, ptype)
		w.Writefmtln("            raise TypeError('Expected property %s to be a %s')", pname, ptype)

		// Now perform the assignment, and follow it with a """ doc comment if there was one found.
		w.Writefmtln("        __self__.%[1]s = %[1]s", pname)
		if prop.doc != "" {
			g.emitDocComment(w, prop.doc, "        ")
		} else if prop.rawdoc != "" {
			g.emitRawDocComment(w, prop.rawdoc, "        ")
		}

		// And add it to the dictionary.
		w.Writefmtln("        __props__['%s'] = %s", prop.name, pname)
		w.Writefmtln("")

		ins[prop.name] = true
	}

	var wroteOuts bool
	for _, prop := range res.outprops {
		// Default any pure output properties to UNKNOWN.  This ensures they are available as properties, even if
		// they don't ever get assigned a real value, and get documentation if available.
		if !ins[prop.name] {
			w.Writefmtln("        __self__.%s = pulumi.runtime.UNKNOWN", pyName(prop.name))
			if prop.doc != "" {
				g.emitDocComment(w, prop.doc, "        ")
			} else if prop.rawdoc != "" {
				g.emitRawDocComment(w, prop.rawdoc, "        ")
			}
			wroteOuts = true
		}
	}
	if wroteOuts {
		w.Writefmtln("")
	}

	// Finally, chain to the base constructor, which will actually register the resource.
	w.Writefmtln("        super(%s, __self__).__init__(", res.name)
	w.Writefmtln("            '%s',", res.info.Tok)
	w.Writefmtln("            __name__,")
	w.Writefmtln("            __props__,")
	w.Writefmtln("            __opts__)")
	w.Writefmtln("")

	// Now override the set_outputs function so that this resource can demangle names to assign output properties.
	w.Writefmtln("    def set_outputs(self, outs):")
	for _, prop := range res.outprops {
		w.Writefmtln("        if '%s' in outs:", prop.name)
		w.Writefmtln("            self.%s = outs['%s']", pyName(prop.name), prop.name)
	}

	return name, nil
}

func (g *pythonGenerator) emitResourceFunc(mod *module, fun *resourceFunc) (string, error) {
	// Create a vars.ts file into which all configuration variables will go.
	name := pyName(fun.name)
	w, err := g.openWriter(mod, name+".py", true)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	// If there is a return type, emit it.
	if fun.retst != nil {
		g.emitPlainOldType(w, fun.retst)
		w.Writefmtln("")
	}

	// Write out the function signature.
	w.Writefmt("def %s(", name)
	for i, arg := range fun.args {
		if i > 0 {
			w.Writefmt(", ")
		}
		w.Writefmt("%s=None", pyName(arg.name))
	}
	w.Writefmtln("):")

	// Write the TypeDoc/JSDoc for the data source function.
	if fun.doc != "" {
		g.emitDocComment(w, fun.doc, "    ")
	}

	// Copy the function arguments into a dictionary.
	w.Writefmtln("    __args__ = dict()")
	w.Writefmtln("")
	for _, arg := range fun.args {
		// TODO: args validation.
		w.Writefmtln("    __args__['%s'] = %s", arg.name, pyName(arg.name))
	}

	// Now simply invoke the runtime function with the arguments.
	w.Writefmtln("    __ret__ = pulumi.runtime.invoke('%s', __args__)", fun.info.Tok)
	w.Writefmtln("")

	// And copy the results to an object, if there are indeed any expected returns.
	if fun.retst != nil {
		w.Writefmtln("    return %s(", fun.retst.name)
		for i, ret := range fun.rets {
			w.Writefmt("        %s=__ret__.get('%s')", pyName(ret.name), ret.name)
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
	if err := copyFile(overlay.src, dst); err != nil {
		return "", err
	}

	// And then export the overlay's contents from the index.
	return dst, nil
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
	w.Writefmtln("from setuptools import setup, find_packages")
	w.Writefmtln("from setuptools.command.install import install")
	w.Writefmtln("from subprocess import check_call")
	w.Writefmtln("")

	// Create a command that will install the Pulumi plugin for this resource provider.
	w.Writefmtln("class InstallPluginCommand(install):")
	w.Writefmtln("    def run(self):")
	w.Writefmtln("        install.run(self)")
	w.Writefmtln("        check_call(['pulumi', 'plugin', 'install', 'resource', '%s', '${PLUGIN_VERSION}'])",
		pack.name)
	w.Writefmtln("")

	// Generate a readme method which will load README.rst, we use this to fill out the
	// long_description field in the setup call.
	w.Writefmtln("def readme():")
	w.Writefmtln("    with open('README.rst') as f:")
	w.Writefmtln("        return f.read()")
	w.Writefmtln("")

	// Finally, the actual setup part.
	w.Writefmtln("setup(name='%s',", pyPack(pack.name))
	w.Writefmtln("      version='${VERSION}',")
	if g.info.Description != "" {
		w.Writefmtln("      description='%s',", g.info.Description)
	}
	w.Writefmtln("      long_description=readme(),")
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

	// Emit all requires clauses.
	var reqs map[string]string
	if g.info.Python != nil {
		reqs = g.info.Python.Requires
	} else {
		reqs = make(map[string]string)
	}

	// Ensure that the Pulumi SDK has an entry if not specified.
	if _, ok := reqs["pulumi"]; !ok {
		reqs["pulumi"] = ""
	}

	// Sort the entries so they are deterministic.
	var reqnames []string
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

// pyType returns the expected runtime type for the given variable.  Of course, being a dynamic language, this
// check is not exhaustive, but it should be good enough to catch 80% of the cases early on.
func pyType(v *variable) string {
	return pyTypeFromSchema(v.schema, v.info)
}

func pyTypeFromSchema(sch *schema.Schema, info *tfbridge.SchemaInfo) string {
	// If this is an asset or archive type, return the proper Pulumi SDK type name.
	if info != nil && info.Asset != nil {
		return "pulumi." + info.Asset.Type()
	}

	switch sch.Type {
	case schema.TypeBool:
		return "bool"
	case schema.TypeInt:
		return "int"
	case schema.TypeFloat:
		return "float"
	case schema.TypeString:
		return "basestring"
	case schema.TypeSet, schema.TypeList:
		if tfbridge.IsMaxItemsOne(sch, info) {
			// This isn't supposed to be projected as a list; project it as a scalar.
			if elem, ok := sch.Elem.(*schema.Schema); ok {
				var schInfo *tfbridge.SchemaInfo
				if info != nil {
					schInfo = info.Elem
				}
				// If the elem is a schema type, see if we can do better than just "dict".
				return pyTypeFromSchema(elem, schInfo)
			}
			// Otherwise, return "dict".
			return "dict"
		}
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
	return ensurePythonKeywordSafe(name)
}

// pyName turns a variable or function name, normally using camelCase, to an underscore_case name.
func pyName(name string) string {
	// This method is a state machine with four states:
	//   stateFirst - the initial state.
	//   stateUpper - The last character we saw was an uppercase letter and the character before it
	//                was either a number or a lowercase letter.
	//   stateAcronym - The last character we saw was an uppercase letter and the character before it
	//                  was an uppercase letter.
	//   stateLowerOrNumber - The last character we saw was a lowercase letter or a number.
	//
	// The following are the state transitions of this state machine:
	//   stateFirst -> (uppercase letter) -> stateUpper
	//   stateFirst -> (lowercase letter or number) -> stateLowerOrNumber
	//      Append the lower-case form of the character to currentComponent.
	//
	//   stateUpper -> (uppercase letter) -> stateAcronym
	//   stateUpper -> (lowercase letter or number) -> stateLowerOrNumber
	//      Append the lower-case form of the character to currentComponent.
	//
	//   stateAcronym -> (uppercase letter) -> stateAcronym
	//		Append the lower-case form of the character to currentComponent.
	//   stateAcronym -> (number) -> stateLowerOrNumber
	//      Append the character to currentComponent.
	//   stateAcronym -> (lowercase letter) -> stateLowerOrNumber
	//      Take all but the last character in currentComponent, turn that into
	//      a string, and append that to components. Set currentComponent to the
	//      last two characters seen.
	//
	//   stateLowerOrNumber -> (uppercase letter) -> stateUpper
	//      Take all characters in currentComponent, turn that into a string,
	//      and append that to components. Set currentComponent to the last
	//      character seen.
	//	 stateLowerOrNumber -> (lowercase letter) -> stateLowerOrNumber
	//      Append the character to currentComponent.
	//
	// The Go libraries that convert camelCase to snake_case deviate subtly from
	// the semantics we're going for in this method, namely that they separate
	// numbers and lowercase letters. We don't want this in all cases (we want e.g. Sha256Hash to
	// be converted as sha256_hash). We also want SHA256Hash to be converted as sha256_hash, so
	// we must at least be aware of digits when in the stateAcronym state.
	//
	// As for why this is a state machine, the libraries that do this all pretty much use
	// either regular expressions or state machines, which I suppose are ultimately the same thing.
	const (
		stateFirst = iota
		stateUpper
		stateAcronym
		stateLowerOrNumber
	)

	var components []string     // The components that will be joined together with underscores
	var currentComponent []rune // The characters composing the current component being built
	state := stateFirst
	for _, char := range name {
		switch state {
		case stateFirst:
			if unicode.IsUpper(char) {
				// stateFirst -> stateUpper
				state = stateUpper
				currentComponent = append(currentComponent, unicode.ToLower(char))
				continue
			}

			// stateFirst -> stateLowerOrNumber
			state = stateLowerOrNumber
			currentComponent = append(currentComponent, char)
			continue

		case stateUpper:
			if unicode.IsUpper(char) {
				// stateUpper -> stateAcronym
				state = stateAcronym
				currentComponent = append(currentComponent, unicode.ToLower(char))
				continue
			}

			// stateUpper -> stateLowerOrNumber
			state = stateLowerOrNumber
			currentComponent = append(currentComponent, char)
			continue

		case stateAcronym:
			if unicode.IsUpper(char) {
				// stateAcronym -> stateAcronym
				currentComponent = append(currentComponent, unicode.ToLower(char))
				continue
			}

			// We want to fold digits immediately following an acronym into the same
			// component as the acronym.
			if unicode.IsDigit(char) {
				// stateAcronym -> stateLowerOrNumber
				currentComponent = append(currentComponent, char)
				state = stateLowerOrNumber
				continue
			}

			// stateAcronym -> stateLowerOrNumber
			last, rest := currentComponent[len(currentComponent)-1], currentComponent[:len(currentComponent)-1]
			components = append(components, string(rest))
			currentComponent = []rune{last, char}
			state = stateLowerOrNumber
			continue

		case stateLowerOrNumber:
			if unicode.IsUpper(char) {
				// stateLowerOrNumber -> stateUpper
				components = append(components, string(currentComponent))
				currentComponent = []rune{unicode.ToLower(char)}
				state = stateUpper
				continue
			}

			// stateLowerOrNumber -> stateLowerOrNumber
			currentComponent = append(currentComponent, char)
			continue
		}
	}

	components = append(components, string(currentComponent))
	result := strings.Join(components, "_")
	return ensurePythonKeywordSafe(result)
}

// pythonKeywords is a map of reserved keywords used by Python 2 and 3.  We use this to avoid generating unspeakable
// names in the resulting code.  This map was sourced by merging the following reference material:
//
//     * Python 2: https://docs.python.org/2.5/ref/keywords.html
//     * Python 3: https://docs.python.org/3/reference/lexical_analysis.html#keywords
//
var pythonKeywords = map[string]bool{
	"False":    true,
	"None":     true,
	"True":     true,
	"and":      true,
	"as":       true,
	"assert":   true,
	"async":    true,
	"await":    true,
	"break":    true,
	"class":    true,
	"continue": true,
	"def":      true,
	"del":      true,
	"elif":     true,
	"else":     true,
	"except":   true,
	"exec":     true,
	"finally":  true,
	"for":      true,
	"from":     true,
	"global":   true,
	"if":       true,
	"import":   true,
	"in":       true,
	"is":       true,
	"lambda":   true,
	"nonlocal": true,
	"not":      true,
	"or":       true,
	"pass":     true,
	"print":    true,
	"raise":    true,
	"return":   true,
	"try":      true,
	"while":    true,
	"with":     true,
	"yield":    true,
}

// ensurePythonKeywordSafe adds a trailing underscore if the generated name clashes with a Python 2 or 3 keyword, per
// PEP 8: https://www.python.org/dev/peps/pep-0008/?#function-and-method-arguments
func ensurePythonKeywordSafe(name string) string {
	if _, isKeyword := pythonKeywords[name]; isKeyword {
		return name + "_"
	}
	return name
}
