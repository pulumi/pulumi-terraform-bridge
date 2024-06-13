package sdkv2

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2/internal/tf/configs/configschema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func findSchemaContext(topLevel *schema.Resource, path walk.SchemaPath) schemaContext {
	var c schemaContext = &blockSchemaContext{topLevel}
	for _, step := range path {
		switch step := step.(type) {
		case walk.ElementStep:
			c = c.element()
			if c == nil {
				return nil
			}
		case walk.GetAttrStep:
			c = c.attribute(step.Name)
			if c == nil {
				return nil
			}
		}
	}
	return c
}

type schemaContext interface {
	element() schemaContext
	attribute(name string) schemaContext
}

type blockSchemaContext struct {
	resource *schema.Resource
}

func (*blockSchemaContext) element() schemaContext { return nil }

func (b *blockSchemaContext) attribute(name string) schemaContext {
	s := b.resource.CoreConfigSchema()
	_, isAttr := s.Attributes[name]
	if isAttr {
		return &attrSchemaContext{b.resource, name}
	}
	blk, isBlock := s.BlockTypes[name]
	if isBlock {
		switch configschema.NestingMode(blk.Nesting) {
		case configschema.NestingSingle:
			// The only SDKv2- expressible blocks of NestingMode=Single seem to be the special "timeout"
			// blocks that are not part of the user-defined schema. The code cannot panic on these but opts
			// to silently skip processing.
			return nil
		case configschema.NestingGroup:
			contract.Failf("NestingMode=Group blocks not expressible with SDKv2: %v", blk.Nesting)
		case configschema.NestingList, configschema.NestingSet: // list, set
			x, ok := b.resource.SchemaMap()[name]
			contract.Assertf(ok, "expected to find %q in SchemaMap()", name)
			subr, ok := x.Elem.(*schema.Resource)
			contract.Assertf(ok, "expected Elem() to be a *schema.Resource")
			return &blockNestingSchemaContext{subr}
		case configschema.NestingMap: // map
			contract.Failf("NestingMode={Map} blocks not expressible with SDKv2: %v", blk.Nesting)
		default:
			contract.Failf("invalid block type %v", blk.Nesting)
			panic("invalid block type")
		}
	}
	return nil
}

// Intermediate node for stepping through collection-nested blocks.
type blockNestingSchemaContext struct {
	elem *schema.Resource
}

func (b *blockNestingSchemaContext) element() schemaContext            { return &blockSchemaContext{b.elem} }
func (*blockNestingSchemaContext) attribute(name string) schemaContext { return nil }

var _ schemaContext = (*blockSchemaContext)(nil)

// This is a leaf node of the schema context tree. While value types can be further nested, including maps and objects,
// these will not be attributes in the Terraform sense of supporting Required, Computed etc annotations.
type attrSchemaContext struct {
	resource *schema.Resource
	name     string
}

func (*attrSchemaContext) element() schemaContext              { return nil }
func (*attrSchemaContext) attribute(name string) schemaContext { return nil }

var _ schemaContext = (*attrSchemaContext)(nil)
