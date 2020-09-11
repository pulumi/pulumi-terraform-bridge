package sdkv2

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
)

func warningsAndErrors(diags diag.Diagnostics) ([]string, []error) {
	var warnings []string
	var errors []error
	for _, d := range diags {
		switch d.Severity {
		case diag.Error:
			errors = append(errors, fmt.Errorf("%s", d.Summary))
		case diag.Warning:
			warnings = append(warnings, d.Summary)
		}
	}
	return warnings, errors
}

func errors(diags diag.Diagnostics) error {
	var err error
	for _, d := range diags {
		if d.Severity == diag.Error {
			err = multierror.Append(err, fmt.Errorf("%s", d.Summary))
		}
	}
	return err
}
