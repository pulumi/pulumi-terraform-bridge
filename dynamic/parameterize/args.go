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

package parameterize

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// Args represents a parsed CLI argument from a parameterize call.
type Args struct {
	Remote *RemoteArgs
	Local  *LocalArgs
	// Includes is the list of resource and datasource TF tokens to include in the
	// provider.  If empty, all resources and datasources are included.
	Includes []string
	// ProviderName is the custom name for the generated provider.
	// If empty, the default Terraform provider name is used.
	ProviderName string
}

// RemoteArgs represents a TF provider referenced by name.
type RemoteArgs struct {
	// Name is a (possibly qualified) name of the provider.
	Name string
	// Version is the (possibly empty) version constraint on the provider.
	Version string
	// Docs indicates if full schema documentation should be generated.
	Docs bool
	// IndexDocOutDir allows us to set a specific directory to write `_index.md` to.
	IndexDocOutDir string
}

// LocalArgs represents a local TF provider referenced by path.
type LocalArgs struct {
	// Path is the path to the provider binary. It can be relative or absolute.
	Path string
	// UpstreamRepoPath (if provided) is the local path to the dynamically bridged Terraform provider's repo.
	//
	// If set, full documentation will be generated for the provider.
	// If not set, only documentation from the TF provider's schema will be used.
	UpstreamRepoPath string
	// IndexDocOutDir allows us to set a specific directory to write `_index.md` to.
	IndexDocOutDir string
}

func ParseArgs(ctx context.Context, cliArgs []string) (Args, error) {
	var args Args
	var fullDocs bool
	var upstreamRepoPath string
	var indexDocOutDir string
	var includes []string
	var providerName string

	// If --help is included in `a`, then `RunE` will never be executed.
	//
	// We detect that and make sure we return an error instead of continuing.
	var cmdWasRun bool

	cmd := cobra.Command{
		Use: "./local | remote version",
		RunE: func(cmd *cobra.Command, a []string) error {
			cmdWasRun = true
			var err error
			args, err = parseArgs(cmd.Context(), a, fullDocs, upstreamRepoPath, indexDocOutDir, includes, providerName)
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			err := cobra.RangeArgs(1, 2)(cmd, args)
			if err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&fullDocs, "fullDocs", false,
		"Generate a schema with full docs, at the expense of speed")
	cmd.Flags().StringVar(&upstreamRepoPath, "upstreamRepoPath", "",
		"Specify a local file path to the root of the Git repository of the provider being dynamically bridged")
	cmd.Flags().StringVar(&indexDocOutDir, "indexDocOutDir", "",
		"Specify a local output directory for the provider's _index.md file")
	cmd.Flags().StringSliceVar(&includes, "include", nil,
		`Comma-separated list of resource and datasource Terraform tokens to include in the provider `+
			`(e.g. aws_instance,aws_vpc).

If no include filter is specified, all resources and datasources are mapped.`)
	cmd.Flags().StringVar(&providerName, "provider-name", "",
		"Custom name for the generated provider to avoid name collisions")

	// We hide docs flags since they are not intended for end users, and they may not be stable.
	if !env.Dev.Value() {
		contract.AssertNoErrorf(
			errors.Join(
				cmd.Flags().MarkHidden("fullDocs"),
				cmd.Flags().MarkHidden("upstreamRepoPath"),
				cmd.Flags().MarkHidden("indexDocOutDir"),
			),
			"impossible - these are static values and should never fail",
		)
	}

	cmd.SetArgs(cliArgs)

	// We want to show the stdout of this command to the user, if there is
	// any. pulumi/pulumi#17943 started hiding unstructured output by default. This
	// block writes the output of `cmd` to `out`, and then logs what was written to
	// `out` to info, which will be displayed directly to the user (without any
	// prefix, warning and error have a prefix).
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	defer func() {
		if out.Len() == 0 {
			return
		}
		tfbridge.GetLogger(ctx).Info(out.String())
	}()

	err := cmd.ExecuteContext(ctx)
	if err != nil {
		return args, err
	}
	if !cmdWasRun {
		return args, errors.New("help text displayed")
	}

	return args, nil
}

func parseArgs(
	_ context.Context,
	args []string,
	fullDocs bool,
	upstreamRepoPath, indexDocOutDir string,
	includes []string,
	providerName string,
) (Args, error) {
	// If we see a local prefix (starts with '.' or '/'), parse args for a local provider
	if strings.HasPrefix(args[0], ".") || strings.HasPrefix(args[0], "/") {
		if len(args) > 1 {
			return Args{}, fmt.Errorf("local providers only accept one argument, found %d", len(args))
		}
		if fullDocs {
			msg := "fullDocs only applies to remote providers"
			if upstreamRepoPath == "" {
				msg += ", consider specifying upstreamRepoPath instead"
			}
			return Args{}, errors.New(msg)
		}
		return Args{
			Local: &LocalArgs{
				Path:             args[0],
				UpstreamRepoPath: upstreamRepoPath,
				IndexDocOutDir:   indexDocOutDir,
			},
			Includes:     includes,
			ProviderName: providerName,
		}, nil
	}

	if upstreamRepoPath != "" {
		msg := "upstreamRepoPath only applies to local providers"
		if upstreamRepoPath == "" {
			msg += ", consider specifying fullDocs instead"
		}
		return Args{}, errors.New(msg)
	}

	var version string
	if len(args) > 1 {
		version = args[1]
	}

	return Args{Remote: &RemoteArgs{
		Name:           args[0],
		Version:        version,
		Docs:           fullDocs,
		IndexDocOutDir: indexDocOutDir,
	}, Includes: includes, ProviderName: providerName}, nil
}
