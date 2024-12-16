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

package testutil

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
)

// A sink that [tfbridge.GetLogger] can write to.
//
// This API is experimental and may change or be removed during minor releases.
type LoggingSink interface {
	Log(context context.Context, sev diag.Severity, urn resource.URN, msg string) error
	LogStatus(context context.Context, sev diag.Severity, urn resource.URN, msg string) error
}

type discardSink struct{}

func (*discardSink) Log(context.Context, diag.Severity, resource.URN, string) error {
	return nil
}

func (*discardSink) LogStatus(context.Context, diag.Severity, resource.URN, string) error {
	return nil
}

// InitLogging equips ctx with a logger usable by [tfbridge.GetLogger].
//
// This API is experimental and may change or be removed during minor releases.
//
//nolint:revive // Let t come before ctx.
func InitLogging(t *testing.T, ctx context.Context, sink LoggingSink) context.Context {
	contract.Assertf(t != nil, "t cannot be nil")
	if sink == nil {
		sink = &discardSink{}
	}
	return logging.InitLogging(ctx, logging.LogOptions{LogSink: sink})
}
