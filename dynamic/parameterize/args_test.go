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
	"context"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/testutil"
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
			name:   "local too many args",
			args:   []string{"./my-provider", "nonsense"},
			errMsg: autogold.Expect(`local providers only accept one argument, found 2`),
		},
		{
			name: "local with docs location",
			args: []string{"./my-provider", "--upstreamRepoPath=./my-provider"},
			expect: Args{
				Local: &LocalArgs{
					Path:             "./my-provider",
					UpstreamRepoPath: "./my-provider",
				},
			},
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
			errMsg: autogold.Expect("accepts between 1 and 2 arg(s), received 0"),
		},
		{
			name:   "too many args",
			args:   []string{"arg1", "arg2", "arg3", "arg4"},
			errMsg: autogold.Expect("accepts between 1 and 2 arg(s), received 4"),
		},
		{
			name:   "invalid third arg",
			args:   []string{"arg1", "arg2", "arg3"},
			errMsg: autogold.Expect(`accepts between 1 and 2 arg(s), received 3`),
		},
		{
			name: "empty fullDocs flag defaults false",
			args: []string{"arg1", "arg2"},
			expect: Args{Remote: &RemoteArgs{
				Name:    "arg1",
				Version: "arg2",
				Docs:    false,
			}},
		},
		{
			name: "fullDocs flag invalid input",
			args: []string{"my-registry.io/typ", "1.2.3", "--fullDocs=invalid-input"},
			//nolint:lll
			errMsg: autogold.Expect(
				`invalid argument "invalid-input" for "--fullDocs" flag: strconv.ParseBool: parsing "invalid-input": invalid syntax`,
			),
		},
		{
			name: "indexDocOutDir flag empty",
			args: []string{"arg1", "arg2", "--indexDocOutDir="},
			expect: Args{Remote: &RemoteArgs{
				Name:           "arg1",
				Version:        "arg2",
				Docs:           false,
				IndexDocOutDir: "",
			}},
		},
		{
			name: "indexDocOutDir sets location",
			args: []string{"arg1", "arg2", "--indexDocOutDir=localDir"},
			expect: Args{Remote: &RemoteArgs{
				Name:           "arg1",
				Version:        "arg2",
				Docs:           false,
				IndexDocOutDir: "localDir",
			}},
		},
		{
			name: "invalid flag",
			args: []string{"arg1", "arg2", "--invalid=wrong"},
			errMsg: autogold.Expect(
				"unknown flag: --invalid",
			),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutil.InitLogging(t, context.Background(), nil)
			actual, err := ParseArgs(ctx, tt.args)
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
