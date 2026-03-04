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
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
)

// testLogSink captures log output for assertions.
type testLogSink struct {
	buf *bytes.Buffer
}

func (s *testLogSink) Log(_ context.Context, sev diag.Severity, _ resource.URN, msg string) error {
	fmt.Fprintf(s.buf, "%v: %s\n", sev, msg)
	return nil
}

func (s *testLogSink) LogStatus(_ context.Context, sev diag.Severity, _ resource.URN, msg string) error {
	fmt.Fprintf(s.buf, "[status] %v: %s\n", sev, msg)
	return nil
}

func initTestLogging(sink logging.Sink) context.Context {
	return logging.InitLogging(context.Background(), logging.LogOptions{LogSink: sink})
}

func TestLogDiagnosticWarning(t *testing.T) {
	t.Parallel()
	sink := &testLogSink{buf: &bytes.Buffer{}}
	ctx := initTestLogging(sink)
	p := &provider{}

	p.logDiagnostic(ctx, &tfprotov6.Diagnostic{
		Severity:  tfprotov6.DiagnosticSeverityWarning,
		Summary:   "field is deprecated",
		Detail:    "Use other_field instead",
		Attribute: tftypes.NewAttributePath().WithAttributeName("old_field"),
	})

	out := sink.buf.String()
	assert.Contains(t, out, "warning:")
	assert.Contains(t, out, "Use other_field instead")
	assert.Contains(t, out, "field is deprecated")
}

func TestLogDiagnosticError(t *testing.T) {
	t.Parallel()
	sink := &testLogSink{buf: &bytes.Buffer{}}
	ctx := initTestLogging(sink)
	p := &provider{}

	p.logDiagnostic(ctx, &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityError,
		Summary:  "something broke",
		Detail:   "invalid value",
	})

	out := sink.buf.String()
	assert.Contains(t, out, "error:")
	assert.Contains(t, out, "invalid value")
}

func TestLogDiagnosticNoLogger(t *testing.T) {
	t.Parallel()
	p := &provider{}

	// Should not panic when context has no logger.
	assert.NotPanics(t, func() {
		p.logDiagnostic(context.Background(), &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "some warning",
			Detail:   "some detail",
		})
	})
}

func TestLogDiagnosticInvalid(t *testing.T) {
	t.Parallel()
	sink := &testLogSink{buf: &bytes.Buffer{}}
	ctx := initTestLogging(sink)
	p := &provider{}

	p.logDiagnostic(ctx, &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityInvalid,
		Detail:   "invalid severity diag",
	})

	// Invalid severity is routed to Error.
	out := sink.buf.String()
	assert.Contains(t, out, "error:")
	assert.Contains(t, out, "invalid severity diag")
}

func TestLogDiagnosticMessageFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		diag     *tfprotov6.Diagnostic
		contains []string
	}{
		{
			name: "detail only",
			diag: &tfprotov6.Diagnostic{
				Severity: tfprotov6.DiagnosticSeverityWarning,
				Detail:   "some detail",
			},
			contains: []string{"[WARNING] some detail"},
		},
		{
			name: "detail and summary",
			diag: &tfprotov6.Diagnostic{
				Severity: tfprotov6.DiagnosticSeverityWarning,
				Summary:  "some summary",
				Detail:   "some detail",
			},
			contains: []string{"[WARNING] some detail: some summary"},
		},
		{
			name: "detail summary and attribute",
			diag: &tfprotov6.Diagnostic{
				Severity:  tfprotov6.DiagnosticSeverityWarning,
				Summary:   "some summary",
				Detail:    "some detail",
				Attribute: tftypes.NewAttributePath().WithAttributeName("attr"),
			},
			contains: []string{
				"[WARNING] some detail: some summary",
				"at attribute",
			},
		},
		{
			name: "empty detail with summary",
			diag: &tfprotov6.Diagnostic{
				Severity: tfprotov6.DiagnosticSeverityWarning,
				Summary:  "some summary",
			},
			contains: []string{"[WARNING] : some summary"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sink := &testLogSink{buf: &bytes.Buffer{}}
			ctx := initTestLogging(sink)
			p := &provider{}

			p.logDiagnostic(ctx, tc.diag)

			out := sink.buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, out, s)
			}
		})
	}
}

func TestProcessDiagnosticsWarningOnly(t *testing.T) {
	t.Parallel()
	sink := &testLogSink{buf: &bytes.Buffer{}}
	ctx := initTestLogging(sink)
	p := &provider{}

	err := p.processDiagnostics(ctx, []*tfprotov6.Diagnostic{
		{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "deprecated field",
			Detail:   "use new_field instead",
		},
	})

	assert.NoError(t, err)
	assert.Contains(t, sink.buf.String(), "use new_field instead")
}

func TestProcessDiagnosticsErrorReturned(t *testing.T) {
	t.Parallel()
	sink := &testLogSink{buf: &bytes.Buffer{}}
	ctx := initTestLogging(sink)
	p := &provider{}

	err := p.processDiagnostics(ctx, []*tfprotov6.Diagnostic{
		{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "validation failed",
			Detail:   "invalid input",
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, err.Error(), "invalid input")
}

func TestProcessDiagnosticsErrorWithAttribute(t *testing.T) {
	t.Parallel()
	sink := &testLogSink{buf: &bytes.Buffer{}}
	ctx := initTestLogging(sink)
	p := &provider{}

	err := p.processDiagnostics(ctx, []*tfprotov6.Diagnostic{
		{
			Severity:  tfprotov6.DiagnosticSeverityError,
			Summary:   "bad value",
			Detail:    "must be positive",
			Attribute: tftypes.NewAttributePath().WithAttributeName("foo"),
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `[AttributeName("foo")]`)
	assert.Contains(t, err.Error(), "bad value")
}

func TestProcessDiagnosticsErrorSameSummaryDetail(t *testing.T) {
	t.Parallel()
	sink := &testLogSink{buf: &bytes.Buffer{}}
	ctx := initTestLogging(sink)
	p := &provider{}

	err := p.processDiagnostics(ctx, []*tfprotov6.Diagnostic{
		{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "something went wrong",
			Detail:   "something went wrong",
		},
	})

	require.Error(t, err)
	// When summary == detail, the message should not be duplicated.
	assert.Equal(t, "something went wrong", err.Error())
}

func TestProcessDiagnosticsMixed(t *testing.T) {
	t.Parallel()
	sink := &testLogSink{buf: &bytes.Buffer{}}
	ctx := initTestLogging(sink)
	p := &provider{}

	err := p.processDiagnostics(ctx, []*tfprotov6.Diagnostic{
		{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "warn1",
			Detail:   "warning one",
		},
		{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "warn2",
			Detail:   "warning two",
		},
		{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "err1",
			Detail:   "error one",
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "err1")

	// Both warnings should have been logged before the error was returned.
	out := sink.buf.String()
	assert.Contains(t, out, "warning one")
	assert.Contains(t, out, "warning two")
}

func TestProcessDiagnosticsEmpty(t *testing.T) {
	t.Parallel()
	sink := &testLogSink{buf: &bytes.Buffer{}}
	ctx := initTestLogging(sink)
	p := &provider{}

	err := p.processDiagnostics(ctx, []*tfprotov6.Diagnostic{})

	assert.NoError(t, err)
	assert.Empty(t, sink.buf.String())
}

func TestDeprecationWarningLogged(t *testing.T) {
	t.Parallel()
	sink := &testLogSink{buf: &bytes.Buffer{}}
	ctx := initTestLogging(sink)
	p := &provider{}

	// Reproduce the exact pattern from issue #1370.
	err := p.processDiagnostics(ctx, []*tfprotov6.Diagnostic{
		{
			Severity:  tfprotov6.DiagnosticSeverityWarning,
			Summary:   "Attribute Deprecated",
			Detail:    "**NOTE**: This is deprecated, use numeric instead.",
			Attribute: tftypes.NewAttributePath().WithAttributeName("number"),
		},
	})

	assert.NoError(t, err, "deprecation warnings should not cause errors")
	out := sink.buf.String()
	assert.Contains(t, out, "**NOTE**: This is deprecated, use numeric instead.")
	assert.Contains(t, out, "Attribute Deprecated")
}
