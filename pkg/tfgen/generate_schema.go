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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gedex/inflector"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/pkg/errors"
	pschema "github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
)

type schemaGenerator struct {
	pkg     string
	version string
	info    tfbridge.ProviderInfo
	outDir  string
}

// newSchemaGenerator returns a language generator that understands how to produce Pulumi schemas.
func newSchemaGenerator(pkg, version string, info tfbridge.ProviderInfo, outDir string) langGenerator {
	return &schemaGenerator{
		pkg:     pkg,
		version: version,
		info:    info,
		outDir:  outDir,
	}
}

func (g *schemaGenerator) emitPackage(pack *pkg) error {
	spec, err := g.genPackageSpec(pack)
	if err != nil {
		return errors.Wrap(err, "generating Pulumi schema")
	}

	packageSpec.Version = ""
	schema, err := json.MarshalIndent(spec, "", "    ")
	if err != nil {
		return errors.Wrap(err, "marshaling Pulumi schema")
	}

	if err := emitFile(g.outDir, "schema.json", schema); err != nil {
		return errors.Wrap(err, "emitting schema.json")
	}
	return nil
}

func (g *schemaGenerator) typeName(r *resourceType) string {
	return r.name
}

type schemaNestedType struct {
	typ       *propertyType
	pyMapCase bool
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
		nt.gatherFromProperties(member, member.name, "", member.inprops, true)
		nt.gatherFromProperties(member, member.name, "", member.outprops, true)
		if !member.IsProvider() {
			nt.gatherFromProperties(member, member.name, "", member.statet.properties, true)
		}
	case *resourceFunc:
		nt.gatherFromProperties(member, member.name, "", member.args, true)
		nt.gatherFromProperties(member, member.name, "", member.rets, true)
	case *variable:
		nt.gatherFromPropertyType(member, member.name, "", "", member.typ, true)
	}
}

type declarer interface {
	Name() string
}

func (nt *schemaNestedTypes) declareType(
	declarer declarer, namePrefix, name, nameSuffix string, typ *propertyType, pyMapCase bool) string {

	// Generate a name for this nested type.
	baseName := namePrefix + strings.Title(name)

	// Override the nested type name, if necessary.
	if typ.nestedType.Name().String() != "" {
		baseName = typ.nestedType.Name().String()
	}

	typeName := baseName + strings.Title(nameSuffix)
	typ.name = typeName

	nt.nameToType[typeName] = &schemaNestedType{typ: typ, pyMapCase: pyMapCase}
	return baseName
}

func (nt *schemaNestedTypes) gatherFromProperties(declarer declarer, namePrefix, nameSuffix string, ps []*variable,
	pyMapCase bool) {

	for _, p := range ps {
		name := p.name
		if p.typ.kind == kindList || p.typ.kind == kindSet {
			name = inflector.Singularize(name)
		}

		// Due to bugs in earlier versions of the bridge, we want to keep the Python code generator from case-mapping
		// properties an object-typed element that are not Map types. This is consistent with the earlier behavior. See
		// https://github.com/pulumi/pulumi/issues/3151 for more details.
		mapCase := pyMapCase && p.typ.kind == kindObject && p.schema.Type == schema.TypeMap
		nt.gatherFromPropertyType(declarer, namePrefix, name, nameSuffix, p.typ, mapCase)
	}
}

func (nt *schemaNestedTypes) gatherFromPropertyType(
	declarer declarer, namePrefix, name, nameSuffix string, typ *propertyType, pyMapCase bool) {

	switch typ.kind {
	case kindList, kindSet, kindMap:
		if typ.element != nil {
			nt.gatherFromPropertyType(declarer, namePrefix, name, nameSuffix, typ.element, pyMapCase)
		}
	case kindObject:
		baseName := nt.declareType(declarer, namePrefix, name, nameSuffix, typ, pyMapCase)
		nt.gatherFromProperties(declarer, baseName, nameSuffix, typ.properties, pyMapCase)
	}
}

func rawMessage(v interface{}) json.RawMessage {
	bytes, err := json.Marshal(v)
	contract.Assert(err == nil)
	return json.RawMessage(bytes)
}

func genPulumiSchema(pack *pkg, name, version string, info tfbridge.ProviderInfo) (*pschema.Package, error) {
	g := &schemaGenerator{
		pkg:     name,
		version: version,
		info:    info,
	}
	spec, err := g.genPackageSpec(pack)
	if err != nil {
		return nil, err
	}
	return pschema.ImportSpec(spec, nil)
}

func (g *schemaGenerator) genPackageSpec(pack *pkg) (pschema.PackageSpec, error) {
	spec := pschema.PackageSpec{
		Name:       g.pkg,
		Version:    g.version,
		Keywords:   g.info.Keywords,
		Homepage:   g.info.Homepage,
		License:    g.info.License,
		Repository: g.info.Repository,
		Resources:  map[string]pschema.ResourceSpec{},
		Functions:  map[string]pschema.FunctionSpec{},
		Types:      map[string]pschema.ObjectTypeSpec{},
		Language:   map[string]json.RawMessage{},

		Meta: &pschema.MetadataSpec{
			ModuleFormat: "(.*)(?:/[^/]*)",
		},
	}

	spec.Description = g.info.Description
	spec.Attribution = fmt.Sprintf(attributionFormatString, g.info.Name, g.info.GetGitHubOrg())

	var config []*variable
	for _, mod := range pack.modules.values() {
		// Generate nested types.
		for _, t := range gatherSchemaNestedTypesForModule(mod) {
			tok, ts := g.genObjectType(mod.name, t)
			spec.Types[tok] = ts
		}

		// Enumerate each module member, in the order presented to us, and do the right thing.
		for _, member := range mod.members {
			switch t := member.(type) {
			case *resourceType:
				spec.Resources[string(t.info.Tok)] = g.genResourceType(mod.name, t)
			case *resourceFunc:
				spec.Functions[string(t.info.Tok)] = g.genDatasourceFunc(mod.name, t)
			case *variable:
				contract.Assert(mod.config())
				config = append(config, t)
			}
		}
	}

	if len(config) != 0 {
		spec.Config = g.genConfig(config)
	}

	if pack.provider != nil {
		for _, t := range gatherSchemaNestedTypesForMember(pack.provider) {
			tok, ts := g.genObjectType("index", t)
			spec.Types[tok] = ts
		}
		spec.Provider = g.genResourceType("index", pack.provider)
	}

	for token, typ := range g.info.ExtraTypes {
		if _, defined := spec.Types[token]; defined {
			return pschema.PackageSpec{}, fmt.Errorf("failed to define extra types: %v is already defined", token)
		}
		spec.Types[token] = typ
	}

	if jsi := g.info.JavaScript; jsi != nil {
		spec.Language["nodejs"] = rawMessage(map[string]interface{}{
			"packageName":        jsi.PackageName,
			"packageDescription": generateManifestDescription(g.info),
			"dependencies":       jsi.Dependencies,
			"devDependencies":    jsi.DevDependencies,
			"typescriptVersion":  jsi.TypeScriptVersion,
		})
	}

	if pi := g.info.Python; pi != nil {
		spec.Language["python"] = rawMessage(map[string]interface{}{
			"requires": pi.Requires,
		})
	}

	if csi := g.info.CSharp; csi != nil {
		spec.Language["csharp"] = rawMessage(map[string]interface{}{
			"packageReferences": csi.PackageReferences,
			"namespaces":        csi.Namespaces,
		})
	}

	return spec, nil
}

func (g *schemaGenerator) genDocComment(comment, docURL string) string {
	if comment == elidedDocComment && docURL == "" {
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

func (g *schemaGenerator) genProperty(mod string, prop *variable, pyMapCase bool) pschema.PropertySpec {
	description := ""
	if prop.doc != "" && prop.doc != elidedDocComment {
		description = g.genDocComment(prop.doc, prop.docURL)
	} else if prop.rawdoc != "" {
		description = g.genRawDocComment(prop.rawdoc)
	}

	language := map[string]json.RawMessage{}
	if prop.info != nil && prop.info.CSharpName != "" {
		language["csharp"] = rawMessage(map[string]string{"name": prop.info.CSharpName})
	}

	if !pyMapCase {
		language["python"] = rawMessage(map[string]interface{}{"mapCase": false})
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

	return pschema.PropertySpec{
		TypeSpec:           g.schemaType(mod, prop.typ, prop.out),
		Description:        description,
		Default:            defaultValue,
		DefaultInfo:        defaultInfo,
		DeprecationMessage: prop.deprecationMessage(),
		Language:           language,
	}
}

func (g *schemaGenerator) genConfig(variables []*variable) pschema.ConfigSpec {
	spec := pschema.ConfigSpec{
		Variables: make(map[string]pschema.PropertySpec),
	}
	for _, v := range variables {
		spec.Variables[v.name] = g.genProperty("config", v, true)

		if !v.optional() {
			spec.Required = append(spec.Required, v.name)
		}
	}
	return spec
}

func (g *schemaGenerator) genResourceType(mod string, res *resourceType) pschema.ResourceSpec {
	var spec pschema.ResourceSpec

	description := ""
	if res.doc != "" {
		description = g.genDocComment(res.doc, res.docURL)
	}
	if !res.IsProvider() {
		if res.info.DeprecationMessage != "" {
			spec.DeprecationMessage = res.info.DeprecationMessage
		}
	}
	spec.Description = description

	spec.Properties = map[string]pschema.PropertySpec{}
	for _, prop := range res.outprops {
		spec.Properties[prop.name] = g.genProperty(mod, prop, true)

		if !prop.optional() {
			spec.Required = append(spec.Required, prop.name)
		}
	}

	spec.InputProperties = map[string]pschema.PropertySpec{}
	for _, prop := range res.inprops {
		spec.InputProperties[prop.name] = g.genProperty(mod, prop, true)

		if !prop.optional() {
			spec.RequiredInputs = append(spec.RequiredInputs, prop.name)
		}
	}

	if !res.IsProvider() {
		_, stateInputs := g.genObjectType(mod, &schemaNestedType{typ: res.statet, pyMapCase: true})
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

func (g *schemaGenerator) genDatasourceFunc(mod string, fun *resourceFunc) pschema.FunctionSpec {
	var spec pschema.FunctionSpec

	description := ""
	if fun.doc != "" {
		description = g.genDocComment(fun.doc, fun.docURL)
	}
	if fun.info.DeprecationMessage != "" {
		spec.DeprecationMessage = fun.info.DeprecationMessage
	}
	spec.Description = description

	// If there are argument and/or return types, emit them.
	if fun.argst != nil {
		_, t := g.genObjectType(mod, &schemaNestedType{typ: fun.argst, pyMapCase: true})
		spec.Inputs = &t
	}
	if fun.retst != nil {
		_, t := g.genObjectType(mod, &schemaNestedType{typ: fun.retst, pyMapCase: true})
		spec.Outputs = &t
	}

	return spec
}

func (g *schemaGenerator) genObjectType(mod string, typInfo *schemaNestedType) (string, pschema.ObjectTypeSpec) {
	typ := typInfo.typ
	contract.Assert(typ.kind == kindObject)

	name := typ.name
	if typ.nestedType != "" {
		name = string(typ.nestedType)
	}
	token := fmt.Sprintf("%s:%s/%s:%s", g.pkg, mod, name, name)

	spec := pschema.ObjectTypeSpec{
		Type: "object",
	}

	if typ.doc != "" {
		spec.Description = g.genDocComment(typ.doc, "")
	}

	spec.Properties = map[string]pschema.PropertySpec{}
	for _, prop := range typ.properties {
		spec.Properties[prop.name] = g.genProperty(mod, prop, typInfo.pyMapCase)

		if !prop.optional() {
			spec.Required = append(spec.Required, prop.name)
		}
	}

	return token, spec
}

func (g *schemaGenerator) schemaType(mod string, typ *propertyType, out bool) pschema.TypeSpec {
	// Prefer overrides over the underlying type.
	switch {
	case typ == nil:
		return pschema.TypeSpec{Ref: "pulumi.json#/Any"}
	case typ.typ != "" || len(typ.altTypes) != 0:
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
				pkg := string(t.Module().Package().Name())
				if pkg == g.pkg {
					pkg = ""
				}
				spec := pschema.TypeSpec{Ref: fmt.Sprintf("%s#/types/%s", pkg, strings.TrimSuffix(string(t), "[]"))}
				switch typ.kind {
				case kindBool:
					spec.Type = "boolean"
				case kindInt:
					spec.Type = "integer"
				case kindFloat:
					spec.Type = "number"
				case kindString:
					spec.Type = "string"
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
		return pschema.TypeSpec{OneOf: typs}
	case typ.asset != nil:
		if typ.asset.IsArchive() {
			return pschema.TypeSpec{Ref: "pulumi.json#/Archive"}
		}
		return pschema.TypeSpec{Ref: "pulumi.json#/Asset"}
	}

	// First figure out the raw type.
	switch typ.kind {
	case kindBool:
		return pschema.TypeSpec{Type: "boolean"}
	case kindInt:
		return pschema.TypeSpec{Type: "integer"}
	case kindFloat:
		return pschema.TypeSpec{Type: "number"}
	case kindString:
		return pschema.TypeSpec{Type: "string"}
	case kindSet, kindList:
		items := g.schemaType(mod, typ.element, out)
		return pschema.TypeSpec{Type: "array", Items: &items}
	case kindMap:
		additionalProperties := g.schemaType(mod, typ.element, out)
		return pschema.TypeSpec{Type: "object", AdditionalProperties: &additionalProperties}
	case kindObject:
		return pschema.TypeSpec{Ref: fmt.Sprintf("#/types/%s:%s/%s:%s", g.pkg, mod, typ.name, typ.name)}
	default:
		contract.Failf("Unrecognized type kind: %v", typ.kind)
		return pschema.TypeSpec{}
	}
}
