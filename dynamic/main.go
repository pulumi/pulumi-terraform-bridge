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
	"io"
	"os"

	"github.com/opentofu/opentofu/shim"
	"github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func main() {
	ctx := context.Background()

	p, err := shim.LoadProvider(ctx, "random", ">3.0.0")
	if err != nil {
		fmt.Printf("Error: %s", err.Error())
		os.Exit(1)
	}
	defer p.Close()

	info := providerInfo(ctx, p)

	packageSchema, err := tfgen.GenerateSchemaWithOptions(tfgen.GenerateSchemaOptions{
		ProviderInfo: info,
		DiagnosticsSink: diag.DefaultSink(io.Discard, os.Stderr, diag.FormatOptions{
			Color: colors.Always,
		}),
		XInMemoryDocs: true,
	})
	if err != nil {
		fmt.Printf("Error: %s", err.Error())
		os.Exit(1)
	}

	schemaBytes, err := json.Marshal(packageSchema.PackageSpec)
	contract.AssertNoErrorf(err, "This is a provider bug, the SchemaSpec should always marshal.")

	tfbridge.Main(ctx, p.Name(), info, tfbridge.ProviderMetadata{
		PackageSchema: schemaBytes,
	})
}
