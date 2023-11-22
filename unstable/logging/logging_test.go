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
	"bytes"
	"context"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestLogging(t *testing.T) {
	urn := resource.URN("urn:pulumi:prod::web::custom:resources:Resource$random:index/password:Password::my-pw")

	cases := []struct {
		name string
		opts LogOptions
		emit func(context.Context)
		logs []log
		env  map[string]string
	}{
		{
			name: "WARN and higher propagates by default",
			opts: LogOptions{},
			emit: func(ctx context.Context) {
				tflog.Trace(ctx, "Something went wrong TRACE")
				tflog.Debug(ctx, "Something went wrong DEBUG")
				tflog.Info(ctx, "Something went wrong INFO")
				tflog.Warn(ctx, "Something went wrong WARN")
				tflog.Error(ctx, "Something went wrong ERROR ")
			},
			logs: []log{
				{
					msg: `Something went wrong WARN`,
					sev: diag.Warning,
				},
				{
					msg: `Something went wrong ERROR`,
					sev: diag.Error,
				},
			},
		},
		{
			name: "TF_LOG env var filtering can restrict logs",
			opts: LogOptions{},
			emit: func(ctx context.Context) {
				tflog.Trace(ctx, "Something went wrong TRACE")
				tflog.Debug(ctx, "Something went wrong DEBUG")
				tflog.Info(ctx, "Something went wrong INFO")
				tflog.Warn(ctx, "Something went wrong WARN")
				tflog.Error(ctx, "Something went wrong ERROR ")
			},
			env: map[string]string{
				"TF_LOG": "ERROR",
			},
			logs: []log{
				{
					msg: `Something went wrong ERROR`,
					sev: diag.Error,
				},
			},
		},
		{
			name: "TF_LOG env var filtering can enable more logging",
			opts: LogOptions{},
			emit: func(ctx context.Context) {
				tflog.Trace(ctx, "Something went wrong TRACE")
				tflog.Debug(ctx, "Something went wrong DEBUG")
				tflog.Info(ctx, "Something went wrong INFO")
				tflog.Warn(ctx, "Something went wrong WARN")
				tflog.Error(ctx, "Something went wrong ERROR ")
			},
			env: map[string]string{
				"TF_LOG": "DEBUG",
			},
			logs: []log{
				{
					msg: `Something went wrong DEBUG`,
					sev: diag.Debug,
				},
				{
					msg: `Something went wrong INFO`,
					sev: diag.Info,
				},
				{
					msg: `Something went wrong WARN`,
					sev: diag.Warning,
				},
				{
					msg: `Something went wrong ERROR`,
					sev: diag.Error,
				},
			},
		},
		{
			name: "URN propagates when set",
			opts: LogOptions{URN: urn},
			emit: func(ctx context.Context) {
				tflog.Warn(ctx, "OK")
			},
			logs: []log{{sev: diag.Warning, msg: `OK`, urn: urn}},
		},
		{
			name: "Provider propagates when set",
			opts: LogOptions{ProviderName: "random"},
			emit: func(ctx context.Context) {
				tflog.Warn(ctx, "OK")
			},
			logs: []log{{sev: diag.Warning, msg: `provider\=random`}},
		},
		{
			name: "ProviderVersion propagates when set",
			opts: LogOptions{ProviderName: "random", ProviderVersion: "4.12.0"},
			emit: func(ctx context.Context) {
				tflog.Warn(ctx, "OK")
			},
			logs: []log{{sev: diag.Warning, msg: `provider\=random@4.12.0`}},
		},
		{
			name: "User Logging",
			opts: LogOptions{URN: urn},
			emit: func(ctx context.Context) {
				log := getLogger(ctx)
				log.Warn("warn")
				log.Status().Info("info - status")

			},
			logs: []log{
				{
					urn: urn,
					msg: "warn",
					sev: diag.Warning,
				},
				{
					urn:       urn,
					msg:       "info - status",
					sev:       diag.Info,
					ephemeral: true,
				},
			},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			for k, v := range c.env {
				t.Setenv(k, v)
			}

			ctx := context.Background()
			opts := c.opts
			s := &testLogSink{}
			opts.LogSink = s
			ctx = InitLogging(ctx, opts)
			c.emit(ctx)

			require.Equal(t, len(c.logs), len(s.logs))

			for i := range c.logs {
				assert.Equal(t, c.logs[i].sev, s.logs[i].sev)
				assert.Equal(t, c.logs[i].urn, s.logs[i].urn)
				assert.Equal(t, c.logs[i].ephemeral, s.logs[i].ephemeral)
				assert.Regexp(t, c.logs[i].msg, s.logs[i].msg)
			}
		})
	}
}

func TestSetupRootLoggers(t *testing.T) {
	var buf bytes.Buffer
	ctx := setupRootLoggers(context.Background(), &buf)
	tflog.Error(ctx, "Something went wrong")
	assert.Regexp(t, `\[ERROR\] logging/logging_test.go:\d+: provider: Something went wrong\s*$`, buf.String())
}

func TestParseLevelFromRawString(t *testing.T) {
	msg := "2023-03-15T10:52:48.612-0500 [ERROR] provider/resource_integer.go:113: " +
		"provider: Create RandomInteger - ERROR +fields: superfield=supervalue a=1 b=b"
	lev, rest := parseLevelFromRawString(msg)
	require.Equal(t, hclog.Error, lev)
	require.Equal(t, "2023-03-15T10:52:48.612-0500 provider/resource_integer.go:113: "+
		"provider: Create RandomInteger - ERROR +fields: superfield=supervalue a=1 b=b", rest)
}

func TestParseUrnFromRawString(t *testing.T) {
	t.Run("quoted", func(t *testing.T) {
		msg := "2023-03-15T10:52:48.612-0500 [ERROR] provider/resource_integer.go:113: " +
			"provider: Oops: urn=\"some-urn\" value=2"
		urn, rest := parseUrnFromRawString(msg)
		assert.Equal(t, "some-urn", string(urn))
		assert.Equal(t, "2023-03-15T10:52:48.612-0500 [ERROR] provider/resource_integer.go:113: "+
			"provider: Oops: value=2", rest)
	})
	t.Run("bare", func(t *testing.T) {
		msg := "2023-03-15T10:52:48.612-0500 [ERROR] provider/resource_integer.go:113: provider: " +
			"Oops: urn=some-urn value=2"
		urn, rest := parseUrnFromRawString(msg)
		assert.Equal(t, "some-urn", string(urn))
		assert.Equal(t, "2023-03-15T10:52:48.612-0500 [ERROR] provider/resource_integer.go:113: "+
			"provider: Oops: value=2", rest)
	})
}

type log struct {
	sev       diag.Severity
	urn       resource.URN
	msg       string
	ephemeral bool
}

type testLogSink struct {
	logs []log
}

var _ Sink = &testLogSink{}

func (sink *testLogSink) Log(context context.Context, sev diag.Severity, urn resource.URN, msg string) error {
	sink.logs = append(sink.logs, log{
		sev: sev,
		urn: urn,
		msg: msg,
	})
	return nil
}

func (sink *testLogSink) LogStatus(context context.Context, sev diag.Severity, urn resource.URN, msg string) error {
	sink.logs = append(sink.logs, log{
		sev:       sev,
		urn:       urn,
		msg:       msg,
		ephemeral: true,
	})
	return nil
}
