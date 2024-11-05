package crosstests

import (
	"fmt"
	"io"
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

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
	}
	return pschema.NestedBlockObject{}
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

func NewHCLSchemaPFProvider(sch pschema.Schema) crosstestsimpl.ShimHCLSchema {
	return hclSchemaPFProvider{sch: sch}
}

func (s hclSchemaPFProvider) Attributes() map[string]crosstestsimpl.ShimHCLAttribute {
	attrMap := make(map[string]crosstestsimpl.ShimHCLAttribute)
	for key, attr := range s.sch.Attributes {
		attrMap[key] = attr
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
	for key, attr := range s.nestedObject.Attributes {
		attrMap[key] = attr
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
	for key, attr := range s.sch.Attributes {
		attrMap[key] = attr
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
	for key, attr := range s.nestedObject.Attributes {
		attrMap[key] = attr
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

// Provider writes a provider declaration to an HCL file.
//
// Note that unknowns are not yet supported in cty.Value, it will error out if found.
func writeProvider(out io.Writer, sch pschema.Schema, providerName string, config map[string]cty.Value) error {
	if !cty.ObjectVal(config).IsWhollyKnown() {
		return fmt.Errorf("WriteHCL cannot yet write unknowns")
	}
	f := hclwrite.NewEmptyFile()
	block := f.Body().AppendNewBlock("provider", []string{providerName})
	writePfProvider(block.Body(), sch, config)
	_, err := f.WriteTo(out)
	return err
}

// Resource writes a resource declaration to an HCL file.
//
// Note that unknowns are not yet supported in cty.Value, it will error out if found.
func writeResource(
	out io.Writer, sch rschema.Schema, resourceType, resourceName string, config map[string]cty.Value,
) error {
	if !cty.ObjectVal(config).IsWhollyKnown() {
		return fmt.Errorf("WriteHCL cannot yet write unknowns")
	}
	f := hclwrite.NewEmptyFile()
	block := f.Body().AppendNewBlock("resource", []string{resourceType, resourceName})
	writePfResource(block.Body(), sch, config)
	_, err := f.WriteTo(out)
	return err
}

func writePfProvider(body *hclwrite.Body, schemas pschema.Schema, values map[string]cty.Value) {
	writePfObjectProvider(body, pschema.NestedBlockObject{
		Attributes: schemas.Attributes,
		Blocks:     schemas.Blocks,
	}, values)
}

func writePfResource(body *hclwrite.Body, schemas rschema.Schema, values map[string]cty.Value) {
	writePfObjectResource(body, rschema.NestedBlockObject{
		Attributes: schemas.Attributes,
		Blocks:     schemas.Blocks,
	}, values)
}

// writePfBlockProvider writes the values for a single schema block to parentBody.
//
// Because blocks can be repeated (ListNestedBlock and SetNestedBlock), writePfBlockProvider
// can write an arbitrary number of blocks.
//
// For example, writing a list would add two blocks to parentBody:
//
//	writePfBlockProvider("key", parentBody, ListNestedBlock{count: int}, cty.Value([{count: 1}, {count: 2}]))
//
//	key {
//	  count = 1
//	}
//	key {
//	  count = 2
//	}
//
// This is why writePfBlockProvider is called with parentBody, instead of with the block body
// already created (as with [writeBlock]).
func writePfBlockProvider(key string, parentBody *hclwrite.Body, schemas pschema.Block, value cty.Value) {
	switch schemas := schemas.(type) {
	case pschema.ListNestedBlock:
		for _, v := range value.AsValueSlice() {
			b := parentBody.AppendNewBlock(key, nil).Body()
			writePfObjectProvider(b, schemas.NestedObject, v.AsValueMap())
		}
	case pschema.SetNestedBlock:
		values := value.AsValueSet().Values()
		for _, v := range values {
			b := parentBody.AppendNewBlock(key, nil).Body()
			writePfObjectProvider(b, schemas.NestedObject, v.AsValueMap())
		}
	case pschema.SingleNestedBlock:
		body := parentBody.AppendNewBlock(key, nil).Body()

		if value.IsNull() {
			return
		}

		writePfObjectProvider(body, pschema.NestedBlockObject{
			Attributes: schemas.Attributes,
			Blocks:     schemas.Blocks,
		}, value.AsValueMap())
	default:
		contract.Failf("Unknown block type: %T", schemas)
	}
}

func writePfBlockResource(key string, parentBody *hclwrite.Body, schemas rschema.Block, value cty.Value) {
	switch schemas := schemas.(type) {
	case rschema.ListNestedBlock:
		for _, v := range value.AsValueSlice() {
			b := parentBody.AppendNewBlock(key, nil).Body()
			writePfObjectResource(b, schemas.NestedObject, v.AsValueMap())
		}
	case rschema.SetNestedBlock:
		values := value.AsValueSet().Values()
		for _, v := range values {
			b := parentBody.AppendNewBlock(key, nil).Body()
			writePfObjectResource(b, schemas.NestedObject, v.AsValueMap())
		}
	case rschema.SingleNestedBlock:
		body := parentBody.AppendNewBlock(key, nil).Body()

		if value.IsNull() {
			return
		}

		writePfObjectResource(body, rschema.NestedBlockObject{
			Attributes: schemas.Attributes,
			Blocks:     schemas.Blocks,
		}, value.AsValueMap())
	default:
		contract.Failf("Unknown block type: %T", schemas)
	}
}

func writePfObjectProvider(body *hclwrite.Body, schemas pschema.NestedBlockObject, values map[string]cty.Value) {
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
			writePfBlockProvider(key, body, block, values[key])
			continue
		}
		contract.Failf("Could not find a attr or block for value key %q", key)
	}
}

func writePfObjectResource(body *hclwrite.Body, schemas rschema.NestedBlockObject, values map[string]cty.Value) {
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
			writePfBlockResource(key, body, block, values[key])
			continue
		}
		contract.Failf("Could not find a attr or block for value key %q", key)
	}
}
