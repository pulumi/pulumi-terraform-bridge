// Code copied from github.com/opentofu/opentofu by go generate; DO NOT EDIT.
// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2023 HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"github.com/hashicorp/go-plugin"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/plugin6"
)

// VersionedPlugins includes both protocol 5 and 6 because this is the function
// called in providerFactory (command/meta_providers.go) to set up the initial
// plugin client config.
var VersionedPlugins = map[int]plugin.PluginSet{
	5: {
		"provider": &GRPCProviderPlugin{},
	},
	6: {
		"provider": &plugin6.GRPCProviderPlugin{},
	},
}
