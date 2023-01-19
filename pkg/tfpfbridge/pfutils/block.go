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
	"github.com/hashicorp/terraform-plugin-framework/attr"

	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// Block type works around not being able to link to fwschema.Block from
// "github.com/hashicorp/terraform-plugin-framework/internal/fwschema".
type Block interface {
	BlockLike
	NestedAttrs() map[string]Attr
	NestedBlocks() map[string]Block
	GetMaxItems() int64
	GetMinItems() int64
}

type BlockLike interface {
	GetDeprecationMessage() string
	GetDescription() string
	GetMarkdownDescription() string

	Type() attr.Type
}

func FromProviderBlock(x pschema.Block) Block {
	return FromBlockLike(x)
}

func FromDataSourceBlock(x dschema.Block) Block {
	return FromBlockLike(x)
}

func FromResourceBlock(x rschema.Block) Block {
	return FromBlockLike(x)
}

func FromBlockLike(x BlockLike) Block {
	attrs, blocks, mode := extractBlockNesting(x)
	return &blockAdapter{
		BlockLike:    x,
		nestedAttrs:  attrs,
		nestedBlocks: blocks,
		nestingMode:  mode,
	}
}

type blockAdapter struct {
	nestedAttrs  map[string]Attr
	nestedBlocks map[string]Block
	nestingMode  BlockNestingMode
	BlockLike
}

func (b *blockAdapter) GetMinItems() int64 {
	return 0 // TODO can we guess this somehow?
}

func (b *blockAdapter) GetMaxItems() int64 {
	return 0 // TODO can we guess this somehow?
}

func (b *blockAdapter) NestedAttrs() map[string]Attr {
	return b.nestedAttrs
}

func (b *blockAdapter) NestedBlocks() map[string]Block {
	return b.nestedBlocks
}

func (b *blockAdapter) NestingMode() BlockNestingMode {
	return b.nestingMode
}

type BlockNestingMode uint8

const (
	BlockNestingModeUnknown BlockNestingMode = 0
	BlockNestingModeList    BlockNestingMode = 1
	BlockNestingModeSet     BlockNestingMode = 2
	BlockNestingModeSingle  BlockNestingMode = 3
)
