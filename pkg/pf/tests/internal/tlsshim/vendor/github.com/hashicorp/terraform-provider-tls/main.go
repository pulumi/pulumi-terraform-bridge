package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-provider-tls/internal/provider"
)

// Generate the Terraform provider documentation using `tfplugindocs`:
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address:         "registry.terraform.io/hashicorp/tls",
		Debug:           debug,
		ProtocolVersion: 5,
	})
	if err != nil {
		fmt.Printf("failed to initialize provider: %v\n", err)
		os.Exit(1)
	}
}
