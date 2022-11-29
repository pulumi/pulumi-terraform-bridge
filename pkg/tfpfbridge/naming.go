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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/convert"
)

// Approximate implemenation of property renaming. Currently schemas reuse tfgen which calls PulumiToTerraformName, and
// among other things plurlizes names of list properties. This code accounts only for the pluralization for now. Ideally
// it should account for all forms of renaming.
type simplePropertyNames struct{}

var _ convert.PropertyNames = (*simplePropertyNames)(nil)

func (s *simplePropertyNames) PropertyKey(typeToken tokens.Token,
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
