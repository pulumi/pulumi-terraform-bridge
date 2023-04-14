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

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	sdkBridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
)

// Implements main() logic for a provider build-time helper utility. By convention these utilities are named
// pulumi-tfgen-$provider, for example when building a "random" provider the program would be called
// pulumi-tfgen-random.
//
// The resulting binary is able to generate [Pulumi Package Schema] as well as provider SDK sources in various
// programming languages supported by Pulumi such as TypeScript, Go, and Python.
//
// [Pulumi Package Schema]: https://www.pulumi.com/docs/guides/pulumi-packages/schema/
func Main(provider string, info tfbridge.ProviderInfo) {
	version := info.Version
	ctx := context.Background()
	shimInfo := shimSchemaOnlyProviderInfo(ctx, info)

	tfgen.MainWithCustomGenerate(provider, version, shimInfo, func(opts tfgen.GeneratorOptions) error {

		if info.MetadataInfo == nil {
			return fmt.Errorf("ProviderInfo.MetadataInfo is required and cannot be nil")
		}

		if err := notSupported(opts.Sink, info.ProviderInfo); err != nil {
			return err
		}

		g, err := tfgen.NewGenerator(opts)
		if err != nil {
			return err
		}

		if err := g.Generate(); err != nil {
			return err
		}

		return nil
	})
}

// Implements main() logic for a multi-provider build-time helper utility. By convention these utilities are
// named pulumi-tfgen-$provider, for example when building a "random" provider the program would be called
// pulumi-tfgen-random.
//
// The resulting binary is able to generate [Pulumi Package Schema] as well as provider SDK sources in various
// programming languages supported by Pulumi such as TypeScript, Go, and Python.
//
// This is an experimental API.
//
// [Pulumi Package Schema]: https://www.pulumi.com/docs/guides/pulumi-packages/schema/
func MainWithMuxer(provider string, infos ...tfbridge.Muxed) {
	ctx := context.Background()

	// It is safe to pass infos[0].GetInfo() to MainWithCusomGenerate because it is
	// only used when passed to GenerateOptions.

	gen := func(opts tfgen.GeneratorOptions) error {
		if opts.Sink == nil {
			opts.Sink = cmdutil.Diag()
		}
		muxedInfo, mapping, schema, mergedRenames, err :=
			UnstableMuxProviderInfo(ctx, opts, provider, infos...)
		if err != nil {
			return err
		}

		// TODO Be more careful when assembling the final metadata
		// Right now, only the top layer will be persisted.
		muxedInfo.MetadataInfo = infos[0].GetInfo().MetadataInfo
		contract.Assertf(muxedInfo.MetadataInfo != nil,
			"Must provide a metadata store when muxing providers. See ProviderInfo.MetadataInfo")
		err = metadata.Set(muxedInfo.GetMetadata(), "mux", mapping)
		if err != nil {
			return fmt.Errorf("[pf/tfgen] Failed to set mux data in MetadataInfo: %w", err)
		}

		// In the muxing case precompute renames by merging them and set them to MetadataInfo, this will avoid
		// recomputing the renames in GenerateFromSchema, it will write out the mergedRenames as-is.
		if err := metadata.Set(muxedInfo.GetMetadata(), "renames", mergedRenames); err != nil {
			return fmt.Errorf("[pf/tfgen]: Failed to set renames data in MetadataInfo: %w", err)
		}

		// Having computed the schema, we now want to complete the tfgen process,
		// reusing as much of the standard process as possible.
		opts.ProviderInfo = muxedInfo
		g, err := tfgen.NewGenerator(opts)
		if err != nil {
			return err
		}

		return g.UnstableGenerateFromSchema(&tfgen.GenerateSchemaResult{
			PackageSpec: schema,
			Renames:     mergedRenames,
		})
	}

	tfgen.MainWithCustomGenerate(provider, infos[0].GetInfo().Version, infos[0].GetInfo(), gen)
}

// Mux multiple infos into a unified representation.
//
// This function exposes implementation details for testing. It should not be used outside
// of pulumi-terraform-bridge.  This is an experimental API.
func UnstableMuxProviderInfo(
	ctx context.Context, opts tfgen.GeneratorOptions,
	provider string, infos ...tfbridge.Muxed,
) (sdkBridge.ProviderInfo, muxer.DispatchTable, schema.PackageSpec, tfgen.Renames, error) {
	if opts.Sink == nil {
		opts.Sink = cmdutil.Diag()
	}
	muxedInfo := infos[0].GetInfo()
	schemas := []schema.PackageSpec{}
	var errs multierror.Error

	var renames []tfgen.Renames

	for i, union := range infos {
		i, union := i, union

		anErr := func(err error) bool {
			if err == nil {
				return false
			}
			errs.Errors = append(errs.Errors, fmt.Errorf("Muxed proider[%d]: %w", i, err))
			return true
		}

		info := union.SDK
		if pfInfo := union.PF; pfInfo != nil {
			if err := notSupported(opts.Sink, pfInfo.ProviderInfo); anErr(err) {
				continue
			}

			shimInfo := shimSchemaOnlyProviderInfo(ctx, *pfInfo)
			info = &shimInfo
		}

		// Initially, muxedInfo **is** infos[0]. We don't want to perform
		// any destructive edits on already defined functions.
		if i != 0 {
			// Now we purge already defined items from the next schema,
			// marking the all remaining items as defined for future schemas.

			muxedInfo.IgnoreMappings = append(muxedInfo.IgnoreMappings,
				info.IgnoreMappings...)

			// Resources
			layerUnder(&muxedInfo.Resources, info.Resources, muxedInfo.ExtraResources,
				func(k string) {
					delete(info.Resources, k)
					info.IgnoreMappings = append(info.IgnoreMappings, k)
				})

			layerUnder(&muxedInfo.ExtraResources, info.ExtraResources, muxedInfo.Resources,
				func(k string) {
					delete(info.ExtraResources, k)
					info.IgnoreMappings = append(info.IgnoreMappings, k)
				})

			// Datasources/functions
			layerUnder(&muxedInfo.DataSources, info.DataSources, muxedInfo.ExtraFunctions,
				func(k string) {
					delete(info.DataSources, k)
					info.IgnoreMappings = append(info.IgnoreMappings, k)
				})
			layerUnder(&muxedInfo.ExtraFunctions, info.ExtraFunctions, muxedInfo.DataSources,
				func(k string) {
					delete(info.ExtraFunctions, k)
					info.IgnoreMappings = append(info.IgnoreMappings, k)
				})

			// Config
			layerUnder(&muxedInfo.Config, info.Config, muxedInfo.ExtraConfig,
				func(k string) { delete(info.Config, k) })
			layerUnder(&muxedInfo.ExtraConfig, info.ExtraConfig, muxedInfo.Config,
				func(k string) { delete(info.Config, k) })
		}

		genResult, err := tfgen.GenerateSchemaWithOptions(tfgen.GenerateSchemaOptions{
			ProviderInfo:    *info,
			DiagnosticsSink: opts.Sink,
		})

		if anErr(err) {
			continue
		}

		renames = append(renames, genResult.Renames)
		schemas = append(schemas, genResult.PackageSpec)
	}

	if err := errs.ErrorOrNil(); err != nil {
		return sdkBridge.ProviderInfo{}, muxer.DispatchTable{},
			schema.PackageSpec{}, tfgen.Renames{}, err
	}

	m, s, err := muxer.MergeSchemasAndComputeDispatchTable(schemas)
	return muxedInfo, m, s, mergeRenames(renames), err
}

func mergeRenames(renames []tfgen.Renames) tfgen.Renames {
	main := renames[0]
	for _, rename := range renames[1:] {
		for k, v := range rename.Resources {
			_, exists := main.Resources[k]
			if !exists {
				main.Resources[k] = v
			}
		}

		for k, v := range rename.Functions {
			_, exists := main.Functions[k]
			if !exists {
				main.Functions[k] = v
			}
		}
		for k, v := range rename.RenamedProperties {
			_, exists := main.RenamedProperties[k]
			if !exists {
				main.RenamedProperties[k] = v
			}
		}
		for k, v := range rename.RenamedConfigProperties {
			_, exists := main.RenamedConfigProperties[k]
			if !exists {
				main.RenamedConfigProperties[k] = v
			}
		}
		for k, v := range rename.Resources {
			_, exists := main.Resources[k]
			if !exists {
				main.Resources[k] = v
			}
		}
	}
	return main
}

func layerUnder[T any, O any](
	dst *map[string]T, src map[string]T, alternativeDst map[string]O,
	afterAdd func(k string),
) {
	contract.Assertf(dst != nil, "Cannot assign to a nil destination")
	if src == nil {
		return
	}
	if *dst == nil {
		*dst = map[string]T{}
	}
	for k, v := range src {
		_, ok := (*dst)[k]
		if alternativeDst != nil && !ok {
			_, ok = alternativeDst[k]
		}
		if ok {
			continue
		}
		(*dst)[k] = v
		afterAdd(k)
	}
}
