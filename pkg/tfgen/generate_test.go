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

package tfgen

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv1 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v1"
	"github.com/stretchr/testify/assert"
)

func Test_DeprecationFromTFSchema(t *testing.T) {
	v := &variable{
		name:   "v",
		schema: shimv1.NewSchema(&schema.Schema{Type: schema.TypeString, Deprecated: "This is deprecated"}),
	}

	deprecationMessage := v.deprecationMessage()
	assert.Equal(t, "This is deprecated", deprecationMessage)
}

func Test_ForceNew(t *testing.T) {
	cases := []struct {
		Name           string
		Var            variable
		ShouldForceNew bool
	}{
		{Name: "Pulumi Schema with ForceNew Override ShouldForceNew true",
			Var: variable{
				name: "v",
				schema: shimv1.NewSchema(&schema.Schema{
					Type: schema.TypeString,
				}),
				info: &tfbridge.SchemaInfo{
					ForceNew: tfbridge.True(),
				},
			},
			ShouldForceNew: true,
		},
		{
			Name: "TF Schema ForceNew ShouldForceNew true",
			Var: variable{
				name: "v",
				schema: shimv1.NewSchema(&schema.Schema{
					Type:     schema.TypeString,
					ForceNew: true,
				}),
			},
			ShouldForceNew: true,
		},
		{
			Name: "Output Parameter ShouldForceNew false",
			Var: variable{
				out: true,
			},
			ShouldForceNew: false,
		},
		{
			Name: "Input Non ForceNew Parameter ShouldForceNew false",
			Var: variable{
				name: "v",
				schema: shimv1.NewSchema(&schema.Schema{
					Type: schema.TypeString,
				}),
			},
			ShouldForceNew: false,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			v := &test.Var
			actuallyForcesNew := v.forceNew()
			assert.Equal(t, test.ShouldForceNew, actuallyForcesNew)
		})
	}
}
