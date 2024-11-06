package crosstests

import (
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

func pfNestingToShim(nesting pfNestingMode) crosstestsimpl.Nesting {
	switch nesting {
	case pfNestingModeSingle:
		return crosstestsimpl.NestingSingle
	case pfNestingModeList:
		return crosstestsimpl.NestingList
	case pfNestingModeSet:
		return crosstestsimpl.NestingSet
	default:
		return crosstestsimpl.NestingInvalid
	}
}

func pSchemaBlockToObject(block pschema.Block) pschema.NestedBlockObject {
	switch block := block.(type) {
	case pschema.ListNestedBlock:
		return block.NestedObject
	case pschema.SetNestedBlock:
		return block.NestedObject
	case pschema.SingleNestedBlock:
		return pschema.NestedBlockObject{
			Attributes: block.Attributes,
			Blocks:     block.Blocks,
		}
	default:
		contract.Failf("Unknown block type: %T", block)
		return pschema.NestedBlockObject{}
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

type hclSchemaPFProvider struct {
	sch pschema.Schema
}

var _ crosstestsimpl.ShimHCLSchema = hclSchemaPFProvider{}

func (s hclSchemaPFProvider) Attributes() map[string]crosstestsimpl.ShimHCLAttribute {
	attrMap := make(map[string]crosstestsimpl.ShimHCLAttribute)
	for key := range s.sch.Attributes {
		attrMap[key] = crosstestsimpl.ShimHCLAttribute{}
	}
	return attrMap
}

func (s hclSchemaPFProvider) Blocks() map[string]crosstestsimpl.ShimHCLBlock {
	blockMap := make(map[string]crosstestsimpl.ShimHCLBlock)
	for key, block := range s.sch.Blocks {
		blockMap[key] = hclBlockPFProvider{
			nestedObject: pSchemaBlockToObject(block),
			nesting:      pfNestingToShim(pfNestingMode(block.GetNestingMode())),
		}
	}
	return blockMap
}

type hclBlockPFProvider struct {
	nestedObject pschema.NestedBlockObject
	nesting      crosstestsimpl.Nesting
}

var _ crosstestsimpl.ShimHCLBlock = hclBlockPFProvider{}

func (s hclBlockPFProvider) Attributes() map[string]crosstestsimpl.ShimHCLAttribute {
	attrMap := make(map[string]crosstestsimpl.ShimHCLAttribute)
	for key := range s.nestedObject.Attributes {
		attrMap[key] = crosstestsimpl.ShimHCLAttribute{}
	}
	return attrMap
}

func (s hclBlockPFProvider) Blocks() map[string]crosstestsimpl.ShimHCLBlock {
	blockMap := make(map[string]crosstestsimpl.ShimHCLBlock)
	for key, block := range s.nestedObject.Blocks {
		blockMap[key] = hclBlockPFProvider{
			nestedObject: pSchemaBlockToObject(block),
			nesting:      pfNestingToShim(pfNestingMode(block.GetNestingMode())),
		}
	}
	return blockMap
}

func (s hclBlockPFProvider) GetNestingMode() crosstestsimpl.Nesting {
	return s.nesting
}

type hclSchemaPFResource struct {
	sch rschema.Schema
}

var _ crosstestsimpl.ShimHCLSchema = hclSchemaPFResource{}

func NewHCLSchemaPFResource(sch rschema.Schema) crosstestsimpl.ShimHCLSchema {
	return hclSchemaPFResource{sch: sch}
}

func (s hclSchemaPFResource) Attributes() map[string]crosstestsimpl.ShimHCLAttribute {
	attrMap := make(map[string]crosstestsimpl.ShimHCLAttribute)
	for key := range s.sch.Attributes {
		attrMap[key] = crosstestsimpl.ShimHCLAttribute{}
	}
	return attrMap
}

func (s hclSchemaPFResource) Blocks() map[string]crosstestsimpl.ShimHCLBlock {
	blockMap := make(map[string]crosstestsimpl.ShimHCLBlock)
	for key, block := range s.sch.Blocks {
		blockMap[key] = hclBlockPFResource{
			nestedObject: rSchemaBlockToObject(block),
			nesting:      pfNestingToShim(pfNestingMode(block.GetNestingMode())),
		}
	}
	return blockMap
}

type hclBlockPFResource struct {
	nestedObject rschema.NestedBlockObject
	nesting      crosstestsimpl.Nesting
}

var _ crosstestsimpl.ShimHCLBlock = hclBlockPFResource{}

func (s hclBlockPFResource) Attributes() map[string]crosstestsimpl.ShimHCLAttribute {
	attrMap := make(map[string]crosstestsimpl.ShimHCLAttribute)
	for key := range s.nestedObject.Attributes {
		attrMap[key] = crosstestsimpl.ShimHCLAttribute{}
	}
	return attrMap
}

func (s hclBlockPFResource) Blocks() map[string]crosstestsimpl.ShimHCLBlock {
	blockMap := make(map[string]crosstestsimpl.ShimHCLBlock)
	for key, block := range s.nestedObject.Blocks {
		blockMap[key] = hclBlockPFResource{
			nestedObject: rSchemaBlockToObject(block),
			nesting:      pfNestingToShim(pfNestingMode(block.GetNestingMode())),
		}
	}
	return blockMap
}

func (s hclBlockPFResource) GetNestingMode() crosstestsimpl.Nesting {
	return s.nesting
}
