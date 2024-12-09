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
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		args   []string
		expect Args
		errMsg autogold.Value
	}{
		{
			name:   "local",
			args:   []string{"./my-provider"},
			expect: Args{Local: &LocalArgs{Path: "./my-provider"}},
		},
		{
			name: "local too many args",
			args: []string{"./my-provider", "nonsense"},
			errMsg: autogold.Expect(
				"path based providers are only parameterized by 2 arguments: <path> [upstreamRepoPath=<path/to/files>]",
			),
		},
		{
			name: "local with docs location",
			args: []string{"./my-provider", "upstreamRepoPath=./my-provider"},
			expect: Args{
				Local: &LocalArgs{
					Path:             "./my-provider",
					UpstreamRepoPath: "./my-provider",
				},
			},
		},
		{
			name: "local empty upstreamRepoPath",
			args: []string{"./my-provider", "upstreamRepoPath="},
			errMsg: autogold.Expect(
				"upstreamRepoPath must be set to a non-empty value: upstreamRepoPath=path/to/files",
			),
		},
		{
			name:   "remote",
			args:   []string{"my-registry.io/typ"},
			expect: Args{Remote: &RemoteArgs{Name: "my-registry.io/typ"}},
		},
		{
			name: "remote with version",
			args: []string{"my-registry.io/typ", "v1.2.3"},
			expect: Args{Remote: &RemoteArgs{
				Name:    "my-registry.io/typ",
				Version: "v1.2.3",
			}},
		},
		{
			name:   "no args",
			args:   []string{},
			errMsg: autogold.Expect("expected to be parameterized by 1-3 arguments: <name> [version] [fullDocs]"),
		},
		{
			name:   "too many args",
			args:   []string{"arg1", "arg2", "arg3", "arg4"},
			errMsg: autogold.Expect("expected to be parameterized by 1-3 arguments: <name> [version] [fullDocs]"),
		},
		{
			name: "invalid third arg",
			args: []string{"arg1", "arg2", "arg3"},
			errMsg: autogold.Expect(
				"expected third parameterized argument to be 'fullDocs=true/false' or be empty",
			),
		},
		{
			name: "empty third arg",
			args: []string{"arg1", "arg2"},
			expect: Args{Remote: &RemoteArgs{
				Name:    "arg1",
				Version: "arg2",
				Docs:    false,
			}},
		},
		{
			name: "valid third arg true",
			args: []string{"my-registry.io/typ", "1.2.3", "fullDocs=true"},
			expect: Args{Remote: &RemoteArgs{
				Name:    "my-registry.io/typ",
				Version: "1.2.3",
				Docs:    true,
			}},
		},
		{
			name: "valid third arg false",
			args: []string{"my-registry.io/typ", "1.2.3", "fullDocs=false"},
			expect: Args{Remote: &RemoteArgs{
				Name:    "my-registry.io/typ",
				Version: "1.2.3",
				Docs:    false,
			}},
		},
		{
			name: "third arg invalid input",
			args: []string{"my-registry.io/typ", "1.2.3", "fullDocs=invalid-input"},
			errMsg: autogold.Expect(
				"expected third parameterized argument to be 'fullDocs=true/false' or be empty",
			),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ParseArgs(tt.args)
			if tt.errMsg == nil {
				require.NoError(t, err)
				assert.Equal(t, tt.expect, actual)
			} else {
				require.Error(t, err)
				tt.errMsg.Equal(t, err.Error())
			}
		})
	}
}
