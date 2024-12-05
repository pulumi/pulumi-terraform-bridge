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
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
)

// LogRedirector creates a new redirection writer that takes [log] messages as input, and
// routes it to the Pulumi correct structured logging.
//
// It looks for lines that contain log level information and routes them to their Pulumi
// equivalent. A line is considered to have log level information when it contains one of:
//
// - "[TRACE]"
// - "[DEBUG]"
// - "[INFO]"
// - "[WARN]"
// - "[ERROR]"
type LogRedirector struct {
	ctx    context.Context
	level  level        // log level requested by TF_LOG
	sink   logging.Sink // the sink to write to
	buffer []byte       // a buffer that holds up to a line of output.
}

// Level represents a log level requested by TF_LOG.
type level int32

func (l level) String() string {
	switch l {
	case traceLevel:
		return "trace"
	case debugLevel:
		return "debug"
	case infoLevel:
		return "info"
	case warnLevel:
		return "warn"
	case errorLevel:
		return "error"
	default:
		return "unset"
	}
}

const (
	noLevel    level = 0
	traceLevel level = 1
	debugLevel level = 2
	infoLevel  level = 3
	warnLevel  level = 4
	errorLevel level = 5
)

const (
	tfTracePrefix = "[TRACE]"
	tfDebugPrefix = "[DEBUG]"
	tfInfoPrefix  = "[INFO]"
	tfWarnPrefix  = "[WARN]"
	tfErrorPrefix = "[ERROR]"
)

// NewTerraformLogRedirector creates a [LogRedirector] that responds to the TF_LOG
// environmental variable.
func NewTerraformLogRedirector(ctx context.Context, sink logging.Sink) *LogRedirector {
	lr := &LogRedirector{ctx: ctx, sink: sink}

	if tfLog, ok := os.LookupEnv("TF_LOG"); ok {
		switch strings.ToLower(tfLog) {
		case "trace":
			lr.level = traceLevel
		case "debug":
			lr.level = debugLevel
		case "info":
			lr.level = infoLevel
		case "warn":
			lr.level = warnLevel
		case "error":
			lr.level = errorLevel
		}
	}

	return lr
}

// Deprecated: this function is not in use and will be removed.
func (lr *LogRedirector) Enable() {}

// Deprecated: this function is not in use and will be removed.
func (lr *LogRedirector) Disable() {}

func (lr *LogRedirector) handleLogMessage(label string, msg string) {
	// Only forward lines that start with [ERROR], [WARN] or [INFO] to the sink if explicit logging was requested
	// via TF_LOG at the appropriate level. Pulumi CLI will show these messages to the user.
	if lr.level > 0 {
		switch {
		case label == tfInfoPrefix && lr.level <= infoLevel:
			err := lr.sink.Log(lr.ctx, diag.Info, "", msg)
			contract.IgnoreError(err)
			return
		case label == tfWarnPrefix && lr.level <= warnLevel:
			err := lr.sink.Log(lr.ctx, diag.Warning, "", msg)
			contract.IgnoreError(err)
			return
		case label == tfErrorPrefix && lr.level <= errorLevel:
			err := lr.sink.Log(lr.ctx, diag.Error, "", msg)
			contract.IgnoreError(err)
			return
		}
	}
	// In all other cases, forward the message to the debug sink, re-attaching the label to make it easy to filter
	// the messages from the log files by label.
	if label != "" {
		msg = fmt.Sprintf("%s %s", label, msg)
	}
	err := lr.sink.Log(lr.ctx, diag.Debug, "", msg)
	contract.IgnoreError(err)
}

// Implement io.Writer, parse lines, parse [TRACE], [DEBUG], [INFO], [WARN], and [ERROR] prefixes, and route.
func (lr *LogRedirector) Write(p []byte) (n int, err error) {
	written := 0

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

		lr.handleLogMessage(label, s)

		// Now keep moving on provided there is more left in the buffer.
		lr.buffer = lr.buffer[:0] // clear out the buffer.
		p = p[adv:]               // advance beyond the extracted region.
		written += adv
	}

	return written, nil
}
