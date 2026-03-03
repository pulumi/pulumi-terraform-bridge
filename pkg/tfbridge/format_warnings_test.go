package tfbridge

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
	schemav2 "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"

	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/diagnostics"
)

func TestFormatValidationWarningWithPath(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{
		"resource_group_name": {
			Type:       schemav2.TypeString,
			Optional:   true,
			Deprecated: "use new_field instead",
		},
	})
	schemaInfos := map[string]*SchemaInfo{}

	warn := diagnostics.ValidationWarning{
		AttributePath: cty.GetAttrPath("resource_group_name"),
		Summary:       "Argument is deprecated",
		Detail:        "use new_field instead",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	assert.Equal(t, `property "resourceGroupName" is deprecated: use new_field instead`, result)
}

func TestFormatValidationWarningWithoutPath(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{})
	schemaInfos := map[string]*SchemaInfo{}

	warn := diagnostics.ValidationWarning{
		Summary: "provider is deprecated",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	assert.Equal(t, "provider is deprecated", result)
}

func TestFormatValidationWarningNestedPath(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{
		"network_config": {
			Type:     schemav2.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schemav2.Resource{
				Schema: map[string]*schemav2.Schema{
					"old_setting": {
						Type:       schemav2.TypeString,
						Optional:   true,
						Deprecated: "use new_setting",
					},
				},
			},
		},
	})
	schemaInfos := map[string]*SchemaInfo{}

	warn := diagnostics.ValidationWarning{
		AttributePath: cty.GetAttrPath("network_config").IndexInt(0).GetAttr("old_setting"),
		Summary:       "Argument is deprecated",
		Detail:        "use new_setting",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	assert.Contains(t, result, "deprecated")
	assert.Contains(t, result, "use new_setting")
	// The path should be translated to Pulumi camelCase
	assert.Contains(t, result, "networkConfig")
}

func TestFormatValidationWarningWithCustomSchemaInfo(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{
		"resource_group_name": {
			Type:       schemav2.TypeString,
			Optional:   true,
			Deprecated: "use new_field instead",
		},
	})
	schemaInfos := map[string]*SchemaInfo{
		"resource_group_name": {
			Name: "rgName",
		},
	}

	warn := diagnostics.ValidationWarning{
		AttributePath: cty.GetAttrPath("resource_group_name"),
		Summary:       "Argument is deprecated",
		Detail:        "use new_field instead",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	assert.Equal(t, `property "rgName" is deprecated: use new_field instead`, result)
}

// TestFormatValidationWarningPrefersDetailOverSummary verifies that when both Summary
// and Detail are set, Detail is used as the message text.
func TestFormatValidationWarningPrefersDetailOverSummary(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{
		"old_field": {
			Type:     schemav2.TypeString,
			Optional: true,
		},
	})
	schemaInfos := map[string]*SchemaInfo{}

	warn := diagnostics.ValidationWarning{
		AttributePath: cty.GetAttrPath("old_field"),
		Summary:       "Summary text that should not appear",
		Detail:        "Detail text that should appear",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	assert.Contains(t, result, "Detail text that should appear")
	assert.NotContains(t, result, "Summary text that should not appear")
}

// TestFormatValidationWarningSummaryUsedWhenDetailEmpty verifies that when Detail is
// empty, Summary is used as the message text (with path present).
func TestFormatValidationWarningSummaryUsedWhenDetailEmpty(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{
		"old_field": {
			Type:     schemav2.TypeString,
			Optional: true,
		},
	})
	schemaInfos := map[string]*SchemaInfo{}

	warn := diagnostics.ValidationWarning{
		AttributePath: cty.GetAttrPath("old_field"),
		Summary:       "this field is deprecated",
		Detail:        "",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	assert.Equal(t, `property "oldField" is deprecated: this field is deprecated`, result)
}

// TestFormatValidationWarningCleanLanguageAppliedWithPath verifies that
// CleanTerraformLanguage is applied to the message when an attribute path is present.
func TestFormatValidationWarningCleanLanguageAppliedWithPath(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{
		"old_field": {
			Type:     schemav2.TypeString,
			Optional: true,
		},
	})
	schemaInfos := map[string]*SchemaInfo{}

	warn := diagnostics.ValidationWarning{
		AttributePath: cty.GetAttrPath("old_field"),
		Summary:       "Argument is deprecated",
		Detail:        "will be removed in 4.0 of the Azure Provider. Terraform will remove this.",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	// Both version removal and Terraform reference should be cleaned.
	assert.Contains(t, result, "will be removed in a future version")
	assert.Contains(t, result, "the upstream provider will remove this")
	assert.NotContains(t, result, "4.0 of the Azure Provider")
	assert.NotContains(t, result, "Terraform will")
}

// TestFormatValidationWarningFallbackPreservesOriginal verifies that the no-path
// fallback returns warn.String() verbatim, meaning CleanTerraformLanguage is NOT applied.
// This is the documented SDK v1 behavior.
func TestFormatValidationWarningFallbackPreservesOriginal(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{})
	schemaInfos := map[string]*SchemaInfo{}

	warn := diagnostics.ValidationWarning{
		// No AttributePath — simulates SDK v1 behavior.
		Summary: "Terraform will remove this in 4.0 of the Azure Provider",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	// The fallback path returns warn.String() which does NOT apply CleanTerraformLanguage.
	assert.Equal(t, "Terraform will remove this in 4.0 of the Azure Provider", result)
}

// TestFormatValidationWarningFallbackWithDetailPreservesOriginal verifies the same
// no-path fallback behavior when both Summary and Detail are set.
func TestFormatValidationWarningFallbackWithDetailPreservesOriginal(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{})
	schemaInfos := map[string]*SchemaInfo{}

	warn := diagnostics.ValidationWarning{
		// No AttributePath.
		Summary: "Argument is deprecated",
		Detail:  "Terraform will remove this in 4.0 of the Azure Provider",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	// The fallback path returns warn.String() verbatim.
	assert.Equal(t, "Argument is deprecated: Terraform will remove this in 4.0 of the Azure Provider", result)
}

// TestFormatValidationWarningMapPath tests path translation with a map element step.
func TestFormatValidationWarningMapPath(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{
		"tags": {
			Type:     schemav2.TypeMap,
			Optional: true,
			Elem:     &schemav2.Schema{Type: schemav2.TypeString},
		},
	})
	schemaInfos := map[string]*SchemaInfo{}

	warn := diagnostics.ValidationWarning{
		AttributePath: cty.GetAttrPath("tags").IndexString("deprecated_tag"),
		Summary:       "Argument is deprecated",
		Detail:        "this tag is no longer supported",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	assert.Contains(t, result, "tags")
	assert.Contains(t, result, "deprecated_tag")
	assert.Contains(t, result, "this tag is no longer supported")
}

func TestIsDeprecationMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		summary  string
		expected bool
	}{
		{"SDK v2 deprecation", "Argument is deprecated", true},
		{"PF deprecation lowercase", "Argument deprecated", true},
		{"PF deprecation capitalized", "Attribute Deprecated", true},
		{"generic deprecation", "This feature is deprecated", true},
		{"non-deprecation validation", "Invalid value for argument", false},
		{"non-deprecation informational", "Value may change in next release", false},
		{"empty summary", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsDeprecationMessage(tt.summary))
		})
	}
}

// TestFormatValidationWarningNonDeprecationWithPath verifies that a non-deprecation
// warning with an attribute path does NOT get the "is deprecated" label.
func TestFormatValidationWarningNonDeprecationWithPath(t *testing.T) {
	t.Parallel()
	schemaMap := shimv2.NewSchemaMap(map[string]*schemav2.Schema{
		"some_field": {
			Type:     schemav2.TypeString,
			Optional: true,
		},
	})
	schemaInfos := map[string]*SchemaInfo{}

	warn := diagnostics.ValidationWarning{
		AttributePath: cty.GetAttrPath("some_field"),
		Summary:       "Invalid value for argument",
		Detail:        "value must be between 1 and 100",
	}

	result := formatValidationWarning(warn, schemaMap, schemaInfos)
	assert.Contains(t, result, `property "someField"`)
	assert.Contains(t, result, "value must be between 1 and 100")
	assert.NotContains(t, result, "deprecated")
}

func TestCleanTerraformLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "version removal with major.minor",
			input:    "This field will be removed in 4.0 of the Azure Provider",
			expected: "This field will be removed in a future version",
		},
		{
			name:     "version removal with version prefix",
			input:    "will be removed in version 3.0 of the AWS provider",
			expected: "will be removed in a future version",
		},
		{
			name:     "version removal with semver",
			input:    "will be removed in 4.0.0 of the Google Provider",
			expected: "will be removed in a future version",
		},
		{
			name:     "terraform reference",
			input:    "Terraform will remove this in a future release",
			expected: "the upstream provider will remove this in a future release",
		},
		{
			name:     "terraform reference mid-sentence",
			input:    "Use Terraform state import instead",
			expected: "Use the upstream provider state import instead",
		},
		{
			name:     "no terraform language",
			input:    "use new_field instead",
			expected: "use new_field instead",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "combined version removal and terraform reference",
			input:    "Terraform will remove this field. It will be removed in 4.0 of the Azure Provider",
			expected: "the upstream provider will remove this field. It will be removed in a future version",
		},
		{
			name:     "version removal with v prefix",
			input:    "will be removed in v3.0 of the AWS Provider",
			expected: "will be removed in a future version",
		},
		{
			name:     "terraform as part of compound word is not replaced",
			input:    "Use the TerraformCloud integration",
			expected: "Use the TerraformCloud integration",
		},
		{
			name:     "terraform at end of sentence",
			input:    "This is managed by Terraform.",
			expected: "This is managed by the upstream provider.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, CleanTerraformLanguage(tt.input))
		})
	}
}
