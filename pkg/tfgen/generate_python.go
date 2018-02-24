// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tfgen

import (
	"fmt"
	"path/filepath"
	"reflect"
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
		// TODO: skip overlays for now.
		// return g.emitOverlay(mod, t)
		return "", nil
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
	w.Writefmtln("__config__ = pulumi.Config('%s:config')", g.pkg)
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
				w.Writefmtln(`"""`)
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
	w.Writefmtln("class %s(object):", pot.name)
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
		if !prop.optional() {
			w.Writefmtln("        if not %s:", pname)
			w.Writefmtln("            raise TypeError('Missing required argument %s')", pname)
			w.Writefmt("        elif ")
		} else {
			w.Writefmt("        if %s and ", pname)
		}
		w.Writefmtln("not isinstance(%s, %s):", pname, ptype)
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
	name := pyName(lowerFirst(res.name))
	w, err := g.openWriter(mod, name+".py", true)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	// Produce a class definition with optional """ comment.
	w.Writefmtln("class %s(pulumi.CustomResource):", res.name)
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
	w.Writefmt("def %s(", fun.name)
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
			w.Writefmt("        %s=__ret__['%s']", pyName(ret.name), ret.name)
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

	// Create a Python version.
	version := pack.version
	if len(version) > 0 && version[0] == 'v' {
		version = version[1:] // no leading v
	}
	if dashix := strings.IndexRune(version, '-'); dashix != -1 {
		version = version[:dashix] + "+" + version[dashix+1:] // put all non-"N.N.N" text to the right of a "+"
	}
	version = strings.Replace(version, "-", ".", -1) // replace all remaining "-"s with "."s

	// Now create a standard Python package from the metadata.
	// TODO: how to encode the requirements for downloading a plugin?
	w.Writefmtln("from setuptools import setup, find_packages")
	w.Writefmtln("")
	w.Writefmtln("setup(name='%s',", pyPack(pack.name))
	w.Writefmtln("      version='%s',", version)
	if g.info.Description != "" {
		w.Writefmtln("      description='%s',", g.info.Description)
	}
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

	// Find the version of the Pulumi SDK to include.  If there is none, add "latest" automatically.
	sdk := "pulumi"
	var sdkVersion string
	if overlay := g.info.Overlay; overlay != nil {
		if sdkVersion = overlay.Dependencies[sdk]; sdkVersion == "" {
			if sdkVersion = overlay.DevDependencies[sdk]; sdkVersion == "" {
				sdkVersion = overlay.PeerDependencies[sdk]
			}
		}
	}
	w.Writefmt("      install_requires=['%s", sdk)
	if sdkVersion != "" {
		w.Writefmt(">=%s", sdkVersion)
	}
	w.Writefmtln("'],")

	w.Writefmtln("      zip_safe=False)")
	return nil
}

// pyType returns the expected runtime type for the given variable.  Of course, being a dynamic language, this
// check is not exhaustive, but it should be good enough to catch 80% of the cases early on.
func pyType(v *variable) string {
	switch v.schema.Type {
	case schema.TypeBool:
		return "bool"
	case schema.TypeInt:
		return "int"
	case schema.TypeFloat:
		return "float"
	case schema.TypeString:
		return "basestring"
	case schema.TypeSet, schema.TypeList:
		return fmt.Sprintf("list")
	default:
		return fmt.Sprintf("dict")
	}
}

// pyPack returns the suggested package name for the given string.
func pyPack(s string) string {
	return "pulumi_" + s
}

// pyName turns a variable or function name, normally using camelCase, to an underscore_case name.
func pyName(s string) string {
	var result string

	// Eat any leading __s.
	i := 0
	for ; i < len(s) && s[i] == '_'; i++ {
		result += "_"
	}

	// Now do the substitutions.
	for ; i < len(s); i++ {
		c := s[i]
		contract.Assertf(c != '_', "Unexpected underscore %d in pre-Pythonified name %s", i, s)
		if unicode.IsUpper(rune(c)) {
			result += "_" + string(unicode.ToLower(rune(c)))
		} else {
			result += string(c)
		}
	}

	return result
}
