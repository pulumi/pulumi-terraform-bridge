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

package sdkv2

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestProposedNew(t *testing.T) {
	cases := []struct {
		name   string
		res    *schema.Resource
		config cty.Value
		prior  cty.Value
		expect cty.Value
	}{
		{
			name: "configured set overrides null set",
			res: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"id": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"set_property_value": {
						Type:     schema.TypeSet,
						Elem:     &schema.Schema{Type: schema.TypeString},
						Optional: true,
					},
				},
			},
			prior: cty.ObjectVal(map[string]cty.Value{
				"id":                 cty.StringVal("r1"),
				"set_property_value": cty.NullVal(cty.Set(cty.String)),
			}),
			config: cty.ObjectVal(map[string]cty.Value{
				"id":                 cty.NullVal(cty.String),
				"set_property_value": cty.SetVal([]cty.Value{cty.StringVal("foo")}),
			}),
			expect: cty.ObjectVal(map[string]cty.Value{
				"id":                 cty.StringVal("r1"),
				"set_property_value": cty.SetVal([]cty.Value{cty.StringVal("foo")}),
			}),
		},
	}

	for _, cc := range cases {
		c := cc

		t.Run(c.name, func(t *testing.T) {
			actual, err := proposedNew(c.res, c.prior, c.config)
			require.NoError(t, err)
			assert.Equal(t, c.expect.GoString(), actual.GoString())
		})
	}
}
