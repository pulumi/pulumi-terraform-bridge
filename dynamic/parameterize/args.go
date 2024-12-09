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
	"fmt"
	"strings"
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

func ParseArgs(args []string) (Args, error) {
	// Check for a leading '.' or '/' to indicate a path
	if len(args) >= 1 &&
		(strings.HasPrefix(args[0], "./") || strings.HasPrefix(args[0], "/")) {
		if len(args) > 1 {
			docsArg := args[1]
			upstreamRepoPath, found := strings.CutPrefix(docsArg, "upstreamRepoPath=")
			if !found {
				return Args{}, fmt.Errorf(
					"path based providers are only parameterized by 2 arguments: <path> " +
						"[upstreamRepoPath=<path/to/files>]",
				)
			}
			if upstreamRepoPath == "" {
				return Args{}, fmt.Errorf(
					"upstreamRepoPath must be set to a non-empty value: " +
						"upstreamRepoPath=path/to/files",
				)
			}
			return Args{Local: &LocalArgs{Path: args[0], UpstreamRepoPath: upstreamRepoPath}}, nil
		}
		return Args{Local: &LocalArgs{Path: args[0]}}, nil
	}

	// This is a registry based provider
	var remote RemoteArgs
	switch len(args) {
	// The third argument, if any, is the full docs option for when we need to generate docs
	case 3:
		docsArg := args[2]
		errMsg := "expected third parameterized argument to be 'fullDocs=<true|false>' or be empty"

		fullDocs, found := strings.CutPrefix(docsArg, "fullDocs=")
		if !found {
			return Args{}, fmt.Errorf("%s", errMsg)
		}

		switch fullDocs {
		case "true":
			remote.Docs = true
		case "false":
			// Do nothing
		default:
			return Args{}, fmt.Errorf("%s", errMsg)
		}

		fallthrough
	// The second argument, if any is the version
	case 2:
		remote.Version = args[1]
		fallthrough
	// The first argument is the provider name
	case 1:
		remote.Name = args[0]
		return Args{Remote: &remote}, nil
	default:
		return Args{}, fmt.Errorf("expected to be parameterized by 1-3 arguments: <name> [version] [fullDocs=<true|false>]")
	}
}
