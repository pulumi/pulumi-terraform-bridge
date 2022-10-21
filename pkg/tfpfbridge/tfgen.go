package tfbridge

import (
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
)

// Used to implement main() in programs such as pulumi-tfgen-random.
func Generate(provider, version string, info info.ProviderInfo) {
}
