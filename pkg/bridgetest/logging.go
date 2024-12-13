package bridgetest

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
