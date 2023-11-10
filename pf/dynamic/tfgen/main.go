// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-terraform-bridge/pf/dynamic"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	pfgen "github.com/pulumi/pulumi-terraform-bridge/pf/tfgen"
)

const (
	envPluginCache = "TF_PLUGIN_CACHE_DIR"
)

func main() {
	providerAddr := dynamic.ParseProviderAddr()
	fmt.Printf("addr: %v\n", providerAddr)
	pServer, err := dynamic.LoadProviderServer(providerAddr)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	name, version := providerAddr.Type, ""
	shimProvider := pfutils.SchemaOnlyProvider(name, version, pServer)
	info := dynamic.MkProviderInfo(providerAddr.Type, "", shimProvider)
	pfgen.Main(providerAddr.Type, info)

}
