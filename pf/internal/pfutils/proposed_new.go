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

package pfutils

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfplan"
)

func ProposedNew(ctx context.Context, schema Schema, priorState, checkedInputs tftypes.Value) (tftypes.Value, error) {
	b, err := convertSchema(schema)
	if err != nil {
		return tftypes.Value{}, err
	}
	return tfplan.ProposedNew(b, priorState, checkedInputs)
}

func convertSchema(schema Schema) (*tfprotov6.SchemaBlock, error) {
	b := &tfprotov6.SchemaBlock{
		Attributes: []*tfprotov6.SchemaAttribute{},
		BlockTypes: []*tfprotov6.SchemaNestedBlock{},
		Deprecated: schema.DeprecationMessage() != "",
	}
	for attrName, attr := range schema.Attrs() {
		cattr, err := convertAttr(attrName, attr)
		if err != nil {
			return nil, err
		}
		b.Attributes = append(b.Attributes, cattr)
	}
	sort.Slice(b.Attributes, func(i, j int) bool {
		return b.Attributes[i].Name < b.Attributes[j].Name
	})
	for name, blk := range schema.Blocks() {
		nblk, err := convertNestedBlock(name, blk)
		if err != nil {
			return nil, err
		}
		b.BlockTypes = append(b.BlockTypes, nblk)
	}
	sort.Slice(b.BlockTypes, func(i, j int) bool {
		return b.BlockTypes[i].TypeName < b.BlockTypes[j].TypeName
	})
	return b, nil
}

func convertAttr(name string, attr Attr) (*tfprotov6.SchemaAttribute, error) {
	cattr := &tfprotov6.SchemaAttribute{
		Name:       name,
		Required:   attr.IsRequired(),
		Optional:   attr.IsOptional(),
		Computed:   attr.IsComputed(),
		Sensitive:  attr.IsSensitive(),
		Deprecated: attr.GetDeprecationMessage() != "",
	}

	if attr.IsNested() {
		cattr.NestedType = &tfprotov6.SchemaObject{}
		switch attr.NestingMode() {
		case NestingModeSingle:
			cattr.NestedType.Nesting = tfprotov6.SchemaObjectNestingModeSingle
		case NestingModeList:
			cattr.NestedType.Nesting = tfprotov6.SchemaObjectNestingModeList
		case NestingModeSet:
			cattr.NestedType.Nesting = tfprotov6.SchemaObjectNestingModeSet
		case NestingModeMap:
			cattr.NestedType.Nesting = tfprotov6.SchemaObjectNestingModeMap
		default:
			cattr.NestedType.Nesting = tfprotov6.SchemaObjectNestingModeInvalid
		}
		for name, nattr := range attr.Nested() {
			cnattr, err := convertAttr(name, nattr)
			if err != nil {
				return nil, err
			}
			cattr.NestedType.Attributes = append(cattr.NestedType.Attributes, cnattr)
		}
		sort.Slice(cattr.NestedType.Attributes, func(i, j int) bool {
			return cattr.NestedType.Attributes[i].Name < cattr.NestedType.Attributes[j].Name
		})
	} else {
		cattr.Type = attr.GetType().TerraformType(context.Background())
	}
	return cattr, nil
}

func convertNestedBlock(name string, blk Block) (*tfprotov6.SchemaNestedBlock, error) {
	b := &tfprotov6.SchemaBlock{
		Attributes: []*tfprotov6.SchemaAttribute{},
		BlockTypes: []*tfprotov6.SchemaNestedBlock{},
		Deprecated: blk.GetDeprecationMessage() != "",
	}
	nb := &tfprotov6.SchemaNestedBlock{
		TypeName: name,
		Block:    b,
		MaxItems: blk.GetMaxItems(),
		MinItems: blk.GetMinItems(),
	}
	if nbb, ok := blk.(NestedBlock); ok {
		// TODO tfprotov6.SchemaNestedBlockNestingModeMap is not representable; is this now supported by PF?
		//
		// TODO tfprotov6.SchemaNestedBlockNestingModeGroup is not representable, perhaps a limitation of what
		// got detected in NestedBlock
		switch nbb.NestingMode() {
		case BlockNestingModeList:
			nb.Nesting = tfprotov6.SchemaNestedBlockNestingModeList
		case BlockNestingModeSet:
			nb.Nesting = tfprotov6.SchemaNestedBlockNestingModeSet
		case BlockNestingModeSingle:
			nb.Nesting = tfprotov6.SchemaNestedBlockNestingModeSingle
		default:
			nb.Nesting = tfprotov6.SchemaNestedBlockNestingModeInvalid
		}
	}
	for attrName, attr := range blk.NestedAttrs() {
		cattr, err := convertAttr(attrName, attr)
		if err != nil {
			return nil, err
		}
		b.Attributes = append(b.Attributes, cattr)
	}
	sort.Slice(b.Attributes, func(i, j int) bool {
		return b.Attributes[i].Name < b.Attributes[j].Name
	})
	for nname, blk := range blk.NestedBlocks() {
		nblk, err := convertNestedBlock(nname, blk)
		if err != nil {
			return nil, err
		}
		b.BlockTypes = append(b.BlockTypes, nblk)
	}
	sort.Slice(b.BlockTypes, func(i, j int) bool {
		return b.BlockTypes[i].TypeName < b.BlockTypes[j].TypeName
	})
	return nb, nil
}
