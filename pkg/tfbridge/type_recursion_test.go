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
		outer, inner shim.SchemaMap
		expected     bool
	}{
		{
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
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
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
			Schema: (s.SchemaMap{
				"a": (&s.Schema{
					Type: shim.TypeBool,
				}).Shim(),
			}),
		}).Shim(),
	}).Shim()

}
