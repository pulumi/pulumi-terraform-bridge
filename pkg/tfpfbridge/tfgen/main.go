// Copyright 2016-2022, Pulumi Corporation.
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

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/schemashim"

	tfpf "github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

// Implements main() logic for a provider built-time helper utility. By convention these utilities are named
// pulumi-tfgen-$provider, for example when building a "random" provider the program would be called
// pulumi-tfgen-random.
//
// The resulting binary is able to generate [Pulumi Package Schema] as well as provider SDK sources in various
// programming languages supported by Pulumi such as TypeScript, Go, and Python.
//
// [Pulumi Package Schema]: https://www.pulumi.com/docs/guides/pulumi-packages/schema/
func Main(provider string, info tfpf.ProviderInfo) {
	version := info.Version
	ctx := context.Background()
	shimInfo := schemashim.ShimSchemaOnlyProviderInfo(ctx, info)

	tfgen.MainWithCustomGenerate(provider, version, shimInfo, func(opts tfgen.GeneratorOptions) error {
		g, err := tfgen.NewGenerator(opts)
		if err != nil {
			return err
		}

		if err := g.Generate(); err != nil {
			return err
		}

		if opts.Language == tfgen.Schema {
			if err := writeRenames(g, opts); err != nil {
				return err
			}
		}

		return nil
	})
}

func writeRenames(g *tfgen.Generator, opts tfgen.GeneratorOptions) error {
	renames, err := g.Renames()
	if err != nil {
		return err
	}

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
