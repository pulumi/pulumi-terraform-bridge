package sdkv2

import (
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/diagnostics"
)

func warningsAndErrors(diags diag.Diagnostics) ([]string, []error) {
	var warnings []string
	var errors []error
	for _, d := range diags {
		switch d.Severity {
		case diag.Error:
			errors = append(errors, fromV2Diag(d))
		case diag.Warning:
			// the summary doesn't contain the parameter name for which the warning occurs to
			details := d.Summary
			if d.Detail != "" {
				details = d.Detail
			}
			warnings = append(warnings, details)
		}
	}
	return warnings, errors
}

func errors(diags diag.Diagnostics) error {
	var err error
	for _, d := range diags {
		if d.Severity == diag.Error {
			err = multierror.Append(err, fromV2Diag(d))
		}
	}
	return err
}

func fromV2Diag(diagnostic diag.Diagnostic) error {
	return &diagnostics.ValidationError{
		AttributePath: diagnostic.AttributePath,
		Summary:       diagnostic.Summary,
		Detail:        diagnostic.Detail,
	}
}
