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
	"os"

	"github.com/blang/semver"
	"github.com/opentofu/opentofu/shim/run"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/dynamic/parameterize"
	"github.com/pulumi/pulumi-terraform-bridge/dynamic/version"
	"github.com/pulumi/pulumi-terraform-bridge/pf/proto"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

const (
	// The name of this *unparameterized* provider.
	baseProviderName = "terraform-provider"
)

func initialSetup() (tfbridge.ProviderInfo, pfbridge.ProviderMetadata, func() error) {

	var tfServer run.Provider
	info := tfbridge.ProviderInfo{
		DisplayName:  "Any Terraform Provider",
		P:            proto.Empty(),
		Name:         baseProviderName,
		Version:      version.Version(),
		Description:  "Use any Terraform provider with Pulumi",
		MetadataInfo: &tfbridge.MetadataInfo{Path: "", Data: tfbridge.ProviderMetadata(nil)},
		SchemaPostProcessor: func(spec *schema.PackageSpec) {
			spec.Attribution = ""
			spec.Provider = schema.ResourceSpec{}
			spec.Language = nil
		},
	}

	var metadata pfbridge.ProviderMetadata
	metadata = pfbridge.ProviderMetadata{
		XGetSchema: func(ctx context.Context, req plugin.GetSchemaRequest) ([]byte, error) {
			packageSchema, err := tfgen.GenerateSchemaWithOptions(tfgen.GenerateSchemaOptions{
				ProviderInfo: info,
				DiagnosticsSink: diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
					Color: colors.Always,
				}),
				XInMemoryDocs: true,
			})
			if err != nil {
				return nil, err
			}

			if info.SchemaPostProcessor != nil {
				info.SchemaPostProcessor(&packageSchema.PackageSpec)
			}

			return json.Marshal(packageSchema.PackageSpec)
		},
		XParamaterize: func(ctx context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
			if tfServer != nil {
				return plugin.ParameterizeResponse{},
					newDoubleParameterizeErr(tfServer.Name(), tfServer.Version())
			}

			var args parameterize.Args
			switch params := req.Parameters.(type) {
			case *plugin.ParameterizeValue:
				value, err := parameterize.ParseValue(params.Value)
				if err != nil {
					tfbridge.GetLogger(ctx).Error(fmt.Sprintf(
						"%[1]s is unable to parse the parameter value "+
							"embedded in the generated SDK.\nThis is always a bug in "+
							"%[1]s and should be reported. \n"+
							"The value passed was %[2]q.",
						baseProviderName, string(params.Value),
					))
					return plugin.ParameterizeResponse{}, fmt.Errorf(
						"failed to parse parameterized value: %w", err,
					)
				}
				args = value.IntoArgs()
			case *plugin.ParameterizeArgs:
				var err error
				args, err = parameterize.ParseArgs(params.Args)
				if err != nil {
					return plugin.ParameterizeResponse{}, err
				}
			}

			p, err := getProvider(ctx, args)
			if err != nil {
				return plugin.ParameterizeResponse{}, err
			}

			v, err := semver.Parse(p.Version())
			if err != nil {
				return plugin.ParameterizeResponse{}, err
			}

			var value parameterize.Value
			if args.Local != nil {
				value.Local = &parameterize.LocalValue{
					Path: args.Local.Path,
				}
			} else {
				value.Remote = &parameterize.RemoteValue{
					URL:     p.URL(),
					Version: p.Version(),
				}
			}

			tfServer = p
			if tfServer != nil {
				info, err = providerInfo(ctx, tfServer, value)
				if err != nil {
					return plugin.ParameterizeResponse{}, err
				}
			}

			err = pfbridge.XParameterizeResetProvider(ctx, info, metadata)
			if err != nil {
				return plugin.ParameterizeResponse{}, err
			}

			return plugin.ParameterizeResponse{
				Name:    p.Name(),
				Version: v,
			}, nil
		},
	}

	return info, metadata, func() error {
		if tfServer == nil {
			return nil
		}
		return tfServer.Close()
	}
}

func newDoubleParameterizeErr(name, version string) doubleParameterizeErr {
	return doubleParameterizeErr{
		existing: struct {
			name    string
			version string
		}{
			name:    name,
			version: version,
		},
	}
}

type doubleParameterizeErr struct {
	existing struct{ name, version string }
}

func (d doubleParameterizeErr) Error() string {
	return fmt.Sprintf("provider is already parameterized to (%s, %s)",
		d.existing.name, d.existing.version)
}

func main() {
	ctx := context.Background()

	defaultInfo, metadata, close := initialSetup()

	defer func() {
		if err := close(); err != nil {
			fmt.Printf("Failed to close TF provder: %s", err.Error())
		}
	}()

	pfbridge.Main(ctx, baseProviderName, defaultInfo, metadata)
}

func getProvider(ctx context.Context, args parameterize.Args) (run.Provider, error) {
	if local := args.Local; local != nil {
		return run.LocalProvider(ctx, local.Path)
	}

	remote := args.Remote
	contract.Assertf(remote != nil,
		"local or remote must be specified - and that should have already been validated")

	return run.NamedProvider(ctx, remote.Name, remote.Version)
}
