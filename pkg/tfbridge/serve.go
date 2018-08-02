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

	"github.com/pulumi/pulumi/pkg/resource/provider"
	lumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// Serve fires up a Pulumi resource provider listening to inbound gRPC traffic,
// and translates calls from Pulumi into actions against the provided Terraform Provider.
func Serve(module string, version string, info ProviderInfo) error {
	// Create a new resource provider server and listen for and serve incoming connections.
	return provider.Main(module, func(host *provider.HostClient) (lumirpc.ResourceProviderServer, error) {
		// Create a new bridge provider.
		return NewProvider(context.TODO(), host, module, version, info.P, info), nil
	})
}
