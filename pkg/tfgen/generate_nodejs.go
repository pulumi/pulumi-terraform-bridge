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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/tools"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pulumi/pulumi-terraform/pkg/tfbridge"
)

// newNodeJSGenerator returns a language generator that understands how to produce Type/JavaScript packages.
func newNodeJSGenerator(pkg, version string, info tfbridge.ProviderInfo, overlaysDir, outDir string) langGenerator {
	return &nodeJSGenerator{
		pkg:         pkg,
		version:     version,
		info:        info,
		overlaysDir: overlaysDir,
		outDir:      outDir,
	}
}

type nodeJSGenerator struct {
	pkg         string
	version     string
	info        tfbridge.ProviderInfo
	overlaysDir string
	outDir      string
}

// commentChars returns the comment characters to use for single-line comments.
func (g *nodeJSGenerator) commentChars() string {
	return "//"
}

// moduleDir returns the directory for the given module.
func (g *nodeJSGenerator) moduleDir(mod *module) string {
	dir := g.outDir
	if mod.name != "" {
		dir = filepath.Join(dir, mod.name)
	}
	return dir
}

// relativeRootDir returns the relative path to the root directory for the given module.
func (g *nodeJSGenerator) relativeRootDir(mod *module) string {
	p, err := filepath.Rel(g.moduleDir(mod), g.outDir)
	contract.IgnoreError(err)
	return p
}

// openWriter opens a writer for the given module and file name, emitting the standard header automatically.
func (g *nodeJSGenerator) openWriter(mod *module, name string, needsSDK bool) (*tools.GenWriter, string, error) {
	dir := g.moduleDir(mod)
	file := filepath.Join(dir, name)
	w, err := tools.NewGenWriter(tfgen, file)
	if err != nil {
		return nil, "", err
	}

	// Emit a standard warning header ("do not edit", etc).
	w.EmitHeaderWarning(g.commentChars())

	// If needed, emit the standard Pulumi SDK import statement.
	if needsSDK {
		g.emitSDKImport(mod, w)
	}

	return w, file, nil
}

func (g *nodeJSGenerator) emitSDKImport(mod *module, w *tools.GenWriter) {
	w.Writefmtln("import * as pulumi from \"@pulumi/pulumi\";")
	w.Writefmtln("import * as utilities from \"%s/utilities\";", g.relativeRootDir(mod))
	w.Writefmtln("")
}

// emitPackage emits an entire package pack into the configured output directory with the configured settings.
func (g *nodeJSGenerator) emitPackage(pack *pkg) error {
	// First, generate the individual modules and their contents.
	files, submodules, err := g.emitModules(pack.modules)
	if err != nil {
		return err
	}

	// Generate a top-level index file that re-exports any child modules and a top-level utils file.
	index := pack.modules.ensureModule("")
	if pack.provider != nil {
		index.members = append(index.members, pack.provider)
	}

	indexFiles, _, err := g.emitModule(index, submodules)
	if err != nil {
		return err
	}
	files = append(files, indexFiles...)

	// Finally emit the package metadata (NPM, TypeScript, and so on).
	sort.Strings(files)
	return g.emitPackageMetadata(pack, files)
}

// emitModules emits all modules in the given module map.  It returns a full list of files, a map of module to its
// associated index, and any error that occurred, if any.
func (g *nodeJSGenerator) emitModules(mmap moduleMap) ([]string, map[string]string, error) {
	var allFiles []string
	moduleMap := make(map[string]string)
	for _, mod := range mmap.values() {
		if mod.name == "" {
			continue // skip the root module, it is handled specially.
		}
		files, index, err := g.emitModule(mod, nil)
		if err != nil {
			return nil, nil, err
		}
		allFiles = append(allFiles, files...)
		moduleMap[mod.name] = index
	}
	return allFiles, moduleMap, nil
}

// emitModule emits a module.  This module ends up having many possible ES6 sub-modules which are then re-exported
// at the top level.  This is to make it convenient for overlays to import files within the same module without
// causing problematic cycles.  For example, imagine a module m with many members; the result is:
//
//     m/
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
func (g *nodeJSGenerator) emitModule(mod *module, submods map[string]string) ([]string, string, error) {
	glog.V(3).Infof("emitModule(%s)", mod.name)

	// Ensure that the target module directory exists.
	dir := g.moduleDir(mod)
	if err := tools.EnsureDir(dir); err != nil {
		return nil, "", errors.Wrapf(err, "creating module directory")
	}

	// Now, enumerate each module member, in the order presented to us, and do the right thing.
	var files []string
	for _, member := range mod.members {
		file, err := g.emitModuleMember(mod, member)
		if err != nil {
			return nil, "", errors.Wrapf(err, "emitting module %s member %s", mod.name, member.Name())
		} else if file != "" {
			files = append(files, file)
		}
	}

	// If this is a config module, we need to emit the configuration variables.
	if mod.config() {
		file, err := g.emitConfigVariables(mod)
		if err != nil {
			return nil, "", errors.Wrapf(err, "emitting config module variables")
		}
		files = append(files, file)
	}

	// Generate an index file for this module.
	index, err := g.emitIndex(mod, files, submods)
	if err != nil {
		return nil, "", errors.Wrapf(err, "emitting module %s index", mod.name)
	}
	files = append(files, index)

	// Lastly, if this is the root module, we need to emit a file containing utility functions consumed by other
	// modules.
	if mod.root() {
		utils, err := g.emitUtilities(mod)
		if err != nil {
			return nil, "", errors.Wrapf(err, "emitting utility file for root module")
		}
		files = append(files, utils)
	}

	return files, index, nil
}

// emitIndex emits an index module, optionally re-exporting other members or submodules.
func (g *nodeJSGenerator) emitIndex(mod *module, exports []string, submods map[string]string) (string, error) {
	// Open the index.ts file for this module, and ready it for writing.
	w, index, err := g.openWriter(mod, "index.ts", false)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	// Export anything flatly that is a direct export rather than sub-module.
	if len(exports) > 0 {
		w.Writefmtln("// Export members:")
		var exps []string
		exps = append(exps, exports...)
		sort.Strings(exps)
		for _, exp := range exps {
			rel, err := g.relModule(mod, exp)
			if err != nil {
				return "", err
			}
			w.Writefmtln("export * from \"%s\";", rel)
		}
	}

	// Finally, f there are submodules, export them.
	if len(submods) > 0 {
		if len(exports) > 0 {
			w.Writefmtln("")
		}
		w.Writefmtln("// Export sub-modules:")
		var subs []string
		for sub := range submods {
			subs = append(subs, sub)
		}
		sort.Strings(subs)
		for _, sub := range subs {
			rel, err := g.relModule(mod, submods[sub])
			if err != nil {
				return "", err
			}
			w.Writefmtln("import * as %s from \"%s\";", sub, rel)
		}
		w.Writefmt("export {")
		for i, sub := range subs {
			if i > 0 {
				w.Writefmt(", ")
			}
			w.Writefmt(sub)
		}
		w.Writefmtln("};")
	}

	return index, nil
}

// emitUtilities emits a utilities file for submodules to consume.
func (g *nodeJSGenerator) emitUtilities(mod *module) (string, error) {
	contract.Require(mod.root(), "mod.root()")

	// Open the utilities.ts file for this module and ready it for writing.
	w, utilities, err := g.openWriter(mod, "utilities.ts", false)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	// TODO: use w.WriteString
	w.Writefmt(tsUtilitiesFile)
	return utilities, nil
}

// emitModuleMember emits the given member, and returns the module file that it was emitted into (if any).
func (g *nodeJSGenerator) emitModuleMember(mod *module, member moduleMember) (string, error) {
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
func (g *nodeJSGenerator) emitConfigVariables(mod *module) (string, error) {
	// Create a vars.ts file into which all configuration variables will go.
	w, config, err := g.openWriter(mod, "vars.ts", true)
	if err != nil {
		return "", err
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
		return "", err
	}

	// Create a config bag for the variables to pull from.
	w.Writefmtln("let __config = new pulumi.Config(\"%v\");", g.pkg)
	w.Writefmtln("")

	// Emit an entry for all config variables.
	for _, member := range mod.members {
		if v, ok := member.(*variable); ok {
			g.emitConfigVariable(w, v)
		}
	}

	return config, nil
}

func (g *nodeJSGenerator) emitConfigVariable(w *tools.GenWriter, v *variable) {
	var getfunc string
	if v.optional() {
		getfunc = "get"
	} else {
		getfunc = "require"
	}
	if v.schema.Type != schema.TypeString {
		// Only try to parse a JSON object if the config isn't a straight string.
		getfunc = fmt.Sprintf("%sObject<%s>", getfunc, tsType(v, false /*noflags*/, !v.out /*wrapInput*/))
	}
	var anycast string
	if v.info != nil && v.info.Type != "" {
		// If there's a custom type, we need to inject a cast to silence the compiler.
		anycast = "<any>"
	}
	if v.doc != "" {
		g.emitDocComment(w, v.doc, v.docURL, "")
	} else if v.rawdoc != "" {
		g.emitRawDocComment(w, v.rawdoc, "")
	}

	configFetch := fmt.Sprintf("__config.%s(\"%s\")", getfunc, v.name)
	if defaultValue := tsDefaultValue(v); defaultValue != "undefined" {
		if v.optional() {
			configFetch += " || " + defaultValue
		} else {
			configFetch = fmt.Sprintf("utilities.requireWithDefault(() => %s, %s)", configFetch, defaultValue)
		}
	}

	w.Writefmtln("export let %s: %s = %s%s;", v.name, tsType(v, true /*noflags*/, !v.out /*wrapInput*/), anycast,
		configFetch)
}

// sanitizeForDocComment ensures that no `*/` sequence appears in the string, to avoid
// accidentally closing the comment block.
func sanitizeForDocComment(str string) string {
	return strings.Replace(str, "*/", "*&#47;", -1)
}

func (g *nodeJSGenerator) emitDocComment(w *tools.GenWriter, comment, docURL, prefix string) {
	if comment != "" {
		lines := strings.Split(comment, "\n")
		w.Writefmtln("%v/**", prefix)
		for i, docLine := range lines {
			docLine = sanitizeForDocComment(docLine)
			// Break if we get to the last line and it's empty
			if i == len(lines)-1 && strings.TrimSpace(docLine) == "" {
				break
			}
			// Print the line of documentation
			w.Writefmtln("%v * %s", prefix, docLine)
		}
		w.Writefmtln("%v */", prefix)
	}
}

func (g *nodeJSGenerator) emitRawDocComment(w *tools.GenWriter, comment, prefix string) {
	if comment != "" {
		curr := 0
		w.Writefmtln("%v/**", prefix)
		w.Writefmt("%v * ", prefix)
		for _, word := range strings.Fields(comment) {
			word = sanitizeForDocComment(word)
			if curr > 0 {
				if curr+len(word)+1 > (maxWidth - len(prefix)) {
					curr = 0
					w.Writefmt("\n%v * ", prefix)
				} else {
					w.Writefmt(" ")
					curr++
				}
			}
			w.Writefmt(word)
			curr += len(word)
		}
		w.Writefmtln("")
		w.Writefmtln("%v */", prefix)
	}
}

func (g *nodeJSGenerator) emitPlainOldType(w *tools.GenWriter, pot *plainOldType, wrapInput bool) {
	if pot.doc != "" {
		g.emitDocComment(w, pot.doc, "", "")
	}
	w.Writefmtln("export interface %s {", pot.name)
	for _, prop := range pot.props {
		if prop.doc != "" {
			g.emitDocComment(w, prop.doc, prop.docURL, "    ")
		} else if prop.rawdoc != "" {
			g.emitRawDocComment(w, prop.rawdoc, "    ")
		}
		w.Writefmtln("    readonly %s%s: %s;", prop.name, tsFlags(prop), tsType(prop, false, wrapInput))
	}
	w.Writefmtln("}")
}

//nolint:lll
func (g *nodeJSGenerator) emitResourceType(mod *module, res *resourceType) (string, error) {
	// Create a resource module file into which all of this resource's types will go.
	name := res.name
	w, file, err := g.openWriter(mod, lowerFirst(name)+".ts", true)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	// Ensure that we've emitted any custom imports pertaining to any of the field types.
	var fldinfos []*tfbridge.SchemaInfo
	for _, fldinfo := range res.info.Fields {
		fldinfos = append(fldinfos, fldinfo)
	}
	if err = g.emitCustomImports(w, mod, fldinfos); err != nil {
		return "", err
	}

	// Write the TypeDoc/JSDoc for the resource class
	if res.doc != "" {
		g.emitDocComment(w, res.doc, res.docURL, "")
	}

	baseType := "CustomResource"
	if res.IsProvider() {
		baseType = "ProviderResource"
	}

	// Begin defining the class.
	w.Writefmtln("export class %s extends pulumi.%s {", name, baseType)

	// Emit a static factory to read instances of this resource unless this is a provider resource.
	stateType := res.statet.name
	if !res.IsProvider() {
		w.Writefmtln("    /**")
		w.Writefmtln("     * Get an existing %s resource's state with the given name, ID, and optional extra", name)
		w.Writefmtln("     * properties used to qualify the lookup.")
		w.Writefmtln("     *")
		w.Writefmtln("     * @param name The _unique_ name of the resulting resource.")
		w.Writefmtln("     * @param id The _unique_ provider ID of the resource to lookup.")
		w.Writefmtln("     * @param state Any extra arguments used during the lookup.")
		w.Writefmtln("     */")
		w.Writefmtln("    public static get(name: string, id: pulumi.Input<pulumi.ID>, state?: %s, opts?: pulumi.CustomResourceOptions): %s {", stateType, name)
		w.Writefmtln("        return new %s(name, <any>state, { ...opts, id: id });", name)
		w.Writefmtln("    }")
		w.Writefmtln("")
	}

	// Emit all properties (using their output types).
	// TODO[pulumi/pulumi#397]: represent sensitive types using a Secret<T> type.
	ins := make(map[string]bool)
	for _, prop := range res.inprops {
		ins[prop.name] = true
	}
	for _, prop := range res.outprops {
		if prop.doc != "" {
			g.emitDocComment(w, prop.doc, prop.docURL, "    ")
		} else if prop.rawdoc != "" {
			g.emitRawDocComment(w, prop.rawdoc, "    ")
		}

		// Make a little comment in the code so it's easy to pick out output properties.
		var outcomment string
		if !ins[prop.name] {
			outcomment = "/*out*/ "
		}

		w.Writefmtln("    public %sreadonly %s: pulumi.Output<%s>;",
			outcomment, prop.name, tsType(prop, true /*noflags*/, !prop.out /*wrapInput*/))
	}
	w.Writefmtln("")

	// Now create a constructor that chains supercalls and stores into properties.
	w.Writefmtln("    /**")
	w.Writefmtln("     * Create a %s resource with the given unique name, arguments, and options.", name)
	w.Writefmtln("     *")
	w.Writefmtln("     * @param name The _unique_ name of the resource.")
	w.Writefmtln("     * @param args The arguments to use to populate this resource's properties.")
	w.Writefmtln("     * @param opts A bag of options that control this resource's behavior.")
	w.Writefmtln("     */")

	// Write out callable constructor: We only emit a single public constructor, even though we use a private signature
	// as well as part of the implementation of `.get`. This is complicated slightly by the fact that, if there is no
	// args type, we will emit a constructor lacking that parameter.
	var argsFlags string
	if len(res.reqprops) == 0 {
		// If the number of required input properties was zero, we can make the args object optional.
		argsFlags = "?"
	}
	argsType := res.argst.name
	trailingBrace := ""
	if res.IsProvider() {
		trailingBrace = " {"
	}
	optionsType := "CustomResourceOptions"
	if res.IsProvider() {
		optionsType = "ResourceOptions"
	}

	w.Writefmtln("    constructor(name: string, args%s: %s, opts?: pulumi.%s)%s", argsFlags, argsType,
		optionsType, trailingBrace)

	if !res.IsProvider() {
		// Now write out a general purpose constructor implementation that can handle the public signautre as well as the
		// signature to support construction via `.get`.  And then emit the body preamble which will pluck out the
		// conditional state into sensible variables using dynamic type tests.
		w.Writefmtln("    constructor(name: string, argsOrState?: %s | %s, opts?: pulumi.CustomResourceOptions) {",
			argsType, stateType)
		w.Writefmtln("        let inputs: pulumi.Inputs = {};")
		// The lookup case:
		w.Writefmtln("        if (opts && opts.id) {")
		w.Writefmtln("            const state: %[1]s = argsOrState as %[1]s | undefined;", stateType)
		for _, prop := range res.outprops {
			w.Writefmtln(`            inputs["%[1]s"] = state ? state.%[1]s : undefined;`, prop.name)
		}
		// The creation case (with args):
		w.Writefmtln("        } else {")
		w.Writefmtln("            const args = argsOrState as %s | undefined;", argsType)
	} else {
		w.Writefmtln("        let inputs: pulumi.Inputs = {};")
		w.Writefmtln("        {")
	}
	for _, prop := range res.inprops {
		if !prop.optional() {
			w.Writefmtln("            if (!args || args.%s === undefined) {", prop.name)
			w.Writefmtln("                throw new Error(\"Missing required property '%s'\");", prop.name)
			w.Writefmtln("            }")
		}
	}
	for _, prop := range res.inprops {
		arg := fmt.Sprintf("args ? args.%[1]s : undefined", prop.name)
		if defaultValue := tsDefaultValue(prop); defaultValue != "undefined" {
			arg = fmt.Sprintf("(%s) || %s", arg, defaultValue)
		}

		// provider properties must be marshaled as JSON strings.
		if res.IsProvider() && prop.schema != nil && prop.schema.Type != schema.TypeString {
			arg = fmt.Sprintf("pulumi.output(%s).apply(JSON.stringify)", arg)
		}

		w.Writefmtln(`            inputs["%s"] = %s;`, prop.name, arg)
	}
	for _, prop := range res.outprops {
		if !ins[prop.name] {
			w.Writefmtln(`            inputs["%s"] = undefined /*out*/;`, prop.name)
		}
	}
	w.Writefmtln("        }")

	// Now invoke the super constructor with the type, name, and a property map.
	w.Writefmtln(`        super("%s", name, inputs, opts);`, res.info.Tok)

	// Finish the class.
	w.Writefmtln("    }")
	w.Writefmtln("}")

	// Emit the state type for get methods.
	if !res.IsProvider() {
		w.Writefmtln("")
		g.emitPlainOldType(w, res.statet, true /*wrapInput*/)
	}

	// Emit the argument type for construction.
	w.Writefmtln("")
	g.emitPlainOldType(w, res.argst, true /*wrapInput*/)

	return file, nil
}

func (g *nodeJSGenerator) emitResourceFunc(mod *module, fun *resourceFunc) (string, error) {
	// Create a vars.ts file into which all configuration variables will go.
	w, file, err := g.openWriter(mod, fun.name+".ts", true)
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(w)

	// Ensure that we've emitted any custom imports pertaining to any of the field types.
	var fldinfos []*tfbridge.SchemaInfo
	for _, fldinfo := range fun.info.Fields {
		fldinfos = append(fldinfos, fldinfo)
	}
	if err = g.emitCustomImports(w, mod, fldinfos); err != nil {
		return "", err
	}

	// Write the TypeDoc/JSDoc for the data source function.
	if fun.doc != "" {
		g.emitDocComment(w, fun.doc, fun.docURL, "")
	}

	// Now, emit the function signature.
	var argsig string
	if fun.argst != nil {
		var optflag string
		if len(fun.reqargs) == 0 {
			optflag = "?"
		}
		argsig = fmt.Sprintf("args%s: %s, ", optflag, fun.argst.name)
	}
	var retty string
	if fun.retst == nil {
		retty = "void"
	} else {
		retty = fun.retst.name
	}
	w.Writefmtln("export function %s(%sopts?: pulumi.InvokeOptions): Promise<%s> {", fun.name, argsig, retty)

	// Zero initialize the args if empty and necessary.
	if len(fun.args) > 0 && len(fun.reqargs) == 0 {
		w.Writefmtln("    args = args || {};")
	}

	// Now simply invoke the runtime function with the arguments, returning the results.
	w.Writefmtln("    return pulumi.runtime.invoke(\"%s\", {", fun.info.Tok)
	for _, arg := range fun.args {
		// Pass the argument to the invocation.
		w.Writefmtln("        \"%[1]s\": args.%[1]s,", arg.name)
	}
	w.Writefmtln("    }, opts);")

	w.Writefmtln("}")

	// If there are argument and/or return types, emit them.
	if fun.argst != nil {
		w.Writefmtln("")
		g.emitPlainOldType(w, fun.argst, false /*wrapInput*/)
	}
	if fun.retst != nil {
		w.Writefmtln("")
		g.emitPlainOldType(w, fun.retst, false /*wrapInput*/)
	}

	return file, nil
}

// emitOverlay copies an overlay from its source to the target, and returns the resulting file to be exported.
func (g *nodeJSGenerator) emitOverlay(mod *module, overlay *overlayFile) (string, error) {
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

// emitPackageMetadata generates all the non-code metadata required by a Pulumi package.
func (g *nodeJSGenerator) emitPackageMetadata(pack *pkg, files []string) error {
	// The generator already emitted Pulumi.yaml, so that leaves two more files to write out:
	//     1) package.json: minimal NPM package metadata
	//     2) tsconfig.json: instructions for TypeScript compilation
	if err := g.emitNPMPackageMetadata(pack); err != nil {
		return err
	}
	return g.emitTypeScriptProjectFile(pack, files)
}

type npmPackage struct {
	Name             string            `json:"name"`
	Version          string            `json:"version"`
	Description      string            `json:"description,omitempty"`
	Keywords         []string          `json:"keywords,omitempty"`
	Homepage         string            `json:"homepage,omitempty"`
	Repository       string            `json:"repository,omitempty"`
	License          string            `json:"license,omitempty"`
	Scripts          map[string]string `json:"scripts,omitempty"`
	Dependencies     map[string]string `json:"dependencies,omitempty"`
	DevDependencies  map[string]string `json:"devDependencies,omitempty"`
	PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
	Pulumi           npmPulumiManifest `json:"pulumi,omitempty"`
}

type npmPulumiManifest struct {
	Resource bool `json:"resource,omitempty"`
}

func (g *nodeJSGenerator) emitNPMPackageMetadata(pack *pkg) error {
	w, err := tools.NewGenWriter(tfgen, filepath.Join(g.outDir, "package.json"))
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)

	// Create info that will get serialized into an NPM package.json.
	npminfo := npmPackage{
		Name:        fmt.Sprintf("@pulumi/%s", pack.name),
		Version:     "${VERSION}",
		Description: g.info.Description,
		Keywords:    g.info.Keywords,
		Homepage:    g.info.Homepage,
		Repository:  g.info.Repository,
		License:     g.info.License,
		// Ideally, this `scripts` section would include an install script that installs the provider, however, doing
		// so causes problems when we try to restore package dependencies, since we must do an install for that. So
		// we have another process that adds the install script when generating the package.json that we actually
		// publish.
		Scripts: map[string]string{
			"build": "tsc",
		},
		DevDependencies: map[string]string{
			"typescript": "^3.0.0",
		},
		Pulumi: npmPulumiManifest{
			Resource: true,
		},
	}

	// Copy the overlay dependencies, if any.
	if jso := g.info.JavaScript; jso != nil {
		for depk, depv := range jso.Dependencies {
			if npminfo.Dependencies == nil {
				npminfo.Dependencies = make(map[string]string)
			}
			npminfo.Dependencies[depk] = depv
		}
		for depk, depv := range jso.DevDependencies {
			if npminfo.DevDependencies == nil {
				npminfo.DevDependencies = make(map[string]string)
			}
			npminfo.DevDependencies[depk] = depv
		}
		for depk, depv := range jso.PeerDependencies {
			if npminfo.PeerDependencies == nil {
				npminfo.PeerDependencies = make(map[string]string)
			}
			npminfo.PeerDependencies[depk] = depv
		}
	}

	// If there is no @pulumi/pulumi, add "latest" as a peer dependency (for npm linking style usage).
	sdkPack := "@pulumi/pulumi"
	if npminfo.Dependencies[sdkPack] == "" &&
		npminfo.DevDependencies[sdkPack] == "" &&
		npminfo.PeerDependencies[sdkPack] == "" {
		if npminfo.PeerDependencies == nil {
			npminfo.PeerDependencies = make(map[string]string)
		}
		npminfo.PeerDependencies["@pulumi/pulumi"] = "latest"
	}

	// Now write out the serialized form.
	npmjson, err := json.MarshalIndent(npminfo, "", "    ")
	if err != nil {
		return err
	}
	w.Writefmtln(string(npmjson))
	return nil
}

func (g *nodeJSGenerator) emitTypeScriptProjectFile(pack *pkg, files []string) error {
	w, err := tools.NewGenWriter(tfgen, filepath.Join(g.outDir, "tsconfig.json"))
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(w)
	w.Writefmtln(`{
    "compilerOptions": {
        "outDir": "bin",
        "target": "es6",
        "module": "commonjs",
        "moduleResolution": "node",
        "declaration": true,
        "sourceMap": true
    },
    "files": [`)
	for i, file := range files {
		var suffix string
		if i != len(files)-1 {
			suffix = ","
		}
		relfile, err := filepath.Rel(g.outDir, file)
		if err != nil {
			return err
		}
		w.Writefmtln("        \"%s\"%s", relfile, suffix)
	}
	w.Writefmtln(`    ]
}
`)
	return nil
}

// relModule removes the path suffix from a module and makes it relative to the root path.
func (g *nodeJSGenerator) relModule(mod *module, path string) (string, error) {
	// Return the path as a relative path to the root, so that imports are relative.
	dir := g.moduleDir(mod)
	file, err := filepath.Rel(dir, path)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(file, ".") {
		file = "./" + file
	}
	return removeExtension(file, ".ts"), nil
}

// removeExtension removes the file extension, if any.
func removeExtension(file, ext string) string {
	if strings.HasSuffix(file, ext) {
		return file[:len(file)-len(ext)]
	}
	return file
}

// importMap is a map of module name to a map of members imported.
type importMap map[string]map[string]bool

// emitCustomImports traverses a custom schema map, deeply, to figure out the set of imported names and files that
// will be required to access those names.  WARNING: this routine doesn't (yet) attempt to eliminate naming collisions.
func (g *nodeJSGenerator) emitCustomImports(w *tools.GenWriter, mod *module, infos []*tfbridge.SchemaInfo) error {
	// First gather up all imports into a map of import module to a list of imported members.
	imports := make(importMap)
	for _, info := range infos {
		if err := g.gatherCustomImports(mod, info, imports); err != nil {
			return err
		}
	}

	// Next, if there were any imports, generate the import statement.  We must sort names to ensure determinism.
	if len(imports) > 0 {
		var files []string
		for file := range imports {
			files = append(files, file)
		}
		sort.Strings(files)

		for _, file := range files {
			var names []string
			for name := range imports[file] {
				names = append(names, name)
			}
			sort.Strings(names)

			w.Writefmt("import {")
			for i, name := range names {
				if i > 0 {
					w.Writefmt(", ")
				}
				w.Writefmt(name)
			}
			w.Writefmtln("} from \"%v\";", file)
		}
		w.Writefmtln("")
	}
	return nil
}

// gatherCustomImports gathers imports from an entire map of schema info, and places them into the target map.
func (g *nodeJSGenerator) gatherCustomImports(mod *module, info *tfbridge.SchemaInfo, imports importMap) error {
	if info != nil {
		// If this property has custom schema types that aren't "simple" (e.g., string, etc), then we need to
		// create a relative module import.  Note that we assume this is local to the current package!
		var custty []tokens.Type
		if info.Type != "" {
			custty = append(custty, info.Type)
			custty = append(custty, info.AltTypes...)
		}
		for _, ct := range custty {
			if !tokens.Token(ct).Simple() {
				// Make a relative module import, based on the module we are importing within.
				haspkg := string(ct.Module().Package().Name())
				if haspkg != g.pkg {
					return errors.Errorf("custom schema type %s was not in the current package %s", haspkg, g.pkg)
				}
				modname := ct.Module().Name()
				modfile := filepath.Join(g.outDir,
					strings.Replace(string(modname), tokens.TokenDelimiter, string(filepath.Separator), -1))
				relmod, err := g.relModule(mod, modfile)
				if err != nil {
					return err
				}

				// Now just mark the member in the resulting map.
				if imports[relmod] == nil {
					imports[relmod] = make(map[string]bool)
				}
				imports[relmod][string(ct.Name())] = true
			}
		}

		// If the property has an element type, recurse and propagate any results.
		if err := g.gatherCustomImports(mod, info.Elem, imports); err != nil {
			return err
		}

		// If the property has fields, then simply recurse and propagate any results, if any, to our map.
		for _, info := range info.Fields {
			if err := g.gatherCustomImports(mod, info, imports); err != nil {
				return err
			}
		}
	}

	return nil
}

// tsFlags returns the TypeScript flags for a given variable.
func tsFlags(v *variable) string {
	return tsFlagsComplex(v.schema, v.info, v.opt, v.out, v.config)
}

// tsFlagsComplex is just like tsFlags, except that it permits recursing into component pieces individually.
func tsFlagsComplex(sch *schema.Schema, info *tfbridge.SchemaInfo, opt, out, config bool) string {
	if opt || optionalComplex(sch, info, out, config) {
		return "?"
	}
	return ""
}

// tsType returns the TypeScript type name for a given schema property.  noflags may be passed as true to create a type
// that represents the optional nature of a variable, even when flags will not be present; this is often needed when
// turning the type into a generic type argument, for example, since there will be no opportunity for "?" there.
// wrapInput can be set to true to cause the generated type to be deeply wrapped with `pulumi.Input<T>`.
func tsType(v *variable, noflags, wrapInput bool) string {
	return tsTypeComplex(v.schema, v.info, noflags, v.out, wrapInput, v.config)
}

// tsTypeComplex is just like tsType, but permits recursing using component pieces rather than a true variable.
func tsTypeComplex(sch *schema.Schema, info *tfbridge.SchemaInfo, noflags, out, wrapInput, config bool) string {
	// First, see if there is a custom override.  If yes, use it directly.
	var t string
	var elem *tfbridge.SchemaInfo
	if info != nil {
		if info.Type != "" {
			t = string(info.Type.Name())
			if !out {
				// Only include AltTypes on inputs, as outputs will always have a concrete type
				if len(info.AltTypes) > 0 {
					for _, at := range info.AltTypes {
						t = fmt.Sprintf("%s | %s", t, at.Name())
					}
				}
			}
			if wrapInput {
				t = fmt.Sprintf("pulumi.Input<%s>", t)
			}
		} else if info.Asset != nil {
			t = "pulumi.asset." + info.Asset.Type()

			if wrapInput {
				t = fmt.Sprintf("pulumi.Input<%s>", t)
			}
		}

		elem = info.Elem
	}

	// If nothing was found, generate the primitive type name for this.
	if t == "" {
		flatten := tfbridge.IsMaxItemsOne(sch, info)
		t = tsPrimitive(sch.Type, sch.Elem, elem, flatten, out, wrapInput, config)
	}

	// If we aren't using optional flags, we need to use TypeScript union types to permit undefined values.
	if noflags {
		if opt := optionalComplex(sch, info, out, config); opt {
			t += " | undefined"
		}
	}

	return t
}

// tsPrimitive returns the TypeScript type name for a given schema value type and element kind.
func tsPrimitive(vt schema.ValueType, elem interface{}, eleminfo *tfbridge.SchemaInfo,
	flatten, out, wrapInput, config bool) string {

	// First figure out the raw type.
	var t string
	switch vt {
	case schema.TypeBool:
		t = "boolean"
	case schema.TypeInt, schema.TypeFloat:
		t = "number"
	case schema.TypeString:
		t = "string"
	case schema.TypeSet, schema.TypeList:
		elemType := tsElemType(elem, eleminfo, out, wrapInput, config)
		if flatten {
			return elemType
		}
		t = fmt.Sprintf("%s[]", elemType)
	case schema.TypeMap:
		// If this map has a "resource" element type, just use the generated element type. This works around a bug in
		// TF that effectively forces this behavior.
		elemType := tsElemType(elem, eleminfo, out, wrapInput, config)
		if _, hasResourceElem := elem.(*schema.Resource); hasResourceElem {
			return elemType
		}
		t = fmt.Sprintf("{[key: string]: %v}", elemType)
	default:
		contract.Failf("Unrecognized schema type: %v", vt)
	}

	if wrapInput {
		t = fmt.Sprintf("pulumi.Input<%s>", t)
	}

	return t
}

// tsElemType returns the TypeScript type for a given schema element.  This element may be either a simple schema
// property or a complex structure.  In the case of a complex structure, this will expand to its nominal type.
func tsElemType(elem interface{}, info *tfbridge.SchemaInfo, out, wrapInput, config bool) string {
	// If there is no element type specified, we will accept anything.
	if elem == nil {
		return "any"
	}

	switch e := elem.(type) {
	case schema.ValueType:
		return tsPrimitive(e, nil, info, false, out, wrapInput, config)
	case *schema.Schema:
		// A simple type, just return its type name.
		return tsTypeComplex(e, info, true /*noflags*/, out, wrapInput, config)
	case *schema.Resource:
		// A complex type, just expand to its nominal type name.
		// TODO: spill all complex structures in advance so that we don't have insane inline expansions.
		t := "{ "
		c := 0
		for _, s := range stableSchemas(e.Schema) {
			var fldinfo *tfbridge.SchemaInfo
			if info != nil {
				fldinfo = info.Fields[s]
			}
			sch := e.Schema[s]
			if name := propertyName(s, sch, fldinfo); name != "" {
				if c > 0 {
					t += ", "
				}
				flg := tsFlagsComplex(sch, fldinfo, false, out, config)
				typ := tsTypeComplex(sch, fldinfo, false /*noflags*/, out, wrapInput, config)
				t += fmt.Sprintf("%s%s: %s", name, flg, typ)
				c++
			}
		}
		t += " }"
		if wrapInput {
			t = fmt.Sprintf("pulumi.Input<%s>", t)
		}
		return t
	default:
		contract.Failf("Unrecognized schema element type: %v", e)
		return ""
	}
}

func tsPrimitiveValue(value interface{}) (string, error) {
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

func tsDefaultValue(prop *variable) string {
	defaultValue := "undefined"
	if prop.info == nil || prop.info.Default == nil {
		return defaultValue
	}
	defaults := prop.info.Default

	if defaults.Value != nil {
		dv, err := tsPrimitiveValue(defaults.Value)
		if err != nil {
			cmdutil.Diag().Warningf(diag.Message("", err.Error()))
			return defaultValue
		}
		defaultValue = dv
	}

	if len(defaults.EnvVars) != 0 {
		getType := ""
		switch prop.schema.Type {
		case schema.TypeBool:
			getType = "Boolean"
		case schema.TypeInt, schema.TypeFloat:
			getType = "Number"
		}

		envVars := fmt.Sprintf("%q", defaults.EnvVars[0])
		for _, e := range defaults.EnvVars[1:] {
			envVars += fmt.Sprintf(", %q", e)
		}

		getEnv := fmt.Sprintf("utilities.getEnv%s(%s)", getType, envVars)
		if defaultValue != "undefined" {
			defaultValue = fmt.Sprintf("(%s || %s)", getEnv, defaultValue)
		} else {
			defaultValue = getEnv
		}
	}

	return defaultValue
}
