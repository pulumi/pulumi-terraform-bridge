package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"

	"github.com/pulumi/pulumi-terraform-bridge/v2/internal/testprovider"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: testprovider.ProviderV2,
		Logger: hclog.New(&hclog.LoggerOptions{
			Level: hclog.Trace,
		}),
	})
}
