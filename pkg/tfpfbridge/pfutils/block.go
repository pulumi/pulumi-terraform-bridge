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
	// "fmt"
	// "reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"

	// "github.com/hashicorp/terraform-plugin-framework/tfsdk"
	// "github.com/hashicorp/terraform-plugin-go/tftypes"

	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// Block type works around not being able to link to fwschema.Block from
// "github.com/hashicorp/terraform-plugin-framework/internal/fwschema"
type Block interface {
	GetDeprecationMessage() string
	GetDescription() string
	GetMarkdownDescription() string

	GetMaxItems() int64
	GetMinItems() int64

	Type() attr.Type

	NestedAttrs() map[string]Attr
}

func FromProviderBlock(x pschema.Block) Block {
	panic("TODO")
}

func FromDataSourceBlock(x dschema.Block) Block {
	panic("TODO")
}

func FromResourceBlock(x rschema.Block) Block {
	//x.Type().TerraformType(context.Context)
	panic("TODO")
}

// // Block type works around not being able to link to fwschema.Block from
// // "github.com/hashicorp/terraform-plugin-framework/internal/fwschema"
// //
// // Most methods from fwschema.Block have simple signatures and are copied into blockLike interface. Casting to blockLike
// // exposes these methods.
// //
// // There are some exceptions though such as GetBlocks() map[string]Block and GetAttributes() map[string]Attribute. These
// // signatures refer to further internal types. Instead of direct linking, this information is recovered and recorded in
// // new dedicated fields.
// type Block struct {
// 	BlockLike
// 	BlockNestingMode BlockNestingMode
// 	NestedBlocks     map[string]Block
// 	NestedAttrs      map[string]Attr
// }

// type BlockLike interface {
// 	GetDeprecationMessage() string
// 	GetDescription() string
// 	GetMarkdownDescription() string

// 	GetMaxItems() int64
// 	GetMinItems() int64

// 	Type() pfattr.Type
// }

// func BlockAtTerraformPath(schema *tfsdk.Schema, path *tftypes.AttributePath) (Block, error) {
// 	res, remaining, err := tftypes.WalkAttributePath(*schema, path)
// 	if err != nil {
// 		return Block{}, fmt.Errorf("%v still remains in the path: %w", remaining, err)
// 	}
// 	switch r := res.(type) {
// 	case tfsdk.Block:
// 		m := SchemaToBlockMap(&tfsdk.Schema{
// 			Blocks: map[string]tfsdk.Block{
// 				"x": r,
// 			},
// 		})
// 		return m["x"], nil
// 	default:
// 		return Block{}, fmt.Errorf("Expected a Block but found %s at path %s",
// 			reflect.TypeOf(r), path)
// 	}
// }

// func SchemaToBlockMap(schema *tfsdk.Schema) map[string]Block {
// 	if schema.GetBlocks() == nil || len(schema.GetBlocks()) == 0 {
// 		return map[string]Block{}
// 	}

// 	queue := schema.GetBlocks()
// 	for k := range queue {
// 		delete(queue, k)
// 	}

// 	type dest struct {
// 		toMap map[string]Block
// 		key   string
// 	}

// 	dests := map[string]dest{}

// 	jobCounter := 0

// 	finalMap := map[string]Block{}
// 	for k, v := range schema.GetBlocks() {
// 		job := fmt.Sprintf("%d", jobCounter)
// 		jobCounter++
// 		queue[job] = v
// 		dests[job] = dest{toMap: finalMap, key: k}
// 	}

// 	for len(queue) > 0 {
// 		job, inBlock := pop(queue)
// 		blockDest := popAt(dests, job)

// 		// outBlock := convert(inBlock)
// 		outBlock := Block{
// 			BlockLike:        inBlock,
// 			BlockNestingMode: BlockNestingMode(uint8(inBlock.GetNestingMode())),
// 			NestedBlocks:     map[string]Block{},
// 			NestedAttrs:      map[string]Attr{},
// 		}

// 		for k, v := range inBlock.GetBlocks() {
// 			job := fmt.Sprintf("%d", jobCounter)
// 			jobCounter++
// 			queue[job] = v
// 			dests[job] = dest{toMap: outBlock.NestedBlocks, key: k}
// 		}

// 		if attributes := inBlock.GetAttributes(); attributes != nil {
// 			m := make(map[string]tfsdk.Attribute)
// 			for k, v := range attributes {
// 				m[k] = v.(tfsdk.Attribute)
// 			}
// 			outBlock.NestedAttrs = SchemaToAttrMap(&tfsdk.Schema{Attributes: m})
// 		}

// 		blockDest.toMap[blockDest.key] = outBlock
// 	}

// 	return finalMap
// }

// type BlockNestingMode uint8

// const (
// 	BlockNestingModeUnknown BlockNestingMode = 0
// 	BlockNestingModeList    BlockNestingMode = 1
// 	BlockNestingModeSet     BlockNestingMode = 2
// 	BlockNestingModeSingle  BlockNestingMode = 3
// )
