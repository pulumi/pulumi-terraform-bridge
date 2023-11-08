// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	providerAddr := parseProviderAddr()
	fmt.Printf("addr: %v\n", providerAddr)

	providerCacheDir := os.Getenv(envPluginCache)
	providersMap := shim.NewProviderCache(providerCacheDir)
	fmt.Printf("providers: %v\n", providersMap.AllAvailablePackages())

	providerfactory := providersMap.GetProviderFactory(providerAddr)
	if providerfactory == nil {
		fmt.Printf("provider not found in cache: %v\n", providerAddr)
		os.Exit(1)
	}

	server, err := providerfactory()
	if err != nil {
		panic(err)
	}
	pServer := dynamic.NewDynamicServer(server)

	resp := server.GetProviderSchema()
	fmt.Printf("schema: %v\n", resp)

	resp2, _ := pServer.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	fmt.Printf("schema again: %v\n", resp2)
}

func parseProviderAddr() tfaddr.Provider {
	addr := tfaddr.Provider{
		Type:      "random",
		Namespace: "hashicorp",
		Hostname:  svchost.Hostname("registry.terraform.io"),
	}
	if len(os.Args) < 2 {
		return addr
	}

	segments := strings.Split(os.Args[1], "/")

	if len(segments) > 0 {
		addr.Type = segments[len(segments)-1]
	}
	if len(segments) > 1 {
		addr.Namespace = segments[len(segments)-2]
	}

	if len(segments) > 2 {
		addr.Hostname = svchost.Hostname(segments[len(segments)-3])
	}

	if len(segments) > 3 {
		panic("invalid provider arg")
	}

	return addr
}
