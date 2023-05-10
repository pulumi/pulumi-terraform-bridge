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

package tfbridge

import (
	"github.com/gedex/inflector"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

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

func functionPropertyKey(ds datasourceHandle, path *tftypes.AttributePath) (resource.PropertyKey, bool) {
	if path == nil {
		return "", false
	}
	if len(path.Steps()) != 1 {
		return "", false
	}
	switch attrName := path.LastStep().(type) {
	case tftypes.AttributeName:
		pulumiName := tfbridge.TerraformToPulumiNameV2(string(attrName),
			ds.schemaOnlyShim.Schema(), ds.pulumiDataSourceInfo.GetFields())
		return resource.PropertyKey(pulumiName), true
	default:
		return "", false
	}
}
