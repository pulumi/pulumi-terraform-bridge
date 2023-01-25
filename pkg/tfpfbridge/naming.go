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

package tfpfbridge

import (
	"github.com/gedex/inflector"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

type precisePropertyNames struct {
	renames       map[tokens.Token]map[convert.TerraformPropertyName]resource.PropertyKey
	configRenames map[convert.TerraformPropertyName]resource.PropertyKey
}

func newPrecisePropertyNames(renames tfgen.Renames) *precisePropertyNames {
	renamesTable := map[tokens.Token]map[convert.TerraformPropertyName]resource.PropertyKey{}
	// Invert renames.RenamedProperties maps for faster dynamic lookup.
	for typ, typRenames := range renames.RenamedProperties {
		m := map[convert.TerraformPropertyName]resource.PropertyKey{}
		for k, v := range typRenames {
			m[v] = resource.PropertyKey(string(k))
		}
		renamesTable[typ] = m
	}
	configRenames := map[convert.TerraformPropertyName]resource.PropertyKey{}
	for k, v := range renames.RenamedConfigProperties {
		configRenames[v] = resource.PropertyKey(string(k))
	}
	return &precisePropertyNames{
		renames:       renamesTable,
		configRenames: configRenames,
	}
}

var _ convert.PropertyNames = (*precisePropertyNames)(nil)

func (s *precisePropertyNames) PropertyKey(typeToken tokens.Token,
	property convert.TerraformPropertyName, _ tftypes.Type) resource.PropertyKey {
	if renamedProps, ok := s.renames[typeToken]; ok {
		if propertyKey, renamed := renamedProps[property]; renamed {
			return propertyKey
		}
	}
	return resource.PropertyKey(property)
}

func (s *precisePropertyNames) ConfigPropertyKey(property convert.TerraformPropertyName,
	_ tftypes.Type) resource.PropertyKey {
	if propertyKey, renamed := s.configRenames[property]; renamed {
		return propertyKey
	}
	return resource.PropertyKey(property)
}

// Approximate implemenation of property renaming. Currently schemas reuse tfgen which calls PulumiToTerraformName, and
// among other things plurlizes names of list properties. This code accounts only for the pluralization for now. Ideally
// it should account for all forms of renaming.
type simplePropertyNames struct{}

var _ convert.PropertyNames = (*simplePropertyNames)(nil)

func (s *simplePropertyNames) PropertyKey(typeToken tokens.Token,
	property convert.TerraformPropertyName, typ tftypes.Type) resource.PropertyKey {
	return toPropertyKey(property, typ)
}

func (s *simplePropertyNames) ConfigPropertyKey(
	property convert.TerraformPropertyName, typ tftypes.Type) resource.PropertyKey {
	return toPropertyKey(property, typ)
}

func toPropertyKey(name string, typ tftypes.Type) resource.PropertyKey {
	if pluralized, ok := pluralize(name, typ); ok {
		return resource.PropertyKey(pluralized)
	}
	return resource.PropertyKey(name)
}

func pluralize(name string, typ tftypes.Type) (string, bool) {
	if typ.Is(tftypes.List{}) {
		plu := inflector.Pluralize(name)
		distinct := plu != name
		valid := inflector.Singularize(plu) == name
		if valid && distinct {
			return plu, true
		}
	}
	return name, false
}

func functionPropertyKey(functionToken tokens.ModuleMember, propNames convert.PropertyNames,
	path *tftypes.AttributePath) (resource.PropertyKey, bool) {
	if path == nil {
		return "", false
	}
	if len(path.Steps()) != 1 {
		return "", false
	}
	switch attrName := path.LastStep().(type) {
	case tftypes.AttributeName:
		return propNames.PropertyKey(
			tokens.Token(functionToken),
			convert.TerraformPropertyName(attrName),
			nil), true
	default:
		return "", false
	}
}
