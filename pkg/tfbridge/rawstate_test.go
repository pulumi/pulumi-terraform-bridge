// Copyright 2016-2025, Pulumi Corporation.
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

package tfbridge

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func Test_rawStateInflections_turnaroud(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		schemaMap   shim.SchemaMap
		schemaInfos map[string]*SchemaInfo
		pv          resource.PropertyValue
		cv          cty.Value
	}

	testCases := []testCase{
		{
			name: "null-string",
			pv:   resource.NewNullProperty(),
			cv:   cty.NullVal(cty.String),
		},
		{
			name: "null-number",
			pv:   resource.NewNumberProperty(42.5),
			cv:   cty.NumberFloatVal(42.5),
		},
		{
			name: "empty-string",
			pv:   resource.NewStringProperty(""),
			cv:   cty.StringVal(""),
		},
		{
			name: "simple-string",
			pv:   resource.NewStringProperty("simple"),
			cv:   cty.StringVal("simple"),
		},
		{
			name: "simple-bool",
			pv:   resource.NewBoolProperty(true),
			cv:   cty.BoolVal(true),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ih := inflectHelper{
				schemaMap:   tc.schemaMap,
				schemaInfos: tc.schemaInfos,
			}

			t.Logf("pv: %v", tc.pv.String())
			t.Logf("cv: %v", tc.cv.GoString())

			infl, err := ih.inflections(tc.pv, tc.cv)
			require.NoError(t, err)

			t.Logf("inflections: %#v", infl)

			recoveredCtyValue, err := rawStateRecover(tc.pv, infl)
			require.NoError(t, err)

			t.Logf("cv2:%v", recoveredCtyValue.GoString())

			require.True(t, recoveredCtyValue.RawEquals(tc.cv))
		})
	}
}
