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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// block type works around not being able to link to fwschema.Block from
// "github.com/hashicorp/terraform-plugin-framework/internal/fwschema"
//
// Most methods from fwschema.Block have simple signatures and are copied into blockLike interface. Casting to blockLike
// exposes these methods.
//
// There are some exceptions though such as GetBlocks() map[string]Block and GetAttributes() map[string]Attribute. These
// signatures refer to further internal types. Instead of direct linking, this information is recovered and recorded in
// new dedicated fields.
type block struct {
	blockLike
	blockNestingMode blockNestingMode
	nestedBlocks     map[string]block
	nestedAttrs      map[string]attr
}

type blockLike interface {
	GetDeprecationMessage() string
	GetDescription() string
	GetMarkdownDescription() string

	GetMaxItems() int64
	GetMinItems() int64

	Type() pfattr.Type
}

func schemaToBlockMap(schema *tfsdk.Schema) map[string]block {
	if schema.GetBlocks() == nil || len(schema.GetBlocks()) == 0 {
		return map[string]block{}
	}

	queue := schema.GetBlocks()
	for k := range queue {
		delete(queue, k)
	}

	type dest struct {
		toMap map[string]block
		key   string
	}

	dests := map[string]dest{}

	jobCounter := 0

	finalMap := map[string]block{}
	for k, v := range schema.GetBlocks() {
		job := fmt.Sprintf("%d", jobCounter)
		jobCounter++
		queue[job] = v
		dests[job] = dest{toMap: finalMap, key: k}
	}

	for len(queue) > 0 {
		job, inBlock := pop(queue)
		blockDest := popAt(dests, job)

		// outBlock := convert(inBlock)
		outBlock := block{
			blockLike:        inBlock,
			blockNestingMode: blockNestingMode(uint8(inBlock.GetNestingMode())),
			nestedBlocks:     map[string]block{},
			nestedAttrs:      map[string]attr{},
		}

		for k, v := range inBlock.GetBlocks() {
			job := fmt.Sprintf("%d", jobCounter)
			jobCounter++
			queue[job] = v
			dests[job] = dest{toMap: outBlock.nestedBlocks, key: k}
		}

		if attributes := inBlock.GetAttributes(); attributes != nil {
			m := make(map[string]tfsdk.Attribute)
			for k, v := range attributes {
				m[k] = v.(tfsdk.Attribute)
			}
			outBlock.nestedAttrs = schemaToAttrMap(&tfsdk.Schema{Attributes: m})
		}

		blockDest.toMap[blockDest.key] = outBlock
	}

	return finalMap
}

type blockNestingMode uint8

const (
	blockNestingModeUnknown blockNestingMode = 0
	blockNestingModeList    blockNestingMode = 1
	blockNestingModeSet     blockNestingMode = 2
	blockNestingModeSingle  blockNestingMode = 3
)
