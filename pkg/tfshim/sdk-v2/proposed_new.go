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

package sdkv2

import (
	"fmt"

	hcty "github.com/hashicorp/go-cty/cty"
	hctypack "github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfplan"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2/internal/schemav6"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func proposedNew(res *schema.Resource, prior, config hcty.Value) (hcty.Value, error) {
	t := res.CoreConfigSchema().ImpliedType()
	tt := htype2tftypes(t)
	schema, err := configschemaBlock(res)
	if err != nil {
		return hcty.False, fmt.Errorf("Failed to extract a schema block from a Resource: %w", err)
	}
	priorV, err := hcty2tftypes(t, tt, prior)
	if err != nil {
		return hcty.Value{}, fmt.Errorf("Failed to convert an hcty.Value to a tftypes.Value: %w", err)
	}
	configV, err := hcty2tftypes(t, tt, config)
	if err != nil {
		return hcty.Value{}, fmt.Errorf("Failed to convert an hcty.Value to a tftypes.Value: %w", err)
	}
	planned, err := tfplan.ProposedNew(schema, priorV, configV)
	if err != nil {
		return hcty.Value{}, fmt.Errorf("Failure in ProposedNew: %w", err)
	}
	result, err := tftypes2hcty(t, tt, planned)
	if err != nil {
		return hcty.Value{}, fmt.Errorf("Failed to convert a tftypes.Value to a hcty.Value: %w", err)
	}
	return result, nil
}

func htype2tftypes(t hcty.Type) tftypes.Type {
	// Used t.HasDynamicTypes() as an example of how to pattern-match types.
	switch {
	case t == hcty.NilType:
		// From docs: NilType is an invalid type used when a function is returning an error and has no useful
		// type to return.
		contract.Assertf(false, "NilType is not supported")
	case t == hcty.DynamicPseudoType:
		return tftypes.DynamicPseudoType
	case t.IsPrimitiveType():
		switch {
		case t.Equals(hcty.Bool):
			return tftypes.Bool
		case t.Equals(hcty.String):
			return tftypes.String
		case t.Equals(hcty.Number):
			return tftypes.Number
		default:
			contract.Failf("Match failure on hcty.Type with t.IsPrimitiveType()")
		}
	case t.IsListType():
		return tftypes.List{ElementType: htype2tftypes(*t.ListElementType())}
	case t.IsMapType():
		return tftypes.Map{ElementType: htype2tftypes(*t.MapElementType())}
	case t.IsSetType():
		return tftypes.Set{ElementType: htype2tftypes(*t.SetElementType())}
	case t.IsObjectType():
		attrTypes := t.AttributeTypes()
		if len(attrTypes) == 0 {
			return tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
		}
		converted := map[string]tftypes.Type{}
		for a, at := range attrTypes {
			converted[a] = htype2tftypes(at)
		}
		return tftypes.Object{AttributeTypes: converted}
	case t.IsTupleType():
		elemTypes := t.TupleElementTypes()
		if len(elemTypes) == 0 {
			return tftypes.Tuple{ElementTypes: []tftypes.Type{}}
		}
		converted := []tftypes.Type{}
		for _, et := range elemTypes {
			converted = append(converted, htype2tftypes(et))
		}
		return tftypes.Tuple{ElementTypes: converted}
	case t.IsCapsuleType():
		contract.Assertf(false, "Capsule types are not yet supported")
	}
	contract.Assertf(false, "Match failure on hcty.Type: %v", t.GoString())
	var undefined tftypes.Type
	return undefined
}

func hcty2tftypes(t hcty.Type, tt tftypes.Type, val hcty.Value) (tftypes.Value, error) {
	msgpack, err := hctypack.Marshal(val, t)
	if err != nil {
		return tftypes.Value{}, err
	}
	return tfprotov6.DynamicValue{MsgPack: msgpack}.Unmarshal(tt)
}

func tftypes2hcty(t hcty.Type, tt tftypes.Type, val tftypes.Value) (hcty.Value, error) {
	dv, err := tfprotov6.NewDynamicValue(tt, val)
	if err != nil {
		return hcty.NilVal, nil
	}
	return hctypack.Unmarshal(dv.MsgPack, t)
}

func configschemaBlock(res *schema.Resource) (*tfprotov6.SchemaBlock, error) {
	s, err := schemav6.ResourceSchema(res)
	if err != nil {
		return nil, err
	}
	contract.Assertf(s.Block != nil, "s.Block != nil")
	return s.Block, nil
}
