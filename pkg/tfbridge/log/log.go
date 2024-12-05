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

package log

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
)

// Send logs or status logs to the user.
//
// Logged messages are pre-associated with the resource they are called from.
type Logger interface {
	Log

	// Convert to sending ephemeral status logs to the user.
	Status() Log
}

// The set of logs available to show to the user
type Log interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

// Get access to the [Logger] associated with this context.
func GetLogger(ctx context.Context) Logger {
	logger := ctx.Value(logging.CtxKey)
	contract.Assertf(logger != nil, "Cannot call GetLogger on a context that is not equipped with a Logger")
	return newLoggerAdapter(logger)
}

func newLoggerAdapter(logger any) Logger {
	uLogger, ok := logger.(untypedLogger)
	contract.Assertf(ok, "Context carries a logger that does not implement UntypedLogger")

	return &loggerAdapter{
		Log:     uLogger,
		untyped: uLogger,
	}
}

type loggerAdapter struct {
	Log
	untyped untypedLogger
}

func (a *loggerAdapter) Status() Log {
	return a.untyped.StatusUntyped().(Log)
}

var _ Logger = (*loggerAdapter)(nil)

type untypedLogger interface {
	Log
	StatusUntyped() any
}
