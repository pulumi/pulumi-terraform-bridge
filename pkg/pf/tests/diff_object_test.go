package tfbridgetests

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDiffObject(t *testing.T) {
	t.Parallel()

	attributeSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ObjectAttribute{
				Optional: true,
				AttributeTypes: map[string]attr.Type{
					"nested": types.StringType,
				},
			},
		},
	}

	attributeReplaceSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"key": rschema.ObjectAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				AttributeTypes: map[string]attr.Type{
					"nested": types.StringType,
				},
			},
		},
	}

	nestedAttributeSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
				Attributes: map[string]rschema.Attribute{
					"nested": rschema.StringAttribute{Optional: true},
				},
			},
		},
	}

	nestedAttributeReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
				Attributes: map[string]rschema.Attribute{
					"nested": rschema.StringAttribute{
						Optional: true,
					},
				},
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
			},
		},
	}

	nestedAttributeNestedReplaceSchema := rschema.Schema{
		Blocks: map[string]rschema.Block{
			"key": rschema.SingleNestedBlock{
				Attributes: map[string]rschema.Attribute{
					"nested": rschema.StringAttribute{
						Optional: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
				},
			},
		},
	}
}
