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
	keys := make([]string, 0, len(schemas))
	for key := range schemas {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		sch := schemas[key]
		value, ok := values[key]
		if !ok {
			continue
		}
		contract.Assertf(sch.ConfigMode == 0, "ConfigMode > 0 is not yet supported: %v", sch.ConfigMode)
		switch elem := sch.Elem.(type) {
		case *schema.Resource:
			if sch.Type == schema.TypeMap {
				body.SetAttributeValue(key, value)
			} else if sch.Type == schema.TypeSet {
				if !value.IsNull() {
					for _, v := range value.AsValueSet().Values() {
						newBlock := body.AppendNewBlock(key, nil)
						writeBlock(newBlock.Body(), elem.Schema, v.AsValueMap())
					}
				}
			} else if sch.Type == schema.TypeList {
				if !value.IsNull() {
					for _, v := range value.AsValueSlice() {
						newBlock := body.AppendNewBlock(key, nil)
						writeBlock(newBlock.Body(), elem.Schema, v.AsValueMap())
					}
				}
			} else {
				contract.Failf("unexpected schema type %v", sch.Type)
			}
		default:
			body.SetAttributeValue(key, value)
		}
	}
}
