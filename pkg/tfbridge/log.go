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
	"bufio"
	"context"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// LogRedirector creates a new redirection writer that takes as input plugin stderr output, and routes it to the
// correct Pulumi stream based on the standard Terraform logging output prefixes.
type LogRedirector struct {
	enabled bool                          // true if standard logging is on; false for debug-only.
	writers map[string]func(string) error // the writers for certain labels.
	buffer  []byte                        // a buffer that holds up to a line of output.
}

// NewLogRedirector returns a new LogRedirector with the (unexported) writers field
// set to the given map.
func NewTerraformLogRedirector(ctx context.Context, hostClient *provider.HostClient) *LogRedirector {
	return &LogRedirector{
		writers: map[string]func(string) error{
			tfTracePrefix: func(msg string) error { return hostClient.Log(ctx, diag.Debug, "", msg) },
			tfDebugPrefix: func(msg string) error { return hostClient.Log(ctx, diag.Debug, "", msg) },
			tfInfoPrefix:  func(msg string) error { return hostClient.Log(ctx, diag.Info, "", msg) },
			tfWarnPrefix:  func(msg string) error { return hostClient.Log(ctx, diag.Warning, "", msg) },
			tfErrorPrefix: func(msg string) error { return hostClient.Log(ctx, diag.Error, "", msg) },
		},
	}
}

const (
	tfTracePrefix = "[TRACE]"
	tfDebugPrefix = "[DEBUG]"
	tfInfoPrefix  = "[INFO]"
	tfWarnPrefix  = "[WARN]"
	tfErrorPrefix = "[ERROR]"
)

// Enable turns on full featured logging.  This is the default.
func (lr *LogRedirector) Enable() {
	lr.enabled = true
}

// Disable disables most of the specific logging levels, but it retains debug logging.
func (lr *LogRedirector) Disable() {
	lr.enabled = false
}

func (lr *LogRedirector) Write(p []byte) (n int, err error) {
	written := 0

	// If a line starts with [TRACE], [DEBUG], or [INFO], then we emit to a debug log entry.  If a line starts with
	// [WARN], we emit a warning.  If a line starts with [ERROR], on the other hand, we emit a normal stderr line.
	// All others simply get redirected to stdout as normal output.
	for len(p) > 0 {
		adv, tok, err := bufio.ScanLines(p, false)
		if err != nil {
			return written, err
		}

		// If adv == 0, there was no newline; buffer it all and move on.
		if adv == 0 {
			lr.buffer = append(lr.buffer, p...)
			written += len(p)
			break
		}

		// Otherwise, there was a newline; emit the buffer plus payload to the right place, and keep going if
		// there is more.
		lr.buffer = append(lr.buffer, tok...) // append the buffer.
		s := string(lr.buffer)

		// To do this we need to parse the label if there is one (e.g., [TRACE], et al).
		var label string
		if start := strings.IndexRune(s, '['); start != -1 {
			if end := strings.Index(s[start:], "] "); end != -1 {
				label = s[start : start+end+1]
				s = s[start+end+2:] // skip past the "] " (notice the space)
			}
		}
		w, has := lr.writers[label]
		if !has || !lr.enabled {
			// If there was no writer for this label, or logging is disabled, use the debug label.
			w = lr.writers[tfDebugPrefix]
			contract.Assert(w != nil)
		}
		if err := w(s); err != nil {
			return written, err
		}

		// Now keep moving on provided there is more left in the buffer.
		lr.buffer = lr.buffer[:0] // clear out the buffer.
		p = p[adv:]               // advance beyond the extracted region.
		written += adv
	}

	return written, nil
}
