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

package tfbridge

import (
	"context"
	"encoding/json"
	"io"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	rprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

// Adapts a provider to be usable as a mix-in, see [info.Provider.MuxWith].
//
// info.P must be constructed with ShimProvider or ShimProviderWithContext.
func NewMuxProvider(ctx context.Context, providerInfo info.Provider, meta *ProviderMetadata) (info.MuxProvider, error) {
	// If schema is not pre-built, generate the schema on the fly.
	if meta == nil || meta.PackageSchema == nil {
		noSink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
		spec, err := tfgen.GenerateSchema(providerInfo, noSink)
		if err != nil {
			return nil, err
		}
		bytes, err := json.Marshal(spec)
		if err != nil {
			return nil, err
		}
		meta = &ProviderMetadata{PackageSchema: bytes}
	}
	return &pfProviderAsMuxProvider{
		providerInfo: providerInfo,
		meta:         *meta,
	}, nil
}

type pfProviderAsMuxProvider struct {
	providerInfo info.Provider
	meta         ProviderMetadata
}

func (p *pfProviderAsMuxProvider) GetSpec(_ context.Context, _, _ string) (schema.PackageSpec, error) {
	return p.getSpec()
}

func (p *pfProviderAsMuxProvider) getSpec() (schema.PackageSpec, error) {
	var res schema.PackageSpec
	if err := json.Unmarshal(p.meta.PackageSchema, &res); err != nil {
		return schema.PackageSpec{}, nil
	}
	return res, nil
}

func (p *pfProviderAsMuxProvider) GetInstance(
	ctx context.Context,
	name, version string,
	host *rprovider.HostClient,
) (pulumirpc.ResourceProviderServer, error) {
	return NewProviderServer(ctx, host, p.providerInfo, p.meta)
}

var _ info.MuxProvider = (*pfProviderAsMuxProvider)(nil)
