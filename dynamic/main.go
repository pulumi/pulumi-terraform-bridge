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
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/afero"

	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/run"
	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/parameterize"
	"github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/version"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/proto"
	pfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

const (
	// The name of this *unparameterized* provider.
	baseProviderName = "terraform-provider"
)

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

func initialSetup() (info.Provider, pfbridge.ProviderMetadata, func() error) {
	var tfServer run.Provider
	info := info.Provider{
		DisplayName:  "Any Terraform Provider",
		P:            proto.Empty(),
		Name:         baseProviderName,
		Keywords:     []string{"category/utility"},
		Repository:   "https://github.com/pulumi/pulumi-terraform-provider",
		LogoURL:      "https://raw.githubusercontent.com/pulumi/pulumi-terraform-provider/main/assets/logo.png",
		Version:      version.Version(),
		Description:  "Use any Terraform provider with Pulumi",
		License:      "Apache-2.0",
		MetadataInfo: &info.Metadata{Path: "", Data: tfbridge.ProviderMetadata(nil)},
		SchemaPostProcessor: func(spec *schema.PackageSpec) {
			spec.Attribution = ""
			spec.Provider = schema.ResourceSpec{}
			spec.Language = nil
		},
	}

	var metadata pfbridge.ProviderMetadata
	var fullDocs bool
	var indexDocOutDir string
	metadata = pfbridge.ProviderMetadata{
		XGetSchema: func(ctx context.Context, req plugin.GetSchemaRequest) ([]byte, error) {
			// Create a custom generator for schema. Examples will only be generated if `fullDocs` is set.
			g, err := tfgen.NewGenerator(tfgen.GeneratorOptions{
				Package:       info.Name,
				Version:       info.Version,
				Language:      tfgen.Schema,
				ProviderInfo:  info,
				Root:          afero.NewMemMapFs(),
				Sink:          loggerSink{tfbridge.GetLogger(ctx)},
				XInMemoryDocs: !fullDocs,
				SkipExamples:  !fullDocs,
			})
			if err != nil {
				return nil, errors.Wrapf(err, "failed to create generator")
			}
			packageSchema, err := g.Generate()
			if err != nil {
				return nil, err
			}
			packageSchema.PackageSpec.Version = info.Version

			if info.SchemaPostProcessor != nil {
				info.SchemaPostProcessor(&packageSchema.PackageSpec)
			}

			if indexDocOutDir != "" {
				// Create a custom generator for registry docs (_index.md).
				indexGenerator, err := tfgen.NewGenerator(tfgen.GeneratorOptions{
					Package:       info.Name,
					Version:       info.Version,
					Language:      tfgen.RegistryDocs,
					ProviderInfo:  info,
					Root:          afero.NewBasePathFs(afero.NewOsFs(), indexDocOutDir),
					Sink:          loggerSink{tfbridge.GetLogger(ctx)},
					XInMemoryDocs: false,
					SkipExamples:  false,
				})
				if err != nil {
					return nil, errors.Wrapf(err, "failed to create generator")
				}
				_, err = indexGenerator.Generate()
				if err != nil {
					return nil, err
				}
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
				args, err = parameterize.ParseArgs(ctx, params.Args)
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

			switch args.Remote {
			case nil:
				// We're using local args.
				indexDocOutDir = args.Local.IndexDocOutDir
				if args.Local.UpstreamRepoPath != "" {
					info.UpstreamRepoPath = args.Local.UpstreamRepoPath
					fullDocs = true
				}
			default:
				indexDocOutDir = args.Remote.IndexDocOutDir
				fullDocs = args.Remote.Docs
				if fullDocs || indexDocOutDir != "" {
					// Write the upstream files at this version to a temporary directory
					tmpDir, err := os.MkdirTemp("", "upstreamRepoDir")
					if err != nil {
						return plugin.ParameterizeResponse{}, err
					}
					versionTag := "v" + info.Version
					cloneArgs := &git.CloneOptions{
						URL:           info.Repository,
						Depth:         1,
						Tags:          git.NoTags,
						ReferenceName: plumbing.NewTagReferenceName(versionTag),
					}
					_, err = git.PlainCloneContext(ctx, tmpDir, false, cloneArgs)

					// If we don't have a spec at v+info.Version, maybe we have a spec at info.Version itself.
					if errors.Is(err, git.NoMatchingRefSpecError{}) {
						versionTag = info.Version
						cloneArgs.ReferenceName = plumbing.NewTagReferenceName(versionTag)
						_, err = git.PlainCloneContext(ctx, tmpDir, false, cloneArgs)
					}
					if err != nil {
						return plugin.ParameterizeResponse{}, fmt.Errorf("failed to clone %q@%q: %w",
							info.Repository, versionTag, err)
					}
					info.UpstreamRepoPath = tmpDir
				}
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

func getProvider(ctx context.Context, args parameterize.Args) (run.Provider, error) {
	if local := args.Local; local != nil {
		return run.LocalProvider(ctx, local.Path)
	}

	remote := args.Remote
	contract.Assertf(remote != nil,
		"local or remote must be specified - and that should have already been validated")

	return run.NamedProvider(ctx, remote.Name, remote.Version)
}

type loggerSink struct{ logger tfbridge.Logger }

func (l loggerSink) Logf(sev diag.Severity, d *diag.Diag, args ...interface{}) {
	msg, detail := l.Stringify(sev, d, args...)
	var log func(string)
	switch sev {
	case diag.Debug:
		log = l.logger.Debug
	case diag.Infoerr:
		log = l.logger.Info
	case diag.Warning:
		log = l.logger.Warn
	case diag.Error:
		log = l.logger.Error
	case diag.Info:
		fallthrough
	default:
		log = l.logger.Info
	}

	log(msg)
	if detail != "" {
		log(detail)
	}
}

func (l loggerSink) Debugf(d *diag.Diag, args ...interface{})   { l.Logf(diag.Debug, d, args...) }
func (l loggerSink) Infof(d *diag.Diag, args ...interface{})    { l.Logf(diag.Info, d, args...) }
func (l loggerSink) Infoerrf(d *diag.Diag, args ...interface{}) { l.Logf(diag.Infoerr, d, args...) }
func (l loggerSink) Errorf(d *diag.Diag, args ...interface{})   { l.Logf(diag.Error, d, args...) }
func (l loggerSink) Warningf(d *diag.Diag, args ...interface{}) { l.Logf(diag.Warning, d, args...) }

func (l loggerSink) Stringify(_ diag.Severity, d *diag.Diag, args ...interface{}) (string, string) {
	if d.Raw {
		return fmt.Sprint(append([]any{d.Message}, args...)), ""
	}
	return fmt.Sprintf(d.Message, args...), ""
}
