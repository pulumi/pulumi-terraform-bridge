// hclwrite is a shared interface for writing HCL files for cross-tests.
// Both the Terraform Plugin SDK bridge and the Pulumi Framework bridge implement this interface.
package hclwrite

import (
	"fmt"
	"io"
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

// Nesting is the nesting mode of a block.
type Nesting string

const (
	NestingInvalid Nesting = "NestingInvalid"
	NestingSingle  Nesting = "NestingSingle"
	NestingList    Nesting = "NestingList"
	NestingSet     Nesting = "NestingSet"
)

// ShimHCLAttribute is an attribute used to write a value to an HCL file.
type ShimHCLAttribute struct{}

// ShimHCLBlock is a block used to write a value to an HCL file.
type ShimHCLBlock interface {
	GetNestingMode() Nesting
	GetAttributes() map[string]ShimHCLAttribute
	GetBlocks() map[string]ShimHCLBlock
}

// ShimHCLSchema is the schema used to write a value to an HCL file.
type ShimHCLSchema interface {
	GetAttributes() map[string]ShimHCLAttribute
	GetBlocks() map[string]ShimHCLBlock
}

// WriteProvider writes a provider declaration to an HCL file.
//
// Note that unknowns are not yet supported in cty.Value, it will error out if found.
func WriteProvider(w io.Writer, schema ShimHCLSchema, providerType string, config map[string]cty.Value) error {
	if !cty.ObjectVal(config).IsWhollyKnown() {
		return fmt.Errorf("WriteProvider cannot yet write unknowns")
	}
	f := hclwrite.NewEmptyFile()
	block := f.Body().AppendNewBlock("provider", []string{providerType})
	writeBlock(block.Body(), schema, config)
	_, err := f.WriteTo(w)
	return err
}

type lifecycleArgs struct {
	CreateBeforeDestroy bool
}

type writeResourceOptions struct {
	lifecycleArgs lifecycleArgs
}

// WriteResourceOption is an option for WriteResource.
type WriteResourceOption func(*writeResourceOptions)

// WithCreateBeforeDestroy is an option to set the create_before_destroy attribute on a resource.
func WithCreateBeforeDestroy(createBeforeDestroy bool) WriteResourceOption {
	return func(o *writeResourceOptions) {
		o.lifecycleArgs.CreateBeforeDestroy = createBeforeDestroy
	}
}

// WriteResource writes a resource declaration to an HCL file.
//
// Note that unknowns are not yet supported in cty.Value, it will error out if found.
func WriteResource(
	w io.Writer, schema ShimHCLSchema, resourceType, resourceName string, config map[string]cty.Value,
	opts ...WriteResourceOption,
) error {
	if !cty.ObjectVal(config).IsWhollyKnown() {
		return fmt.Errorf("WriteResource cannot yet write unknowns")
	}
	o := &writeResourceOptions{}
	for _, opt := range opts {
		opt(o)
	}

	if config == nil {
		config = map[string]cty.Value{}
	}

	f := hclwrite.NewEmptyFile()
	block := f.Body().AppendNewBlock("resource", []string{resourceType, resourceName})
	writeBlock(block.Body(), schema, config)

	// lifecycle block
	contract.Assertf(config["lifecycle"].IsNull(), "lifecycle block should be specified with a lifecycle option")
	lifecycle := map[string]cty.Value{}
	if o.lifecycleArgs.CreateBeforeDestroy {
		lifecycle["create_before_destroy"] = cty.True
	}
	if len(lifecycle) > 0 {
		newBlock := block.Body().AppendNewBlock("lifecycle", nil)
		writeBlock(newBlock.Body(), &lifecycleBlock{}, lifecycle)
	}
	_, err := f.WriteTo(w)
	return err
}

type lifecycleBlock struct{}

var _ ShimHCLBlock = &lifecycleBlock{}

func (b *lifecycleBlock) GetNestingMode() Nesting {
	return NestingSingle
}

func (b *lifecycleBlock) GetAttributes() map[string]ShimHCLAttribute {
	return map[string]ShimHCLAttribute{
		"create_before_destroy": {},
	}
}

func (b *lifecycleBlock) GetBlocks() map[string]ShimHCLBlock {
	return map[string]ShimHCLBlock{}
}

func writeBlock(body *hclwrite.Body, schema ShimHCLSchema, config map[string]cty.Value) {
	attributeList := make([]string, 0, len(schema.GetAttributes()))
	for key := range schema.GetAttributes() {
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

	blockList := make([]string, 0, len(schema.GetBlocks()))
	for key := range schema.GetBlocks() {
		blockList = append(blockList, key)
	}
	sort.Strings(blockList)

	for _, key := range blockList {
		block := schema.GetBlocks()[key]
		if v, ok := config[key]; !ok || v.IsNull() {
			continue
		}

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
