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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/tools"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfbridge"
)

// newGoGenerator returns a language generator that understands how to produce Go packages.
func newGoGenerator(pkg, version string, info tfbridge.ProviderInfo, overlaysDir, outDir string) langGenerator {
	return &goGenerator{
		pkg:         pkg,
		version:     version,
		info:        info,
		overlaysDir: overlaysDir,
		outDir:      outDir,
	}
}

type goGenerator struct {
	pkg         string
	version     string
	info        tfbridge.ProviderInfo
	overlaysDir string
	outDir      string
	needsUtils  bool
}

// commentChars returns the comment characters to use for single-line comments.
func (g *goGenerator) commentChars() string {
	return "//"
}

// moduleDir returns the directory for the given module.
func (g *goGenerator) moduleDir(mod *module) string {
	dir := filepath.Join(g.outDir, g.pkg)
	if mod.name != "" {
		dir = filepath.Join(dir, mod.name)
	}
	return dir
}

type imports struct {
	Errors bool // true to import github.com/pkg/errors.
	Pulumi bool // true to import github.com/pulumi/pulumi/sdk/go/pulumi.
	Config bool // true to import github.com/pulumi/pulumi/sdk/go/pulumi/config.
}

// openWriter opens a writer for the given module and file name, emitting the standard header automatically.
func (g *goGenerator) openWriter(mod *module, name string, ims imports) (*tools.GenWriter, error) {
	dir := g.moduleDir(mod)
	file := filepath.Join(dir, name)
	w, err := tools.NewGenWriter(tfgen, file)
	if err != nil {
		return nil, err
	}

	// Emit a standard warning header ("do not edit", etc).
	w.EmitHeaderWarning(g.commentChars())

	// Emit the Go package name.
	pkg := g.goPackageName(mod)
	w.Writefmtln("package %s", pkg)
	w.Writefmtln("")

	// If needed, emit import statements.
	g.emitImports(w, ims)

	return w, nil
}

// goPackageName returns the Go package name for this module (package if no sub-module, module otherwise).
func (g *goGenerator) goPackageName(mod *module) string {
	if mod.name == "" {
		return g.pkg
	}
	return mod.name
}

func (g *goGenerator) emitImports(w *tools.GenWriter, ims imports) {
	if ims.Errors || ims.Pulumi || ims.Config {
		w.Writefmtln("import (")
		if ims.Errors {
			w.Writefmtln("\t\"github.com/pkg/errors\"")
		}
		if ims.Pulumi {
			w.Writefmtln("\t\"github.com/pulumi/pulumi/sdk/go/pulumi\"")
		}
		if ims.Config {
			w.Writefmtln("\t\"github.com/pulumi/pulumi/sdk/go/pulumi/config\"")
		}
		w.Writefmtln(")")
		w.Writefmtln("")
	}
}

// typeName returns a type name for a given resource type.
func (g *goGenerator) typeName(r *resourceType) string {
	return r.name
}

// emitPackage emits an entire package pack into the configured output directory with the configured settings.
func (g *goGenerator) emitPackage(pack *pkg) error {
	// Generate individual modules and their contents as packages.
	if err := g.emitModules(pack.modules); err != nil {
		return err
	}

	// Finally emit the non-code package metadata.
	return g.emitPackageMetadata(pack)
}

// emitModules emits all modules in the given module map.  It returns a full list of files, a map of module to its
// associated index, and any error that occurred, if any.
func (g *goGenerator) emitModules(mmap moduleMap) error {
	for _, mod := range mmap.values() {
		if err := g.emitModule(mod); err != nil {
			return err
		}
	}
	return nil
}

// emitModule emits a module as a Go package.  This emits a single file per member just for ease of managemnet.
// For example, imagine a module m with many members; the result is:
//
//     m/
//         m.go
//         member1.go
//         member<etc>.go
//         memberN.go
//
// The one special case is the configuration module, which yields a vars.ts file containing all exported variables.
//
// Note that the special module "" represents the top-most package module and won't be placed in a sub-directory.
func (g *goGenerator) emitModule(mod *module) error {
	glog.V(3).Infof("emitModule(%s)", mod.name)

	defer func() { g.needsUtils = false }()

	// Ensure that the target module directory exists.
	dir := g.moduleDir(mod)
	if err := tools.EnsureDir(dir); err != nil {
		return errors.Wrapf(err, "creating module directory")
	}

	// Ensure that the module has a module-wide comment.
	if err := g.ensurePackageComment(mod, dir); err != nil {
		return errors.Wrapf(err, "creating module comment file")
	}

	// Now, enumerate each module member, in the order presented to us, and do the right thing.
	for _, member := range mod.members {
		if err := g.emitModuleMember(mod, member); err != nil {
			return errors.Wrapf(err, "emitting module %s member %s", mod.name, member.Name())
		}
	}

	// If this is a config module, we need to emit the configuration variables.
	if mod.config() {
		if err := g.emitConfigVariables(mod); err != nil {
			return errors.Wrapf(err, "emitting config module variables")
		}
	}

	// If any part of this module needs internal utilities, emit them now.
	if g.needsUtils {
		if err := g.emitUtilities(mod); err != nil {
			return errors.Wrapf(err, "emitting utilities file")
		}
	}

	return nil
}

// ensurePackageComment writes out a file with a module-wide comment, provided one doesn't already exist.
func (g *goGenerator) ensurePackageComment(mod *module, dir string) error {
	pkg := g.goPackageName(mod)
	rf := filepath.Join(dir, "_about.go")
	_, err := os.Stat(rf)
	if err == nil {
		return nil // file already exists, exit right away.
	} else if !os.IsNotExist(err) {
		return err // legitimate error, propagate it.
	}

	// If we got here, the module comment file doesn't already exist -- write out a stock one.
	w, err := tools.NewGenWriter(tfgen, rf)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	w.Writefmtln("//nolint: lll")
	// Fake up a comment that makes it clear to Go that this is a module-wide comment.
	w.Writefmtln("// Package %[1]s exports types, functions, subpackages for provisioning %[1]s resources.", pkg)
	w.Writefmtln("//")

	downstreamLicense := g.info.GetTFProviderLicense()
	licenseTypeURL := getLicenseTypeURL(downstreamLicense)
	readme := fmt.Sprintf(standardDocReadme, g.pkg, g.info.Name, g.info.GetGitHubOrg(), downstreamLicense, licenseTypeURL)
	for _, line := range strings.Split(readme, "\n") {
		w.Writefmtln("// %s", line)
	}

	w.Writefmtln("package %s", pkg)

	return nil
}

// emitModuleMember emits the given member into its own file.
func (g *goGenerator) emitModuleMember(mod *module, member moduleMember) error {
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
		return nil
	case *overlayFile:
		return g.emitOverlay(mod, t)
	default:
		contract.Failf("unexpected member type: %v", reflect.TypeOf(member))
		return nil
	}
}

// emitConfigVariables emits all config vaiables in the given module into its own file.
func (g *goGenerator) emitConfigVariables(mod *module) error {
	// Create a single file into which all configuration variables will go.
	w, err := g.openWriter(mod, "config.go", imports{Pulumi: true, Config: true})
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	// Ensure we import any custom schemas referenced by the variables.
	var infos []*tfbridge.SchemaInfo
	for _, member := range mod.members {
		if v, ok := member.(*variable); ok {
			infos = append(infos, v.info)
		}
	}
	if err = g.emitCustomImports(w, mod, infos); err != nil {
		return err
	}

	// For each config variable, emit a helper function that reads from the context.
	for i, member := range mod.members {
		if v, ok := member.(*variable); ok {
			g.emitConfigAccessor(w, v)
		}
		if i != len(mod.members)-1 {
			w.Writefmtln("")
		}
	}

	return nil
}

func (g *goGenerator) emitConfigAccessor(w *tools.GenWriter, v *variable) {
	getfunc := "Get"

	var gettype string
	var functype string
	switch v.schema.Type {
	case schema.TypeBool:
		gettype, functype = "bool", "Bool"
	case schema.TypeInt:
		gettype, functype = "int", "Int"
	case schema.TypeFloat:
		gettype, functype = "float64", "Float64"
	default:
		gettype, functype = "string", ""
	}

	if v.doc != "" && v.doc != elidedDocComment {
		g.emitDocComment(w, v.doc, v.docURL, "")
	} else if v.rawdoc != "" {
		g.emitRawDocComment(w, v.rawdoc, "")
	}

	defaultValue, configKey := g.goDefaultValue(v), fmt.Sprintf("\"%s:%s\"", g.pkg, v.name)

	w.Writefmtln("func Get%s(ctx *pulumi.Context) %s {", upperFirst(v.name), gettype)
	if defaultValue != "" {
		w.Writefmtln("\tv, err := config.Try%s(ctx, %s)", functype, configKey)
		w.Writefmtln("\tif err == nil {")
		w.Writefmtln("\t\treturn v")
		w.Writefmtln("\t}")
		w.Writefmtln("\tif dv, ok := %s.(%s); ok {", defaultValue, gettype)
		w.Writefmtln("\t\treturn dv")
		w.Writefmtln("\t}")
		w.Writefmtln("\treturn v")
	} else {
		w.Writefmtln("\treturn config.%s%s(ctx, \"%s:%s\")", getfunc, functype, g.pkg, v.name)
	}

	w.Writefmtln("}")
}

// emitUtilities
func (g *goGenerator) emitUtilities(mod *module) error {
	// Open the utilities.ts file for this module and ready it for writing.
	w, err := g.openWriter(mod, "internal_utilities.go", imports{})
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	// TODO: use w.WriteString
	w.Writefmt(goUtilitiesFile)
	return nil
}

func (g *goGenerator) emitDocComment(w *tools.GenWriter, comment, docURL, prefix string) {
	if comment == elidedDocComment && docURL == "" {
		return
	}

	if comment != elidedDocComment {
		lines := strings.Split(comment, "\n")
		for i, docLine := range lines {
			// Break if we get to the last line and it's empty
			if i == len(lines)-1 && strings.TrimSpace(docLine) == "" {
				break
			}
			// Print the line of documentation
			w.Writefmtln("%s// %s", prefix, docLine)
		}

		if docURL != "" {
			w.Writefmtln("%s//", prefix)
		}
	}

	if docURL != "" {
		w.Writefmtln("%s// > This content is derived from %s.", prefix, docURL)
	}
}

func (g *goGenerator) emitRawDocComment(w *tools.GenWriter, comment, prefix string) {
	if comment != "" {
		curr := 0
		w.Writefmt("%s// ", prefix)
		for _, word := range strings.Fields(comment) {
			word = sanitizeForDocComment(word)
			if curr > 0 {
				if curr+len(word)+1 > (maxWidth - len(prefix)) {
					curr = 0
					w.Writefmt("\n%s// ", prefix)
				} else {
					w.Writefmt(" ")
					curr++
				}
			}
			w.Writefmt(word)
			curr += len(word)
		}
		w.Writefmtln("")
	}
}

func (g *goGenerator) emitPlainOldType(w *tools.GenWriter, pot *propertyType) {
	if pot.doc != "" {
		g.emitDocComment(w, pot.doc, "", "")
	}
	w.Writefmtln("type %s struct {", pot.name)
	for _, prop := range pot.properties {
		if prop.doc != "" && prop.doc != elidedDocComment {
			g.emitDocComment(w, prop.doc, prop.docURL, "\t")
		} else if prop.rawdoc != "" {
			g.emitRawDocComment(w, prop.rawdoc, "\t")
		}
		w.Writefmtln("\t%s interface{}", upperFirst(prop.name))
	}
	w.Writefmtln("}")
}

func (g *goGenerator) emitResourceType(mod *module, res *resourceType) error {
	// See if we'll be generating an error.  If yes, we need to import.
	var importErrors bool
	ins := make(map[string]bool)
	for _, prop := range res.inprops {
		ins[prop.name] = true
		if !prop.optional() {
			importErrors = true
		}
	}

	// Create a resource module file into which all of this resource's types will go.
	name := res.name
	w, err := g.openWriter(mod, lowerFirst(name)+".go", imports{Pulumi: true, Errors: importErrors})
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	// Ensure that we've emitted any custom imports pertaining to any of the field types.
	var fldinfos []*tfbridge.SchemaInfo
	for _, fldinfo := range res.info.Fields {
		fldinfos = append(fldinfos, fldinfo)
	}
	if err = g.emitCustomImports(w, mod, fldinfos); err != nil {
		return err
	}

	// Define the resource type structure, just a basic wrapper around the resource registration information.
	if res.doc != "" {
		g.emitDocComment(w, res.doc, res.docURL, "")
	}
	if !res.IsProvider() {
		if res.info.DeprecationMessage != "" {
			w.Writefmtln("// Deprecated: %s", res.info.DeprecationMessage)
		}
	}
	w.Writefmtln("type %s struct {", name)
	w.Writefmtln("\ts *pulumi.ResourceState")
	w.Writefmtln("}")
	w.Writefmtln("")

	// Create a constructor function that registers a new instance of this resource.
	argsType := res.argst.name
	w.Writefmtln("// New%s registers a new resource with the given unique name, arguments, and options.", name)
	if res.info.DeprecationMessage != "" {
		w.Writefmtln("// Deprecated: %s", res.info.DeprecationMessage)
	}
	w.Writefmtln("func New%s(ctx *pulumi.Context,", name)
	w.Writefmtln("\tname string, args *%s, opts ...pulumi.ResourceOpt) (*%s, error) {", argsType, name)

	// Ensure required arguments are present.
	for _, prop := range res.inprops {
		if !prop.optional() {
			w.Writefmtln("\tif args == nil || args.%s == nil {", upperFirst(prop.name))
			w.Writefmtln("\t\treturn nil, errors.New(\"missing required argument '%s'\")", upperFirst(prop.name))
			w.Writefmtln("\t}")
		}
	}

	// Produce the input map.
	w.Writefmtln("\tinputs := make(map[string]interface{})")
	hasDefault := make(map[*variable]bool)
	for _, prop := range res.inprops {
		if defaultValue := g.goDefaultValue(prop); defaultValue != "" {
			hasDefault[prop] = true
			w.Writefmtln("\tinputs[\"%s\"] = %s", prop.name, defaultValue)
		}
	}
	w.Writefmtln("\tif args == nil {")
	for _, prop := range res.inprops {
		if !hasDefault[prop] {
			w.Writefmtln("\t\tinputs[\"%s\"] = nil", prop.name)
		}
	}
	w.Writefmtln("\t} else {")
	for _, prop := range res.inprops {
		w.Writefmtln("\t\tinputs[\"%s\"] = args.%s", prop.name, upperFirst(prop.name))
	}
	w.Writefmtln("\t}")
	for _, prop := range res.outprops {
		if !ins[prop.name] {
			w.Writefmtln("\tinputs[\"%s\"] = nil", prop.name)
		}
	}

	// Finally make the call to registration.
	w.Writefmtln("\ts, err := ctx.RegisterResource(\"%s\", name, true, inputs, opts...)", res.info.Tok)
	w.Writefmtln("\tif err != nil {")
	w.Writefmtln("\t\treturn nil, err")
	w.Writefmtln("\t}")
	w.Writefmtln("\treturn &%s{s: s}, nil", name)
	w.Writefmtln("}")
	w.Writefmtln("")

	// Emit a factory function that reads existing instances of this resource.
	stateType := res.statet.name
	w.Writefmtln("// Get%[1]s gets an existing %[1]s resource's state with the given name, ID, and optional", name)
	w.Writefmtln("// state properties that are used to uniquely qualify the lookup (nil if not required).")
	if res.info.DeprecationMessage != "" {
		w.Writefmtln("// Deprecated: %s", res.info.DeprecationMessage)
	}
	w.Writefmtln("func Get%s(ctx *pulumi.Context,", name)
	w.Writefmtln("\tname string, id pulumi.ID, state *%s, opts ...pulumi.ResourceOpt) (*%s, error) {", stateType, name)
	w.Writefmtln("\tinputs := make(map[string]interface{})")
	w.Writefmtln("\tif state != nil {")
	for _, prop := range res.outprops {
		w.Writefmtln("\t\tinputs[\"%s\"] = state.%s", prop.name, upperFirst(prop.name))
	}
	w.Writefmtln("\t}")
	w.Writefmtln("\ts, err := ctx.ReadResource(\"%s\", name, id, inputs, opts...)", res.info.Tok)
	w.Writefmtln("\tif err != nil {")
	w.Writefmtln("\t\treturn nil, err")
	w.Writefmtln("\t}")
	w.Writefmtln("\treturn &%s{s: s}, nil", name)
	w.Writefmtln("}")
	w.Writefmtln("")

	// Create accessors for all of the properties inside of the resulting resource structure.
	w.Writefmtln("// URN is this resource's unique name assigned by Pulumi.")
	w.Writefmtln("func (r *%s) URN() pulumi.URNOutput {", name)
	w.Writefmtln("\treturn r.s.URN()")
	w.Writefmtln("}")
	w.Writefmtln("")
	w.Writefmtln("// ID is this resource's unique identifier assigned by its provider.")
	w.Writefmtln("func (r *%s) ID() pulumi.IDOutput {", name)
	w.Writefmtln("\treturn r.s.ID()")
	w.Writefmtln("}")
	w.Writefmtln("")
	for _, prop := range res.outprops {
		if prop.doc != "" && prop.doc != elidedDocComment {
			g.emitDocComment(w, prop.doc, prop.docURL, "")
		} else if prop.rawdoc != "" {
			g.emitRawDocComment(w, prop.rawdoc, "")
		}

		outType := goOutputType(prop)
		w.Writefmtln("func (r *%s) %s() %s {", name, upperFirst(prop.name), outType)
		if outType == defaultGoOutType {
			w.Writefmtln("\treturn r.s.State[\"%s\"]", prop.name)
		} else {
			// If not the default type, we need a cast.
			w.Writefmtln("\treturn (%s)(r.s.State[\"%s\"])", outType, prop.name)
		}
		w.Writefmtln("}")
		w.Writefmtln("")
	}

	// Emit the state type for get methods.
	g.emitPlainOldType(w, res.statet)
	w.Writefmtln("")

	// Emit the argument type for construction.
	g.emitPlainOldType(w, res.argst)

	return nil
}

func (g *goGenerator) emitResourceFunc(mod *module, fun *resourceFunc) error {
	// Create a vars.ts file into which all configuration variables will go.
	w, err := g.openWriter(mod, fun.name+".go", imports{Pulumi: true})
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	// Ensure that we've emitted any custom imports pertaining to any of the field types.
	var fldinfos []*tfbridge.SchemaInfo
	for _, fldinfo := range fun.info.Fields {
		fldinfos = append(fldinfos, fldinfo)
	}
	if err := g.emitCustomImports(w, mod, fldinfos); err != nil {
		return err
	}

	// Write the TypeDoc/JSDoc for the data source function.
	if fun.doc != "" {
		g.emitDocComment(w, fun.doc, fun.docURL, "")
	}

	if fun.info.DeprecationMessage != "" {
		w.Writefmtln("// Deprecated: %s", fun.info.DeprecationMessage)
	}

	// If the function starts with New or Get, it will conflict; so rename them.
	funname := upperFirst(fun.name)
	if strings.Index(funname, "New") == 0 {
		funname = "Create" + funname[3:]
	} else if strings.Index(funname, "Get") == 0 {
		funname = "Lookup" + funname[3:]
	}

	// Now, emit the function signature.
	argsig := "ctx *pulumi.Context"
	if fun.argst != nil {
		argsig = fmt.Sprintf("%s, args *%s", argsig, fun.argst.name)
	}
	var retty string
	if fun.retst == nil {
		retty = "error"
	} else {
		retty = fmt.Sprintf("(*%s, error)", fun.retst.name)
	}
	w.Writefmtln("func %s(%s) %s {", funname, argsig, retty)

	// Make a map of inputs to pass to the runtime function.
	var inputsVar string
	if fun.argst == nil {
		inputsVar = "nil"
	} else {
		inputsVar = "inputs"
		w.Writefmtln("\tinputs := make(map[string]interface{})")
		w.Writefmtln("\tif args != nil {")
		for _, arg := range fun.args {
			w.Writefmtln("\t\tinputs[\"%s\"] = args.%s", arg.name, upperFirst(arg.name))
		}
		w.Writefmtln("\t}")
	}

	// Now simply invoke the runtime function with the arguments.
	var outputsVar string
	if fun.retst == nil {
		outputsVar = "_"
	} else {
		outputsVar = "outputs"
	}
	w.Writefmtln("\t%s, err := ctx.Invoke(\"%s\", %s)", outputsVar, fun.info.Tok, inputsVar)

	if fun.retst == nil {
		w.Writefmtln("\treturn err")
	} else {
		// Check the error before proceeding.
		w.Writefmtln("\tif err != nil {")
		w.Writefmtln("\t\treturn nil, err")
		w.Writefmtln("\t}")

		// Get the outputs and return the structure, awaiting each one and propagating any errors.
		w.Writefmtln("\treturn &%s{", fun.retst.name)
		for _, ret := range fun.rets {
			// TODO: ideally, we would have some strong typing on these outputs.
			w.Writefmtln("\t\t%s: outputs[\"%s\"],", upperFirst(ret.name), ret.name)
		}
		w.Writefmtln("\t}, nil")
	}
	w.Writefmtln("}")

	// If there are argument and/or return types, emit them.
	if fun.argst != nil {
		w.Writefmtln("")
		g.emitPlainOldType(w, fun.argst)
	}
	if fun.retst != nil {
		w.Writefmtln("")
		g.emitPlainOldType(w, fun.retst)
	}

	return nil
}

// emitOverlay copies an overlay from its source to the target, and returns the resulting file to be exported.
func (g *goGenerator) emitOverlay(mod *module, overlay *overlayFile) error {
	// Copy the file from the overlays directory to the destination.
	dir := g.moduleDir(mod)
	dst := filepath.Join(dir, overlay.name)
	if overlay.Copy() {
		return copyFile(overlay.src, dst)
	}
	_, err := os.Stat(dst)
	return err
}

// emitPackageMetadata generates all the non-code metadata required by a Pulumi package.
func (g *goGenerator) emitPackageMetadata(pack *pkg) error {
	// TODO: generate Gopkg.* files?
	return nil
}

// emitCustomImports traverses a custom schema map, deeply, to figure out the set of imported names and files that
// will be required to access those names.  WARNING: this routine doesn't (yet) attempt to eliminate naming collisions.
func (g *goGenerator) emitCustomImports(w *tools.GenWriter, mod *module, infos []*tfbridge.SchemaInfo) error {
	// TODO: implement this; until we do, we can't easily add overlays with intra- or inter-package references.
	return nil
}

// goOutputType returns the Go output type name for a resource property.
func goOutputType(v *variable) string {
	return goSchemaOutputType(v.schema, v.info)
}

const defaultGoOutType = "pulumi.Output"

// goSchemaOutputType returns the Go output type name for a given Terraform schema and bridge override info.
func goSchemaOutputType(sch *schema.Schema, info *tfbridge.SchemaInfo) string {
	if sch != nil {
		switch sch.Type {
		case schema.TypeBool:
			return "pulumi.BoolOutput"
		case schema.TypeInt:
			return "pulumi.IntOutput"
		case schema.TypeFloat:
			return "pulumi.Float64Output"
		case schema.TypeString:
			return "pulumi.StringOutput"
		case schema.TypeSet, schema.TypeList:
			if tfbridge.IsMaxItemsOne(sch, info) {
				if elemSch, ok := sch.Elem.(*schema.Schema); ok {
					var elemInfo *tfbridge.SchemaInfo
					if info != nil {
						elemInfo = info.Elem
					}
					return goSchemaOutputType(elemSch, elemInfo)
				}
				return goSchemaOutputType(nil, nil)
			}
			return "pulumi.ArrayOutput"
		case schema.TypeMap:
			// If this map has a "resource" element type, just use the generated element type. This works around a bug
			// in TF that effectively forces this behavior.
			if _, hasResourceElem := sch.Elem.(*schema.Resource); hasResourceElem {
				return goSchemaOutputType(nil, nil)
			}
			return "pulumi.MapOutput"
		}
	}

	return defaultGoOutType
}

func goPrimitiveValue(value interface{}) (string, error) {
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

func (g *goGenerator) goDefaultValue(v *variable) string {
	defaultValue := ""
	if v.info == nil || v.info.Default == nil {
		return defaultValue
	}
	defaults := v.info.Default

	if defaults.Value != nil {
		dv, err := goPrimitiveValue(defaults.Value)
		if err != nil {
			cmdutil.Diag().Warningf(diag.Message("", err.Error()))
			return defaultValue
		}
		defaultValue = dv
	}

	if len(defaults.EnvVars) > 0 {
		g.needsUtils = true

		parser, outDefault := "nil", "\"\""
		switch v.schema.Type {
		case schema.TypeBool:
			parser, outDefault = "parseEnvBool", "false"
		case schema.TypeInt:
			parser, outDefault = "parseEnvInt", "0"
		case schema.TypeFloat:
			parser, outDefault = "parseEnvFloat", "0.0"
		}

		if defaultValue == "" {
			if v.out {
				defaultValue = outDefault
			} else {
				defaultValue = "nil"
			}
		}

		defaultValue = fmt.Sprintf("getEnvOrDefault(%s, %s", defaultValue, parser)
		for _, e := range defaults.EnvVars {
			defaultValue += fmt.Sprintf(", %q", e)
		}
		defaultValue += ")"
	}

	return defaultValue
}
