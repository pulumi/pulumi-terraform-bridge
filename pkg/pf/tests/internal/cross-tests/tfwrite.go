package crosstests

import (
	prschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl/hclwrite"
)

// This is a copy of the BlockNestingMode enum in the Terraform Plugin Framework.
// It is duplicated here because the type is not exported.
type pfNestingMode uint8

const (
	pfNestingModeUnknown pfNestingMode = 0
	pfNestingModeList    pfNestingMode = 1
	pfNestingModeSet     pfNestingMode = 2
	pfNestingModeSingle  pfNestingMode = 3
)

func pfNestingToShim(nesting pfNestingMode) hclwrite.Nesting {
	switch nesting {
	case pfNestingModeSingle:
		return hclwrite.NestingSingle
	case pfNestingModeList:
		return hclwrite.NestingList
	case pfNestingModeSet:
		return hclwrite.NestingSet
	default:
		return hclwrite.NestingInvalid
	}
}

func pSchemaBlockToObject(block prschema.Block) prschema.NestedBlockObject {
	switch block := block.(type) {
	case prschema.ListNestedBlock:
		return block.NestedObject
	case prschema.SetNestedBlock:
		return block.NestedObject
	case prschema.SingleNestedBlock:
		return prschema.NestedBlockObject{
			Attributes: block.Attributes,
			Blocks:     block.Blocks,
		}
	default:
		contract.Failf("Unknown block type: %T", block)
		return prschema.NestedBlockObject{}
	}
}

func rSchemaBlockToObject(block rschema.Block) rschema.NestedBlockObject {
	switch block := block.(type) {
	case rschema.ListNestedBlock:
		return block.NestedObject
	case rschema.SetNestedBlock:
		return block.NestedObject
	case rschema.SingleNestedBlock:
		return rschema.NestedBlockObject{
			Attributes: block.Attributes,
			Blocks:     block.Blocks,
		}
	default:
		contract.Failf("Unknown block type: %T", block)
	}
	return rschema.NestedBlockObject{}
}

type hclSchemaPFProvider prschema.Schema

var _ hclwrite.ShimHCLSchema = hclSchemaPFProvider{}

func (s hclSchemaPFProvider) GetAttributes() map[string]hclwrite.ShimHCLAttribute {
	attrMap := make(map[string]hclwrite.ShimHCLAttribute)
	for key := range s.Attributes {
		attrMap[key] = hclwrite.ShimHCLAttribute{}
	}
	return attrMap
}

func (s hclSchemaPFProvider) GetBlocks() map[string]hclwrite.ShimHCLBlock {
	blockMap := make(map[string]hclwrite.ShimHCLBlock)
	for key, block := range s.Blocks {
		blockMap[key] = hclBlockPFProvider{
			nestedObject: pSchemaBlockToObject(block),
			nesting:      pfNestingToShim(pfNestingMode(block.GetNestingMode())),
		}
	}
	return blockMap
}

type hclBlockPFProvider struct {
	nestedObject prschema.NestedBlockObject
	nesting      hclwrite.Nesting
}

var _ hclwrite.ShimHCLBlock = hclBlockPFProvider{}

func (s hclBlockPFProvider) GetAttributes() map[string]hclwrite.ShimHCLAttribute {
	attrMap := make(map[string]hclwrite.ShimHCLAttribute)
	for key := range s.nestedObject.Attributes {
		attrMap[key] = hclwrite.ShimHCLAttribute{}
	}
	return attrMap
}

func (s hclBlockPFProvider) GetBlocks() map[string]hclwrite.ShimHCLBlock {
	blockMap := make(map[string]hclwrite.ShimHCLBlock)
	for key, block := range s.nestedObject.Blocks {
		blockMap[key] = hclBlockPFProvider{
			nestedObject: pSchemaBlockToObject(block),
			nesting:      pfNestingToShim(pfNestingMode(block.GetNestingMode())),
		}
	}
	return blockMap
}

func (s hclBlockPFProvider) GetNestingMode() hclwrite.Nesting {
	return s.nesting
}

type hclSchemaPFResource rschema.Schema

var _ hclwrite.ShimHCLSchema = hclSchemaPFResource{}

func (s hclSchemaPFResource) GetAttributes() map[string]hclwrite.ShimHCLAttribute {
	attrMap := make(map[string]hclwrite.ShimHCLAttribute)
	for key := range s.Attributes {
		attrMap[key] = hclwrite.ShimHCLAttribute{}
	}
	return attrMap
}

func (s hclSchemaPFResource) GetBlocks() map[string]hclwrite.ShimHCLBlock {
	blockMap := make(map[string]hclwrite.ShimHCLBlock)
	for key, block := range s.Blocks {
		blockMap[key] = hclBlockPFResource{
			nestedObject: rSchemaBlockToObject(block),
			nesting:      pfNestingToShim(pfNestingMode(block.GetNestingMode())),
		}
	}
	return blockMap
}

type hclBlockPFResource struct {
	nestedObject rschema.NestedBlockObject
	nesting      hclwrite.Nesting
}

var _ hclwrite.ShimHCLBlock = hclBlockPFResource{}

func (s hclBlockPFResource) GetAttributes() map[string]hclwrite.ShimHCLAttribute {
	attrMap := make(map[string]hclwrite.ShimHCLAttribute)
	for key := range s.nestedObject.Attributes {
		attrMap[key] = hclwrite.ShimHCLAttribute{}
	}
	return attrMap
}

func (s hclBlockPFResource) GetBlocks() map[string]hclwrite.ShimHCLBlock {
	blockMap := make(map[string]hclwrite.ShimHCLBlock)
	for key, block := range s.nestedObject.Blocks {
		blockMap[key] = hclBlockPFResource{
			nestedObject: rSchemaBlockToObject(block),
			nesting:      pfNestingToShim(pfNestingMode(block.GetNestingMode())),
		}
	}
	return blockMap
}

func (s hclBlockPFResource) GetNestingMode() hclwrite.Nesting {
	return s.nesting
}
