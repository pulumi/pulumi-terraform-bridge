// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Deprecation Warning Translation — Plugin Framework (PF) and Dynamic Providers
//
// This file contains the diagnostic processing logic for PF-based providers, including
// dynamic bridged providers. For SDK v2 and SDK v1 behavior, see
// pkg/tfbridge/format_warnings.go.
//
// # Plugin Framework (PF) Providers
//
// PF providers do NOT go through the shim Validate/ValidateResource/ValidateDataSource
// methods. They call the TF gRPC server directly via tfprotov6.ProviderServer.
//
// Code path:
//
//	TF PF provider ValidateResourceConfig() returns tfprotov6.Diagnostic with Severity,
//	Summary, Detail, and Attribute (*tftypes.AttributePath).
//	  → validateResourceConfig() in pkg/pf/tfbridge/provider_check.go:
//	      Errors: converted to CheckFailures via detectCheckFailure().
//	      Remaining diagnostics (warnings): passed to processDiagnosticsWithContext()
//	      with a schemaContext built from the resource's schema and SchemaInfo.
//	  → processDiagnosticsWithContext() in this file:
//	      Calls logDiagnostic(ctx, d, sc) for each diagnostic.
//	  → logDiagnostic():
//	      Calls formatDiagnosticMessage(d, sc).
//	      Routes result via context-based logging (log.TryGetLogger).
//	  → formatDiagnosticMessage():
//	      When warning + schema context + attribute path present:
//	        1. Calls formatAttributePathAsPropertyPath()
//	           (pkg/pf/tfbridge/detect_check_failures.go)
//	           — this is the TFTYPES version, NOT the cty version
//	           — processes tftypes.AttributeName, tftypes.ElementKeyInt,
//	             tftypes.ElementKeyString, tftypes.ElementKeyValue steps
//	        2. Applies CleanTerraformLanguage() to the detail text
//	        3. If Summary contains "deprecated" (case-insensitive):
//	           Returns: [WARNING] property "deprecatedField" is deprecated: <cleaned message>
//	           Otherwise returns: [WARNING] property "someField": <cleaned message>
//	      When schema context is NOT available:
//	        Falls back to: [WARNING] <Detail>: <Summary> at attribute <path>
//
// Three operations receive schema context (enabling property name translation):
//   - Check (resource validation): provider_check.go validateResourceConfig()
//   - CheckConfig (provider config validation): provider_checkconfig.go validateProviderConfig()
//   - Invoke (data source validation): provider_invoke.go processInvokeDiagnostics()
//
// Operations WITHOUT schema context (call processDiagnostics() which passes nil):
//   - Diff, Create, Update, Read/Import, Delete, Configure
//
// Warnings emitted during these operations use the default format with raw TF
// attribute paths.
//
// Key difference from SDK v2 path: PF uses tftypes.AttributePath (with
// tftypes.AttributeName, tftypes.ElementKeyInt, etc.) while SDK v2 uses cty.Path (with
// cty.GetAttrStep, cty.IndexStep). There are two separate formatAttributePathAsPropertyPath
// functions:
//   - pkg/tfbridge/adapt_check_failures.go — cty.Path version (SDK v2)
//   - pkg/pf/tfbridge/detect_check_failures.go — tftypes version (PF)
//
// Both ultimately call NewCheckFailurePath() → TerraformToPulumiNameV2() for the
// actual name translation.
//
// # Dynamic Bridged Providers
//
// Dynamic providers (dynamic/main.go) use pfbridge.Main(), so their runtime operations
// flow through the PF bridge code in this file. The underlying TF provider is accessed
// as a tfprotov6.ProviderServer.
//
// Diagnostic flow for v6 protocol providers:
//
//	TF provider (gRPC) → tfplugin6 protobuf
//	  → dynamic/internal/shim/protov6/provider.go ValidateResourceConfig()
//	      Uses grpcutil.Translate() to convert tfplugin6 → tfprotov6
//	  → dynamic/internal/shim/protov6/translate/tfplugin6.go diagnostic()
//	      Preserves: Severity, Summary, Detail, Attribute (via attributePathRequest)
//	  → Result is tfprotov6.Diagnostic with fully preserved attribute path
//	  → Flows into PF bridge code path (same as PF providers above)
//
// Diagnostic flow for v5 protocol providers:
//
//	TF provider (gRPC, v5 protocol)
//	  → dynamic/internal/shim/protov5/ wraps as tfprotov5.ProviderServer
//	  → tf5to6server.UpgradeServer() converts tfprotov5 → tfprotov6, including diagnostics
//	  → Result is a tfprotov6.ProviderServer used identically to v6 providers
//	  → Flows into PF bridge code path (same as PF providers above)
//
// Behavior: Identical to PF providers for warning translation:
//   - During Check/CheckConfig/Invoke: property paths are translated to Pulumi camelCase,
//     CleanTerraformLanguage is applied.
//   - During Diff/Create/Update/Read/Delete: warnings use default format (no path translation).
//
// SchemaInfo limitation: Dynamic providers have NO custom SchemaInfo overlays. This means
// TerraformToPulumiNameV2() falls back to its default camelCase conversion (e.g.
// deprecated_field → deprecatedField) rather than any custom name mapping. This is
// correct and expected — dynamic providers don't have hand-authored name mappings.
package tfbridge

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/log"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// schemaContext provides optional schema information for translating diagnostic attribute paths
// to Pulumi property names.
type schemaContext struct {
	schemaMap   shim.SchemaMap
	schemaInfos map[string]*tfbridge.SchemaInfo
}

func (p *provider) processDiagnostics(ctx context.Context, diagnostics []*tfprotov6.Diagnostic) error {
	return p.processDiagnosticsWithContext(ctx, diagnostics, nil)
}

func (p *provider) processDiagnosticsWithContext(
	ctx context.Context,
	diagnostics []*tfprotov6.Diagnostic,
	sc *schemaContext,
) error {
	// Format and log diagnostics via context-based logging.
	for _, d := range diagnostics {
		p.logDiagnostic(ctx, d, sc)
	}

	// Check for errors and return non-nil if there is an error.
	for _, d := range diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			prefix := ""
			if d.Attribute != nil {
				prefix = fmt.Sprintf("[%s] ", d.Attribute.String())
			}
			if d.Summary == d.Detail {
				return fmt.Errorf("%s%s", prefix, d.Summary)
			}
			return fmt.Errorf("%s%s: %s", prefix, d.Summary, d.Detail)
		}
	}

	return nil
}

func (p *provider) logDiagnostic(ctx context.Context, d *tfprotov6.Diagnostic, sc *schemaContext) {
	logger := log.TryGetLogger(ctx)
	if logger == nil {
		return
	}
	msg := p.formatDiagnosticMessage(d, sc)
	switch d.Severity {
	case tfprotov6.DiagnosticSeverityError, tfprotov6.DiagnosticSeverityInvalid:
		logger.Error(msg)
	case tfprotov6.DiagnosticSeverityWarning:
		logger.Warn(msg)
	}
}

func (p *provider) formatDiagnosticMessage(d *tfprotov6.Diagnostic, sc *schemaContext) string {
	// For warnings with schema context and an attribute path, translate to Pulumi property names.
	if d.Severity == tfprotov6.DiagnosticSeverityWarning && sc != nil && d.Attribute != nil &&
		len(d.Attribute.Steps()) > 0 {
		pp, err := formatAttributePathAsPropertyPath(sc.schemaMap, sc.schemaInfos, d.Attribute)
		if err == nil {
			detail := d.Detail
			if detail == "" {
				detail = d.Summary
			}
			detail = tfbridge.CleanTerraformLanguage(detail)
			if tfbridge.IsDeprecationMessage(d.Summary) {
				return fmt.Sprintf("[%s] property %q is deprecated: %s",
					d.Severity.String(), pp.ValuePath(), detail)
			}
			return fmt.Sprintf("[%s] property %q: %s",
				d.Severity.String(), pp.ValuePath(), detail)
		}
		glog.V(9).Infof("Failed to translate warning attribute path: %v", err)
	}

	// Default formatting.
	msg := fmt.Sprintf("[%s] %s", d.Severity.String(), d.Detail)
	if d.Summary != "" {
		msg += fmt.Sprintf(": %s", d.Summary)
	}
	if d.Attribute != nil {
		msg += fmt.Sprintf(" at attribute %s", d.Attribute.String())
	}
	return msg
}
