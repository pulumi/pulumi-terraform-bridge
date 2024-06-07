package sdkv2

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	"github.com/stretchr/testify/require"
)

func TestFindSchemaContext(t *testing.T) {
	topLevel := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"attr": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"blk": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"foo": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
				MaxItems: 1, // should not matter
			},
			"mattr": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	}

	require.NoError(t, topLevel.InternalValidate(nil, true))

	t.Run("attr", func(t *testing.T) {
		c := findSchemaContext(topLevel, walk.NewSchemaPath().GetAttr("attr"))
		a := c.(*attrSchemaContext)
		require.True(t, a.resource.CoreConfigSchema().Attributes[a.name].Optional)
	})

	t.Run("blk", func(t *testing.T) {
		c := findSchemaContext(topLevel, walk.NewSchemaPath().GetAttr("blk").Element())
		bb := c.(*blockSchemaContext)
		_, ok := bb.resource.Schema["foo"]
		require.True(t, ok)
	})

	t.Run("blk.foo", func(t *testing.T) {
		c := findSchemaContext(topLevel, walk.NewSchemaPath().GetAttr("blk").Element().GetAttr("foo"))
		a := c.(*attrSchemaContext)
		require.True(t, a.resource.CoreConfigSchema().Attributes[a.name].Optional)
	})

	t.Run("mblk", func(t *testing.T) {
		c := findSchemaContext(topLevel, walk.NewSchemaPath().GetAttr("mattr"))
		a := c.(*attrSchemaContext)
		require.True(t, a.resource.CoreConfigSchema().Attributes[a.name].Optional)
	})
}
