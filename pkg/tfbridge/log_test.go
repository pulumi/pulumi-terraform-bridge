// Copyright 2016-2018, Pulumi Corporation.
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

	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

// Ensure that logging redirects to the right place.
func TestLogRedirector(t *testing.T) {
	t.Parallel()
	lines := []string{
		"no prefix #1\n",
		"[TRACE] trace line #1\n",
		"[TRACE] trace line #2\n",
		"no prefix #2\n",
		"[DEBUG] debug line #1\n",
		"[DEBUG] debug line #2\n",
		"[INFO] info line #1\n",
		"no prefix #3\n",
		"[INFO] info line #2\n",
		"[WARN] warning line #1\n",
		"[WARN] warning line #2\n",
		"[ERROR] error line #1\n",
		"[ERROR] error line #2\n",
		"no prefix #4\n",
		"[TRACE] trace line #3\n",
		"[DEBUG] debug line #3\n",
		"[INFO] info line #3\n",
		"[WARN] warning line #3\n",
		"[ERROR] error line #3\n",
		"no prefix #5\n",
	}

	testCases := []struct {
		tfLog  level
		expect autogold.Value
	}{
		{noLevel, autogold.Expect(`[debug] [] no prefix #1
[debug] [] [TRACE] trace line #1
[debug] [] [TRACE] trace line #2
[debug] [] no prefix #2
[debug] [] [DEBUG] debug line #1
[debug] [] [DEBUG] debug line #2
[debug] [] [INFO] info line #1
[debug] [] no prefix #3
[debug] [] [INFO] info line #2
[debug] [] [WARN] warning line #1
[debug] [] [WARN] warning line #2
[debug] [] [ERROR] error line #1
[debug] [] [ERROR] error line #2
[debug] [] no prefix #4
[debug] [] [TRACE] trace line #3
[debug] [] [DEBUG] debug line #3
[debug] [] [INFO] info line #3
[debug] [] [WARN] warning line #3
[debug] [] [ERROR] error line #3
[debug] [] no prefix #5
`)},
		{traceLevel, autogold.Expect(`[debug] [] no prefix #1
[debug] [] [TRACE] trace line #1
[debug] [] [TRACE] trace line #2
[debug] [] no prefix #2
[debug] [] [DEBUG] debug line #1
[debug] [] [DEBUG] debug line #2
[info] [] info line #1
[debug] [] no prefix #3
[info] [] info line #2
[warning] [] warning line #1
[warning] [] warning line #2
[error] [] error line #1
[error] [] error line #2
[debug] [] no prefix #4
[debug] [] [TRACE] trace line #3
[debug] [] [DEBUG] debug line #3
[info] [] info line #3
[warning] [] warning line #3
[error] [] error line #3
[debug] [] no prefix #5
`)},
		{debugLevel, autogold.Expect(`[debug] [] no prefix #1
[debug] [] [TRACE] trace line #1
[debug] [] [TRACE] trace line #2
[debug] [] no prefix #2
[debug] [] [DEBUG] debug line #1
[debug] [] [DEBUG] debug line #2
[info] [] info line #1
[debug] [] no prefix #3
[info] [] info line #2
[warning] [] warning line #1
[warning] [] warning line #2
[error] [] error line #1
[error] [] error line #2
[debug] [] no prefix #4
[debug] [] [TRACE] trace line #3
[debug] [] [DEBUG] debug line #3
[info] [] info line #3
[warning] [] warning line #3
[error] [] error line #3
[debug] [] no prefix #5
`)},
		{infoLevel, autogold.Expect(`[debug] [] no prefix #1
[debug] [] [TRACE] trace line #1
[debug] [] [TRACE] trace line #2
[debug] [] no prefix #2
[debug] [] [DEBUG] debug line #1
[debug] [] [DEBUG] debug line #2
[info] [] info line #1
[debug] [] no prefix #3
[info] [] info line #2
[warning] [] warning line #1
[warning] [] warning line #2
[error] [] error line #1
[error] [] error line #2
[debug] [] no prefix #4
[debug] [] [TRACE] trace line #3
[debug] [] [DEBUG] debug line #3
[info] [] info line #3
[warning] [] warning line #3
[error] [] error line #3
[debug] [] no prefix #5
`)},
		{warnLevel, autogold.Expect(`[debug] [] no prefix #1
[debug] [] [TRACE] trace line #1
[debug] [] [TRACE] trace line #2
[debug] [] no prefix #2
[debug] [] [DEBUG] debug line #1
[debug] [] [DEBUG] debug line #2
[debug] [] [INFO] info line #1
[debug] [] no prefix #3
[debug] [] [INFO] info line #2
[warning] [] warning line #1
[warning] [] warning line #2
[error] [] error line #1
[error] [] error line #2
[debug] [] no prefix #4
[debug] [] [TRACE] trace line #3
[debug] [] [DEBUG] debug line #3
[debug] [] [INFO] info line #3
[warning] [] warning line #3
[error] [] error line #3
[debug] [] no prefix #5
`)},
		{errorLevel, autogold.Expect(`[debug] [] no prefix #1
[debug] [] [TRACE] trace line #1
[debug] [] [TRACE] trace line #2
[debug] [] no prefix #2
[debug] [] [DEBUG] debug line #1
[debug] [] [DEBUG] debug line #2
[debug] [] [INFO] info line #1
[debug] [] no prefix #3
[debug] [] [INFO] info line #2
[debug] [] [WARN] warning line #1
[debug] [] [WARN] warning line #2
[error] [] error line #1
[error] [] error line #2
[debug] [] no prefix #4
[debug] [] [TRACE] trace line #3
[debug] [] [DEBUG] debug line #3
[debug] [] [INFO] info line #3
[debug] [] [WARN] warning line #3
[error] [] error line #3
[debug] [] no prefix #5
`)},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("TF_LOG=%v", tc.tfLog), func(t *testing.T) {
			ld := &LogRedirector{
				level: tc.tfLog,
				sink:  &testLogSink{buf: &bytes.Buffer{}},
			}

			// For each line, spit 16 byte increments into the redirector.
			for _, line := range lines {
				for len(line) > 0 {
					sz := 16
					if sz > len(line) {
						sz = len(line)
					}
					n, err := ld.Write([]byte(line[:sz]))
					assert.Nil(t, err)
					assert.Equal(t, n, sz)
					line = line[sz:]
				}
			}

			got := ld.sink.(*testLogSink).buf.String()
			tc.expect.Equal(t, got)
		})
	}
}

// Check if framework logs emitted by SDKv2 based resources actually are captured by Pulumi.
func TestLogCapture(t *testing.T) {
	t.Setenv("TF_LOG", "WARN")
	ctx := context.Background()
	var logs bytes.Buffer

	ctx = logging.InitLogging(ctx, logging.LogOptions{
		LogSink: &testLogSink{&logs},
	})

	p := testprovider.ProviderV2()
	provider := &Provider{
		tf:     shimv2.NewProvider(p),
		config: shimv2.NewSchemaMap(p.Schema),
	}

	_, err := provider.Configure(ctx, &pulumirpc.ConfigureRequest{})
	assert.NoError(t, err)

	_, err = provider.Configure(ctx, &pulumirpc.ConfigureRequest{})
	assert.NoError(t, err)

	// Calling Configure twice actually emits a warning from the framework.
	assert.Contains(t, logs.String(), "Previously configured provider being re-configured.")
}

type testLogSink struct {
	buf *bytes.Buffer
}

var _ logging.Sink = &testLogSink{}

func (s *testLogSink) Log(context context.Context, sev diag.Severity, urn resource.URN, msg string) error {
	fmt.Fprintf(s.buf, "[%v] [%v] %s\n", sev, urn, msg)
	return nil
}

func (s *testLogSink) LogStatus(context context.Context, sev diag.Severity, urn resource.URN, msg string) error {
	fmt.Fprintf(s.buf, "[status] [%v] [%v] %s\n", sev, urn, msg)
	return nil
}
