package tfbridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveIndexes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		arr      []string
		indexes  []int
		expected []string
	}{
		{
			arr:      []string{"a", "b", "c"},
			indexes:  []int{1},
			expected: []string{"a", "c"},
		},
		{
			arr:      []string{"a", "b", "c"},
			indexes:  []int{2},
			expected: []string{"a", "b"},
		},
		{
			arr:      []string{"a", "b", "c"},
			indexes:  []int{0},
			expected: []string{"b", "c"},
		},
		{
			arr:      []string{"a", "b", "c"},
			indexes:  []int{0, 2},
			expected: []string{"b"},
		},
		{
			arr:      []string{"a", "b", "c"},
			indexes:  []int{0, 1},
			expected: []string{"c"},
		},
		{
			arr:      []string{"a", "b"},
			expected: []string{"a", "b"},
		},
		{
			arr:      []string{"a", "b"},
			indexes:  []int{0, 1},
			expected: []string{},
		},
		{}, // Empty inputs
	}

	for _, tt := range tests {
		tt := tt

		t.Run("", func(t *testing.T) {
			actual := removeIndexes(tt.arr, tt.indexes)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
