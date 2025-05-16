// Copyright 2016-2025, Pulumi Corporation.
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

package valueshim

import (
	"encoding/json"

	"github.com/hashicorp/go-cty/cty"
	ctyjson "github.com/hashicorp/go-cty/cty/json"
	ctymsgpack "github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func toCtyType(t tftypes.Type) (cty.Type, error) {
	typeBytes, err := json.Marshal(t)
	if err != nil {
		return cty.NilType, err
	}
	return ctyjson.UnmarshalType(typeBytes)
}

func toCtyValue(schemaType tftypes.Type, schemaCtyType cty.Type, value tftypes.Value) (cty.Value, error) {
	dv, err := tfprotov6.NewDynamicValue(schemaType, value)
	if err != nil {
		return cty.NilVal, err
	}
	return ctymsgpack.Unmarshal(dv.MsgPack, schemaCtyType)
}
