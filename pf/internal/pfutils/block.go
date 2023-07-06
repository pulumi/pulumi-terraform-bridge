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
	"regexp"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
	minItems, maxItems, _ := detectSizeConstraints(x)
	attrs, blocks, mode := extractBlockNesting(x)
	return &blockAdapter{
		BlockLike:    x,
		nestedAttrs:  attrs,
		nestedBlocks: blocks,
		nestingMode:  mode,
		minItems:     minItems,
		maxItems:     maxItems,
	}
}

type hasListValidators interface {
	ListValidators() []validator.List
}

var listSizeRegExpAtLeastAtMost = regexp.MustCompile(
	`^list must contain at least (\d+) elements and at most (\d+) elements$`)

var listSizeRegExpAtMost = regexp.MustCompile(`^list must contain at most (\d+) elements$`)

func detectSizeConstraints(x BlockLike) (int64, int64, bool) {
	ctx := context.Background()

	// List size constraints are especially important to Pulumi so this code goes the extra mile to try to detect
	// them. This influences flattening lists with MaxItems=1.
	if listBlock, isList := x.(hasListValidators); isList {
		for _, v := range listBlock.ListValidators() {
			desc := v.Description(ctx)
			if m := listSizeRegExpAtLeastAtMost.FindStringSubmatch(desc); m != nil {
				minElements, err := strconv.Atoi(m[1])
				contract.AssertNoErrorf(err, "Atoi failed on %q", m[1])
				maxElements, err := strconv.Atoi(m[2])
				contract.AssertNoErrorf(err, "Atoi failed on %q", m[2])
				return int64(minElements), int64(maxElements), true
			}
			if m := listSizeRegExpAtMost.FindStringSubmatch(desc); m != nil {
				maxElements, err := strconv.Atoi(m[1])
				contract.AssertNoErrorf(err, "Atoi failed on %q", m[1])
				return int64(0), int64(maxElements), true
			}
		}
	}

	return 0, 0, false
}

type blockAdapter struct {
	nestedAttrs  map[string]Attr
	nestedBlocks map[string]Block
	nestingMode  BlockNestingMode
	minItems     int64
	maxItems     int64
	BlockLike
}

func (b *blockAdapter) HasNestedObject() bool {
	switch b.NestingMode() {
	case BlockNestingModeList, BlockNestingModeSet:
		return true
	default:
		return false
	}
}

func (b *blockAdapter) GetMinItems() int64 {
	return b.minItems
}

func (b *blockAdapter) GetMaxItems() int64 {
	return b.maxItems
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

type NestedBlock interface {
	NestingMode() BlockNestingMode
}

type BlockNestingMode uint8

const (
	BlockNestingModeUnknown BlockNestingMode = 0
	BlockNestingModeList    BlockNestingMode = 1
	BlockNestingModeSet     BlockNestingMode = 2
	BlockNestingModeSingle  BlockNestingMode = 3
)
