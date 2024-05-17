package tfplugin6

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/opentofu/opentofu/internal/tfplugin6"
)

func GetMetadataRequest(*tfprotov6.GetMetadataRequest) *tfplugin6.GetMetadata_Request {
	return &tfplugin6.GetMetadata_Request{}
}

func GetMetadataResult(r *tfplugin6.GetMetadata_Response) *tfprotov6.GetMetadataResponse {
	if r == nil {
		return nil
	}
	return &tfprotov6.GetMetadataResponse{
		ServerCapabilities: serverCapabilities(r.ServerCapabilities),
		Diagnostics:        applyArray(r.Diagnostics, diagnostic),
		DataSources:        nil,
		Functions:          nil,
		Resources:          nil,
	}
}

func applyArray[From, To any, F func(From) To](arr []From, f F) []To {
	to := make([]To, len(arr))
	for i, v := range arr {
		to[i] = f(v)
	}
	return to
}

func serverCapabilities(i *tfplugin6.ServerCapabilities) *tfprotov6.ServerCapabilities {
	if i == nil {
		return nil
	}
	return &tfprotov6.ServerCapabilities{
		GetProviderSchemaOptional: i.GetProviderSchemaOptional,
		MoveResourceState:         i.MoveResourceState,
		PlanDestroy:               i.PlanDestroy,
	}
}

func diagnostic(i *tfplugin6.Diagnostic) *tfprotov6.Diagnostic {
	if i == nil {
		return nil
	}
	return &tfprotov6.Diagnostic{
		Severity:  diagnosticSeverity(i.Severity),
		Summary:   i.Summary,
		Detail:    i.Detail,
		Attribute: attributePath(i.Attribute),
	}
}
func diagnosticSeverity(i tfplugin6.Diagnostic_Severity) tfprotov6.DiagnosticSeverity {
	switch i {
	case tfplugin6.Diagnostic_ERROR:
		return tfprotov6.DiagnosticSeverityError
	case tfplugin6.Diagnostic_WARNING:
		return tfprotov6.DiagnosticSeverityWarning
	default:
		return tfprotov6.DiagnosticSeverityInvalid
	}
}

func attributePath(i *tfplugin6.AttributePath) *tftypes.AttributePath {
	if i == nil {
		return nil
	}
	path := make([]tftypes.AttributePathStep, len(i.Steps))
	for i, step := range i.Steps {
		switch s := step.Selector.(type) {
		case *tfplugin6.AttributePath_Step_AttributeName:
			path[i] = tftypes.AttributeName(s.AttributeName)
		case *tfplugin6.AttributePath_Step_ElementKeyInt:
			path[i] = tftypes.ElementKeyInt(s.ElementKeyInt)
		case *tfplugin6.AttributePath_Step_ElementKeyString:
			path[i] = tftypes.ElementKeyString(s.ElementKeyString)
		}
	}

	return tftypes.NewAttributePathWithSteps(path)
}
