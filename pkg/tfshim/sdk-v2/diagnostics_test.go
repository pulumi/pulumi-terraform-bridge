package sdkv2

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/diagnostics"
)

func TestWarningsAndErrorsPreservesStructuredWarnings(t *testing.T) {
	t.Parallel()

	path := cty.GetAttrPath("resource_group_name")
	diags := diag.Diagnostics{
		{
			Severity:      diag.Warning,
			Summary:       "Argument is deprecated",
			Detail:        "use new_field instead",
			AttributePath: path,
		},
	}

	warnings, errors := warningsAndErrors(diags)

	require.Len(t, warnings, 1)
	assert.Empty(t, errors)

	w := warnings[0]
	assert.Equal(t, "Argument is deprecated", w.Summary)
	assert.Equal(t, "use new_field instead", w.Detail)
	assert.Equal(t, path, w.AttributePath)
}

func TestWarningsAndErrorsSeparatesWarningsAndErrors(t *testing.T) {
	t.Parallel()

	diags := diag.Diagnostics{
		{
			Severity:      diag.Warning,
			Summary:       "Argument is deprecated",
			Detail:        "use new_field",
			AttributePath: cty.GetAttrPath("old_field"),
		},
		{
			Severity:      diag.Error,
			Summary:       "Missing required field",
			Detail:        "name is required",
			AttributePath: cty.GetAttrPath("name"),
		},
		{
			Severity: diag.Warning,
			Summary:  "provider warning",
		},
	}

	warnings, errs := warningsAndErrors(diags)

	require.Len(t, warnings, 2)
	require.Len(t, errs, 1)

	// First warning has structured path
	assert.Equal(t, "Argument is deprecated", warnings[0].Summary)
	assert.Equal(t, "use new_field", warnings[0].Detail)
	assert.Equal(t, cty.GetAttrPath("old_field"), warnings[0].AttributePath)

	// Second warning has no path
	assert.Equal(t, "provider warning", warnings[1].Summary)
	assert.Empty(t, warnings[1].AttributePath)

	// Error is a ValidationError
	var valErr *diagnostics.ValidationError
	assert.ErrorAs(t, errs[0], &valErr)
	assert.Equal(t, "Missing required field", valErr.Summary)
}

// TestWarningsAndErrorsEmpty tests that an empty diagnostic list returns no warnings or errors.
func TestWarningsAndErrorsEmpty(t *testing.T) {
	t.Parallel()
	warnings, errs := warningsAndErrors(diag.Diagnostics{})
	assert.Empty(t, warnings)
	assert.Empty(t, errs)
}

// TestWarningsAndErrorsPreservesNestedPath tests that nested cty.Path values are
// preserved through the conversion.
func TestWarningsAndErrorsPreservesNestedPath(t *testing.T) {
	t.Parallel()

	path := cty.GetAttrPath("network_config").IndexInt(0).GetAttr("old_setting")
	diags := diag.Diagnostics{
		{
			Severity:      diag.Warning,
			Summary:       "Argument is deprecated",
			Detail:        "use new_setting",
			AttributePath: path,
		},
	}

	warnings, errs := warningsAndErrors(diags)

	require.Len(t, warnings, 1)
	assert.Empty(t, errs)
	assert.Equal(t, path, warnings[0].AttributePath)
	assert.Equal(t, "use new_setting", warnings[0].Detail)
}

// TestWarningsAndErrorsWarningWithoutDetail tests that a warning without Detail
// correctly sets an empty Detail field.
func TestWarningsAndErrorsWarningWithoutDetail(t *testing.T) {
	t.Parallel()

	diags := diag.Diagnostics{
		{
			Severity:      diag.Warning,
			Summary:       "field deprecated",
			AttributePath: cty.GetAttrPath("some_field"),
		},
	}

	warnings, errs := warningsAndErrors(diags)

	require.Len(t, warnings, 1)
	assert.Empty(t, errs)
	assert.Equal(t, "field deprecated", warnings[0].Summary)
	assert.Empty(t, warnings[0].Detail)
	assert.Equal(t, cty.GetAttrPath("some_field"), warnings[0].AttributePath)
}

// TestWarningsAndErrorsMultipleErrors tests that multiple errors are all preserved.
func TestWarningsAndErrorsMultipleErrors(t *testing.T) {
	t.Parallel()

	diags := diag.Diagnostics{
		{
			Severity:      diag.Error,
			Summary:       "first error",
			AttributePath: cty.GetAttrPath("field_a"),
		},
		{
			Severity:      diag.Error,
			Summary:       "second error",
			Detail:        "more details",
			AttributePath: cty.GetAttrPath("field_b"),
		},
	}

	warnings, errs := warningsAndErrors(diags)

	assert.Empty(t, warnings)
	require.Len(t, errs, 2)

	var valErr *diagnostics.ValidationError
	assert.ErrorAs(t, errs[0], &valErr)
	assert.Equal(t, "first error", valErr.Summary)

	assert.ErrorAs(t, errs[1], &valErr)
	assert.Equal(t, "second error", valErr.Summary)
	assert.Equal(t, "more details", valErr.Detail)
}
