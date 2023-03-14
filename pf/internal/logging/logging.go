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
	"encoding/json"
	"io"

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
	sdkLoggerOptions := makeLoggerOptions("sdk", hclog.NoLevel, output)
	ctx = context.WithValue(ctx, sdkKey, hclog.New(sdkLoggerOptions))
	ctx = context.WithValue(ctx, sdkOptionsKey, sdkLoggerOptions)
	providerLoggerOptions := makeLoggerOptions("provider", hclog.NoLevel, output)
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
		JSONFormat:        true,
		IndependentLevels: true,
		IncludeLocation:   true,
	}
}

// Re-interprets strucutred JSON logs as calls against LogSink. To be used with SetupRootLoggers.
func LogSinkWriter(ctx context.Context, sink LogSink) io.Writer {
	return &logSinkWriter{
		ctx:  ctx,
		sink: sink,
	}
}

type logSinkWriter struct {
	ctx  context.Context
	sink LogSink
}

var _ io.Writer = &logSinkWriter{}

func (w *logSinkWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	if w.sink == nil {
		return
	}
	var m map[string]interface{}
	err = json.Unmarshal(p, &m)
	if err != nil {
		return
	}
	err = w.sink.Log(w.ctx, diag.Error, "", m["@message"].(string))
	if err != nil {
		return
	}
	return
}
