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

package schemashim

import (
	"context"
	"fmt"

	pfattr "github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func convertType(typ pfattr.Type) (shim.ValueType, error) {
	// typ.TerraformType is 1:1 to the TF wire format, and is thus closed to outsider
	// implementation. This allows us to maintain an exhaustive list of possible types and
	// values.
	tftype := typ.TerraformType(context.Background())
	is := func(typ tftypes.Type) bool { return typ.Equal(tftype) }
	switch {
	case is(tftypes.Bool):
		return shim.TypeBool, nil
	case typ.Equal(types.Int64Type):
		// We special case int, since it is a stable type but not present on the wire.
		return shim.TypeInt, nil
	case is(tftypes.Number):
		return shim.TypeFloat, nil
	case is(tftypes.String):
		return shim.TypeString, nil
	case is(tftypes.DynamicPseudoType):
		// This means that any type can be used,
		return shim.TypeInvalid, nil
	default:
		switch tftype.(type) {
		case tftypes.List:
			return shim.TypeList, nil
		case tftypes.Map, tftypes.Object, tftypes.Tuple:
			return shim.TypeMap, nil
		default:
			return shim.TypeInvalid, fmt.Errorf("[pf/tfbridge] Failed to translate type %v (%[1]T) to Pulumi", typ)
		}
	}
}
