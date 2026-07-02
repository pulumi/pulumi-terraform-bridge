// Copyright 2016-2026, Pulumi Corporation.
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
	"fmt"
	"os"
	"sort"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/functions"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// providerFunc is a Terraform provider-defined function that is exposed as a Pulumi
// invoke with positional arguments (multiArgumentInputs) and a direct return type.
type providerFunc struct {
	mod                tokens.Module
	name               string
	tok                tokens.ModuleMember
	tfName             string
	fn                 shim.Function
	info               *tfbridge.FunctionInfo
	doc                string
	deprecationMessage string
}

func (pf *providerFunc) Name() string { return pf.name }
func (pf *providerFunc) Doc() string  { return pf.doc }

// gatherFunctions returns all modules and the provider-defined functions they contain.
func (g *Generator) gatherFunctions() (moduleMap, error) {
	functions := g.provider().Functions()
	if len(functions) == 0 {
		return nil, nil
	}
	modules := make(moduleMap)

	skipFailBuildOnMissingMapError := cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_MISSING_MAPPING_ERROR")) ||
		cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_PROVIDER_MAP_ERROR"))
	skipFailBuildOnExtraMapError := cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_EXTRA_MAPPING_ERROR"))

	var functionMappingErrors error

	names := make([]string, 0, len(functions))
	for name := range functions {
		names = append(names, name)
	}
	sort.Strings(names)

	var fnerr error
	seen := make(map[string]bool)
	for _, name := range names {
		fninfo := g.info.Functions[name]
		if fninfo == nil {
			if sliceContains(g.info.IgnoreMappings, name) {
				g.debug("TF function %q not found in provider map but ignored", name)
				continue
			}

			if !skipFailBuildOnMissingMapError {
				functionMappingErrors = multierror.Append(functionMappingErrors,
					fmt.Errorf("TF function %q not mapped to the Pulumi provider", name))
			} else {
				g.warn("TF function %q not found in provider map", name)
			}
			continue
		}
		seen[name] = true

		fun, err := g.gatherFunction(name, functions[name], fninfo)
		if err != nil {
			// Keep track of the error, but keep going, so we can expose more at once.
			fnerr = multierror.Append(fnerr, err)
		} else {
			modules.ensureModule(fun.mod).addMember(fun)
		}
	}
	if fnerr != nil {
		return nil, fnerr
	}

	// Emit an error if there is a map but some names didn't match.
	var mapped []string
	for name := range g.info.Functions {
		mapped = append(mapped, name)
	}
	sort.Strings(mapped)
	for _, name := range mapped {
		if !seen[name] {
			if !skipFailBuildOnExtraMapError {
				functionMappingErrors = multierror.Append(functionMappingErrors,
					fmt.Errorf("Pulumi token %q is mapped to TF provider function %q, but no such "+
						"function found. Remove the mapping and try again",
						g.info.Functions[name].Tok, name))
			} else {
				g.warn("Pulumi token %q is mapped to TF provider function %q, but no such "+
					"function found. The mapping will be ignored in the generated provider",
					g.info.Functions[name].Tok, name)
			}
		}
	}
	if functionMappingErrors != nil {
		return nil, functionMappingErrors
	}

	if err := g.checkFunctionTokenCollisions(); err != nil {
		return nil, err
	}

	return modules, nil
}

// checkFunctionTokenCollisions rejects provider functions whose Pulumi token collides
// with a data source invoke token. Both share the "functions" section of the Pulumi
// schema, while in Terraform they live in separate namespaces; a collision would let
// one silently shadow the other.
func (g *Generator) checkFunctionTokenCollisions() error {
	dataSourceToks := make(map[string]string, len(g.info.DataSources))
	for tfName, ds := range g.info.DataSources {
		dataSourceToks[string(ds.Tok)] = tfName
	}
	var errs multierror.Error
	names := make([]string, 0, len(g.info.Functions))
	for name := range g.info.Functions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		tok := string(g.info.Functions[name].Tok)
		if dsName, ok := dataSourceToks[tok]; ok {
			errs.Errors = append(errs.Errors, fmt.Errorf(
				"TF function %q and TF data source %q are both mapped to the Pulumi token %q; "+
					"override one of the mappings to avoid the collision", name, dsName, tok))
		}
	}
	return errs.ErrorOrNil()
}

// gatherFunction returns the module member for a single provider-defined function.
func (g *Generator) gatherFunction(rawname string,
	fn shim.Function, info *tfbridge.FunctionInfo,
) (*providerFunc, error) {
	name, moduleName := functionName(rawname, info)
	mod := tokens.NewModuleToken(g.pkg, moduleName)
	tok := tokens.NewModuleMemberToken(mod, name)

	// Collect documentation information for this function.
	var entityDocs entityDocs
	if !g.noDocsRepo {
		source := NewGitRepoDocsSource(g)
		docs, err := getDocsForResource(g, source, FunctionDocs, rawname, info)
		if err == nil {
			entityDocs = docs
		} else if !g.checkNoDocsError(err) {
			return nil, err
		}
	}

	doc := entityDocs.Description
	if doc == "" {
		// Fall back to the documentation the provider ships in its schema.
		doc = fn.Description
		if fn.Summary != "" {
			if doc == "" {
				doc = fn.Summary
			} else {
				doc = fn.Summary + "\n\n" + doc
			}
		}
	}

	deprecationMessage := info.DeprecationMessage
	if deprecationMessage == "" {
		deprecationMessage = fn.DeprecationMessage
	}

	return &providerFunc{
		mod:                mod,
		name:               name.String(),
		tok:                tok,
		tfName:             rawname,
		fn:                 fn,
		info:               info,
		doc:                doc,
		deprecationMessage: deprecationMessage,
	}, nil
}

// functionName translates a Terraform function name into its Pulumi name equivalent,
// plus a module name.
func functionName(tfName string, info *tfbridge.FunctionInfo) (tokens.ModuleMemberName, tokens.ModuleName) {
	if info == nil || info.Tok == "" {
		// Terraform function names are unprefixed; functions map into the top-level
		// module by default.
		name := tfbridge.TerraformToPulumiNameV2(tfName, nil, nil)
		return tokens.ModuleMemberName(name), tokens.ModuleName(string(indexMod) + "/" + name)
	}
	return info.Tok.Name(), info.Tok.Module().Name()
}

// genProviderFunc generates the Pulumi schema for a provider-defined function.
//
// Terraform positional parameters project via multiArgumentInputs and the return type
// projects via returnType, possibly as a direct non-object return. A trailing variadic
// parameter projects to a final list-typed argument, since the Pulumi schema has no
// variadic concept. Auxiliary named object types are added to types.
func (g *schemaGenerator) genProviderFunc(
	fun *providerFunc, types map[string]pschema.ComplexTypeSpec,
) (pschema.FunctionSpec, error) {
	spec := pschema.FunctionSpec{
		DeprecationMessage: fun.deprecationMessage,
	}
	if fun.doc != "" {
		spec.Description = g.genDocComment(fun.doc)
	}

	conv := &functionTypeGen{pkg: g.pkg, tfName: fun.tfName, types: types}
	fn := fun.fn
	argNames := functions.ArgumentNames(fn)

	if len(argNames) > 0 {
		inputs := &pschema.ObjectTypeSpec{
			Type:       "object",
			Properties: map[string]pschema.PropertySpec{},
		}
		multiArgs := make([]string, 0, len(argNames))

		// A parameter that allows null is only projected as optional if every later
		// parameter is optional too: target languages cannot express a required
		// positional argument after an optional one.
		required := make([]bool, len(fn.Parameters))
		anyRequiredAfter := false
		for i := len(fn.Parameters) - 1; i >= 0; i-- {
			required[i] = !fn.Parameters[i].AllowNullValue || anyRequiredAfter
			if required[i] {
				anyRequiredAfter = true
			}
		}

		for i, p := range fn.Parameters {
			name := argNames[i]
			ts, err := conv.typeSpec(p.Type, fun.name+upperFirst(name))
			if err != nil {
				return pschema.FunctionSpec{}, fmt.Errorf("function %q: parameter %q: %w", fun.tfName, name, err)
			}
			prop := pschema.PropertySpec{TypeSpec: ts}
			if p.Description != "" {
				prop.Description = g.genDocComment(p.Description)
			}
			inputs.Properties[name] = prop
			multiArgs = append(multiArgs, name)
			if required[i] {
				inputs.Required = append(inputs.Required, name)
			}
		}

		if v := fn.VariadicParameter; v != nil {
			name := argNames[len(fn.Parameters)]
			elem, err := conv.typeSpec(v.Type, fun.name+upperFirst(name))
			if err != nil {
				return pschema.FunctionSpec{}, fmt.Errorf("function %q: variadic parameter %q: %w",
					fun.tfName, name, err)
			}
			prop := pschema.PropertySpec{TypeSpec: pschema.TypeSpec{Type: "array", Items: &elem}}
			if v.Description != "" {
				prop.Description = g.genDocComment(v.Description)
			}
			// A variadic parameter accepts zero or more arguments, so it is never
			// required.
			inputs.Properties[name] = prop
			multiArgs = append(multiArgs, name)
		}

		sort.Strings(inputs.Required)
		spec.Inputs = inputs
		spec.MultiArgumentInputs = multiArgs
	}

	if fn.Return == nil {
		return pschema.FunctionSpec{}, fmt.Errorf("function %q: missing return type", fun.tfName)
	}
	if obj, ok := fn.Return.(tftypes.Object); ok {
		ret, err := conv.objectSpec(obj, fun.name+"Result")
		if err != nil {
			return pschema.FunctionSpec{}, fmt.Errorf("function %q: return: %w", fun.tfName, err)
		}
		spec.ReturnType = &pschema.ReturnTypeSpec{ObjectTypeSpec: &ret}
	} else {
		ts, err := conv.typeSpec(fn.Return, fun.name+"Result")
		if err != nil {
			return pschema.FunctionSpec{}, fmt.Errorf("function %q: return: %w", fun.tfName, err)
		}
		spec.ReturnType = &pschema.ReturnTypeSpec{TypeSpec: &ts}
	}

	return spec, nil
}

// functionTypeGen converts the tftypes types describing a provider-defined function
// signature into Pulumi schema types. Anonymous Terraform object types become named
// auxiliary types, added to the types sink under a deterministic name derived from the
// function and parameter names.
type functionTypeGen struct {
	pkg    tokens.Package
	tfName string
	types  map[string]pschema.ComplexTypeSpec
}

func (c *functionTypeGen) typeSpec(t tftypes.Type, nameHint string) (pschema.TypeSpec, error) {
	switch t := t.(type) {
	case tftypes.Object:
		obj, err := c.objectSpec(t, nameHint)
		if err != nil {
			return pschema.TypeSpec{}, err
		}
		tok := fmt.Sprintf("%s:%s/%s:%s", c.pkg, indexMod, nameHint, nameHint)
		if existing, ok := c.types[tok]; ok {
			return pschema.TypeSpec{}, fmt.Errorf(
				"generated type token %q is already defined as %v", tok, existing)
		}
		c.types[tok] = pschema.ComplexTypeSpec{ObjectTypeSpec: obj}
		return pschema.TypeSpec{Ref: "#/types/" + tok}, nil
	case tftypes.List:
		items, err := c.typeSpec(t.ElementType, nameHint)
		if err != nil {
			return pschema.TypeSpec{}, err
		}
		return pschema.TypeSpec{Type: "array", Items: &items}, nil
	case tftypes.Set:
		items, err := c.typeSpec(t.ElementType, nameHint)
		if err != nil {
			return pschema.TypeSpec{}, err
		}
		return pschema.TypeSpec{Type: "array", Items: &items}, nil
	case tftypes.Map:
		additional, err := c.typeSpec(t.ElementType, nameHint)
		if err != nil {
			return pschema.TypeSpec{}, err
		}
		return pschema.TypeSpec{Type: "object", AdditionalProperties: &additional}, nil
	case tftypes.Tuple:
		// No provider function in the wild uses tuples, and Pulumi schema has no
		// equivalent type.
		return pschema.TypeSpec{}, fmt.Errorf("tuple types are not supported")
	}
	switch {
	case t.Is(tftypes.DynamicPseudoType):
		return pschema.TypeSpec{Ref: "pulumi.json#/Any"}, nil
	case t.Is(tftypes.String):
		return pschema.TypeSpec{Type: "string"}, nil
	case t.Is(tftypes.Bool):
		return pschema.TypeSpec{Type: "boolean"}, nil
	case t.Is(tftypes.Number):
		return pschema.TypeSpec{Type: "number"}, nil
	}
	return pschema.TypeSpec{}, fmt.Errorf("unsupported type %v", t)
}

func (c *functionTypeGen) objectSpec(t tftypes.Object, nameHint string) (pschema.ObjectTypeSpec, error) {
	spec := pschema.ObjectTypeSpec{
		Type:       "object",
		Properties: map[string]pschema.PropertySpec{},
	}
	for _, attr := range functions.SortedAttributeNames(t) {
		name := functions.PropertyName(attr)
		if _, conflict := spec.Properties[name]; conflict {
			return pschema.ObjectTypeSpec{}, fmt.Errorf(
				"attribute %q: property name %q is already taken by another attribute", attr, name)
		}
		ts, err := c.typeSpec(t.AttributeTypes[attr], nameHint+upperFirst(name))
		if err != nil {
			return pschema.ObjectTypeSpec{}, fmt.Errorf("attribute %q: %w", attr, err)
		}
		spec.Properties[name] = pschema.PropertySpec{TypeSpec: ts}
		if _, optional := t.OptionalAttributes[attr]; !optional {
			spec.Required = append(spec.Required, name)
		}
	}
	return spec, nil
}
