// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dynamic

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	svchost "github.com/hashicorp/terraform-svchost"
	"github.com/opentofu/opentofu/shim"
	tfaddr "github.com/opentofu/registry-address"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	flag "github.com/spf13/pflag"
)

const (
	envPluginCache = "TF_PLUGIN_CACHE_DIR"
)

func MkProviderInfo(name, version string, server provider.Provider) tfbridge.ProviderInfo {
	prov := tfbridge.ProviderInfo{
		P:                 pfbridge.ShimProvider(server),
		Name:              name,
		Version:           "0.0.1",
		TFProviderVersion: version,
		ResourcePrefix:    name,
		MetadataInfo:      tfbridge.NewProviderMetadata(nil),
	}
	prov.MustComputeTokens(tokens.SingleModule(name, "index", tokens.MakeStandard(name)))
	return prov
}

func LoadProviderServer(addr tfaddr.Provider) (tfprotov6.ProviderServer, error) {
	providerCacheDir := os.Getenv(envPluginCache)
	providersMap := shim.NewProviderCache(providerCacheDir)
	//fmt.Printf("cached providers: %v\n", providersMap.AllAvailablePackages())

	providerfactory := providersMap.GetProviderFactory(addr)
	if providerfactory == nil {
		return nil, fmt.Errorf("provider not found in cache: %v\n", addr)
	}

	server, err := providerfactory()
	if err != nil {
		return nil, err
	}
	pServer := NewDynamicServer(server)
	/*
		resp := server.GetProviderSchema()
		fmt.Printf("schema: %v\n", resp)

		resp2, _ := pServer.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
		fmt.Printf("schema again: %v\n", resp2)
	*/

	return pServer, nil
}

func ParseProviderAddr() tfaddr.Provider {
	flags := flag.NewFlagSet("dyanamic_provider", flag.ContinueOnError)
	var providerFlag string
	flags.StringVarP(
		&providerFlag, "provider", "p", "", "tf provider to point at")
	flags.Parse(os.Args)

	addr := tfaddr.Provider{
		Type:      "random",
		Namespace: "hashicorp",
		Hostname:  svchost.Hostname("registry.terraform.io"),
	}

	if providerFlag == "" {
		return addr
	}

	segments := strings.Split(providerFlag, "/")

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
