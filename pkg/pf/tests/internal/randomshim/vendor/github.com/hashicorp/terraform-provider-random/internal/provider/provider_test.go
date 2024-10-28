// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

//nolint:unparam
func protoV5ProviderFactories() map[string]func() (tfprotov5.ProviderServer, error) {
	return map[string]func() (tfprotov5.ProviderServer, error){
		"random": providerserver.NewProtocol5WithError(New()),
	}
}

func providerVersion221() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"random": {
			VersionConstraint: "2.2.1",
			Source:            "hashicorp/random",
		},
	}
}

func providerVersion313() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"random": {
			VersionConstraint: "3.1.3",
			Source:            "hashicorp/random",
		},
	}
}

func providerVersion320() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"random": {
			VersionConstraint: "3.2.0",
			Source:            "hashicorp/random",
		},
	}
}

func providerVersion332() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"random": {
			VersionConstraint: "3.3.2",
			Source:            "hashicorp/random",
		},
	}
}

func providerVersion342() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"random": {
			VersionConstraint: "3.4.2",
			Source:            "hashicorp/random",
		},
	}
}
