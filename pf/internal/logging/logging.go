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
	"regexp"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	rprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Sets up Context-scoped loggers to route Terraform logs to the Pulumi CLI process so they are visible to the user.
//
// Log verbosity is controlled by the TF_LOG environment variable (set to TRACE, DEBUG, INFO, WARN, ERROR or OFF). By
// default, INFO-level logs are emitted.
//
// See also:
//
// - https://developer.hashicorp.com/terraform/plugin/log/writing
// - https://www.pulumi.com/docs/support/troubleshooting
func InitLogging(ctx context.Context, opts LogOptions) context.Context {
	ctx = setupRootLoggers(ctx, newLogSinkWriter(ctx, opts.LogSink))

	if opts.URN != "" {
		ctx = tflog.SetField(ctx, "urn", string(opts.URN))
	}

	if opts.ProviderName != "" {
		p := opts.ProviderName
		if opts.ProviderVersion != "" {
			p += "@" + opts.ProviderVersion
		}
		ctx = tflog.SetField(ctx, "provider", p)
	}

	return ctx
}

// See InitLogging.
type LogOptions struct {
	LogSink         LogSink
	ProviderName    string
	ProviderVersion string
	URN             resource.URN
}

// Abstracts the logging interface to HostClient. This is the interface providers use to report logging information back
// to the Pulumi CLI over gRPC.
type LogSink interface {
	Log(context context.Context, sev diag.Severity, urn resource.URN, msg string) error
}

var _ LogSink = (*rprovider.HostClient)(nil)

// Directs any logs written using the tflog API in the given Context to the given output.
func setupRootLoggers(ctx context.Context, output io.Writer) context.Context {
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
		level = hclog.Info
	}
	return &hclog.LoggerOptions{
		Name:              name,
		Output:            output,
		Level:             level,
		IndependentLevels: true,
		IncludeLocation:   true,
		TimeFormat:        " ",

		// Empirically the value of 1 seems to work in the current Pulumi setup, @caller field now points to the
		// file where the logging originates.
		AdditionalLocationOffset: 1,
	}
}

// Re-interprets strucutred JSON logs as calls against LogSink. To be used with SetupRootLoggers.
func newLogSinkWriter(ctx context.Context, sink LogSink) io.Writer {
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
	level, raw := parseLevelFromRawString(raw)

	if level <= w.desiredLevel {
		return
	}

	urn, raw := parseUrnFromRawString(raw)
	severity := logLevelToSeverity(level)

	err = w.sink.Log(w.ctx, severity, urn, raw)
	return
}

func parseTfLogEnvVar() hclog.Level {
	env, present := os.LookupEnv("TF_LOG")
	if !present {
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

var quotedUrnPattern = regexp.MustCompile(`[ ]urn=["]([^"]+)["]`)
var bareUrnPattern = regexp.MustCompile(`[ ]urn=([^ ]+)`)

func parseUrnFromRawString(s string) (resource.URN, string) {
	if ok := quotedUrnPattern.FindStringSubmatch(s); len(ok) > 0 {
		return resource.URN(ok[1]), strings.Replace(s, ok[0], "", 1)
	}
	if ok := bareUrnPattern.FindStringSubmatch(s); len(ok) > 0 {
		return resource.URN(ok[1]), strings.Replace(s, ok[0], "", 1)
	}
	return "", s
}

func parseLevelFromRawString(s string) (hclog.Level, string) {
	i := strings.Index(s, "[")
	j := strings.Index(s[i:], "]")
	remainder := s[0:i] + s[i+j+2:]
	switch s[i : i+j+1] {
	case "[ERROR]":
		return hclog.Error, remainder
	case "[WARN]":
		return hclog.Warn, remainder
	case "[DEBUG]":
		return hclog.Debug, remainder
	case "[INFO]":
		return hclog.Info, remainder
	case "[TRACE]":
		return hclog.Trace, remainder
	default:
		return hclog.Trace, remainder
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
