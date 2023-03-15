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

package logging

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/go-hclog"

	rprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Abstracts the logging interface to HostClient. This is the interface providers use to report logging information back
// to the Pulumi CLI over gRPC.
type LogSink interface {
	Log(context context.Context, sev diag.Severity, urn resource.URN, msg string) error
	LogStatus(context context.Context, sev diag.Severity, urn resource.URN, msg string) error
}

var _ LogSink = (*rprovider.HostClient)(nil)

// Directs any logs written using the tflog API in the given Context as JSON messages to the given output.
//
// See https://developer.hashicorp.com/terraform/plugin/log/writing
func SetupRootLoggers(ctx context.Context, output io.Writer) context.Context {
	desiredLevel := parseTfLogEnvVar()
	sdkLoggerOptions := makeLoggerOptions("sdk", desiredLevel, output)
	ctx = context.WithValue(ctx, sdkKey, hclog.New(sdkLoggerOptions))
	ctx = context.WithValue(ctx, sdkOptionsKey, sdkLoggerOptions)
	providerLoggerOptions := makeLoggerOptions("provider", desiredLevel, output)
	ctx = context.WithValue(ctx, providerKey, hclog.New(providerLoggerOptions))
	ctx = context.WithValue(ctx, providerOptionsKey, providerLoggerOptions)
	return ctx
}

func makeLoggerOptions(name string, level hclog.Level, output io.Writer) *hclog.LoggerOptions {
	if level == hclog.NoLevel {
		level = hclog.Trace
	}
	return &hclog.LoggerOptions{
		Name:              name,
		Output:            output,
		Level:             level,
		IndependentLevels: true,
		IncludeLocation:   true,

		// Empirically the value of 1 seems to work in the current Pulumi setup, @caller field now points to the
		// file where the logging originates.
		AdditionalLocationOffset: 1,
	}
}

// Re-interprets strucutred JSON logs as calls against LogSink. To be used with SetupRootLoggers.
func LogSinkWriter(ctx context.Context, sink LogSink) io.Writer {
	desiredLevel := parseTfLogEnvVar()
	return &logSinkWriter{
		desiredLevel: desiredLevel,
		ctx:          ctx,
		sink:         sink,
	}
}

type logSinkWriter struct {
	desiredLevel hclog.Level
	ctx          context.Context
	sink         LogSink
}

var _ io.Writer = &logSinkWriter{}

func (w *logSinkWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	if w.sink == nil {
		return
	}

	raw := string(p)
	level := parseLevelFromRawString(raw)

	if level <= w.desiredLevel {
		return
	}

	severity := logLevelToSeverity(level)
	var urn resource.URN
	err = w.sink.Log(w.ctx, severity, urn, raw)
	return
}

// From https://www.pulumi.com/docs/support/troubleshooting/:
//
// Pulumi providers that use a bridged Terraform provider can make use of the TF_LOG environment variable (set to TRACE,
// DEBUG, INFO, WARN or ERROR) in order to provide additional diagnostic information.
//
// The code provides another option, OFF, to remove all Terraform logs.
func parseTfLogEnvVar() hclog.Level {
	env := os.Getenv("TF_LOG")
	if env == "" {
		return hclog.NoLevel
	}
	switch strings.ToUpper(env) {
	case "ERROR":
		return hclog.Error
	case "WARN":
		return hclog.Warn
	case "INFO":
		return hclog.Info
	case "TRACE":
		return hclog.Trace
	case "DEBUG":
		return hclog.Debug
	case "OFF":
		return hclog.Off
	default:
		return hclog.NoLevel
	}
}

func parseLevelFromRawString(s string) hclog.Level {
	i := strings.Index(s, "[")
	j := strings.Index(s[i:], "]")
	switch s[i : i+j+1] {
	case "[ERROR]":
		return hclog.Error
	case "[WARN]":
		return hclog.Warn
	case "[DEBUG]":
		return hclog.Debug
	case "[INFO]":
		return hclog.Info
	case "[TRACE]":
		return hclog.Trace
	default:
		return hclog.Trace
	}
}

func logLevelToSeverity(l hclog.Level) diag.Severity {
	switch l {
	case hclog.Error:
		return diag.Error
	case hclog.Warn:
		return diag.Warning
	case hclog.Info:
		return diag.Info
	case hclog.Debug:
		return diag.Debug
	case hclog.Trace:
		return diag.Debug
	default:
		return diag.Debug
	}
}
