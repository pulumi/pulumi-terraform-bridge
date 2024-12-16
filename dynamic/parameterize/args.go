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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// Args represents a parsed CLI argument from a parameterize call.
type Args struct {
	Remote *RemoteArgs
	Local  *LocalArgs
}

// RemoteArgs represents a TF provider referenced by name.
type RemoteArgs struct {
	// Name is a (possibly qualified) name of the provider.
	Name string
	// Version is the (possibly empty) version constraint on the provider.
	Version string

	// Docs indicates if full schema documentation should be generated.
	Docs bool
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
}

func ParseArgs(ctx context.Context, a []string) (Args, error) {
	var args Args
	var fullDocs bool
	var upstreamRepoPath string
	cmd := cobra.Command{
		Use: "./local | remote version",
		RunE: func(cmd *cobra.Command, a []string) error {
			var err error
			args, err = parseArgs(cmd.Context(), a, fullDocs, upstreamRepoPath)
			return err
		},
		Args: cobra.RangeArgs(1, 2),
	}
	cmd.Flags().BoolVar(&fullDocs, "fullDocs", false,
		"Generate a schema with full docs, at the expense of speed")
	cmd.Flags().StringVar(&upstreamRepoPath, "upstreamRepoPath", "",
		"Specify a local file path to the root of the Git repository of the provider being dynamically bridged")

	// We hide docs flags since they are not intended for end users, and they may not be stable.
	if !env.Dev.Value() {
		contract.AssertNoErrorf(
			errors.Join(
				cmd.Flags().MarkHidden("fullDocs"),
				cmd.Flags().MarkHidden("upstreamRepoPath"),
			),
			"impossible - these are static values and should never fail",
		)
	}

	cmd.SetArgs(a)

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

	return args, cmd.ExecuteContext(ctx)
}

func parseArgs(_ context.Context, args []string, fullDocs bool, upstreamRepoPath string) (Args, error) {
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
		return Args{Local: &LocalArgs{Path: args[0], UpstreamRepoPath: upstreamRepoPath}}, nil
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
		Name:    args[0],
		Version: version,
		Docs:    fullDocs,
	}}, nil
}
