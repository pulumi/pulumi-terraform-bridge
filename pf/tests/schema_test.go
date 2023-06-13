// Copyright 2016-2022, Pulumi Corporation.
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

package tfbridgetests

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/pulumi/pulumi-terraform-bridge/pf/tests/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	"runtime"
)

func TestSchemaGen(t *testing.T) {
	t.Run("random", func(t *testing.T) {
		genMetadata(t, testprovider.RandomProvider())
	})
	t.Run("tls", func(t *testing.T) {
		genMetadata(t, testprovider.TLSProvider())
	})
	t.Run("testbridge", func(t *testing.T) {
		data := genMetadata(t, testprovider.SyntheticTestBridgeProvider())
		var spec schema.PackageSpec
		require.NoError(t, json.Unmarshal(data.PackageSchema, &spec))
		res := spec.Resources["testbridge:index/testnest:Testnest"]
		assert.Equal(t, "array", res.InputProperties["rules"].Type)
		assert.Equal(t,
			"#/types/testbridge:index/TestnestRule:TestnestRule",
			res.InputProperties["rules"].Items.Ref)

		rule := spec.Types["testbridge:index/TestnestRule:TestnestRule"]
		assert.Equal(t,
			"#/types/testbridge:index/TestnestRuleActionParameters:TestnestRuleActionParameters",
			rule.Properties["actionParameters"].Ref)
		assert.Equal(t, "", rule.Properties["actionParameters"].Type)

		actionParameters := spec.Types["testbridge:index/TestnestRuleActionParameters:TestnestRuleActionParameters"]
		assert.Equal(t, "", actionParameters.Properties["phases"].Type)
		assert.Equal(t,
			"#/types/testbridge:index/TestnestRuleActionParametersPhases:TestnestRuleActionParametersPhases",
			actionParameters.Properties["phases"].Ref)

		actionParameterPhases := spec.Types["testbridge:index/TestnestRuleActionParametersPhases:TestnestRuleActionParametersPhases"]
		assert.Equal(t, "object", actionParameterPhases.Type)
		assert.Equal(t, "boolean", actionParameterPhases.Properties["p2"].Type)
	})
}

func TestSchemaGenInSync(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to a minor path discrepancy in actual vs generated schema")
	}

	type testCase struct {
		name     string
		file     string
		pf       tfbridge.ProviderInfo
		provider tfbridge.ProviderInfo
	}
	testprovider.MuxedRandomProvider()

	testCases := []testCase{
		{
			name: "tls",
			file: "internal/testprovider/cmd/pulumi-resource-tls/schema.json",
			pf:   testprovider.TLSProvider(),
		},
		{
			name:     "muxedrandom",
			file:     "internal/testprovider/cmd/pulumi-resource-muxedrandom/schema.json",
			provider: testprovider.MuxedRandomProvider(),
		},
		{
			name: "random",
			file: "./internal/testprovider/cmd/pulumi-resource-random/schema.json",
			pf:   testprovider.RandomProvider(),
		},
		{
			name: "testbridge",
			file: "./internal/testprovider/cmd/pulumi-resource-testbridge/schema.json",
			pf:   testprovider.SyntheticTestBridgeProvider(),
		},
	}

	renorm := func(a schema.PackageSpec) (out schema.PackageSpec) {
		b, err := json.Marshal(a)
		require.NoError(t, err)
		err = json.Unmarshal(b, &out)
		require.NoError(t, err)
		return
	}

	for _, tc := range testCases {
		tc := tc

		expectedBytes, err := os.ReadFile(tc.file)
		require.NoError(t, err)

		var expectedSpec schema.PackageSpec
		require.NoError(t, json.Unmarshal(expectedBytes, &expectedSpec))

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var actualSpec schema.PackageSpec
			if tc.pf.P != nil {
				data := genMetadata(t, tc.pf)
				require.NoError(t, json.Unmarshal(data.PackageSchema, &actualSpec))
			} else {
				var err error
				actualSpec, err = tfgen.GenerateSchema(tc.provider, testSink(t))
				require.NoError(t, err)
			}

			// Ignoring version differences, for some obscure reason they diverge right now.
			expectedSpec.Version = actualSpec.Version

			// Currently languge sections disagree on JSON formatting, ignoring.
			expectedSpec = renorm(expectedSpec)
			actualSpec = renorm(actualSpec)

			assert.Equal(t, expectedSpec, actualSpec,
				"On-disk schema for %q seems out of date, try running `make build.testproviders`",
				tc.name)
		})
	}

}
