package pfutils

import (
	"testing"

	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

func TestComputedEqSets(t *testing.T) {
	schema := runtimeSchemaAdapter{FromResourceSchema(rschema.Schema{
		Blocks: map[string]rschema.Block{
			"foo": rschema.SetNestedBlock{
				NestedObject: rschema.NestedBlockObject{
					Attributes: map[string]rschema.Attribute{
						"bar": rschema.StringAttribute{
							// This non-computed attribute will serve
							// as our matching key for propagating
							// "baz" from elements in the prior value.
							Optional: true,
						},
						"baz": rschema.StringAttribute{
							Optional: true,
							Computed: true,
						},
					},
				},
			},
		},
	})}

	typ3 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"bar": tftypes.String,
			"baz": tftypes.String,
		},
	}

	typ2 := tftypes.Set{
		ElementType: typ3,
	}

	typ1 := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"foo": typ2,
		},
	}

	// The prior value has a computed attribute "baz" that we want to propagate.
	prior := tftypes.NewValue(typ1, map[string]tftypes.Value{
		"foo": tftypes.NewValue(typ2, []tftypes.Value{
			tftypes.NewValue(typ3, map[string]tftypes.Value{
				"bar": tftypes.NewValue(tftypes.String, "hello"),
				"baz": tftypes.NewValue(tftypes.String, "world"),
			}),
		}),
	})

	// The config value has a non-computed attribute "bar" that we want to match
	// with the prior value.
	config := tftypes.NewValue(typ1, map[string]tftypes.Value{
		"foo": tftypes.NewValue(typ2, []tftypes.Value{
			tftypes.NewValue(typ3, map[string]tftypes.Value{
				"bar": tftypes.NewValue(tftypes.String, "hello"),
				"baz": tftypes.NewValue(tftypes.String, nil),
			}),
		}),
	})

	// The computed equality function should propagate the "baz" attribute from
	// the prior value to the config value.
	eq := ComputedEq(schema)

	equal, err := eq.Equal(tftypes.NewAttributePath(), prior, config)
	require.NoError(t, err)
	require.True(t, equal)

	equal, err = eq.Equal(tftypes.NewAttributePath().WithAttributeName("foo").WithAttributeName("baz"), prior, config)
	require.NoError(t, err)
	require.True(t, equal)
}
