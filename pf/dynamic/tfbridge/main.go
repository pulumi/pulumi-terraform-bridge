package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/pulumi/pulumi-terraform-bridge/pf/dynamic"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	rprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func main() {
	providerAddr := dynamic.ProviderAddrFromFilename()
	//fmt.Printf("addr: %v\n", providerAddr)
	pServer, err := dynamic.LoadProviderServer(providerAddr)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	name, version := providerAddr.Type, ""
	shimProvider := pfutils.SchemaOnlyProvider(name, version, pServer)
	info := dynamic.MkProviderInfo(name, "", shimProvider)
	//fmt.Printf("info: %v\n", info)
	//fmt.Printf("infoResources: %v\n", info.Resources["random_random_pet"])

	packageSpec, err := tfgen.GenerateSchema(info, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	bytes, err := json.MarshalIndent(packageSpec, "", "    ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	meta := pfbridge.ProviderMetadata{
		PackageSchema: bytes,
	}

	serve(context.Background(), name, info, meta, pServer)
}

func serve(ctx context.Context, pkg string, prov tfbridge.ProviderInfo, meta pfbridge.ProviderMetadata, server tfprotov6.ProviderServer) error {
	return rprovider.Main(pkg, func(host *rprovider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return pfbridge.NewProviderServer2(ctx, host, prov, meta, server)
	})
}
