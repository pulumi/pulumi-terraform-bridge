// Copyright 2016-2026, Pulumi Corporation.
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

// This test lives in an external test package (unlike info_test.go) because it keeps
// Provider's methods reflection-reachable, which requires the pkg/tfbridge linkname
// targets of external_methods.go to be linked into the test binary.
package info_test

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge" // linkname targets
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	schemashim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

// Provider-defined function signatures round-trip through the JSON wire form, so
// mapping consumers can recover the full Terraform signature (including variadic-ness)
// from the provider schema section.
func TestMarshallableProviderFunctions(t *testing.T) {
	t.Parallel()

	functions := map[string]shim.Function{
		"concat": {
			Parameters: []shim.FunctionParameter{{
				Name:        "separator",
				Type:        tftypes.String,
				Description: "String placed between each part.",
			}},
			VariadicParameter: &shim.FunctionParameter{
				Name:           "parts",
				Type:           tftypes.String,
				AllowNullValue: true,
			},
			Return:  tftypes.String,
			Summary: "Concatenates strings with a separator.",
		},
		"parse_id": {
			Parameters: []shim.FunctionParameter{
				{Name: "id", Type: tftypes.String},
				{Name: "extra", Type: tftypes.DynamicPseudoType},
				{Name: "tags", Type: tftypes.List{ElementType: tftypes.String}},
			},
			Return: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"prefix": tftypes.String,
					"suffix": tftypes.String,
				},
				OptionalAttributes: map[string]struct{}{"suffix": {}},
			},
			Description:        "Splits prefix/suffix identifiers.",
			DeprecationMessage: "use split_id instead",
		},
	}

	provider := &info.Provider{
		Name: "test",
		P:    (&schemashim.Provider{Functions: functions}).Shim(),
		Functions: map[string]*info.Function{
			"concat":   {Tok: "test:index/concat:concat"},
			"parse_id": {Tok: "test:index/parseId:parseId"},
		},
	}

	data, err := json.Marshal(info.MarshalProvider(provider))
	require.NoError(t, err)
	var decoded info.MarshallableProvider
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, functions, decoded.Unmarshal().P.Functions())
}
