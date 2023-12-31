// Copyright 2016-2018, Pulumi Corporation.
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

package tfbridge

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Serve fires up a Pulumi resource provider listening to inbound gRPC traffic,
// and translates calls from Pulumi into actions against the provided Terraform Provider.
func Serve(module string, version string, info ProviderInfo, pulumiSchema []byte) error {
	// Create a new resource provider server and listen for and serve incoming connections.
	return provider.Main(module, func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		if len(info.MuxWith) > 0 {
			// If we have multiple providers to serve, Mux them together.

			var mapping muxer.DispatchTable
			if m, found, err := metadata.Get[muxer.DispatchTable](info.GetMetadata(), "mux"); err != nil {
				return nil, err
			} else if found {
				mapping = m
			} else {
				return nil, fmt.Errorf("missing pre-computed muxer mapping")
			}

			servers := []muxer.Endpoint{{
				Server: func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
					return NewProvider(context.Background(), host, module, version, info.P, info, pulumiSchema), nil
				},
			}}
			for _, f := range info.MuxWith {
				servers = append(servers, muxer.Endpoint{Server: f.GetInstance})
			}

			return muxer.Main{
				Schema:        pulumiSchema,
				DispatchTable: mapping,
				Servers:       servers,
			}.Server(host, module, version)
		}

		// Create a new bridge provider.
		return NewProvider(context.TODO(), host, module, version, info.P, info, pulumiSchema), nil
	})
}
