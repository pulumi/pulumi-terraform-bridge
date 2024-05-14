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
	"fmt"
	"path"
	"strings"

	"github.com/opentofu/opentofu/shim"

	"github.com/pulumi/pulumi-terraform-bridge/pf/proto"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func providerInfo(ctx context.Context, p shim.Provider) tfbridge.ProviderInfo {
	prov := tfbridge.ProviderInfo{
		P:           proto.New(ctx, p),
		Name:        p.Name(),
		Version:     p.Version(),
		Description: "A Pulumi provider dynamically bridged from " + p.Name() + ".",

		// To avoid bogging down schema generation speed, we skip all examples.
		SkipExamples: func(tfbridge.SkipExamplesArgs) bool { return true },

		MetadataInfo: &tfbridge.MetadataInfo{
			Path: "", Data: tfbridge.ProviderMetadata(nil),
		},

		Golang: &tfbridge.GolangInfo{
			ImportBasePath: path.Join(
				fmt.Sprintf("github.com/pulumi/pulumi-%[1]s/sdk/", p.Name()),
				tfbridge.GetModuleMajorVersion("0.0.0"),
				"go",
				p.Name(),
			),

			LiftSingleValueMethodReturns: true,
			GenerateExtraInputTypes:      true,
			RespectSchemaVersion:         true,
		},
	}

	prov.MustComputeTokens(tokens.SingleModule(p.Name()+"_", "index", tokens.MakeStandard(p.Name())))
	prov.SetAutonaming(255, "-")

	return prov
}

type paramaterizeArgs struct {
	name    string
	version string
	path    string
}

func parseParamaterizeParameters(req plugin.ParameterizeRequest) (paramaterizeArgs, error) {
	switch req := req.Parameters.(type) {
	case plugin.ParameterizeArgs:

		// Check for a leading '.' or '/' to indicate a
		if len(req.Args) >= 1 &&
			(strings.HasPrefix(req.Args[0], "./") || strings.HasPrefix(req.Args[0], "/")) {
			if len(req.Args) > 1 {
				return paramaterizeArgs{}, fmt.Errorf("path based providers are only parameterized by 1 argument: <path>")
			}
			return paramaterizeArgs{path: req.Args[0]}, nil
		}

		// This is a registry based provider
		var ret paramaterizeArgs
		switch len(req.Args) {
		// The second argument, if any is the version
		case 2:
			ret.version = req.Args[1]
			fallthrough
		// The first argument is the provider name
		case 1:
			ret.name = req.Args[0]
			return ret, nil
		default:
			return ret, fmt.Errorf("expected to be parameterized by 1-2 arguments: <name> [version]")
		}
	case plugin.ParameterizeValue:
		return paramaterizeArgs{}, fmt.Errorf("parameters from Value are not yet implemented")
	default:
		return paramaterizeArgs{}, fmt.Errorf("unknown parameter type %T", req)
	}
}
