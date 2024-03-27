package crosstests

import (
	"fmt"
	"io"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
	for key, sch := range schemas {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch elem := sch.Elem.(type) {
		case *schema.Resource: // TODO sch.ConfigMode
			newBlock := body.AppendNewBlock(key, nil)
			writeBlock(newBlock.Body(), elem.Schema, value.AsValueMap())
		default:
			body.SetAttributeValue(key, value)
		}
	}
}
