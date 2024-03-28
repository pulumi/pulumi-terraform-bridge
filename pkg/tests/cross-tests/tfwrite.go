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
		switch elem := sch.Elem.(type) {
		case *schema.Resource: // TODO sch.ConfigMode
			if sch.Type == schema.TypeMap {
				body.SetAttributeValue(key, value)
			} else if sch.Type == schema.TypeSet {
				for _, v := range value.AsValueSet().Values() {
					newBlock := body.AppendNewBlock(key, nil)
					writeBlock(newBlock.Body(), elem.Schema, v.AsValueMap())
				}
			} else if sch.Type == schema.TypeList {
				body.SetAttributeValue(key, value)
			} else {
				contract.Failf("unexpected schema type %v", sch.Type)
			}
		default:
			body.SetAttributeValue(key, value)
		}
	}
}
