// Copyright 2016-2024, Pulumi Corporation.
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

// Helper code to emit Terraform HCL code to drive the Terraform CLI.
package crosstests

import (
	"fmt"
	"io"
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
	pfproviderschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

func WriteSDKv2(out io.Writer) SDKv2Writer { return SDKv2Writer{out} }

type SDKv2Writer struct{ out io.Writer }

// Provider writes a provider declaration to SDKv2Writer.
//
// Note that unknowns are not yet supported in cty.Value, it will error out if found.
func (w SDKv2Writer) Provider(sch map[string]*schema.Schema, typ string, config cty.Value) error {
	if !config.IsWhollyKnown() {
		return fmt.Errorf("WriteHCL cannot yet write unknowns")
	}
	f := hclwrite.NewEmptyFile()
	block := f.Body().AppendNewBlock("provider", []string{typ})
	writeBlock(block.Body(), sch, config.AsValueMap())
	_, err := f.WriteTo(w.out)
	return err
}

// Resource writes a resource declaration to SDKv2Writer.
//
// Note that unknowns are not yet supported in cty.Value, it will error out if found.
func (w SDKv2Writer) Resource(
	sch map[string]*schema.Schema, resourceType, resourceName string, config cty.Value,
) error {
	if !config.IsWhollyKnown() {
		return fmt.Errorf("WriteHCL cannot yet write unknowns")
	}
	f := hclwrite.NewEmptyFile()
	block := f.Body().AppendNewBlock("resource", []string{resourceType, resourceName})
	writeBlock(block.Body(), sch, config.AsValueMap())
	_, err := f.WriteTo(w.out)
	return err
}

type PFWriter struct{ out io.Writer }

func WritePF(out io.Writer) PFWriter { return PFWriter{out} }

func (w PFWriter) Provider(sch pfproviderschema.Schema, providerName string, config map[string]cty.Value) error {
	if !cty.ObjectVal(config).IsWhollyKnown() {
		return fmt.Errorf("WriteHCL cannot yet write unknowns")
	}
	f := hclwrite.NewEmptyFile()
	block := f.Body().AppendNewBlock("provider", []string{providerName})
	writePfProvider(block.Body(), sch, config)
	_, err := f.WriteTo(w.out)
	return err
}

func writePfProvider(body *hclwrite.Body, schemas pfproviderschema.Schema, values map[string]cty.Value) {
	writePfObject(body, pfproviderschema.NestedBlockObject{
		Attributes: schemas.Attributes,
		Blocks:     schemas.Blocks,
	}, values)
}

// writePfBlock writes the values for a single schema block to parentBody.
//
// Because blocks can be repeated (ListNestedBlock and SetNestedBlock), writePfBlock
// can write an arbitrary number of blocks.
//
// For example, writing a list would add two blocks to parentBody:
//
//	writePfBlock("key", parentBody, ListNestedBlock{count: int}, cty.Value([{count: 1}, {count: 2}]))
//
//	key {
//	  count = 1
//	}
//	key {
//	  count = 2
//	}
//
// This is why writePfBlock is called with parentBody, instead of with the block body
// already created (as with [writeBlock]).
func writePfBlock(key string, parentBody *hclwrite.Body, schemas pfproviderschema.Block, value cty.Value) {
	switch schemas := schemas.(type) {
	case pfproviderschema.ListNestedBlock:
		for _, v := range value.AsValueSlice() {
			b := parentBody.AppendNewBlock(key, nil).Body()
			writePfObject(b, schemas.NestedObject, v.AsValueMap())
		}
	case pfproviderschema.SetNestedBlock:
		values := value.AsValueSet().Values()
		for _, v := range values {
			b := parentBody.AppendNewBlock(key, nil).Body()
			writePfObject(b, schemas.NestedObject, v.AsValueMap())
		}
	case pfproviderschema.SingleNestedBlock:
		body := parentBody.AppendNewBlock(key, nil).Body()

		writePfObject(body, pfproviderschema.NestedBlockObject{
			Attributes: schemas.Attributes,
			Blocks:     schemas.Blocks,
		}, value.AsValueMap())
	default:
		contract.Failf("Unknown block type: %T", schemas)
	}
}

func writePfObject(body *hclwrite.Body, schemas pfproviderschema.NestedBlockObject, values map[string]cty.Value) {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if _, ok := schemas.Attributes[key]; ok {
			body.SetAttributeValue(key, values[key])
			continue
		}
		if block, ok := schemas.Blocks[key]; ok {
			writePfBlock(key, body, block, values[key])
			continue
		}
		contract.Failf("Could not find a attr or block for value key %q", key)
	}
}

func writeBlock(body *hclwrite.Body, schemas map[string]*schema.Schema, values map[string]cty.Value) {
	internalMap := schema.InternalMap(schemas)
	coreConfigSchema := internalMap.CoreConfigSchema()

	blockKeys := make([]string, 0, len(coreConfigSchema.BlockTypes))
	for key := range coreConfigSchema.BlockTypes {
		blockKeys = append(blockKeys, key)
	}
	sort.Strings(blockKeys)

	for _, key := range blockKeys {
		bl := coreConfigSchema.BlockTypes[key]
		switch bl.Nesting.String() {
		case "NestingSingle":
			v, ok := values[key]
			if !ok {
				continue
			}
			newBlock := body.AppendNewBlock(key, nil)
			res, ok := schemas[key].Elem.(*schema.Resource)
			if !ok {
				contract.Failf("unexpected schema type %s", key)
			}
			writeBlock(newBlock.Body(), res.Schema, v.AsValueMap())
		case "NestingGroup":
			contract.Failf("unexpected NestingGroup for %s with schema %s", key, schemas[key].GoString())
		case "NestingList", "NestingSet":
			v, ok := values[key]
			if !ok {
				continue
			}
			res, ok := schemas[key].Elem.(*schema.Resource)
			if !ok {
				contract.Failf("unexpected schema type %s", key)
			}
			for _, elem := range v.AsValueSlice() {
				newBlock := body.AppendNewBlock(key, nil)
				writeBlock(newBlock.Body(), res.Schema, elem.AsValueMap())
			}
		case "NestingMap":
			contract.Failf("unexpected NestingMap for %s with schema %s", key, schemas[key].GoString())
		default:
			contract.Failf("unexpected nesting mode %v", bl.Nesting)
		}
	}

	// lifecycle block
	if _, ok := values["lifecycle"]; ok {
		newBlock := body.AppendNewBlock("lifecycle", nil)
		lifecycleSchema := map[string]*schema.Schema{
			"create_before_destroy": {
				Type:     schema.TypeBool,
				Optional: true,
			},
		}
		writeBlock(newBlock.Body(), lifecycleSchema, values["lifecycle"].AsValueMap())
	}

	attrKeys := make([]string, 0, len(coreConfigSchema.Attributes))
	for key := range coreConfigSchema.Attributes {
		attrKeys = append(attrKeys, key)
	}
	sort.Strings(attrKeys)

	for _, key := range attrKeys {
		v, ok := values[key]
		if !ok {
			continue
		}
		body.SetAttributeValue(key, v)
	}
}
