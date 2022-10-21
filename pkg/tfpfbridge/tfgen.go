package tfbridge

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfpfbridge/info"
)

// Used to implement main() in programs such as pulumi-tfgen-random.
func Generate(provider, version string, info info.ProviderInfo) {
}
