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
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-log/tfsdklog"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// See InitLogging.
type LogOptions struct {
	LogSink         Sink
	ProviderName    string
	ProviderVersion string
	URN             resource.URN
}

// Sets up Context-scoped loggers to route Terraform logs to the Pulumi CLI process so they are
// visible to the user.
//
// Log verbosity is controlled by the TF_LOG environment variable (set to TRACE, DEBUG, INFO, WARN,
// ERROR or OFF). By default, INFO-level logs are emitted.
//
// See also:
//
// - https://developer.hashicorp.com/terraform/plugin/log/writing
// - https://www.pulumi.com/docs/support/troubleshooting
func InitLogging(ctx context.Context, opts LogOptions) context.Context {
	ctx = setupRootLoggers(ctx, newLogSinkWriter(ctx, opts.LogSink))

	if opts.URN != "" {
		ctx = tflog.SetField(ctx, "urn", string(opts.URN))
		ctx = tfsdklog.SetField(ctx, "urn", string(opts.URN))
	}

	if opts.ProviderName != "" {
		p := opts.ProviderName
		if opts.ProviderVersion != "" {
			p += "@" + opts.ProviderVersion
		}
		ctx = tflog.SetField(ctx, "provider", p)
		ctx = tfsdklog.SetField(ctx, "provider", p)
	}

	// This call needs to happen after urn and provider fields are set, otherwise logs emitted
	// by SDKv2 code against subsystems are not tagged with the urn and provider fields.
	ctx = setupSubsystems(ctx, opts)

	return context.WithValue(ctx, CtxKey, newHost[logLike](ctx, opts.LogSink, opts.URN, func(l *host[logLike]) logLike {
		return l
	}))
}

// Providers based on https://developer.hashicorp.com/terraform/plugin/sdkv2 emit logs via
// helper_schema and helper_resource sub-systems, that need to be registered here.
func setupSubsystems(ctx context.Context, opts LogOptions) context.Context {

	// TF providers respect finer-grained control via TF_LOG_SDK and
	// TF_LOG_SDK_HELPER_RESOURCE variables, but only TF_LOG is respected here for the
	// moment, as that is the only option documented for Pulumi.
	level := tfsdklog.WithLevelFromEnv(tfLogEnvVar)

	ctx = tfsdklog.NewSubsystem(ctx, "helper_schema",
		tfsdklog.WithAdditionalLocationOffset(1),
		level,
		tfsdklog.WithRootFields(), // ensure urn and provider field tagging
	)

	ctx = tfsdklog.NewSubsystem(ctx, "helper_resource",
		tfsdklog.WithAdditionalLocationOffset(1),
		level,
		tfsdklog.WithRootFields(), // ensure urn and provider field tagging
	)

	return ctx
}

type ctxKey struct{}

// The key used to retrieve tfbridge.Logger from a context.
var CtxKey = ctxKey{}

// Abstracts the logging interface to HostClient. This is the interface providers use to report logging information back
// to the Pulumi CLI over gRPC.
type Sink interface {
	Log(context context.Context, sev diag.Severity, urn resource.URN, msg string) error
	LogStatus(context context.Context, sev diag.Severity, urn resource.URN, msg string) error
}

var _ Sink = (*provider.HostClient)(nil)

// A user friendly interface to log against, shared by SDKv2 and PF providers.
//
// Host[tfbridge.Log] implements [tfbridge.Logger].
//
// Because [tfbridge.Logger] has a dependent type: [tfbridge.Log] and because we are
// unable to name either type here, we need to parameterize [Host] with `L`, where `L` is
// always [tfbridge.Log]. This allows us to return `L` from Host[tfbridge.Log].Status()
// and satisfy the [tfbridge.Logger] interface.
type host[L any] struct {
	ctx  context.Context
	sink Sink
	urn  resource.URN

	// If we are logging ephemerally.
	status bool

	// This is the identity function, but must be defined at the call-site because we
	// cannot name [tfbridge.Log].
	mkStatus func(*host[L]) L
}

func newHost[L any](ctx context.Context,
	sink Sink,
	urn resource.URN,
	mkStatus func(*host[L]) L,
) *host[L] {
	// This nil check catches half-nil fat pointers from casting
	// `*provider.HostClient` to `Sink`.
	if host, ok := sink.(*provider.HostClient); ok && host == nil {
		sink = nil
	}
	return &host[L]{ctx, sink, urn, false /*status*/, mkStatus}
}

func (l *host[L]) f(severity diag.Severity, msg string) {
	if l.sink != nil {
		f := l.sink.Log
		if l.status {
			f = l.sink.LogStatus
		}
		err := f(l.ctx, severity, l.urn, msg)
		if err == nil {
			// We successfully wrote out our value, so we're done.
			return
		}
	}

	// We failed to write out a clean error message, so lets write to glog.
	var sev string
	switch severity {
	case diag.Debug:
		sev = "Debug"
	case diag.Info:
		sev = "Info"
	case diag.Warning:
		sev = "Warning"
	case diag.Error:
		sev = "Error"
	default:
		sev = fmt.Sprintf("%#v", severity)
	}

	glog.V(9).Infof("[%s]: %q", sev, msg)
}

func (l *host[L]) Status() L {
	copy := *l
	copy.status = true
	return l.mkStatus(&copy)
}

func (l *host[L]) StatusUntyped() any {
	return l.Status()
}

func (l *host[L]) Debug(msg string) { l.f(diag.Debug, msg) }
func (l *host[L]) Info(msg string)  { l.f(diag.Info, msg) }
func (l *host[L]) Warn(msg string)  { l.f(diag.Warning, msg) }
func (l *host[L]) Error(msg string) { l.f(diag.Error, msg) }

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

// Choose the default level carefully: logs at this level or higher (more severe) will be shown to the user of Pulumi
// CLI directly by default. Experimentally it seems that WARN is too verbose:
//
// - AWS (via terraform-plugin-sdk@v2) emits developer logs at WARN which are not shown to terraform users.
//
// Citation:
// - https://github.com/hashicorp/terraform-plugin-sdk/blob/43cfd3282307f68ea77eb4c15548100386f3a317/helper/customdiff/force_new.go#L32-L35
// - https://github.com/pulumi/pulumi-aws/issues/3389
//
//nolint:lll
func defaultTFLogLevel() hclog.Level {
	return hclog.Error
}

func makeLoggerOptions(name string, level hclog.Level, output io.Writer) *hclog.LoggerOptions {
	if level == hclog.NoLevel {
		level = defaultTFLogLevel()
	}
	return &hclog.LoggerOptions{
		Name:              name,
		Output:            output,
		Level:             level,
		IndependentLevels: true,
		IncludeLocation:   true,
		TimeFormat:        " ", // Do not print time

		// Empirically the value of 1 seems to work in the current Pulumi setup, @caller field now points to the
		// file where the logging originates.
		AdditionalLocationOffset: 1,
	}
}

// Re-interprets structured JSON logs as calls against LogSink. To be used with SetupRootLoggers.
func newLogSinkWriter(ctx context.Context, sink Sink) io.Writer {
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
	sink         Sink
}

var _ io.Writer = &logSinkWriter{}

func (w *logSinkWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	if w.sink == nil {
		return
	}

	raw := string(p)
	level, raw := parseLevelFromRawString(raw)

	if level < w.desiredLevel {
		return
	}

	urn, raw := parseUrnFromRawString(raw)
	severity := logLevelToSeverity(level)

	err = w.sink.Log(w.ctx, severity, urn, raw)
	return
}

func parseTfLogEnvVar() hclog.Level {
	return hclog.LevelFromString(os.Getenv(tfLogEnvVar))
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

func getLogger(ctx context.Context) *host[logLike] {
	return ctx.Value(CtxKey).(*host[logLike])
}

// The set of logs available to show to the user
type logLike interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}
