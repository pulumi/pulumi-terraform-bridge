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
	"fmt"

	pfattr "github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func convertType(typ pfattr.Type) (shim.ValueType, error) {
	switch {
	case typ.Equal(types.BoolType):
		return shim.TypeBool, nil
	case typ.Equal(types.Int64Type):
		return shim.TypeInt, nil
	case typ.Equal(types.Float64Type):
		return shim.TypeFloat, nil
	case typ.Equal(types.NumberType):
		return shim.TypeFloat, nil
	case typ.Equal(types.StringType):
		return shim.TypeString, nil
	default:
		switch typ.(type) {
		case types.ListType:
			return shim.TypeList, nil
		case types.MapType:
			return shim.TypeMap, nil
		case types.ObjectType:
			return shim.TypeMap, nil
		default:
			return shim.TypeInvalid, fmt.Errorf("[pf/tfbridge] Failed to translate type %v to Pulumi", typ)
		}
	}
}
