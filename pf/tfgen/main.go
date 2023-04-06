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

	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
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
