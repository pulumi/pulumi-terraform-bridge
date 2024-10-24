package tlsshim

import (
	_ "unsafe" // For go:linkname.

	"github.com/hashicorp/terraform-plugin-framework/provider"
	framework "github.com/hashicorp/terraform-plugin-framework/provider"
)

//go:linkname New github.com/terraform-providers/terraform-provider-tls/internal/provider.New
var New func() provider.Provider

func NewProvider() framework.Provider {
	return New()
}
