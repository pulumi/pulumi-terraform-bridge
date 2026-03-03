// Copyright 2016-2024, Pulumi Corporation.
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

package tfbridge

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	schemav2 "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	pdiag "github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// testLogSink captures log messages for test verification.
type testLogSink struct {
	buf *bytes.Buffer
}

var _ logging.Sink = &testLogSink{}

func (s *testLogSink) Log(_ context.Context, sev pdiag.Severity, _ resource.URN, msg string) error {
	fmt.Fprintf(s.buf, "%v: %s\n", sev, msg)
	return nil
}

func (s *testLogSink) LogStatus(_ context.Context, sev pdiag.Severity, urn resource.URN, msg string) error {
	fmt.Fprintf(s.buf, "[status] [%v] [%v] %s\n", sev, urn, msg)
	return nil
}

// initTestLoggingContext creates a context with logging initialized for testing.
func initTestLoggingContext(t *testing.T, buf *bytes.Buffer) context.Context {
	t.Helper()
	ctx := context.Background()
	return logging.InitLogging(ctx, logging.LogOptions{
		LogSink: &testLogSink{buf: buf},
	})
}

// TestFormatDiagnosticMessageWarningWithSchemaContext tests the PF path where a warning
// diagnostic has an attribute path and schema context. The attribute name should be
// translated to Pulumi camelCase.
func TestFormatDiagnosticMessageWarningWithSchemaContext(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"deprecated_field": {
				Type:       schemav2.TypeString,
				Optional:   true,
				Deprecated: "use new_field instead",
			},
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "Argument deprecated",
		Detail:    "use new_field instead",
		Attribute: tftypes.NewAttributePath().WithAttributeName("deprecated_field"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	assert.Equal(t, `[WARNING] property "deprecatedField" is deprecated: use new_field instead`, result)
}

// TestFormatDiagnosticMessageWarningWithCustomSchemaInfo tests that custom SchemaInfo.Name
// overrides are respected in the PF path.
func TestFormatDiagnosticMessageWarningWithCustomSchemaInfo(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"deprecated_field": {
				Type:       schemav2.TypeString,
				Optional:   true,
				Deprecated: "use new_field instead",
			},
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{
			"deprecated_field": {Name: "myCustomName"},
		},
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "Argument deprecated",
		Detail:    "use new_field instead",
		Attribute: tftypes.NewAttributePath().WithAttributeName("deprecated_field"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	assert.Equal(t, `[WARNING] property "myCustomName" is deprecated: use new_field instead`, result)
}

// TestFormatDiagnosticMessageWarningNilSchemaContext tests the PF path where schema
// context is nil (e.g. during Diff, Create, Update, Read, Delete). The default format
// is used with raw attribute paths.
func TestFormatDiagnosticMessageWarningNilSchemaContext(t *testing.T) {
	t.Parallel()
	p := &provider{}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "Argument deprecated",
		Detail:    "use new_field instead",
		Attribute: tftypes.NewAttributePath().WithAttributeName("deprecated_field"),
	}

	result := p.formatDiagnosticMessage(d, nil)
	assert.Equal(t, `[WARNING] use new_field instead: Argument deprecated at attribute AttributeName("deprecated_field")`, result)
}

// TestFormatDiagnosticMessageWarningNoAttribute tests the PF path where no attribute
// path is present. Falls back to default format without a path suffix.
func TestFormatDiagnosticMessageWarningNoAttribute(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap:   shimv2.NewSchemaMap(map[string]*schemav2.Schema{}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityWarning,
		Summary:  "provider is deprecated",
		Detail:   "upgrade to v2",
	}

	result := p.formatDiagnosticMessage(d, sc)
	assert.Equal(t, "[WARNING] upgrade to v2: provider is deprecated", result)
}

// TestFormatDiagnosticMessageError tests that error diagnostics always use the default
// format, even when schema context and attribute path are available.
func TestFormatDiagnosticMessageError(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"some_field": {Type: schemav2.TypeString, Optional: true},
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityError,
		Summary:   "Invalid value",
		Detail:    "must be non-empty",
		Attribute: tftypes.NewAttributePath().WithAttributeName("some_field"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	// Errors use the default format, not the property-path translation.
	assert.Equal(t, `[ERROR] must be non-empty: Invalid value at attribute AttributeName("some_field")`, result)
}

// TestFormatDiagnosticMessageCleanTerraformLanguage tests that CleanTerraformLanguage is
// applied to the detail text in the property-path translation branch.
func TestFormatDiagnosticMessageCleanTerraformLanguage(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"old_field": {
				Type:       schemav2.TypeString,
				Optional:   true,
				Deprecated: "will be removed in 4.0 of the Azure Provider",
			},
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "Argument deprecated",
		Detail:    "will be removed in 4.0 of the Azure Provider",
		Attribute: tftypes.NewAttributePath().WithAttributeName("old_field"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	assert.Equal(t, `[WARNING] property "oldField" is deprecated: will be removed in a future version`, result)
}

// TestFormatDiagnosticMessageCleanTerraformRef tests that "Terraform" references are
// replaced in the property-path translation branch.
func TestFormatDiagnosticMessageCleanTerraformRef(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"old_field": {Type: schemav2.TypeString, Optional: true},
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "Argument deprecated",
		Detail:    "Terraform will remove this in a future release",
		Attribute: tftypes.NewAttributePath().WithAttributeName("old_field"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	assert.Equal(t,
		`[WARNING] property "oldField" is deprecated: the upstream provider will remove this in a future release`,
		result)
}

// TestFormatDiagnosticMessageNoCleanLanguageInDefaultPath tests that CleanTerraformLanguage
// is NOT applied when using the default formatting path (no schema context).
func TestFormatDiagnosticMessageNoCleanLanguageInDefaultPath(t *testing.T) {
	t.Parallel()
	p := &provider{}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "Argument deprecated",
		Detail:    "Terraform will remove this in 4.0 of the Azure Provider",
		Attribute: tftypes.NewAttributePath().WithAttributeName("old_field"),
	}

	// No schema context → default format path → no CleanTerraformLanguage.
	result := p.formatDiagnosticMessage(d, nil)
	assert.Contains(t, result, "Terraform will remove this in 4.0 of the Azure Provider")
}

// TestFormatDiagnosticMessageDetailOverSummary tests that Detail is preferred over
// Summary when both are present.
func TestFormatDiagnosticMessageDetailOverSummary(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"old_field": {Type: schemav2.TypeString, Optional: true},
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "Argument deprecated",
		Detail:    "specific detail about the deprecation",
		Attribute: tftypes.NewAttributePath().WithAttributeName("old_field"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	assert.Contains(t, result, "specific detail about the deprecation")
	assert.NotContains(t, result, "Argument deprecated")
}

// TestFormatDiagnosticMessageSummaryFallback tests that Summary is used when Detail is
// empty.
func TestFormatDiagnosticMessageSummaryFallback(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"old_field": {Type: schemav2.TypeString, Optional: true},
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "this field is deprecated",
		Detail:    "",
		Attribute: tftypes.NewAttributePath().WithAttributeName("old_field"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	assert.Equal(t, `[WARNING] property "oldField" is deprecated: this field is deprecated`, result)
}

// TestFormatDiagnosticMessageNestedPath tests that multi-step tftypes attribute paths
// are correctly translated.
func TestFormatDiagnosticMessageNestedPath(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
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
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityWarning,
		Summary:  "Argument deprecated",
		Detail:   "use new_setting",
		Attribute: tftypes.NewAttributePath().
			WithAttributeName("network_config").
			WithElementKeyInt(0).
			WithAttributeName("old_setting"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	assert.Contains(t, result, "networkConfig")
	assert.Contains(t, result, "oldSetting")
	assert.Contains(t, result, "deprecated")
	assert.Contains(t, result, "use new_setting")
}

// TestFormatDiagnosticMessageDynamicProvider tests the dynamic provider scenario where
// no custom SchemaInfo is present (nil schemaMap). TerraformToPulumiNameV2 should fall
// back to default camelCase conversion.
func TestFormatDiagnosticMessageDynamicProvider(t *testing.T) {
	t.Parallel()
	p := &provider{}
	// Dynamic providers have no custom SchemaInfo overlays.
	sc := &schemaContext{
		schemaMap:   nil,
		schemaInfos: nil,
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "Argument deprecated",
		Detail:    "use new_field instead",
		Attribute: tftypes.NewAttributePath().WithAttributeName("deprecated_field"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	// Default camelCase conversion: deprecated_field → deprecatedField
	assert.Equal(t, `[WARNING] property "deprecatedField" is deprecated: use new_field instead`, result)
}

// TestFormatDiagnosticMessageEmptyPath tests that an empty tftypes attribute path
// (no steps) falls through to the default format.
func TestFormatDiagnosticMessageEmptyPath(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap:   shimv2.NewSchemaMap(map[string]*schemav2.Schema{}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "provider-level warning",
		Detail:    "some detail",
		Attribute: tftypes.NewAttributePath(), // empty path, no steps
	}

	result := p.formatDiagnosticMessage(d, sc)
	// Empty path (no steps) falls through to default format. The Attribute is still
	// non-nil so its String() is appended, but it's an empty string.
	assert.Equal(t, "[WARNING] some detail: provider-level warning at attribute ", result)
}

// TestFormatDiagnosticMessageDefaultFormatSummaryOnly tests the default format when
// only Summary is provided (no Detail).
func TestFormatDiagnosticMessageDefaultFormatSummaryOnly(t *testing.T) {
	t.Parallel()
	p := &provider{}

	d := &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityWarning,
		Summary:  "general warning",
	}

	result := p.formatDiagnosticMessage(d, nil)
	assert.Equal(t, "[WARNING] : general warning", result)
}

// TestFormatDiagnosticMessageDefaultFormatDetailOnly tests the default format when
// only Detail is provided (no Summary).
func TestFormatDiagnosticMessageDefaultFormatDetailOnly(t *testing.T) {
	t.Parallel()
	p := &provider{}

	d := &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityWarning,
		Detail:   "detailed warning",
	}

	result := p.formatDiagnosticMessage(d, nil)
	assert.Equal(t, "[WARNING] detailed warning", result)
}

// TestFormatDiagnosticMessageNonDeprecationWarningWithSchemaContext tests that a
// non-deprecation warning with an attribute path and schema context does NOT get
// the "is deprecated" label.
func TestFormatDiagnosticMessageNonDeprecationWarningWithSchemaContext(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"some_field": {
				Type:     schemav2.TypeString,
				Optional: true,
			},
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "Invalid value for argument",
		Detail:    "value must be between 1 and 100",
		Attribute: tftypes.NewAttributePath().WithAttributeName("some_field"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	assert.Contains(t, result, `property "someField"`)
	assert.Contains(t, result, "value must be between 1 and 100")
	assert.NotContains(t, result, "deprecated")
}

// TestFormatDiagnosticMessageNonDeprecationCleanLanguage tests that CleanTerraformLanguage
// is still applied to non-deprecation warnings with attribute paths and schema context.
func TestFormatDiagnosticMessageNonDeprecationCleanLanguage(t *testing.T) {
	t.Parallel()
	p := &provider{}
	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"some_field": {
				Type:     schemav2.TypeString,
				Optional: true,
			},
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	d := &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "Value may change",
		Detail:    "Terraform will update this in a future release",
		Attribute: tftypes.NewAttributePath().WithAttributeName("some_field"),
	}

	result := p.formatDiagnosticMessage(d, sc)
	assert.Contains(t, result, "the upstream provider will update this in a future release")
	assert.NotContains(t, result, "Terraform")
	assert.NotContains(t, result, "deprecated")
}

// TestProcessDiagnosticsWithContextReturnsError tests that processDiagnosticsWithContext
// returns an error when an error-severity diagnostic is present.
func TestProcessDiagnosticsWithContextReturnsError(t *testing.T) {
	t.Parallel()
	p := &provider{}
	ctx := context.Background()

	diags := []*tfprotov6.Diagnostic{
		{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Invalid configuration",
			Detail:   "field must be set",
		},
	}

	err := p.processDiagnosticsWithContext(ctx, diags, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid configuration")
	assert.Contains(t, err.Error(), "field must be set")
}

// TestProcessDiagnosticsWithContextWarningsReturnNil tests that
// processDiagnosticsWithContext returns nil when only warnings are present.
func TestProcessDiagnosticsWithContextWarningsReturnNil(t *testing.T) {
	t.Parallel()
	p := &provider{}
	ctx := context.Background()

	diags := []*tfprotov6.Diagnostic{
		{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "Argument deprecated",
			Detail:   "use new_field",
		},
		{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "Another warning",
		},
	}

	err := p.processDiagnosticsWithContext(ctx, diags, nil)
	assert.NoError(t, err)
}

// TestProcessDiagnosticsWithContextEmpty tests that an empty diagnostic list returns nil.
func TestProcessDiagnosticsWithContextEmpty(t *testing.T) {
	t.Parallel()
	p := &provider{}
	ctx := context.Background()

	err := p.processDiagnosticsWithContext(ctx, nil, nil)
	assert.NoError(t, err)
}

// TestProcessDiagnosticsWithContextErrorWithAttribute tests that the error message
// includes the attribute path prefix when an attribute is present.
func TestProcessDiagnosticsWithContextErrorWithAttribute(t *testing.T) {
	t.Parallel()
	p := &provider{}
	ctx := context.Background()

	diags := []*tfprotov6.Diagnostic{
		{
			Severity:  tfprotov6.DiagnosticSeverityError,
			Summary:   "Invalid value",
			Detail:    "must be non-empty",
			Attribute: tftypes.NewAttributePath().WithAttributeName("some_field"),
		},
	}

	err := p.processDiagnosticsWithContext(ctx, diags, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "some_field")
	assert.Contains(t, err.Error(), "Invalid value")
	assert.Contains(t, err.Error(), "must be non-empty")
}

// TestProcessDiagnosticsWithContextErrorSameSummaryDetail tests that when Summary equals
// Detail, the error message does not duplicate the text.
func TestProcessDiagnosticsWithContextErrorSameSummaryDetail(t *testing.T) {
	t.Parallel()
	p := &provider{}
	ctx := context.Background()

	diags := []*tfprotov6.Diagnostic{
		{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "configuration is invalid",
			Detail:   "configuration is invalid",
		},
	}

	err := p.processDiagnosticsWithContext(ctx, diags, nil)
	require.Error(t, err)
	assert.Equal(t, "configuration is invalid", err.Error())
}

// TestLogDiagnosticRoutesWarnings tests that warning diagnostics are routed to
// the context logger's Warn method.
func TestLogDiagnosticRoutesWarnings(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	ctx := initTestLoggingContext(t, &logs)
	p := &provider{}

	d := &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityWarning,
		Summary:  "Argument deprecated",
		Detail:   "use new_field",
	}

	p.logDiagnostic(ctx, d, nil)

	assert.Contains(t, logs.String(), "use new_field")
	assert.Contains(t, logs.String(), "warning")
}

// TestLogDiagnosticRoutesErrors tests that error diagnostics are routed to
// the context logger's Error method.
func TestLogDiagnosticRoutesErrors(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	ctx := initTestLoggingContext(t, &logs)
	p := &provider{}

	d := &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityError,
		Summary:  "Invalid value",
		Detail:   "must be non-empty",
	}

	p.logDiagnostic(ctx, d, nil)

	assert.Contains(t, logs.String(), "must be non-empty")
	assert.Contains(t, logs.String(), "error")
}

// TestLogDiagnosticNoLogger tests that logDiagnostic does not panic when no logger
// is set up in the context.
func TestLogDiagnosticNoLogger(t *testing.T) {
	t.Parallel()
	p := &provider{}
	ctx := context.Background()

	d := &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityWarning,
		Summary:  "anything",
	}

	// Should not panic.
	p.logDiagnostic(ctx, d, nil)
}

// TestProcessDiagnosticsWithContextWarningsLogged tests that warnings are logged
// via the context logger when processing diagnostics with context.
func TestProcessDiagnosticsWithContextWarningsLogged(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	ctx := initTestLoggingContext(t, &logs)
	p := &provider{}

	sc := &schemaContext{
		schemaMap: shimv2.NewSchemaMap(map[string]*schemav2.Schema{
			"old_field": {Type: schemav2.TypeString, Optional: true},
		}),
		schemaInfos: map[string]*tfbridge.SchemaInfo{},
	}

	diags := []*tfprotov6.Diagnostic{
		{
			Severity:  tfprotov6.DiagnosticSeverityWarning,
			Summary:   "Argument deprecated",
			Detail:    "use new_field",
			Attribute: tftypes.NewAttributePath().WithAttributeName("old_field"),
		},
	}

	err := p.processDiagnosticsWithContext(ctx, diags, sc)
	assert.NoError(t, err)
	assert.Contains(t, logs.String(), `property "oldField" is deprecated`)
	assert.Contains(t, logs.String(), "use new_field")
}

// TestProcessDiagnosticsWithContextMixedDiagnostics tests processing a mix of warnings
// and errors. Warnings should be logged, and the first error should be returned.
func TestProcessDiagnosticsWithContextMixedDiagnostics(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	ctx := initTestLoggingContext(t, &logs)
	p := &provider{}

	diags := []*tfprotov6.Diagnostic{
		{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "field deprecated",
			Detail:   "use replacement",
		},
		{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Invalid config",
			Detail:   "required field missing",
		},
	}

	err := p.processDiagnosticsWithContext(ctx, diags, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid config")

	// Warning should still have been logged before the error was returned.
	assert.Contains(t, logs.String(), "use replacement")
}
