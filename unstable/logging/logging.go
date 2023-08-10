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
	"fmt"
)

import (
	"context"

	"github.com/golang/glog"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

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
type Host[L any] struct {
	ctx  context.Context
	sink Sink
	urn  resource.URN

	// If we are logging ephemerally.
	status bool

	// This is the identity function, but must be defined at the call-site because we
	// cannot name [tfbridge.Log].
	mkStatus func(*Host[L]) L
}

func NewHost[L any](ctx context.Context,
	sink Sink,
	urn resource.URN,
	mkStatus func(*Host[L]) L,
) *Host[L] {
	// This nil check catches half-nil fat pointers from casting
	// `*provider.HostClient` to `Sink`.
	if host, ok := sink.(*provider.HostClient); ok && host == nil {
		sink = nil
	}
	return &Host[L]{ctx, sink, urn, false /*status*/, mkStatus}
}

func (l *Host[L]) f(severity diag.Severity, msg string) {
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

func (l *Host[L]) Status() L {
	copy := *l
	copy.status = true
	return l.mkStatus(&copy)
}

func (l *Host[L]) Debug(msg string) { l.f(diag.Debug, msg) }
func (l *Host[L]) Info(msg string)  { l.f(diag.Info, msg) }
func (l *Host[L]) Warn(msg string)  { l.f(diag.Warning, msg) }
func (l *Host[L]) Error(msg string) { l.f(diag.Error, msg) }
