// Copyright 2016-2023, Pulumi Corporation.
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

package tfgen

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/check"
	pfmuxer "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/muxer"
	sdkBridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
)

// Implements main() logic for a provider build-time helper utility. By convention these utilities are named
// pulumi-tfgen-$provider, for example when building a "random" provider the program would be called
// pulumi-tfgen-random.
//
// The resulting binary is able to generate [Pulumi Package Schema] as well as provider SDK sources in various
// programming languages supported by Pulumi such as TypeScript, Go, and Python.
//
// Before generating, Main runs bridge PF checks and Terraform Plugin Framework
// ValidateImplementation checks for the generated provider, resource, data
// source, and list resource schemas. Framework validation failures are reported
// as build-time errors so static providers do not need to call full
// GetProviderSchema during runtime startup.
//
// info.P must be constructed with ShimProvider or ShimProviderWithContext so
// the validation step can reach the original Framework provider.
//
// [Pulumi Package Schema]: https://www.pulumi.com/docs/guides/pulumi-packages/schema/
func Main(provider string, info sdkBridge.ProviderInfo) {
	version := info.Version

	tfgen.MainWithCustomGenerate(provider, version, info, func(opts tfgen.GeneratorOptions) error {
		if info.MetadataInfo == nil {
			return fmt.Errorf("ProviderInfo.MetadataInfo is required and cannot be nil")
		}

		g, err := tfgen.NewGenerator(opts)
		if err != nil {
			return err
		}

		if err := check.Provider(nil, g.Sink(), info); err != nil {
			return err
		}
		_, err = g.Generate(context.Background())

		return err
	})
}

// Implements main() logic for a multi-provider build-time helper utility. By convention these utilities are
// named pulumi-tfgen-$provider, for example when building a "random" provider the program would be called
// pulumi-tfgen-random.
//
// The resulting binary is able to generate [Pulumi Package Schema] as well as provider SDK sources in various
// programming languages supported by Pulumi such as TypeScript, Go, and Python.
//
// Before generating, MainWithMuxer runs bridge PF checks and Terraform Plugin
// Framework ValidateImplementation checks for generated PF provider, resource,
// data source, and list resource schemas in the muxed provider. Framework
// validation failures are reported as build-time errors so SDKv2-only runtime
// operations do not need to materialize the PF provider schema at startup.
// info.P must be constructed with a PF mux helper so the validation step can
// reach the original Framework provider.
//
// This is an experimental API.
//
// [Pulumi Package Schema]: https://www.pulumi.com/docs/guides/pulumi-packages/schema/
func MainWithMuxer(provider string, info sdkBridge.ProviderInfo) {
	if len(info.MuxWith) > 0 {
		panic("mixin providers via tfbridge.ProviderInfo.MuxWith is currently not supported")
	}

	shim, ok := info.P.(*pfmuxer.ProviderShim)
	contract.Assertf(ok, "MainWithMuxer must have a ProviderInfo.P created with AugmentShimWithPF")

	// Validate any sdk providers that are being muxed in.
	for _, prov := range shim.MuxedProviders {
		err := prov.InternalValidate()
		if err != nil {
			_, fmterr := fmt.Fprintf(os.Stderr, "Internal validation of the provider failed: %v\n", err)
			contract.IgnoreError(fmterr)
			os.Exit(-1)
		}
	}

	tfgen.MainWithCustomGenerate(provider, info.Version, info, func(opts tfgen.GeneratorOptions) error {
		g, err := tfgen.NewGenerator(opts)
		if err != nil {
			return err
		}

		if err := check.Provider(nil, g.Sink(), info); err != nil {
			return err
		}

		if info.MetadataInfo == nil {
			return fmt.Errorf("ProviderInfo.MetadataInfo is required and cannot be nil")
		}

		dispatch, err := shim.ResolveDispatch(&info)
		if err != nil {
			return fmt.Errorf("failed to compute dispatch for muxed provider: %w", err)
		}
		err = metadata.Set(info.GetMetadata(), "mux", dispatch)
		if err != nil {
			return err
		}

		_, err = g.Generate(context.Background())
		return err
	})
}
