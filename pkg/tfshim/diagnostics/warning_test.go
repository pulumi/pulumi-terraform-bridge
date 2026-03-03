package diagnostics

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/stretchr/testify/assert"
)

func TestValidationWarningString(t *testing.T) {
	tests := []struct {
		name     string
		warning  ValidationWarning
		expected string
	}{
		{
			name: "summary only",
			warning: ValidationWarning{
				Summary: "this field is deprecated",
			},
			expected: "this field is deprecated",
		},
		{
			name: "summary and detail",
			warning: ValidationWarning{
				Summary: "Argument is deprecated",
				Detail:  "use new_field instead",
			},
			expected: "Argument is deprecated: use new_field instead",
		},
		{
			name: "with attribute path",
			warning: ValidationWarning{
				AttributePath: cty.GetAttrPath("resource_group_name"),
				Summary:       "Argument is deprecated",
				Detail:        "use new_field instead",
			},
			expected: "[resource_group_name] Argument is deprecated: use new_field instead",
		},
		{
			name: "with nested attribute path",
			warning: ValidationWarning{
				AttributePath: cty.GetAttrPath("network_config").GetAttr("old_setting"),
				Summary:       "Argument is deprecated",
				Detail:        "use new_setting instead",
			},
			expected: "[network_config.old_setting] Argument is deprecated: use new_setting instead",
		},
		{
			name: "with indexed path",
			warning: ValidationWarning{
				AttributePath: cty.GetAttrPath("rules").IndexInt(0).GetAttr("old_field"),
				Summary:       "Argument is deprecated",
			},
			expected: "[rules[0].old_field] Argument is deprecated",
		},
		{
			name: "empty path summary only",
			warning: ValidationWarning{
				AttributePath: cty.Path{},
				Summary:       "provider is deprecated",
			},
			expected: "provider is deprecated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.warning.String())
		})
	}
}
