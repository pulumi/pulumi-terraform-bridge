// Code copied from github.com/opentofu/opentofu by go generate; DO NOT EDIT.
// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2023 HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package getproviders

import (
	"context"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/addrs"
)

// A Source can query a particular source for information about providers
// that are available to install.
type Source interface {
	AvailableVersions(ctx context.Context, provider addrs.Provider) (VersionList, Warnings, error)
	PackageMeta(ctx context.Context, provider addrs.Provider, version Version, target Platform) (PackageMeta, error)
	ForDisplay(provider addrs.Provider) string
}
