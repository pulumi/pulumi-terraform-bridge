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

	"github.com/pulumi/pulumi-terraform-bridge/dynamic/internal/fixup"
	"github.com/pulumi/pulumi-terraform-bridge/dynamic/parameterize"
	"github.com/pulumi/pulumi-terraform-bridge/dynamic/version"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/proto"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func providerInfo(ctx context.Context, p run.Provider, value parameterize.Value) (tfbridge.ProviderInfo, error) {

	provider := proto.New(ctx, p)
	prov := tfbridge.ProviderInfo{
		P:           provider,
		Name:        p.Name(),
		Version:     p.Version(),
		Description: "A Pulumi provider dynamically bridged from " + p.Name() + ".",
		Publisher:   "Pulumi",

		ResourcePrefix: inferResourcePrefix(provider),

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
			RootPackageName:              p.Name(),
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

	if err := fixup.Default(&prov); err != nil {
		return prov, err
	}

	err := prov.ComputeTokens(tokens.SingleModule(
		prov.GetResourcePrefix(), "index", tokens.MakeStandard(p.Name())))
	if err != nil {
		return prov, err
	}

	prov.SetAutonaming(255, "-")

	return prov, nil
}

// inferResourcePrefix makes a best attempt effort at finding the resource prefix for p.
func inferResourcePrefix(p shim.Provider) string {
	var canidate string
	p.ResourcesMap().Range(func(key string, _ shim.Resource) bool {
		parts := strings.Split(key, "_")
		if len(parts) < 2 {
			// This might not be a valid resource, just ignore it. Errors will
			// be reported later as part of token mapping.
			return true
		}
		// Set parts[0] as the candidate
		if canidate == "" {
			canidate = parts[0]
			return true
		}

		// We already have a candidate, we are now checking if it's consistent.

		if canidate == parts[0] {
			// The candidate still holds, so keep iterating.
			return true
		}

		// The candidate did not hold, so reset the candidate and give up.
		canidate = ""
		return false
	})
	return canidate
}
