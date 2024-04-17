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

package info

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// MuxProvider defines an interface which must be implemented by providers
// that shall be used as mixins of a wrapped Terraform provider
type MuxProvider interface {
	GetSpec(ctx context.Context,
		name, version string) (schema.PackageSpec, error)
	GetInstance(ctx context.Context,
		name, version string,
		host *provider.HostClient) (pulumirpc.ResourceProviderServer, error)
}
