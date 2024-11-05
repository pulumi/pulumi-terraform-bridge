package crosstestsimpl

import (
	"fmt"
	"io"
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type Nesting string

const (
	NestingInvalid Nesting = "NestingInvalid"
	NestingSingle  Nesting = "NestingSingle"
	NestingList    Nesting = "NestingList"
	NestingSet     Nesting = "NestingSet"
)

type ShimHCLAttribute interface{}

type ShimHCLBlock interface {
	GetNestingMode() Nesting
	Attributes() map[string]ShimHCLAttribute
	Blocks() map[string]ShimHCLBlock
}

type ShimHCLSchema interface {
	Attributes() map[string]ShimHCLAttribute
	Blocks() map[string]ShimHCLBlock
}

func WriteProvider(w io.Writer, schema ShimHCLSchema, providerType string, config map[string]cty.Value) error {
	if !cty.ObjectVal(config).IsWhollyKnown() {
		return fmt.Errorf("WriteHCL cannot yet write unknowns")
	}
	f := hclwrite.NewEmptyFile()
	block := f.Body().AppendNewBlock("provider", []string{providerType})
	writeBlock(block.Body(), schema, config)
	_, err := f.WriteTo(w)
	return err
}

func WriteResource(
	w io.Writer, schema ShimHCLSchema, resourceType, resourceName string, config map[string]cty.Value,
) error {
	if !cty.ObjectVal(config).IsWhollyKnown() {
		return fmt.Errorf("WriteHCL cannot yet write unknowns")
	}
	f := hclwrite.NewEmptyFile()
	block := f.Body().AppendNewBlock("resource", []string{resourceType, resourceName})
	writeBlock(block.Body(), schema, config)
	_, err := f.WriteTo(w)
	return err
}

func writeBlock(body *hclwrite.Body, schema ShimHCLSchema, config map[string]cty.Value) {
	attributeList := make([]string, 0, len(schema.Attributes()))
	for key := range schema.Attributes() {
		attributeList = append(attributeList, key)
	}
	sort.Strings(attributeList)
	for _, key := range attributeList {
		v, ok := config[key]
		if !ok {
			continue
		}
		body.SetAttributeValue(key, v)
	}

	blockList := make([]string, 0, len(schema.Blocks()))
	for key := range schema.Blocks() {
		blockList = append(blockList, key)
	}
	sort.Strings(blockList)

	for _, key := range blockList {
		block := schema.Blocks()[key]
		switch block.GetNestingMode() {
		case NestingSingle:
			newBlock := body.AppendNewBlock(key, nil)
			writeBlock(newBlock.Body(), block, config[key].AsValueMap())
		case NestingList, NestingSet:
			for _, elem := range config[key].AsValueSlice() {
				newBlock := body.AppendNewBlock(key, nil)
				writeBlock(newBlock.Body(), block, elem.AsValueMap())
			}
		default:
			contract.Failf("unexpected nesting mode %v", block.GetNestingMode())
		}
	}
}
