package getproviders

import (
	"context"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/internal/tf/addrs"
)

// HangingSource is an implementation of Source which hangs until the given
// context is cancelled. This is useful only for unit tests of user-controlled
// cancels.
type HangingSource struct {
}

var _ Source = (*HangingSource)(nil)

func (s *HangingSource) AvailableVersions(ctx context.Context, provider addrs.Provider) (VersionList, Warnings, error) {
	<-ctx.Done()
	return nil, nil, nil
}

func (s *HangingSource) PackageMeta(ctx context.Context, provider addrs.Provider, version Version, target Platform) (PackageMeta, error) {
	<-ctx.Done()
	return PackageMeta{}, nil
}

func (s *HangingSource) ForDisplay(provider addrs.Provider) string {
	return "hanging source"
}
