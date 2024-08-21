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
	"path"
	"strings"

	"github.com/opentofu/opentofu/shim/run"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/pulumi/pulumi-terraform-bridge/dynamic/parameterize"
	"github.com/pulumi/pulumi-terraform-bridge/dynamic/version"
	"github.com/pulumi/pulumi-terraform-bridge/pf/proto"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
)

func providerInfo(ctx context.Context, p run.Provider, value parameterize.Value) tfbridge.ProviderInfo {
	prov := tfbridge.ProviderInfo{
		P:           proto.New(ctx, p),
		Name:        p.Name(),
		Version:     p.Version(),
		Description: "A Pulumi provider dynamically bridged from " + p.Name() + ".",
		Publisher:   "Pulumi",

		// To avoid bogging down schema generation speed, we skip all examples.
		SkipExamples: func(tfbridge.SkipExamplesArgs) bool { return true },

		MetadataInfo: &tfbridge.MetadataInfo{
			Path: "", Data: tfbridge.ProviderMetadata(nil),
		},

		Python: &tfbridge.PythonInfo{
			PyProject:            struct{ Enabled bool }{true},
			RespectSchemaVersion: true,
		},
		JavaScript: &tfbridge.JavaScriptInfo{
			LiftSingleValueMethodReturns: true,
			RespectSchemaVersion:         true,
		},
		CSharp: &tfbridge.CSharpInfo{
			LiftSingleValueMethodReturns: true,
			RespectSchemaVersion:         true,
		},
		Java: &tfbridge.JavaInfo{ /* Java does not have a RespectSchemaVersion flag */ },
		Golang: &tfbridge.GolangInfo{
			ImportBasePath: path.Join(
				"github.com/pulumi/pulumi-terraform-provider/sdks/go",
				p.Name(),
				tfbridge.GetModuleMajorVersion(p.Version()),
			),

			LiftSingleValueMethodReturns: true,
			GenerateExtraInputTypes:      true,
			RespectSchemaVersion:         true,
		},
		SchemaPostProcessor: func(spec *schema.PackageSpec) {
			spec.Parameterization = &schema.ParameterizationSpec{
				BaseProvider: schema.BaseProviderSpec{
					Name:    baseProviderName,
					Version: strings.TrimPrefix(version.Version(), "v"),
				},
				Parameter: value.Marshal(),
			}
		},
	}

	prov.MustComputeTokens(tokens.SingleModule(p.Name()+"_", "index", tokens.MakeStandard(p.Name())))
	prov.SetAutonaming(255, "-")

	return prov
}
