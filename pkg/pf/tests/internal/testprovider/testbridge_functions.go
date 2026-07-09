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

package testprovider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.ProviderWithFunctions = (*syntheticProvider)(nil)

func (p *syntheticProvider) Functions(context.Context) []func() function.Function {
	return SyntheticTestBridgeFunctions()
}

// SyntheticTestBridgeFunctions returns the provider-defined functions of the synthetic
// testbridge provider, for reuse in tests that build ad-hoc providers.
func SyntheticTestBridgeFunctions() []func() function.Function {
	return []func() function.Function{
		func() function.Function { return concatFunction{} },
		func() function.Function { return parseIDFunction{} },
		func() function.Function { return nullableDefaultFunction{} },
	}
}

// concatFunction joins a variadic list of strings with a separator.
type concatFunction struct{}

func (concatFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "concat"
}

func (concatFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Concatenates strings with a separator.",
		Parameters: []function.Parameter{
			function.StringParameter{Name: "separator", Description: "String placed between each part."},
		},
		VariadicParameter: function.StringParameter{Name: "parts", Description: "Strings to join."},
		Return:            function.StringReturn{},
	}
}

func (concatFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var separator string
	var parts []string
	resp.Error = req.Arguments.Get(ctx, &separator, &parts)
	if resp.Error != nil {
		return
	}
	resp.Error = resp.Result.Set(ctx, strings.Join(parts, separator))
}

// parseIDFunction splits "prefix/suffix" identifiers, returning an object. Malformed
// input produces an argument-scoped function error.
type parseIDFunction struct{}

var parseIDReturnAttrTypes = map[string]attr.Type{
	"prefix": types.StringType,
	"suffix": types.StringType,
}

func (parseIDFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "parse_id"
}

func (parseIDFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Parses a prefix/suffix identifier.",
		Parameters: []function.Parameter{
			function.StringParameter{Name: "id"},
		},
		Return: function.ObjectReturn{AttributeTypes: parseIDReturnAttrTypes},
	}
}

func (parseIDFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var id string
	resp.Error = req.Arguments.Get(ctx, &id)
	if resp.Error != nil {
		return
	}
	prefix, suffix, found := strings.Cut(id, "/")
	if !found {
		resp.Error = function.NewArgumentFuncError(0, `expected an id of the form "prefix/suffix"`)
		return
	}
	result, diags := types.ObjectValue(parseIDReturnAttrTypes, map[string]attr.Value{
		"prefix": types.StringValue(prefix),
		"suffix": types.StringValue(suffix),
	})
	if diags.HasError() {
		resp.Error = function.FuncErrorFromDiags(ctx, diags)
		return
	}
	resp.Error = resp.Result.Set(ctx, result)
}

// nullableDefaultFunction accepts a nullable string and substitutes a default for null.
type nullableDefaultFunction struct{}

func (nullableDefaultFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "nullable_default"
}

func (nullableDefaultFunction) Definition(
	_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse,
) {
	resp.Definition = function.Definition{
		Summary: "Returns the value, or a default when the value is null.",
		Parameters: []function.Parameter{
			function.StringParameter{Name: "value", AllowNullValue: true},
		},
		Return: function.StringReturn{},
	}
}

func (nullableDefaultFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var value *string
	resp.Error = req.Arguments.Get(ctx, &value)
	if resp.Error != nil {
		return
	}
	result := "default"
	if value != nil {
		result = *value
	}
	resp.Error = resp.Result.Set(ctx, result)
}
