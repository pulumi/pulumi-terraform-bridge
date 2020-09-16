package tfplugin5

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim/tfplugin5/proto"
)

// unmarshalWarningsAndErrors converts a set of diagnostics from its wire format to a list of warnings and a list of
// errors. Diagnostics with unknown severity will be dropped.
func unmarshalWarningsAndErrors(diags []*proto.Diagnostic) ([]string, []error) {
	var warnings []string
	var errors []error
	for _, d := range diags {
		switch d.Severity {
		case proto.Diagnostic_ERROR:
			errors = append(errors, fmt.Errorf("%s", d.Summary))
		case proto.Diagnostic_WARNING:
			warnings = append(warnings, d.Summary)
		}
	}
	return warnings, errors
}

// unmarshalErrors converts a set of diagnostics from its wire format to a (possibly multi-) error. Diagnostics that
// are not errors are dropped.
func unmarshalErrors(diags []*proto.Diagnostic) error {
	var err error
	for _, d := range diags {
		if d.Severity == proto.Diagnostic_ERROR {
			err = multierror.Append(err, fmt.Errorf("%s", d.Summary))
		}
	}
	return err
}
