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

package unrec

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/require"
)

func TestParseLocalRef(t *testing.T) {
	type testCase struct {
		ref    string
		expect tokens.Type
	}
	testCases := []testCase{
		{
			"#/types/aws:opsworks/RailsAppLayerCloudwatchConfiguration:RailsAppLayerCloudwatchConfiguration",
			"aws:opsworks/RailsAppLayerCloudwatchConfiguration:RailsAppLayerCloudwatchConfiguration",
		},
		{
			"pulumi.json#/Asset",
			"",
		},
	}
	for _, tc := range testCases {
		tok, ok := parseLocalRef(tc.ref)
		if tc.expect != "" {
			require.True(t, ok)
			require.Equal(t, tc.expect, tok)
		} else {
			require.False(t, ok)
		}
	}
}
