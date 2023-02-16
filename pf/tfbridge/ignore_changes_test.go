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

package tfbridge

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func TestIgnoreChanges(t *testing.T) {
	token := "my:res:Res" //nolint:gosec

	schema := &schema.PackageSpec{
		Resources: map[string]schema.ResourceSpec{
			token: {
				InputProperties: map[string]schema.PropertySpec{
					"topProp": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
					"listProp": {
						TypeSpec: schema.TypeSpec{
							Type: "array",
							Items: &schema.TypeSpec{
								Type: "string",
							},
						},
					},
					"mapProp": {
						TypeSpec: schema.TypeSpec{
							Type: "object",
							AdditionalProperties: &schema.TypeSpec{
								Type: "string",
							},
						},
					},
					"refProp": {
						TypeSpec: schema.TypeSpec{
							Type: "object",
							Ref:  "#/types/my:t:Typ",
						},
					},
				},
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"outProp": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
				},
			},
		},
		Types: map[string]schema.ComplexTypeSpec{
			"my:t:Typ": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"objProp": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
				},
			},
		},
	}

	cases := []ignoreChangesTestCase{
		{
			notes:           "no ignoreChanges means nothing is ignored",
			ignoreChanges:   []string{},
			path:            tftypes.NewAttributePath().WithAttributeName("top_prop"),
			shouldNotIgnore: true,
		},
		{
			notes:         "property is ignored recognizing TF/PU name style differences",
			ignoreChanges: []string{"topProp"},
			path:          tftypes.NewAttributePath().WithAttributeName("top_prop"),
		},
		{
			notes:         "wildcard ignores everything",
			ignoreChanges: []string{"*"},
			path:          tftypes.NewAttributePath().WithAttributeName("top_prop"),
		},
		{
			notes:         "known list element is ignored",
			ignoreChanges: []string{"listProp[1]"},
			path:          tftypes.NewAttributePath().WithAttributeName("list_prop").WithElementKeyInt(1),
		},
		{
			notes:           "known list element is not ignored",
			ignoreChanges:   []string{"listProp[2]"},
			path:            tftypes.NewAttributePath().WithAttributeName("list_prop").WithElementKeyInt(1),
			shouldNotIgnore: true,
		},
		{
			notes:         "any list element is ignored",
			ignoreChanges: []string{"listProp[*]"},
			path:          tftypes.NewAttributePath().WithAttributeName("list_prop").WithElementKeyInt(1),
		},
		{
			notes:         "known map element is ignored",
			ignoreChanges: []string{"mapProp.foo"},
			path:          tftypes.NewAttributePath().WithAttributeName("map_prop").WithElementKeyString("foo"),
		},
		{
			notes:           "known map element is not ignored",
			ignoreChanges:   []string{"mapProp.bar"},
			path:            tftypes.NewAttributePath().WithAttributeName("map_prop").WithElementKeyString("foo"),
			shouldNotIgnore: true,
		},
		{
			notes:         "any map element is ignored",
			ignoreChanges: []string{"mapProp[*]"},
			path:          tftypes.NewAttributePath().WithAttributeName("map_prop").WithElementKeyString("foo"),
		},
		{
			notes:           "output-only properties are not ignored",
			ignoreChanges:   []string{"outProp"},
			path:            tftypes.NewAttributePath().WithAttributeName("out_prop"),
			shouldNotIgnore: true,
		},
		{
			notes:         "named object properties are  ignored",
			ignoreChanges: []string{"refProp.objProp"},
			path:          tftypes.NewAttributePath().WithAttributeName("ref_prop").WithAttributeName("obj_prop"),
		},
	}

	for _, c := range cases {
		ic, err := newIgnoreChanges(schema, tokens.Token(token), &testRenames{}, c.ignoreChanges)
		require.NoError(t, err)
		actual := ic.IsIgnored(c.path)
		assert.Equalf(t, !c.shouldNotIgnore, actual, c.notes)
	}
}

type ignoreChangesTestCase struct {
	ignoreChanges   []string
	path            *tftypes.AttributePath
	shouldNotIgnore bool
	notes           string
}

// Use default name manglers.
type testRenames struct{}

func (*testRenames) PropertyKey(_ tokens.Token,
	property convert.TerraformPropertyName, _ tftypes.Type) resource.PropertyKey {
	return resource.PropertyKey(tfbridge.TerraformToPulumiNameV2(property, nil, nil))
}

func (*testRenames) ConfigPropertyKey(property convert.TerraformPropertyName, t tftypes.Type) resource.PropertyKey {
	return resource.PropertyKey(tfbridge.TerraformToPulumiNameV2(property, nil, nil))
}

var _ convert.PropertyNames = (*testRenames)(nil)
