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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
//nolint:goconst
package tfgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/gedex/inflector"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	pygen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/paths"
)

const (
	tfbridge20 = "tfbridge20"
)

type schemaGenerator struct {
	pkg            tokens.Package
	version        string
	info           tfbridge.ProviderInfo
	renamesBuilder *renamesBuilder
}

type schemaNestedType struct {
	typ             *propertyType
	declarer        declarer
	required        codegen.StringSet
	requiredInputs  codegen.StringSet
	requiredOutputs codegen.StringSet

	// The same *propertyType may be found at multiple typePaths and reused. Non-empty.
	typePaths paths.TypePathSet
}

type schemaNestedTypes struct {
	nameToType map[string]*schemaNestedType
}

func gatherSchemaNestedTypesForModule(mod *module) map[string]*schemaNestedType {
	nt := &schemaNestedTypes{
		nameToType: make(map[string]*schemaNestedType),
	}
	for _, member := range mod.members {
		nt.gatherFromMember(member)
	}
	return nt.nameToType
}

func gatherSchemaNestedTypesForMember(member moduleMember) map[string]*schemaNestedType {
	nt := &schemaNestedTypes{
		nameToType: make(map[string]*schemaNestedType),
	}
	nt.gatherFromMember(member)
	return nt.nameToType
}

func (nt *schemaNestedTypes) gatherFromMember(member moduleMember) {
	switch member := member.(type) {
	case *resourceType:
		p := member.resourcePath
		nt.gatherFromProperties(p.Inputs(), member, member.name, member.inprops, true)
		nt.gatherFromProperties(p.Outputs(), member, member.name, member.outprops, false)
		if !member.IsProvider() {
			nt.gatherFromProperties(p.State(), member, member.name, member.statet.properties, true)
		}
	case *resourceFunc:
		p := member.dataSourcePath
		nt.gatherFromProperties(p.Args(), member, member.name, member.args, true)
		nt.gatherFromProperties(p.Results(), member, member.name, member.rets, false)
	case *variable:
		contract.Assertf(member.config, `member.config`)
		p := paths.NewProperyPath(paths.NewConfigPath(), member.propertyName)
		nt.gatherFromPropertyType(p, member, member.name, "", member.typ, false)
	}
}

type declarer interface {
	Name() string
}

func (nt *schemaNestedTypes) declareType(typePath paths.TypePath, declarer declarer, namePrefix, name string,
	typ *propertyType, isInput bool) string {

	// Generate a name for this nested type.
	typeName := namePrefix + cases.Title(language.Und, cases.NoLower).String(name)

	// Override the nested type name, if necessary.
	if typ.nestedType.Name().String() != "" {
		typeName = typ.nestedType.Name().String()
	}

	typ.name = typeName

	required := codegen.StringSet{}
	for _, p := range typ.properties {
		if !p.optional() {
			required.Add(p.name)
		}
	}

	var requiredInputs, requiredOutputs codegen.StringSet
	if isInput {
		requiredInputs = required
	} else {
		requiredOutputs = required
	}

	// Merging makes sure that structurally identical types are shared and not generated more than once.
	if existing, ok := nt.nameToType[typeName]; ok {
		contract.Assertf(existing.declarer == declarer || existing.typ.equals(typ), "duplicate type %v", typeName)

		// Remember that existing type is now also seen at the current typePath.
		existing.typePaths.Add(typePath)

		// For output type conflicts, record the output type's required properties. These will be attached to a
		// nodejs-specific blob in the object type's spec s.t. the node code generator can generate code that
		// matches the code produced by the old tfgen code generator.
		if isInput {
			existing.requiredInputs = requiredInputs
		} else {
			existing.requiredOutputs = requiredOutputs
		}

		existing.typ, existing.required = typ, required
		return typeName
	}

	nt.nameToType[typeName] = &schemaNestedType{
		typ:             typ,
		declarer:        declarer,
		required:        required,
		requiredInputs:  requiredInputs,
		requiredOutputs: requiredOutputs,
		typePaths:       paths.SingletonTypePathSet(typePath),
	}
	return typeName
}

func (nt *schemaNestedTypes) gatherFromProperties(pathContext paths.TypePath,
	declarer declarer, namePrefix string, ps []*variable,
	isInput bool) {

	for _, p := range ps {
		name := p.name
		if p.typ.kind == kindList || p.typ.kind == kindSet {
			name = inflector.Singularize(name)
		}

		nt.gatherFromPropertyType(paths.NewProperyPath(pathContext, p.propertyName),
			declarer, namePrefix, name, p.typ, isInput)
	}
}

func (nt *schemaNestedTypes) gatherFromPropertyType(typePath paths.TypePath, declarer declarer, namePrefix,
	name string, typ *propertyType, isInput bool) {

	switch typ.kind {
	case kindList, kindSet, kindMap:
		if typ.element != nil {
			nt.gatherFromPropertyType(paths.NewElementPath(typePath),
				declarer, namePrefix, name, typ.element, isInput)
		}
	case kindObject:
		baseName := nt.declareType(typePath, declarer, namePrefix, name, typ, isInput)
		nt.gatherFromProperties(typePath, declarer, baseName, typ.properties, isInput)
	}
}

func rawMessage(v interface{}) pschema.RawMessage {
	bytes, err := json.Marshal(v)
	contract.Assertf(err == nil, `err == nil`)
	return pschema.RawMessage(bytes)
}

func genPulumiSchema(pack *pkg, name tokens.Package, version string,
	info tfbridge.ProviderInfo, renamesBuilder *renamesBuilder) (pschema.PackageSpec, error) {

	g := &schemaGenerator{
		pkg:            name,
		version:        version,
		info:           info,
		renamesBuilder: renamesBuilder,
	}
	return g.genPackageSpec(pack)
}

func (g *schemaGenerator) genPackageSpec(pack *pkg) (pschema.PackageSpec, error) {
	spec := pschema.PackageSpec{
		Name:              g.pkg.String(),
		Version:           g.version,
		Keywords:          g.info.Keywords,
		Homepage:          g.info.Homepage,
		License:           g.info.License,
		Repository:        g.info.Repository,
		Publisher:         g.info.Publisher,
		DisplayName:       g.info.DisplayName,
		PluginDownloadURL: g.info.PluginDownloadURL,
		Resources:         map[string]pschema.ResourceSpec{},
		Functions:         map[string]pschema.FunctionSpec{},
		Types:             map[string]pschema.ComplexTypeSpec{},
		Language:          map[string]pschema.RawMessage{},

		Meta: &pschema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
	}

	if g.info.LogoURL != "" {
		spec.LogoURL = g.info.LogoURL
	}

	spec.Description = g.info.Description
	spec.Attribution = fmt.Sprintf(attributionFormatString, g.info.Name, g.info.GetGitHubOrg(), g.info.GetGitHubHost())

	var config []*variable
	for _, mod := range pack.modules.values() {
		// Generate nested types.
		for _, t := range gatherSchemaNestedTypesForModule(mod) {
			tok := g.genObjectTypeToken(t)
			ts := g.genObjectType(t, false)
			spec.Types[tok] = pschema.ComplexTypeSpec{
				ObjectTypeSpec: ts,
			}
		}

		// Enumerate each module member, in the order presented to us, and do the right thing.
		for _, member := range mod.members {
			switch t := member.(type) {
			case *resourceType:
				spec.Resources[string(t.info.Tok)] = g.genResourceType(mod.name, t)
			case *resourceFunc:
				spec.Functions[string(t.info.Tok)] = g.genDatasourceFunc(mod.name, t)
			case *variable:
				contract.Assertf(mod.config(), `mod.config()`)
				config = append(config, t)
			}
		}
	}

	if len(config) != 0 {
		spec.Config = g.genConfig(config)
	}

	if pack.provider != nil {
		indexModToken := tokens.NewModuleToken(g.pkg, indexMod)
		for _, t := range gatherSchemaNestedTypesForMember(pack.provider) {
			tok := g.genObjectTypeToken(t)
			ts := g.genObjectType(t, false)
			spec.Types[tok] = pschema.ComplexTypeSpec{
				ObjectTypeSpec: ts,
			}
		}
		spec.Provider = g.genResourceType(indexModToken, pack.provider)

		// Ensure that input properties are mirrored as output properties, but without fields set which
		// are only meaningful for input properties.
		spec.Provider.Required = spec.Provider.RequiredInputs
		spec.Provider.Properties = map[string]pschema.PropertySpec{}

		for propName, prop := range spec.Provider.InputProperties {
			outputProp := prop
			outputProp.Default = nil
			outputProp.DefaultInfo = nil
			outputProp.Const = nil

			spec.Provider.Properties[propName] = outputProp
		}
	}

	for token, typ := range g.info.ExtraTypes {
		if _, defined := spec.Types[token]; defined {
			return pschema.PackageSpec{}, fmt.Errorf("failed to define extra types: %v is already defined", token)
		}
		spec.Types[token] = typ
	}

	for token, res := range g.info.ExtraResources {
		if _, defined := spec.Resources[token]; defined {
			return pschema.PackageSpec{}, fmt.Errorf("failed to define extra resources: %v is already defined", token)
		}
		spec.Resources[token] = res
	}

	for token, fun := range g.info.ExtraFunctions {
		if _, defined := spec.Functions[token]; defined {
			return pschema.PackageSpec{}, fmt.Errorf("failed to define extra functions: %v is already defined", token)
		}
		spec.Functions[token] = fun
	}

	downstreamLicense := g.info.GetTFProviderLicense()
	licenseTypeURL := getLicenseTypeURL(downstreamLicense)

	readme := ""
	if downstreamLicense != tfbridge.UnlicensedLicenseType {
		readme = getDefaultReadme(g.pkg, g.info.Name, g.info.GetGitHubOrg(), downstreamLicense, licenseTypeURL,
			g.info.GetGitHubHost(), g.info.Repository)
	}

	nodeData := map[string]interface{}{
		"compatibility":           tfbridge20,
		"readme":                  readme,
		"disableUnionOutputTypes": true,
	}
	if jsi := g.info.JavaScript; jsi != nil {
		nodeData["packageName"] = jsi.PackageName
		nodeData["packageDescription"] = generateManifestDescription(g.info)
		nodeData["dependencies"] = jsi.Dependencies
		nodeData["devDependencies"] = jsi.DevDependencies
		nodeData["typescriptVersion"] = jsi.TypeScriptVersion
		nodeData["pluginName"] = jsi.PluginName
		nodeData["pluginVersion"] = jsi.PluginVersion
	}
	spec.Language["nodejs"] = rawMessage(nodeData)

	python, err := pythonLanguageExtensions(g.info.Python, readme)
	if err != nil {
		return pschema.PackageSpec{}, err
	}
	spec.Language["python"] = python

	if csi := g.info.CSharp; csi != nil {
		dotnetData := map[string]interface{}{
			"compatibility":     tfbridge20,
			"packageReferences": csi.PackageReferences,
			"namespaces":        csi.Namespaces,
		}
		if rootNamespace := csi.RootNamespace; rootNamespace != "" {
			dotnetData["rootNamespace"] = rootNamespace
		}
		spec.Language["csharp"] = rawMessage(dotnetData)
	}

	if goi := g.info.Golang; goi != nil {
		goext, err := goLanguageExtensions(goi)
		if err != nil {
			return pschema.PackageSpec{}, err
		}
		spec.Language["go"] = goext
	}

	if javai := g.info.Java; javai != nil {
		spec.Language["java"] = rawMessage(map[string]interface{}{
			"basePackage":                     javai.BasePackage,
			"buildFiles":                      javai.BuildFiles,
			"gradleNexusPublishPluginVersion": javai.GradleNexusPublishPluginVersion,
		})
	}

	// Validate the schema.
	_, diags, err := pschema.BindSpec(spec, nil)
	if err != nil {
		return pschema.PackageSpec{}, err
	}
	if diags.HasErrors() {
		return pschema.PackageSpec{}, diags
	}

	return spec, nil
}

func conflict(prop, subProp, finalProp string) error {
	return fmt.Errorf("ProviderInfo.%[1]s.%[3]s conflicts "+
		"with ProviderInfo.%[1]s.%[2]s, "+
		"please use ProviderInfo.%[1]s.%[2]s.%[3]s instead",
		prop, subProp, finalProp)
}

func goLanguageExtensions(goi *tfbridge.GolangInfo) (pschema.RawMessage, error) {
	if goi.GoPackageInfo != nil {
		if goi.ImportBasePath != "" {
			return nil, conflict("Golang", "GoPackageInfo", "ImportBasePath")
		}
		if goi.GenerateResourceContainerTypes {
			return nil, conflict("Golang", "GoPackageInfo", "GenerateResourceContainerTypes")
		}
		return rawMessage(goi.GoPackageInfo), nil
	}
	return rawMessage(map[string]interface{}{
		"importBasePath":                 goi.ImportBasePath,
		"generateResourceContainerTypes": goi.GenerateResourceContainerTypes,
		"generateExtraInputTypes":        true,
	}), nil
}

func pythonLanguageExtensions(pi *tfbridge.PythonInfo, readme string) (pschema.RawMessage, error) {
	info := pi.PythonPackageInfo
	if info != nil {
		if pi.PackageName != "" {
			return nil, conflict("Python", "PythonPackageInfo", "PackageName")
		}
		if pi.Overlay != nil {
			return nil, conflict("Python", "PythonPackageInfo", "Overlay")
		}
		if len(pi.Requires) > 0 {
			return nil, conflict("Python", "PythonPackageInfo", "Requires")
		}
		if pi.UsesIOClasses {
			return nil, conflict("Python", "PythonPackageInfo", "UsesIOClasses")
		}
	} else {
		info = &pygen.PackageInfo{}
	}
	if info.Compatibility == "" {
		info.Compatibility = tfbridge20
	}
	if info.Readme == "" {
		info.Readme = readme
	}
	return rawMessage(info), nil
}

func getDefaultReadme(pulumiPackageName tokens.Package, tfProviderShortName string, tfGitHubOrg string,
	pulumiProvLicense tfbridge.TFProviderLicense, pulumiProvLicenseURI string, githubHost string,
	pulumiProvRepo string) string {

	//nolint:lll
	standardDocReadme := `> This provider is a derived work of the [Terraform Provider](https://%[6]s/%[3]s/terraform-provider-%[2]s)
> distributed under [%[4]s](%[5]s). If you encounter a bug or missing feature,
> first check the [` + "`pulumi-%[1]s`" + ` repo](%[7]s/issues); however, if that doesn't turn up anything,
> please consult the source [` + "`terraform-provider-%[2]s`" + ` repo](https://%[6]s/%[3]s/terraform-provider-%[2]s/issues).`

	return fmt.Sprintf(standardDocReadme, pulumiPackageName, tfProviderShortName, tfGitHubOrg, pulumiProvLicense,
		pulumiProvLicenseURI, githubHost, pulumiProvRepo)
}

func (g *schemaGenerator) genDocComment(comment string) string {
	if comment == elidedDocComment {
		return ""
	}

	buffer := &bytes.Buffer{}
	if comment != elidedDocComment {
		lines := strings.Split(comment, "\n")
		for i, docLine := range lines {
			// Break if we get to the last line and it's empty
			if i == len(lines)-1 && strings.TrimSpace(docLine) == "" {
				break
			}
			fmt.Fprintf(buffer, "%s\n", docLine)
		}
	}

	return buffer.String()
}

func (g *schemaGenerator) genRawDocComment(comment string) string {
	if comment == "" {
		return ""
	}

	buffer := &bytes.Buffer{}

	curr := 0
	for _, word := range strings.Fields(comment) {
		if curr > 0 {
			if curr+len(word)+1 > maxWidth {
				curr = 0
				fmt.Fprintf(buffer, "\n")
			} else {
				fmt.Fprintf(buffer, " ")
				curr++
			}
		}
		fmt.Fprintf(buffer, "%s", word)
		curr += len(word)
	}
	fmt.Fprintf(buffer, "\n")
	return buffer.String()
}

func (g *schemaGenerator) genProperty(prop *variable) pschema.PropertySpec {
	description := ""
	if prop.doc != "" && prop.doc != elidedDocComment {
		description = g.genDocComment(prop.doc)
	} else if prop.rawdoc != "" {
		description = g.genRawDocComment(prop.rawdoc)
	}

	language := map[string]pschema.RawMessage{}
	if prop.info != nil && prop.info.CSharpName != "" {
		language["csharp"] = rawMessage(map[string]string{"name": prop.info.CSharpName})
	}

	var defaultValue interface{}
	var defaultInfo *pschema.DefaultSpec
	if prop.info != nil && prop.info.Default != nil {
		if defaults := prop.info.Default; defaults.Value != nil || len(defaults.EnvVars) > 0 {
			defaultValue = defaults.Value
			if i, ok := defaultValue.(int); ok {
				defaultValue = float64(i)
			}

			if len(defaults.EnvVars) != 0 {
				defaultInfo = &pschema.DefaultSpec{
					Environment: defaults.EnvVars,
				}
			}
		}
	}

	var secret bool
	// First check if the property is marked sensitive in TF schema.
	if prop.schema.Sensitive() {
		secret = true
	}

	// Check custom info. This order allows custom info to override the above as well if necessary.
	if prop.info != nil && prop.info.Secret != nil {
		secret = *prop.info.Secret
	}

	propPath := paths.NewProperyPath(prop.parentPath, prop.propertyName)
	return pschema.PropertySpec{
		TypeSpec:             g.schemaType(propPath, prop.typ, prop.out),
		Description:          description,
		Default:              defaultValue,
		DefaultInfo:          defaultInfo,
		DeprecationMessage:   prop.deprecationMessage(),
		Language:             language,
		Secret:               secret,
		WillReplaceOnChanges: prop.forceNew(),
	}
}

func (g *schemaGenerator) genConfig(variables []*variable) pschema.ConfigSpec {
	spec := pschema.ConfigSpec{
		Variables: make(map[string]pschema.PropertySpec),
	}
	for _, v := range variables {
		spec.Variables[v.name] = g.genProperty(v)
		if !v.optional() {
			spec.Required = append(spec.Required, v.name)
		}
	}
	return spec
}

func (g *schemaGenerator) genResourceType(mod tokens.Module, res *resourceType) pschema.ResourceSpec {
	var spec pschema.ResourceSpec

	description := ""
	if res.doc != "" {
		description = g.genDocComment(res.doc)
	}
	if !res.IsProvider() {
		if res.info.DeprecationMessage != "" {
			spec.DeprecationMessage = res.info.DeprecationMessage
		}
	}
	spec.Description = description

	spec.Properties = map[string]pschema.PropertySpec{}

	for _, prop := range res.outprops {
		// The property will be dropped from the schema
		if prop.info != nil && prop.info.Omit {
			if prop.schema.Required() {
				contract.Failf("required property %q may not be omitted from binding generation", prop.name)
			} else {
				continue
			}
		}
		// let's check that we are not trying to add a duplicate computed id property
		if prop.name == "id" {
			continue
		}

		spec.Properties[prop.name] = g.genProperty(prop)

		if !prop.optional() {
			spec.Required = append(spec.Required, prop.name)
		}
	}

	spec.InputProperties = map[string]pschema.PropertySpec{}
	for _, prop := range res.inprops {
		if prop.info != nil && prop.info.Omit {
			if prop.schema.Required() {
				contract.Failf("required input property %q may not be omitted from binding generation", prop.name)
			} else {
				continue
			}
		}
		// let's check that we are not trying to add a duplicate computed id property
		if prop.name == "id" {
			continue
		}
		spec.InputProperties[prop.name] = g.genProperty(prop)

		if !prop.optional() {
			spec.RequiredInputs = append(spec.RequiredInputs, prop.name)
		}
	}

	if !res.IsProvider() {
		stateTypePath := res.resourcePath.State()
		stateInputs := g.genObjectType(&schemaNestedType{
			typ:       res.statet,
			typePaths: paths.SingletonTypePathSet(stateTypePath),
		}, true)
		spec.StateInputs = &stateInputs
	}

	for _, a := range res.info.Aliases {
		spec.Aliases = append(spec.Aliases, pschema.AliasSpec{
			Name:    a.Name,
			Project: a.Project,
			Type:    a.Type,
		})
	}

	return spec
}

func (g *schemaGenerator) genDatasourceFunc(mod tokens.Module, fun *resourceFunc) pschema.FunctionSpec {
	var spec pschema.FunctionSpec

	description := ""
	if fun.doc != "" {
		description = g.genDocComment(fun.doc)
	}
	if fun.info.DeprecationMessage != "" {
		spec.DeprecationMessage = fun.info.DeprecationMessage
	}
	spec.Description = description

	// If there are argument and/or return types, emit them.
	if fun.argst != nil {
		t := g.genObjectType(&schemaNestedType{
			typ:       fun.argst,
			typePaths: paths.SingletonTypePathSet(fun.dataSourcePath.Args()),
		}, false)
		spec.Inputs = &t
	}
	if fun.retst != nil {
		t := g.genObjectType(&schemaNestedType{
			typ:       fun.retst,
			typePaths: paths.SingletonTypePathSet(fun.dataSourcePath.Results()),
		}, false)
		spec.Outputs = &t
	}

	return spec
}

func setEquals(a, b codegen.StringSet) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b.Has(k) {
			return false
		}
	}
	return true
}

func (g *schemaGenerator) genObjectTypeToken(typInfo *schemaNestedType) string {
	typ := typInfo.typ
	contract.Assertf(typ.kind == kindObject, `typ.kind == kindObject`)

	name := typ.name
	if typ.nestedType != "" {
		name = string(typ.nestedType)
	}

	mod := modulePlacementForTypeSet(g.pkg, typInfo.typePaths)
	token := fmt.Sprintf("%s/%s:%s", mod.String(), name, name)

	g.renamesBuilder.registerNamedObjectType(typInfo.typePaths, tokens.Type(token))

	return token
}

func (g *schemaGenerator) genObjectType(typInfo *schemaNestedType, isTopLevel bool) pschema.ObjectTypeSpec {
	typ := typInfo.typ
	contract.Assertf(typ.kind == kindObject, `typ.kind == kindObject`)

	spec := pschema.ObjectTypeSpec{
		Type: "object",
	}

	if typ.doc != "" {
		spec.Description = g.genDocComment(typ.doc)
	}

	spec.Properties = map[string]pschema.PropertySpec{}
	for _, prop := range typ.properties {
		if prop.info != nil && prop.info.Omit {
			if prop.schema.Required() {
				contract.Failf("required object property %q may not be omitted from binding generation", prop.name)
			} else {
				continue
			}
		}
		// let's not build any additional ID properties - we don't want to exclude any required id properties
		if isTopLevel && prop.name == "id" {
			continue
		}
		spec.Properties[prop.name] = g.genProperty(prop)

		if !prop.optional() {
			spec.Required = append(spec.Required, prop.name)
		}
	}

	nodeInfo := map[string]interface{}{}
	if !setEquals(typInfo.required, typInfo.requiredInputs) {
		requiredInputs := make([]string, 0, len(typInfo.requiredInputs))
		for name := range typInfo.requiredInputs {
			requiredInputs = append(requiredInputs, name)
		}
		sort.Strings(requiredInputs)
		nodeInfo["requiredInputs"] = requiredInputs
	}
	if !setEquals(typInfo.required, typInfo.requiredOutputs) {
		requiredOutputs := make([]string, 0, len(typInfo.requiredOutputs))
		for name := range typInfo.requiredOutputs {
			requiredOutputs = append(requiredOutputs, name)
		}
		sort.Strings(requiredOutputs)
		nodeInfo["requiredOutputs"] = requiredOutputs
	}
	if len(nodeInfo) != 0 {
		spec.Language = map[string]pschema.RawMessage{
			"nodejs": rawMessage(nodeInfo),
		}
	}

	return spec
}

func (g *schemaGenerator) schemaPrimitiveType(k typeKind) string {
	switch k {
	case kindBool:
		return "boolean"
	case kindInt:
		return "integer"
	case kindFloat:
		return "number"
	case kindString:
		return "string"
	default:
		return ""
	}
}

func (g *schemaGenerator) schemaType(path paths.TypePath, typ *propertyType, out bool) pschema.TypeSpec {
	// Prefer overrides over the underlying type.
	switch {
	case typ == nil:
		return pschema.TypeSpec{Ref: "pulumi.json#/Any"}
	case typ.typ != "" || len(typ.altTypes) != 0:
		// Compute the default type for the union. May be empty.
		defaultType := g.schemaPrimitiveType(typ.kind)

		var toks []tokens.Type
		if typ.typ != "" {
			toks = []tokens.Type{typ.typ}
		}

		if !out {
			toks = append(toks, typ.altTypes...)
		}

		var typs []pschema.TypeSpec
		for _, t := range toks {
			if tokens.Token(t).Simple() {
				typs = append(typs, pschema.TypeSpec{Type: string(t)})
			} else {
				tPkg := t.Module().Package()
				pkg := tPkg.Name().String()
				if tPkg == g.pkg {
					pkg = ""
				}
				spec := pschema.TypeSpec{
					Type: defaultType,
					Ref:  fmt.Sprintf("%s#/types/%s", pkg, strings.TrimSuffix(string(t), "[]")),
				}
				if strings.HasSuffix(string(t), "[]") {
					items := spec
					spec = pschema.TypeSpec{Type: "array", Items: &items}
				}
				typs = append(typs, spec)
			}
		}
		if len(typs) == 1 {
			return typs[0]
		}
		return pschema.TypeSpec{
			Type:  defaultType,
			OneOf: typs,
		}
	case typ.asset != nil:
		if typ.asset.IsArchive() {
			return pschema.TypeSpec{Ref: "pulumi.json#/Archive"}
		}
		return pschema.TypeSpec{Ref: "pulumi.json#/Asset"}
	}

	// First figure out the raw type.
	switch typ.kind {
	case kindBool, kindInt, kindFloat, kindString:
		t := g.schemaPrimitiveType(typ.kind)
		contract.Assertf(t != "", `t != ""`)
		return pschema.TypeSpec{Type: t}
	case kindSet, kindList:
		items := g.schemaType(paths.NewElementPath(path), typ.element, out)
		return pschema.TypeSpec{Type: "array", Items: &items}
	case kindMap:
		additionalProperties := g.schemaType(paths.NewElementPath(path), typ.element, out)
		return pschema.TypeSpec{Type: "object", AdditionalProperties: &additionalProperties}
	case kindObject:
		mod := modulePlacementForType(g.pkg, path)
		ref := fmt.Sprintf("#/types/%s/%s:%s", mod.String(), typ.name, typ.name)
		return pschema.TypeSpec{Ref: ref}
	default:
		contract.Failf("Unrecognized type kind: %v", typ.kind)
		return pschema.TypeSpec{}
	}
}

func (g *Generator) convertExamplesInPropertySpec(path examplePath, spec pschema.PropertySpec) pschema.PropertySpec {
	spec.Description = g.convertExamples(spec.Description, path, false)
	spec.DeprecationMessage = g.convertExamples(spec.DeprecationMessage, path, false)
	return spec
}

func (g *Generator) convertExamplesInObjectSpec(path examplePath, spec pschema.ObjectTypeSpec) pschema.ObjectTypeSpec {
	spec.Description = g.convertExamples(spec.Description, path, false)
	for name, prop := range spec.Properties {
		spec.Properties[name] = g.convertExamplesInPropertySpec(path.Property(name), prop)
	}
	return spec
}

func (g *Generator) convertExamplesInResourceSpec(path examplePath, spec pschema.ResourceSpec) pschema.ResourceSpec {
	spec.Description = g.convertExamples(spec.Description, path, true)
	spec.DeprecationMessage = g.convertExamples(spec.DeprecationMessage, path, false)
	for name, prop := range spec.Properties {
		spec.Properties[name] = g.convertExamplesInPropertySpec(path.Property(name), prop)
	}
	for name, prop := range spec.InputProperties {
		spec.InputProperties[name] = g.convertExamplesInPropertySpec(path.Property(name), prop)
	}
	if spec.StateInputs != nil {
		stateInputs := g.convertExamplesInObjectSpec(path.StateInputs(), *spec.StateInputs)
		spec.StateInputs = &stateInputs
	}
	return spec
}

func (g *Generator) convertExamplesInFunctionSpec(path examplePath, spec pschema.FunctionSpec) pschema.FunctionSpec {
	spec.Description = g.convertExamples(spec.Description, path, true)
	if spec.Inputs != nil {
		inputs := g.convertExamplesInObjectSpec(path.Inputs(), *spec.Inputs)
		spec.Inputs = &inputs
	}
	if spec.Outputs != nil {
		outputs := g.convertExamplesInObjectSpec(path.Outputs(), *spec.Outputs)
		spec.Outputs = &outputs
	}
	return spec
}

func (g *Generator) convertExamplesInSchema(spec pschema.PackageSpec) pschema.PackageSpec {
	for name, variable := range spec.Config.Variables {
		path := newExamplePathForProviderConfigVariable(name)
		spec.Config.Variables[name] = g.convertExamplesInPropertySpec(path, variable)
	}
	for token, object := range spec.Types {
		path := newExamplePathForNamedType(token)
		object.ObjectTypeSpec = g.convertExamplesInObjectSpec(path, object.ObjectTypeSpec)
		spec.Types[token] = object
	}
	spec.Provider = g.convertExamplesInResourceSpec(newExamplePathForProvider(), spec.Provider)
	for token, resource := range spec.Resources {
		path := newExamplePathForResource(token)
		spec.Resources[token] = g.convertExamplesInResourceSpec(path, resource)
	}
	for token, function := range spec.Functions {
		path := newExamplePathForFunction(token)
		spec.Functions[token] = g.convertExamplesInFunctionSpec(path, function)
	}
	return spec
}

func addExtraHclExamplesToResources(extraExamples []tfbridge.HclExampler, spec *pschema.PackageSpec) error {
	var err error
	for _, ex := range extraExamples {
		token := ex.GetToken()
		res, ok := spec.Resources[token]
		if !ok {
			err = multierror.Append(err, fmt.Errorf("there is a supplemental HCL example for the resource with "+
				"token '%s', but no matching resource was found in the schema", token))
			continue
		}

		markdown, markdownErr := ex.GetMarkdown()
		if markdownErr != nil {
			err = multierror.Append(err,
				fmt.Errorf("unable to retrieve markdown for example for '%s': %w", token, markdownErr))
			continue
		}

		res.Description = appendExample(res.Description, markdown)
		spec.Resources[token] = res
	}

	return err
}

func addExtraHclExamplesToFunctions(extraExamples []tfbridge.HclExampler, spec *pschema.PackageSpec) error {
	var err error
	for _, ex := range extraExamples {
		token := ex.GetToken()
		fun, ok := spec.Functions[token]
		if !ok {
			err = multierror.Append(err, fmt.Errorf("there is a supplemental HCL example for the function with "+
				"token '%s', but no matching resource was found in the schema", token))
			continue
		}

		markdown, markdownErr := ex.GetMarkdown()
		if markdownErr != nil {
			err = multierror.Append(err, fmt.Errorf("unable to retrieve markdown for example for '%s': %w",
				token, markdownErr))
			continue
		}

		fun.Description = appendExample(fun.Description, markdown)
		spec.Functions[token] = fun
	}

	return err
}

func appendExample(description, markdownToAppend string) string {
	if markdownToAppend == "" {
		return description
	}

	const exampleUsageHeader = "## Example Usage"

	descLines := strings.Split(description, "\n")
	sections := groupLines(descLines, "## ")

	// If there's already an ## Example Usage section, we need to find this section and append
	if strings.Contains(description, exampleUsageHeader) {
		for i, section := range sections {
			if len(section) == 0 {
				continue
			}

			if strings.Index(section[0], exampleUsageHeader) == 0 {
				sections[i] = append(section, strings.Split(markdownToAppend, "\n")...)
				break
			}
		}
	} else {
		// If not, we need to add the header and append before the first ## in the doc, or EOF, whichever comes first
		markdownToAppend = fmt.Sprintf("%s\n\n%s", exampleUsageHeader, markdownToAppend)

		// If there's no blank line after the content, we need to add it to ensure we have semantically valid Markdown:
		if sections[0][len(sections[0])-1] != "" {
			sections[0] = append(sections[0], "")
		}

		sections[0] = append(sections[0], strings.Split(markdownToAppend, "\n")...)
	}

	reassembledLines := []string{}
	for _, section := range sections {
		reassembledLines = append(reassembledLines, section...)
	}
	return strings.Join(reassembledLines, "\n")
}

func modulePlacementForTypeSet(pkg tokens.Package, set paths.TypePathSet) tokens.Module {
	var firstModule tokens.Module
	seenModules := map[tokens.Module]bool{}
	ambModulePlacements := []string{}
	paths := set.Paths()
	if len(paths) == 0 {
		panic("paths should not be an empty set")
	}
	for i, p := range paths {
		m := modulePlacementForType(pkg, p)
		if i == 0 {
			firstModule = m
		}
		if !seenModules[m] && len(seenModules) > 0 {
			plc := fmt.Sprintf("  %s -> %s", p.String(), m.String())
			ambModulePlacements = append(ambModulePlacements, plc)
		}
		seenModules[m] = true
	}
	if len(ambModulePlacements) > 0 {
		fmt.Printf("WARN: ambiguous module placement (picked %s):\n%s\n",
			firstModule.String(), strings.Join(ambModulePlacements, "\n"))
	}
	return firstModule
}

func modulePlacementForType(pkg tokens.Package, path paths.TypePath) tokens.Module {
	// Compute ancestor path. Every TypePath starts from a root.
	p := path
	for p.Parent() != nil {
		p = p.Parent()
	}

	// Pattern match by ancestor.
	switch pp := p.(type) {
	case *paths.ResourceMemberPath:
		res := pp.ResourcePath
		if res.IsProvider() {
			// Supplementary provider types are defined in
			// the same module as the provider (typically
			// root module).
			return res.Token().Module()
		}
		// Supplementary types are typically defined one level up from the module defining the resource, but may
		// also be defined in the same module.
		m := res.Token().Module()
		return parentModuleOrSelf(m)
	case *paths.DataSourceMemberPath:
		// Supplementary types are typically defined one level up from the module defining the data source, but
		// may also be defined in the same module.
		m := pp.DataSourcePath.Token().Module()
		return parentModuleOrSelf(m)
	case *paths.ConfigPath:
		return tokens.NewModuleToken(pkg, configMod)
	default:
		contract.Assertf(false, "invalid ParentKind")
		return ""
	}
}

func parentModuleOrSelf(self tokens.Module) tokens.Module {
	if m, ok := parentModule(self); ok {
		return m
	}
	return self
}

func parentModule(m tokens.Module) (tokens.Module, bool) {
	if !strings.Contains(string(m.Name()), tokens.QNameDelimiter) {
		return "", false
	}
	return tokens.NewModuleToken(m.Package(), parentModuleName(m.Name())), true
}

func parentModuleName(m tokens.ModuleName) tokens.ModuleName {
	modName := tokens.QName(m)
	parentModName := modName.Namespace()
	return tokens.ModuleName(parentModName)
}
