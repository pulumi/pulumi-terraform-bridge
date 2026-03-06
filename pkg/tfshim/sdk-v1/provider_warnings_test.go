package sdkv1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWrapStringWarningsBasic tests that wrapStringWarnings converts a single string
// warning into a ValidationWarning with Summary set and empty Detail/AttributePath.
func TestWrapStringWarningsBasic(t *testing.T) {
	t.Parallel()
	warnings := wrapStringWarnings([]string{
		`"resource_group_name": [DEPRECATED] use new_field instead`,
	})

	require.Len(t, warnings, 1)
	assert.Equal(t, `"resource_group_name": [DEPRECATED] use new_field instead`, warnings[0].Summary)
	assert.Empty(t, warnings[0].Detail)
	assert.Empty(t, warnings[0].AttributePath)
}

// TestWrapStringWarningsMultiple tests conversion of multiple string warnings.
func TestWrapStringWarningsMultiple(t *testing.T) {
	t.Parallel()
	warnings := wrapStringWarnings([]string{
		"first warning",
		"second warning",
		"third warning",
	})

	require.Len(t, warnings, 3)
	assert.Equal(t, "first warning", warnings[0].Summary)
	assert.Equal(t, "second warning", warnings[1].Summary)
	assert.Equal(t, "third warning", warnings[2].Summary)

	for _, w := range warnings {
		assert.Empty(t, w.Detail)
		assert.Empty(t, w.AttributePath)
	}
}

// TestWrapStringWarningsEmpty tests that an empty input returns nil.
func TestWrapStringWarningsEmpty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, wrapStringWarnings([]string{}))
}

// TestWrapStringWarningsNil tests that nil input returns nil.
func TestWrapStringWarningsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, wrapStringWarnings(nil))
}

// TestWrapStringWarningsPreservesOriginalText tests that the warning text passes
// through verbatim. SDK v1 warnings contain Terraform-specific formatting that is NOT
// cleaned at this layer (cleaning would happen in formatValidationWarning, but it
// falls back to warn.String() since there is no AttributePath).
func TestWrapStringWarningsPreservesOriginalText(t *testing.T) {
	t.Parallel()
	input := `"old_field": Terraform will remove this in 4.0 of the Azure Provider`
	warnings := wrapStringWarnings([]string{input})

	require.Len(t, warnings, 1)
	// The text should be preserved exactly as-is.
	assert.Equal(t, input, warnings[0].Summary)
	// And warn.String() should return it unchanged (since no path or detail).
	assert.Equal(t, input, warnings[0].String())
}
