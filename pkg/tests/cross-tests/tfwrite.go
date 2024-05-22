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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

// Writes a resource delaration. Note that unknowns are not yet supported in cty.Value, it will error out if found.
func WriteHCL(out io.Writer, sch map[string]*schema.Schema, resourceType, resourceName string, config cty.Value) error {
	if !config.IsWhollyKnown() {
		return fmt.Errorf("WriteHCL cannot yet write unknowns")
	}
	f := hclwrite.NewEmptyFile()
	block := f.Body().AppendNewBlock("resource", []string{resourceType, resourceName})
	writeBlock(block.Body(), sch, config.AsValueMap())
	_, err := f.WriteTo(out)
	return err
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
			// Needs to always be present
			newBlock := body.AppendNewBlock(key, nil)
			v, ok := values[key]
			if !ok {
				continue
			}
			res, ok := schemas[key].Elem.(*schema.Resource)
			if !ok {
				contract.Failf("unexpected schema type %s", key)
			}
			writeBlock(newBlock.Body(), res.Schema, v.AsValueMap())
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
			v, ok := values[key]
			if !ok {
				continue
			}
			res, ok := schemas[key].Elem.(*schema.Resource)
			if !ok {
				contract.Failf("unexpected schema type %s", key)
			}
			for k, elem := range v.AsValueMap() {
				newBlock := body.AppendNewBlock(key, []string{k})
				writeBlock(newBlock.Body(), res.Schema, elem.AsValueMap())
			}
		default:
			contract.Failf("unexpected nesting mode %v", bl.Nesting)
		}
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
