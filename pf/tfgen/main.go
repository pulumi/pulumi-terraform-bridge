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
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
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

		if opts.Language == tfgen.Schema {
			renames, err := g.Renames()
			if err != nil {
				return err
			}
			if err := writeRenames(renames, opts); err != nil {
				return err
			}
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
		// To avoid double-initializing the sink (and thus panicking), we eagerly
		// initialize here.
		if opts.Sink == nil {
			diagOpts := diag.FormatOptions{
				Color: cmdutil.GetGlobalColorization(),
				Debug: opts.Debug,
			}
			cmdutil.InitDiag(diagOpts)
			opts.Sink = cmdutil.Diag()
		}

		muxedInfo := infos[0].GetInfo()
		schemas := []schema.PackageSpec{}
		var errs multierror.Error

		var pfRenames []tfgen.Renames

		for i, union := range infos {
			// Concurrency safety
			i := i
			union := union

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
				// Now we purge already defined resources from the next schema,
				// marking the all remaining resources as defined for future
				// schemas.
				muxedInfo.IgnoreMappings = append(muxedInfo.IgnoreMappings,
					info.IgnoreMappings...)

				rDefined := func(k string) bool {
					_, defined := muxedInfo.Resources[k]
					if defined {
						return true
					}
					_, defined = muxedInfo.ExtraResources[k]
					return defined
				}

				fDefined := func(k string) bool {
					_, defined := muxedInfo.DataSources[k]
					if defined {
						return true
					}
					_, defined = muxedInfo.ExtraFunctions[k]
					return defined
				}

				for k, v := range info.Resources {
					// This resource was not already defined. Mark it as
					// defined then leave it alone.
					if !rDefined(k) {
						muxedInfo.Resources[k] = v
						continue
					}

					// This resource was already defined in a previous
					// provider, so it needs to be purged from this provider.
					delete(info.Resources, k)
					info.IgnoreMappings = append(info.IgnoreMappings, k)
				}
				for k, v := range info.ExtraResources {
					if !rDefined(k) {
						muxedInfo.ExtraResources[k] = v
						continue
					}

					// This resource was already defined in a previous
					// provider, so it needs to be purged from this provider.
					delete(info.ExtraResources, k)
					info.IgnoreMappings = append(info.IgnoreMappings, k)
				}

				// We now do the same thing for data sources.

				for k, v := range info.DataSources {
					if !fDefined(k) {
						muxedInfo.DataSources[k] = v
						continue
					}
					delete(info.DataSources, k)
					info.IgnoreMappings = append(info.IgnoreMappings, k)
				}
				for k, v := range info.ExtraFunctions {
					if !fDefined(k) {
						muxedInfo.ExtraFunctions[k] = v
						continue
					}
					delete(info.ExtraFunctions, k)
					info.IgnoreMappings = append(info.IgnoreMappings, k)
				}
			}

			s, renames, err := tfgen.GenerateSchemaAndRenames(*info, opts.Sink)
			if anErr(err) {
				continue
			}

			if union.PF != nil {
				pfRenames = append(pfRenames, renames)
			}

			schemas = append(schemas, s)
		}

		if err := errs.ErrorOrNil(); err != nil {
			return err
		}

		mapping, schema, err := muxer.Mapping(schemas)
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
			return fmt.Errorf("Failed to save metadata: %w", err)
		}

		// Having computed the schema, we now want to complete the tfgen process,
		// reusing as much of the standard process as possible.

		opts.ProviderInfo = muxedInfo
		g, err := tfgen.NewGenerator(opts)
		if err != nil {
			return err
		}

		if err := writeRenames(mergeRenames(pfRenames), opts); err != nil {
			return err
		}

		return g.GenerateFromSchema(schema)
	}

	tfgen.MainWithCustomGenerate(provider, infos[0].GetInfo().Version, infos[0].GetInfo(), gen)
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

func writeRenames(renames tfgen.Renames, opts tfgen.GeneratorOptions) error {
	renamesFile, err := opts.Root.Create("bridge-metadata.json")
	if err != nil {
		return err
	}

	renamesBytes, err := json.MarshalIndent(renames, "", "  ")
	if err != nil {
		return err
	}

	if _, err := renamesFile.Write(renamesBytes); err != nil {
		return err
	}

	if err := renamesFile.Close(); err != nil {
		return err
	}

	return nil
}
