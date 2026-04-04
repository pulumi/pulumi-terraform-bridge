// Copyright 2016-2024, Pulumi Corporation.
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

package naming

import (
	"unicode"

	"github.com/pulumi/inflector"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

// TerraformToPulumiNameV2 performs a standard transformation on the given name
// string, from Terraform's underscore_casing to Pulumi's camelCasing.
func TerraformToPulumiNameV2(name string, sch shim.SchemaMap, ps map[string]*info.Schema) string {
	return terraformToPulumiName(name, sch, ps, false)
}

// TerraformToPulumiName performs a standard transformation on the given name
// string, from Terraform's underscore_casing to Pulumi's PascalCasing (if upper
// is true) or camelCasing (if upper is false).
func TerraformToPulumiName(name string, sch shim.Schema, ps *info.Schema, upper bool) string {
	return terraformToPulumiName(name,
		shimschema.SchemaMap(map[string]shim.Schema{name: sch}),
		map[string]*info.Schema{name: ps},
		upper)
}

func terraformToPulumiName(name string, sch shim.SchemaMap, ps map[string]*info.Schema, upper bool) string {
	var result string
	var nextCap bool
	var prev rune

	var psInfo *info.Schema
	if ps != nil {
		psInfo = ps[name]
	}

	if psInfo != nil && psInfo.Name != "" {
		return psInfo.Name
	}

	tryPluralize := func() bool {
		tfs := sch.Get(name)
		if tfs == nil {
			return false
		}
		switch tfs.Type() {
		case shim.TypeSet, shim.TypeList:
			if psInfo != nil && psInfo.MaxItemsOne != nil {
				return !*psInfo.MaxItemsOne
			}
			return tfs.MaxItems() != 1
		default:
			return false
		}
	}

	if sch != nil && tryPluralize() {
		candidate := inflector.Pluralize(name)
		_, conflict := sch.GetOk(candidate)
		conflictSafe := ps[candidate] != nil &&
			ps[candidate].Name != "" &&
			ps[candidate].Name != candidate
		if !conflict || conflictSafe {
			name = candidate
		}
	}

	casingActivated := false
	for i, c := range name {
		if c == '_' && casingActivated {
			nextCap = true
			prev = c
			continue
		}

		if c != '_' && !casingActivated {
			casingActivated = true
		}

		if ((i == 0 && upper) || nextCap) && c >= 'a' && c <= 'z' {
			result += string(unicode.ToUpper(c))
		} else {
			result += string(c)
		}
		nextCap = false
		prev = c
	}

	if prev == '_' {
		result += "_"
	}

	return result
}
