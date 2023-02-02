// Copyright 2016-2023, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/internal/paths"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type nameCheckPropSet struct {
	location  string
	props     []paths.PropertyName
	schemaMap shim.SchemaMap
	infos     map[string]*tfbridge.SchemaInfo
}

// Checks a generated schema to detect property name collisions and verify automatic naming.
//
// Bridged providers need to perform property name translation between Pulumi and Terraform names. Historically there
// were two classes of issues with naming, both leading to confusing runtime errors:
//
//   - problems from customized ProviderInfo overrides, such as custom SchemaInfo.Name that picks a Pulumi property name
//     that conflicts with another property
//
//   - automatic naming bugs in tfbridge code such as https://github.com/pulumi/pulumi-terraform-bridge/issues/778
//
// Best-effort attempt is made here to detect both classes of issues early (at schema generation aka provider build
// time), decide which one it is.
func nameCheck(
	prov tfbridge.ProviderInfo,
	spec pschema.PackageSpec,
	renamesBuilder *renamesBuilder,
	sink diag.Sink,
) error {
	if sink == nil {
		sink = diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
			Color: colors.Always,
		})
	}

	renames, err := renamesBuilder.BuildRenames()
	if err != nil {
		return err
	}

	props, err := renamesBuilder.BuildProperties()
	if err != nil {
		return err
	}

	all := []nameCheckPropSet{}

	if len(spec.Config.Variables) > 0 {
		configProps := renamesBuilder.BuildConfigProperties()
		ps := nameCheckPropSet{
			location:  "Config",
			props:     configProps,
			schemaMap: prov.P.Schema(),
			infos:     prov.Config,
		}
		all = append(all, ps)
	}

	for tok := range spec.Resources {
		key := renames.Resources[tokens.Type(tok)]
		loc := fmt.Sprintf("Resource: %q\n    Token: %q", tok, key)
		ps := nameCheckPropSet{
			location:  loc,
			props:     props[tokens.Token(tok)],
			schemaMap: prov.P.ResourcesMap().Get(key).Schema(),
			infos:     prov.Resources[key].Fields,
		}
		all = append(all, ps)
	}

	for tok := range spec.Functions {
		key := renames.Functions[tokens.ModuleMember(tok)]
		loc := fmt.Sprintf("Function: %q\n    Token: %q", key, tok)
		ps := nameCheckPropSet{
			location:  loc,
			props:     props[tokens.Token(tok)],
			schemaMap: prov.P.DataSourcesMap().Get(key).Schema(),
			infos:     prov.DataSources[key].Fields,
		}
		all = append(all, ps)
	}

	for tok := range spec.Types {
		typePaths := renamesBuilder.ObjectTypePaths(tokens.Type(tok))
		for _, typePath := range typePaths.Paths() {
			loc := fmt.Sprintf("Type: %q\n    TypePath: %s", tok, typePath.String())
			p, err := locateTypByTypePath(prov, typePath)
			if err != nil {
				return err
			}
			schemaMap, err := p.SchemaMap()
			if err != nil {
				return err
			}
			infos, err := p.SchemaInfos()
			if err != nil {
				return err
			}
			ps := nameCheckPropSet{
				location:  loc,
				props:     props[tokens.Token(tok)],
				schemaMap: schemaMap,
				infos:     infos,
			}
			all = append(all, ps)
		}
	}

	for _, props := range all {
		nameCheckUniquePulumiNames(sink, props)
		nameCheckUniqueTerraformNames(sink, props)
		nameCheckTerraformToPulumiName(sink, props)
		nameCheckPulumiToTerraformName(sink, props)
	}

	return nil
}

func nameCheckTerraformToPulumiName(sink diag.Sink, props nameCheckPropSet) {
	for _, p := range props.props {
		expected := p.Name
		actual := tfbridge.TerraformToPulumiNameV2(p.Key, props.schemaMap, props.infos)
		if actual == expected.String() {
			continue
		}
		m := "TerraformToPulumiNameV2(%q) unexpectedly returns %q, expecting %q from schema generation.\n" +
			"  Please report this as a bug to pulumi/pulumi-terraform-bridge.\n" +
			"  Location:\n    %v\n"
		sink.Warningf(&diag.Diag{Message: m}, p.Key, actual, expected, props.location)
	}
}

func nameCheckPulumiToTerraformName(sink diag.Sink, props nameCheckPropSet) {
	for _, p := range props.props {
		expected := p.Key
		actual := tfbridge.PulumiToTerraformName(p.Name.String(), props.schemaMap, props.infos)
		if actual == expected {
			continue
		}
		m := "PulumiToTerraformName(%q) unexpectedly returns %q, expecting %q from schema generation.\n" +
			"  Please report this as a bug to pulumi/pulumi-terraform-bridge.\n" +
			"  Location:\n    %v\n"
		sink.Warningf(&diag.Diag{Message: m}, p.Key, actual, expected, props.location)
	}
}

func nameCheckUniquePulumiNames(sink diag.Sink, props nameCheckPropSet) {
	index := map[tokens.Name][]string{}
	for _, p := range props.props {
		index[p.Name] = append(index[p.Name], p.Key)
	}
	for k, v := range index {
		if len(v) <= 1 {
			continue
		}
		m := "Pulumi property %q maps to multiple Terraform properties %v.\n" +
			"  This will cause runtime errors in the provider.\n" +
			"  Please set SchemaInfo.Name to rename the Terraform properties.\n" +
			"  Location:\n    %v\n"
		sink.Warningf(&diag.Diag{Message: m}, k, v, props.location)
	}
}

func nameCheckUniqueTerraformNames(sink diag.Sink, props nameCheckPropSet) {
	index := map[string][]tokens.Name{}
	for _, p := range props.props {
		index[p.Key] = append(index[p.Key], p.Name)
	}
	for k, v := range index {
		if len(v) <= 1 {
			continue
		}
		m := "Terraform property %q maps to multiple Pulumi properties %v.\n" +
			"  This will cause runtime errors in the provider.\n" +
			"  Please set SchemaInfo.Name to rename the Terraform properties.\n" +
			"  Location:\n    %v\n"
		sink.Warningf(&diag.Diag{Message: m}, k, v, props.location)
	}
}

// Internal helper family of types typ, objType, collectionType, scalarType assist nameCheck in the task of co-locating
// SchemaInfo and shim.Schema in a provider by drilling down a TypePath. This is currently quite bespoke because of
// convoluted encodings of types into shim.Schema (see documentation on shim.Schema.Elem), and wrinkles in TypePath
// being confused about flattening where MaxItem=1 collections from Terraform are translated to scalars in Pulumi.
type typ interface {
	Property(pn paths.PropertyName) (typ, error)
	Element() (typ, error)
	SchemaMap() (shim.SchemaMap, error)
	SchemaInfos() (map[string]*tfbridge.SchemaInfo, error)
}

type objType struct {
	fields shim.SchemaMap
	infos  map[string]*tfbridge.SchemaInfo
}

func (x *objType) SchemaInfos() (map[string]*tfbridge.SchemaInfo, error) {
	return x.infos, nil
}

func (x *objType) SchemaMap() (shim.SchemaMap, error) {
	return x.fields, nil
}

func (x *objType) Property(pn paths.PropertyName) (typ, error) {
	return newTyp(x.fields.Get(pn.Key), x.infos[pn.Key]), nil
}

func (x *objType) Element() (typ, error) {
	return nil, fmt.Errorf("Element undefined on an objType")
}

var _ typ = (*objType)(nil)

type collectionType struct {
	elem typ
}

func (x *collectionType) Property(pn paths.PropertyName) (typ, error) {
	if _, ok := x.elem.(*objType); ok {
		// Because of flattening MaxItems=1, sometimes this logic is off by one and the collection of objects is
		// asked to lookup a property, where in Pulumi projection it's no longer a collection but just an
		// object.
		return x.elem.Property(pn)
	}
	return nil, fmt.Errorf("Properties are undefined on an collectionType")
}

func (x *collectionType) SchemaInfos() (map[string]*tfbridge.SchemaInfo, error) {
	// Also working around MaxItems=1
	if _, ok := x.elem.(*objType); ok {
		return x.elem.SchemaInfos()
	}
	return nil, fmt.Errorf("SchemaInfos are undefined on an collectionType")
}

func (x *collectionType) SchemaMap() (shim.SchemaMap, error) {
	// Also working around MaxItems=1
	if _, ok := x.elem.(*objType); ok {
		return x.elem.SchemaMap()
	}
	return nil, fmt.Errorf("SchemaMap are undefined on an collectionType")
}

func (x *collectionType) Element() (typ, error) {
	return x.elem, nil
}

var _ typ = (*collectionType)(nil)

type scalarType struct{}

func (x *scalarType) Element() (typ, error) {
	return nil, fmt.Errorf("Element undefined on an scalarType")
}

func (x *scalarType) Property(pn paths.PropertyName) (typ, error) {
	return nil, fmt.Errorf("Properties are undefined on an scalarType")
}

func (x *scalarType) SchemaMap() (shim.SchemaMap, error) {
	return nil, fmt.Errorf("Fields are undefined on an scalarType")
}

func (x *scalarType) SchemaInfos() (map[string]*tfbridge.SchemaInfo, error) {
	return nil, fmt.Errorf("Fields are undefined on an scalarType")
}

var _ typ = (*scalarType)(nil)

func newTyp(s shim.Schema, info *tfbridge.SchemaInfo) typ {
	switch sElem := s.Elem().(type) {
	case shim.Resource:
		switch s.Type() {
		case shim.TypeMap:
			t := &objType{fields: sElem.Schema()}
			if info != nil {
				t.infos = info.Fields
			}
			return t
		case shim.TypeList, shim.TypeSet:
			t := &objType{fields: sElem.Schema()}
			if info != nil && info.Elem != nil {
				t.infos = info.Elem.Fields
			}
			return &collectionType{t}
		default:
			panic("impossible: shim.Schema s with s.Elem() of type shim.Resource must have s.Type()" +
				" be one of shim.TypeMap, shim.TypeSet, shim.TypeList. See Elem() doc comment.")
		}
	case shim.Schema:
		var elem *tfbridge.SchemaInfo
		if info != nil {
			elem = info.Elem
		}
		parsed := newTyp(sElem, elem)
		return &collectionType{parsed}
	default:
		return &scalarType{}
	}
}

func locateTypByTypePath(prov tfbridge.ProviderInfo, path paths.TypePath) (typ, error) {
	switch p := path.(type) {
	case *paths.DataSourceMemberPath:
		key := p.DataSourcePath.Key()
		return &objType{
			fields: prov.P.DataSourcesMap().Get(key).Schema(),
			infos:  prov.DataSources[key].Fields,
		}, nil
	case *paths.ResourceMemberPath:
		if p.ResourcePath.IsProvider() {
			return &objType{
				fields: prov.P.Schema(),
				infos:  prov.Config,
			}, nil
		}
		key := p.ResourcePath.Key()
		return &objType{
			fields: prov.P.ResourcesMap().Get(key).Schema(),
			infos:  prov.Resources[key].Fields,
		}, nil
	case *paths.ConfigPath:
		return &objType{
			fields: prov.P.Schema(),
			infos:  prov.Config,
		}, nil
	case *paths.PropertyPath:
		t, err := locateTypByTypePath(prov, p.Parent())
		if err != nil {
			return nil, err
		}
		return t.Property(p.PropertyName)
	case *paths.ElementPath:
		t, err := locateTypByTypePath(prov, p.Parent())
		if err != nil {
			return nil, err
		}
		return t.Element()
	default:
		panic("impossible match in locateTypByTypePath")
	}
}
