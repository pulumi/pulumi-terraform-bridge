package tfplugin5

import (
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim/diagnostics"

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
			errors = append(errors, fromTF5ProtoDiag(d))
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
			err = multierror.Append(err, fromTF5ProtoDiag(d))
		}
	}
	return err
}

func fromTF5ProtoDiag(diagnostic *proto.Diagnostic) error {
	return &diagnostics.ValidationError{
		AttributePath: pathToCty(diagnostic.Attribute),
		Summary:       diagnostic.Summary,
		Detail:        diagnostic.Detail,
	}
}

func pathToCty(path *proto.AttributePath) cty.Path {
	var p cty.Path
	for _, s := range path.Steps {
		switch s := s.Selector.(type) {
		case *proto.AttributePath_Step_AttributeName:
			p = p.GetAttr(s.AttributeName)
		case *proto.AttributePath_Step_ElementKeyString:
			p = p.IndexString(s.ElementKeyString)
		case *proto.AttributePath_Step_ElementKeyInt:
			p = p.Index(cty.NumberIntVal(s.ElementKeyInt))
		}
	}
	return p
}
