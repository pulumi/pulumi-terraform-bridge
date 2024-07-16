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

package pfutils

import (
	"context"

	proto "github.com/golang/protobuf/proto"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/runtypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/opentofu/configs/configschema"
	opentofuconvert "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/opentofu/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/opentofu/plans/objchange"
	opentofuproto "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/opentofu/tfplugin6"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/terraform-plugin-go/tfprotov6/toproto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	ctymsgpack "github.com/zclconf/go-cty/cty/msgpack"
)

// Computes the ProposedNewState from priorState and config.
func ProposedNew(ctx context.Context, schema runtypes.Schema, priorState, config tftypes.Value) (tftypes.Value, error) {
	objectType := schema.Type(ctx)
	objectTypeCty, err := convertType(objectType)
	if err != nil {
		return tftypes.Value{}, err
	}
	conv := &conversion{
		objectType:    objectType,
		objectTypeCty: objectTypeCty,
	}
	priorStateCty, err := conv.ToCtyValue(priorState)
	if err != nil {
		return tftypes.Value{}, err
	}
	configCty, err := conv.ToCtyValue(config)
	if err != nil {
		return tftypes.Value{}, err
	}
	block, err := convertBlock(extractRawSchema(schema))
	if err != nil {
		return tftypes.Value{}, err
	}
	proposedNewCty := objchange.ProposedNew(block, priorStateCty, configCty)
	return conv.FromCtyValue(proposedNewCty)
}

func extractRawSchema(schema runtypes.Schema) *tfprotov6.Schema {
	panic("TODO")
}

// Turnaround through the proto layer to translate identical but nominally distinct representations of object schemata.
func convertBlock(rawSchema *tfprotov6.Schema) (*configschema.Block, error) {
	protoSchema := toproto.Schema(rawSchema)
	rawBytes, err := proto.Marshal(protoSchema.Block)
	if err != nil {
		return nil, err
	}
	var protoSchema2 *opentofuproto.Schema_Block
	err = proto.Unmarshal(rawBytes, protoSchema2)
	if err != nil {
		return nil, err
	}
	return opentofuconvert.ProtoToConfigSchema(protoSchema2), nil
}

func convertType(t tftypes.Type) (cty.Type, error) {
	ctyTypeJson, err := t.MarshalJSON()
	contract.AssertNoErrorf(err, "tftypes.Type.MarshalJSON() failed unexpectedly")
	return ctyjson.UnmarshalType(ctyTypeJson)
}

type conversion struct {
	objectType    tftypes.Type
	objectTypeCty cty.Type
}

func (c *conversion) ToCtyValue(value tftypes.Value) (cty.Value, error) {
	dv, err := tfprotov6.NewDynamicValue(c.objectType, value)
	if err != nil {
		return cty.NilVal, err
	}
	return ctymsgpack.Unmarshal(dv.MsgPack, c.objectTypeCty)
}

func (c *conversion) FromCtyValue(value cty.Value) (tftypes.Value, error) {
	bytes, err := ctymsgpack.Marshal(value, c.objectTypeCty)
	if err != nil {
		return tftypes.Value{}, err
	}
	dv := tfprotov6.DynamicValue{
		MsgPack: bytes,
	}
	return dv.Unmarshal(c.objectType)
}
