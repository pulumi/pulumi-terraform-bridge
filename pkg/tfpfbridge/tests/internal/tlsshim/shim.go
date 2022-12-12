package shim

import (
	framework "github.com/hashicorp/terraform-plugin-framework/provider"
	provider "github.com/hashicorp/terraform-provider-tls/internal/provider"
)

func NewProvider() framework.Provider {
	return provider.New()
}
