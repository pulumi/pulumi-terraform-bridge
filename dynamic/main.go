// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/opentofu/opentofu/shim"
	"github.com/pulumi/pulumi-terraform-bridge/pf/proto"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

const (
	name    = "terraform-bridge"
	version = "0.0.1"
)

func initialSetup() (tfbridge.ProviderInfo, pfbridge.ProviderMetadata, func() error) {

	var tfServer shim.Provider
	defaultInfo := tfbridge.ProviderInfo{
		DisplayName:  "Any Terraform Provider",
		P:            proto.Empty(),
		Name:         name,
		Version:      version,
		Description:  "A Pulumi provider for dynamically bridging terraform provdiers.",
		MetadataInfo: &tfbridge.MetadataInfo{Path: "", Data: tfbridge.ProviderMetadata(nil)},
	}

	metadata := pfbridge.ProviderMetadata{
		XGetSchema: func(ctx context.Context, req plugin.GetSchemaRequest) ([]byte, error) {

			info := defaultInfo
			if tfServer != nil {
				info = providerInfo(ctx, tfServer)
			}

			packageSchema, err := tfgen.GenerateSchemaWithOptions(tfgen.GenerateSchemaOptions{
				ProviderInfo: info,
				DiagnosticsSink: diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
					Color: colors.Always,
				}),
				XInMemoryDocs: true,
			})
			if err != nil {
				return nil, err
			}
			return json.Marshal(packageSchema.PackageSpec)
		},
		XParamaterize: func(ctx context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
			args, err := parseParamaterizeParameters(req)
			if err != nil {
				return plugin.ParameterizeResponse{}, err
			}

			p, err := getProvider(ctx, args)
			if err != nil {
				return plugin.ParameterizeResponse{}, err
			}

			v, err := semver.Parse(p.Version())
			if err != nil {
				return plugin.ParameterizeResponse{}, err
			}

			tfServer = p
			defaultInfo.P.(*proto.Provider).Replace(p)

			return plugin.ParameterizeResponse{
				Name:    p.Name(),
				Version: &v,
			}, nil
		},
	}

	return defaultInfo, metadata, func() error {
		if tfServer == nil {
			return nil
		}
		return tfServer.Close()
	}
}

func main() {
	ctx := context.Background()

	defaultInfo, metadata, close := initialSetup()

	defer func() {
		if err := close(); err != nil {
			fmt.Printf("Failed to close TF provder: %s", err.Error())
		}
	}()

	pfbridge.Main(ctx, "terraform-bridge", defaultInfo, metadata)
}

func getProvider(ctx context.Context, args paramaterizeArgs) (shim.Provider, error) {
	if args.path != "" {
		return shim.RunLocalProvider(ctx, args.path)
	}

	return shim.LoadProvider(ctx, args.name, args.version)
}
