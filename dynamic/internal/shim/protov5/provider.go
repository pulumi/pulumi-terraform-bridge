package protov5

import (
	"github.com/opentofu/opentofu/internal/tfplugin5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

func New(tfplugin5.ProviderClient) tfprotov5.ProviderServer {
	panic("Unimplimented")
}
