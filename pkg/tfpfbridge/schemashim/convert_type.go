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

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func convertType(ctx context.Context, typ tftypes.Type) (shim.ValueType, error) {
	switch {
	case typ.Is(tftypes.Bool):
		return shim.TypeBool, nil
	case typ.Is(tftypes.Number):
		return shim.TypeFloat, nil // TODO should this ever be TypeInt?
	case typ.Is(tftypes.String):
		return shim.TypeString, nil
	case typ.Is(tftypes.List{}):
		return shim.TypeList, nil
	case typ.Is(tftypes.Map{}):
		return shim.TypeMap, nil
	case typ.Is(tftypes.Object{}):
		return shim.TypeMap, nil
	default:
		return shim.TypeInvalid, fmt.Errorf("[tfpfbridge] Failed to translate type %v to Pulumi", typ)
	}
}
