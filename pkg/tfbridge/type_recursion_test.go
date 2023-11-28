package tfbridge

import (
	"testing"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	s "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/stretchr/testify/assert"
)

func TestIsRecursionOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		outer, inner shim.SchemaMap
		expected     bool
	}{
		{
			name: "depth-1",
			outer: s.SchemaMap{
				"a": scalar(shim.TypeBool),
				"b": nested(s.SchemaMap{
					"a": scalar(shim.TypeBool),
				}),
			},
			inner: s.SchemaMap{
				"a": scalar(shim.TypeBool),
			},
			expected: true,
		},
		{
			name: "inside-out",
			outer: s.SchemaMap{
				"a": scalar(shim.TypeBool),
			},
			inner: s.SchemaMap{
				"a": scalar(shim.TypeBool),
				"b": nested(s.SchemaMap{
					"a": scalar(shim.TypeBool),
				}),
			},
			expected: false,
		},
		{
			name: "depth-2",
			outer: s.SchemaMap{
				"a": nested(s.SchemaMap{
					"a": nested(s.SchemaMap{
						"b": scalar(shim.TypeInt),
					}),
					"b": scalar(shim.TypeInt),
				}),
				"b": scalar(shim.TypeInt),
			},
			inner: s.SchemaMap{
				"a": nested(s.SchemaMap{
					"b": scalar(shim.TypeInt),
				}),
				"b": scalar(shim.TypeInt),
			},
			expected: true,
		},
		{
			name: "depth-3",
			outer: s.SchemaMap{
				"a": nested(s.SchemaMap{
					"a": nested(s.SchemaMap{
						"b": scalar(shim.TypeInt),
						"a": nested(s.SchemaMap{
							"b": scalar(shim.TypeInt),
						}),
					}),
					"b": scalar(shim.TypeInt),
				}),
				"b": scalar(shim.TypeInt),
			},
			inner: s.SchemaMap{
				"a": nested(s.SchemaMap{
					"a": nested(s.SchemaMap{
						"b": scalar(shim.TypeInt),
					}),
					"b": scalar(shim.TypeInt),
				}),
				"b": scalar(shim.TypeInt),
			},
			expected: true,
		},
		{
			name: "depth-mismatch",
			outer: s.SchemaMap{
				"a": nested(s.SchemaMap{
					"a": nested(s.SchemaMap{
						"b": scalar(shim.TypeInt),
						"a": nested(s.SchemaMap{
							"b": scalar(shim.TypeInt),
						}),
					}),
					"b": scalar(shim.TypeInt),
				}),
				"b": scalar(shim.TypeInt),
			},
			inner: s.SchemaMap{
				"a": nested(s.SchemaMap{
					"b": scalar(shim.TypeInt),
				}),
				"b": scalar(shim.TypeInt),
			},
			expected: true,
		},
		// {
		// 	name: "mistype-scalar",
		// 	outer: s.SchemaMap{
		// 		"a": nested(s.SchemaMap{
		// 			"a": nested(s.SchemaMap{
		// 				"b": scalar(shim.TypeInt),
		// 				"a": nested(s.SchemaMap{
		// 					"b": scalar(shim.TypeInt),
		// 				}),
		// 			}),
		// 			"b": scalar(shim.TypeBool),
		// 		}),
		// 		"b": scalar(shim.TypeInt),
		// 	},
		// 	inner: s.SchemaMap{
		// 		"a": nested(s.SchemaMap{
		// 			"b": scalar(shim.TypeInt),
		// 		}),
		// 		"b": scalar(shim.TypeInt),
		// 	},
		// 	expected: false,
		// },
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			actual := isRecursionOf(tt.outer, tt.inner)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func scalar(typ shim.ValueType) shim.Schema { return (&s.Schema{Type: typ}).Shim() }

func nested(m s.SchemaMap) shim.Schema {
	return (&s.Schema{
		Type: shim.TypeMap,
		Elem: (&s.Resource{
			Schema: m,
		}).Shim(),
	}).Shim()

}
