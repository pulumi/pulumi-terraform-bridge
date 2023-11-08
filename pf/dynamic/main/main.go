// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	svchost "github.com/hashicorp/terraform-svchost"
	"github.com/opentofu/opentofu/shim"
	tfaddr "github.com/opentofu/registry-address"
	"github.com/pulumi/pulumi-terraform-bridge/pf/dynamic"
)

const (
	envPluginCache = "TF_PLUGIN_CACHE_DIR"
)

func main() {
	providerCacheDir := os.Getenv(envPluginCache)
	providersMap := shim.NewProviderCache(providerCacheDir)
	fmt.Printf("providers: %v\n", providersMap.AllAvailablePackages())

	randomProviderfactory := providersMap.GetProviderFactory(tfaddr.Provider{
		Type:      "random",
		Namespace: "hashicorp",
		Hostname:  svchost.Hostname("registry.terraform.io"),
	})

	server, err := randomProviderfactory()
	if err != nil {
		panic(err)
	}
	pServer := dynamic.NewDynamicServer(server)

	resp := server.GetProviderSchema()
	fmt.Printf("schema: %v\n", resp)

	resp2, _ := pServer.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	fmt.Printf("schema again: %v\n", resp2)
}
