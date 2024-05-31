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

	"github.com/blang/semver"
	"github.com/opentofu/opentofu/shim"
	"github.com/pulumi/pulumi-terraform-bridge/pf/proto"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func main() {
	ctx := context.Background()

	var packageSchemaBytes []byte
	var toClose func() error

	defer func() {
		if toClose != nil {
			toClose()
		}
	}()

	pfbridge.Main(ctx, "terraform-bridge", tfbridge.ProviderInfo{
		P:       proto.New(ctx, nil),
		Name:    "terraform-bridge",
		Version: "0.0.1",

		// To avoid bogging down schema generation speed, we skip all examples.
		SkipExamples: func(tfbridge.SkipExamplesArgs) bool { return true },

		MetadataInfo: &tfbridge.MetadataInfo{
			Path: "", Data: tfbridge.ProviderMetadata(nil),
		},
	}, pfbridge.ProviderMetadata{
		PackageSchema: []byte("{}"),
		PackageSchemaFunc: func(context.Context) ([]byte, error) {
			return packageSchemaBytes, nil
		},
		Parameterize: func(ctx context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
			var name, version string

			switch p := req.Parameters.(type) {
			case plugin.ParameterizeArgs:
				switch len(p.Args) {
				case 2:
					version = p.Args[1]
					fallthrough
				case 1:
					name = p.Args[0]
				default:
					return plugin.ParameterizeResponse{}, fmt.Errorf("Unknown number of params")
				}
			}

			p, err := shim.LoadProvider(ctx, name, version)
			if err != nil {
				return plugin.ParameterizeResponse{}, err
			}
			toClose = p.Close

			info := providerInfo(ctx, p)

			packageSchema, err := tfgen.GenerateSchemaWithOptions(tfgen.GenerateSchemaOptions{
				ProviderInfo: info,
				DiagnosticsSink: diag.DefaultSink(io.Discard, os.Stderr, diag.FormatOptions{
					Color: colors.Always,
				}),
				XInMemoryDocs: true,
			})
			if err != nil {
				return plugin.ParameterizeResponse{}, err
			}

			schemaBytes, err := json.Marshal(packageSchema.PackageSpec)
			contract.AssertNoErrorf(err, "This is a provider bug, the SchemaSpec should always marshal.")

			packageSchemaBytes = schemaBytes

			var v *semver.Version
			if info.Version != "" {
				ver, err := semver.Parse(info.Version)
				if err != nil {
					return plugin.ParameterizeResponse{}, err
				}
				v = &ver
			}

			return plugin.ParameterizeResponse{
				Name:    info.Name,
				Version: v,
			}, nil
		},
	})
}
