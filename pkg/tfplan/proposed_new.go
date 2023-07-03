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

package tfplan

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfplan/internal/tf/configs/configschema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfplan/internal/tf/plans/objchange"
	"github.com/zclconf/go-cty/cty"
	ctypack "github.com/zclconf/go-cty/cty/msgpack"
)

func ProposedNew(
	block *tfprotov6.SchemaBlock,
	priorState tftypes.Value,
	config tftypes.Value,
) (tftypes.Value, error) {
	schemaBlock, err := convertSchemaBlock(block)
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("ProposedNew failed to convert the schema block: %w", err)
	}

	tt := priorState.Type()
	if tt.Equal(config.Type()) {
		return tftypes.Value{}, fmt.Errorf("ProposedNew expects priorState and config to have the same type")
	}

	t, err := convertType(tt)
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("ProposedNew failed to convert the type: %w", err)
	}

	priorStateV, err := convertValue(tt, t, priorState)
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("ProposedNew failed to convert priorState value %w", err)
	}

	configV, err := convertValue(tt, t, config)
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("ProposedNew failed to convert config value %w", err)
	}

	plannedV := objchange.ProposedNew(schemaBlock, priorStateV, configV)

	res, err := convertValueBack(tt, t, plannedV)
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("ProposedNew failed to convert back the result of planning: %w", err)
	}

	return res, nil
}

func convertType(t tftypes.Type) (cty.Type, error) {
	switch {
	case t.Equal(tftypes.Bool):
		return cty.Bool, nil
	case t.Equal(tftypes.Number):
		return cty.Number, nil
	case t.Equal(tftypes.String):
		return cty.String, nil
	case t.Equal(tftypes.DynamicPseudoType):
		return cty.DynamicPseudoType, nil
	case t.Is(tftypes.List{}):
		eT, err := convertType(t.(tftypes.List).ElementType)
		if err != nil {
			return cty.NilType, nil
		}
		return cty.List(eT), nil
	case t.Is(tftypes.Map{}):
		eT, err := convertType(t.(tftypes.Map).ElementType)
		if err != nil {
			return cty.NilType, nil
		}
		return cty.Map(eT), nil
	case t.Is(tftypes.Set{}):
		eT, err := convertType(t.(tftypes.Set).ElementType)
		if err != nil {
			return cty.NilType, nil
		}
		return cty.Set(eT), nil
	case t.Is(tftypes.Object{}):
		o := t.(tftypes.Object)
		attrTypes := map[string]cty.Type{}
		for k, at := range o.AttributeTypes {
			cat, err := convertType(at)
			if err != nil {
				return cty.NilType, nil
			}
			attrTypes[k] = cat
		}
		return cty.Object(attrTypes), nil
	case t.Is(tftypes.Tuple{}):
		tup := t.(tftypes.Tuple)
		elemTypes := []cty.Type{}
		for _, tt := range tup.ElementTypes {
			ct, err := convertType(tt)
			if err != nil {
				return cty.NilType, nil
			}
			elemTypes = append(elemTypes, ct)
		}
		return cty.Tuple(elemTypes), nil
	default:
		return cty.NilType, fmt.Errorf("Unsupported type: %v", t.String())
	}
}

func convertValue(tt tftypes.Type, t cty.Type, v tftypes.Value) (cty.Value, error) {
	dv, err := tfprotov6.NewDynamicValue(tt, v)
	if err != nil {
		return cty.NilVal, err
	}
	return ctypack.Unmarshal(dv.MsgPack, t)
}

func convertValueBack(tt tftypes.Type, t cty.Type, val cty.Value) (tftypes.Value, error) {
	msgpack, err := ctypack.Marshal(val, t)
	if err != nil {
		return tftypes.Value{}, err
	}
	return tfprotov6.DynamicValue{MsgPack: msgpack}.Unmarshal(tt)
}

func convertSchemaBlock(schema *tfprotov6.SchemaBlock) (*configschema.Block, error) {
	b := &configschema.Block{
		Attributes:      map[string]*configschema.Attribute{},
		BlockTypes:      map[string]*configschema.NestedBlock{},
		Description:     schema.Description,
		DescriptionKind: configschema.StringKind(schema.DescriptionKind),
		Deprecated:      schema.Deprecated,
	}
	for _, attr := range schema.Attributes {
		cattr, err := convertAttr(attr)
		if err != nil {
			return nil, err
		}
		b.Attributes[attr.Name] = cattr
	}
	for _, blk := range schema.BlockTypes {
		nblk, err := convertNestedBlock(blk)
		if err != nil {
			return nil, err
		}
		b.BlockTypes[blk.TypeName] = nblk
	}
	return b, nil
}

func convertNestedBlock(nb *tfprotov6.SchemaNestedBlock) (*configschema.NestedBlock, error) {
	innerBlock, err := convertSchemaBlock(nb.Block)
	if err != nil {
		return nil, err
	}
	cnb := &configschema.NestedBlock{
		Block:    *innerBlock,
		Nesting:  configschema.NestingMode(nb.Nesting),
		MinItems: int(nb.MinItems),
		MaxItems: int(nb.MaxItems),
	}
	return cnb, nil
}

func convertAttr(attr *tfprotov6.SchemaAttribute) (*configschema.Attribute, error) {
	cattr := &configschema.Attribute{
		Description:     attr.Description,
		DescriptionKind: configschema.StringKind(attr.DescriptionKind),
		Required:        attr.Required,
		Optional:        attr.Optional,
		Computed:        attr.Computed,
		Sensitive:       attr.Sensitive,
		Deprecated:      attr.Deprecated,
	}

	if attr.NestedType != nil {
		cattr.NestedType = &configschema.Object{
			Nesting: configschema.NestingMode(attr.NestedType.Nesting),
		}
		cattr.NestedType.Attributes = map[string]*configschema.Attribute{}
		for _, nattr := range attr.NestedType.Attributes {
			cnattr, err := convertAttr(nattr)
			if err != nil {
				return nil, err
			}
			cattr.NestedType.Attributes[nattr.Name] = cnattr
		}
	} else {
		ty, err := convertType(attr.Type)
		if err != nil {
			return nil, err
		}
		cattr.Type = ty
	}
	return cattr, nil
}
