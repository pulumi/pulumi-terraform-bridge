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

package tfbridge_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func testFunction() shim.Function {
	return shim.Function{
		Parameters: []shim.FunctionParameter{{Name: "input", Type: tftypes.String}},
		Return:     tftypes.String,
	}
}

func TestTokensSingleModuleFunctions(t *testing.T) {
	t.Parallel()
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			Functions: map[string]shim.Function{
				"parse_arn":     testFunction(),
				"trim_prefixes": testFunction(),
			},
		}).Shim(),
	}

	err := info.ComputeTokens(tokens.SingleModule("foo_", "index", tokens.MakeStandard("foo")))
	require.NoError(t, err)

	// Functions are unprefixed in Terraform and always map into the top-level module,
	// with no "get" prefix.
	assert.Equal(t, map[string]*tfbridge.FunctionInfo{
		"parse_arn":     {Tok: "foo:index/parseArn:parseArn"},
		"trim_prefixes": {Tok: "foo:index/trimPrefixes:trimPrefixes"},
	}, info.Functions)
}

func TestTokensFunctionsRespectExistingMappings(t *testing.T) {
	t.Parallel()
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			Functions: map[string]shim.Function{
				"parse_arn": testFunction(),
				"other_fn":  testFunction(),
			},
		}).Shim(),
		Functions: map[string]*tfbridge.FunctionInfo{
			"parse_arn": {Tok: "foo:custom/parseMyArn:parseMyArn"},
		},
	}

	err := info.ComputeTokens(tokens.SingleModule("foo_", "index", tokens.MakeStandard("foo")))
	require.NoError(t, err)

	assert.Equal(t, map[string]*tfbridge.FunctionInfo{
		"parse_arn": {Tok: "foo:custom/parseMyArn:parseMyArn"},
		"other_fn":  {Tok: "foo:index/otherFn:otherFn"},
	}, info.Functions)
}

func TestTokensFunctionsRespectIgnoreMappings(t *testing.T) {
	t.Parallel()
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			Functions: map[string]shim.Function{
				"parse_arn": testFunction(),
				"ignored":   testFunction(),
			},
		}).Shim(),
		IgnoreMappings: []string{"ignored"},
	}

	err := info.ComputeTokens(tokens.SingleModule("foo_", "index", tokens.MakeStandard("foo")))
	require.NoError(t, err)

	assert.Equal(t, map[string]*tfbridge.FunctionInfo{
		"parse_arn": {Tok: "foo:index/parseArn:parseArn"},
	}, info.Functions)
}

func TestTokensMappedModulesFunctions(t *testing.T) {
	t.Parallel()
	info := tfbridge.ProviderInfo{
		P: (&schema.Provider{
			Functions: map[string]shim.Function{
				"parse_arn": testFunction(),
			},
		}).Shim(),
	}

	err := info.ComputeTokens(tokens.MappedModules("foo_", "idx", map[string]string{
		"mod": "module",
	}, tokens.MakeStandard("foo")))
	require.NoError(t, err)

	// Module mappings do not apply to functions; they stay in the top-level module.
	assert.Equal(t, map[string]*tfbridge.FunctionInfo{
		"parse_arn": {Tok: "foo:index/parseArn:parseArn"},
	}, info.Functions)
}
