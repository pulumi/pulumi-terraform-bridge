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

package tfbridge

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestTopLevelPropertyKeySet(t *testing.T) {
    t.Parallel()
	str := (&schema.Schema{
		Type: shim.TypeString,
	}).Shim()
	ints := (&schema.Schema{
		Type: shim.TypeInt,
	}).Shim()
	sch := schema.SchemaMap{
		"str": str,
		"obj": (&schema.Schema{
			Type: shim.TypeMap,
			Elem: (&schema.Resource{
				Schema: schema.SchemaMap{
					"string_prop": str,
					"int_prop":    ints,
				},
			}).Shim(),
		}).Shim(),
	}
	type testCase struct {
		name   string
		paths  []*tftypes.AttributePath
		expect []resource.PropertyKey
	}
	testCases := []testCase{
		{
			"empty",
			nil,
			nil,
		},
		{
			"str",
			[]*tftypes.AttributePath{tftypes.NewAttributePath().WithAttributeName("str")},
			[]resource.PropertyKey{"str"},
		},
		{
			"clipped-dedup-obj-keys",
			[]*tftypes.AttributePath{
				tftypes.NewAttributePath().WithAttributeName("obj").WithAttributeName("string_prop"),
				tftypes.NewAttributePath().WithAttributeName("obj").WithAttributeName("int_prop"),
			},
			[]resource.PropertyKey{"obj"},
		},
		{
			"sorted-str-and-obj",
			[]*tftypes.AttributePath{
				tftypes.NewAttributePath().WithAttributeName("str"),
				tftypes.NewAttributePath().WithAttributeName("obj").WithAttributeName("string_prop"),
			},
			[]resource.PropertyKey{"obj", "str"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := topLevelPropertyKeySet(sch, nil, tc.paths)
			require.Equal(t, tc.expect, actual)
		})
	}
}
