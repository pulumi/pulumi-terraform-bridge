// Copyright 2016-2023, Pulumi Corporation.
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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestIsOfTypeMap(t *testing.T) {
	testCases := []struct {
		name        string
		expectIsMap bool
		schema      shim.Schema
	}{
		{
			"nil", false,
			nil,
		},
		{
			"int", false,
			(&schema.Schema{
				Type: shim.TypeInt,
			}).Shim(),
		},
		{
			"string_map", true,
			(&schema.Schema{
				Type: shim.TypeMap,
				Elem: (&schema.Schema{
					Type: shim.TypeString,
				}).Shim(),
			}).Shim(),
		},
		{
			"single_nested_block", false,
			(&schema.Schema{
				Type: shim.TypeMap,
				Elem: (&schema.Resource{
					Schema: &schema.SchemaMap{},
				}).Shim(),
			}).Shim(),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actual := IsOfTypeMap(tc.schema)
			assert.Equal(t, tc.expectIsMap, actual)
		})
	}
}
