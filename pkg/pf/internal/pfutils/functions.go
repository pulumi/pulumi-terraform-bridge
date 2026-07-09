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

package pfutils

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// GatherFunctions returns the provider-defined functions of prov, keyed by their
// unprefixed Terraform name. Providers that do not implement
// [provider.ProviderWithFunctions] have no functions.
func GatherFunctions(ctx context.Context, prov provider.Provider) (map[string]shim.Function, error) {
	provWithFunctions, ok := prov.(provider.ProviderWithFunctions)
	if !ok {
		return nil, nil
	}

	functions := map[string]shim.Function{}
	for _, makeFunction := range provWithFunctions.Functions(ctx) {
		fn := makeFunction()

		meta := function.MetadataResponse{}
		fn.Metadata(ctx, function.MetadataRequest{}, &meta)

		def := function.DefinitionResponse{}
		fn.Definition(ctx, function.DefinitionRequest{}, &def)
		if err := checkDiagsForErrors(def.Diagnostics); err != nil {
			return nil, fmt.Errorf("function %s Definition() error: %w", meta.Name, err)
		}

		shimFn, err := fromFunctionDefinition(ctx, def.Definition)
		if err != nil {
			return nil, fmt.Errorf("function %s: %w", meta.Name, err)
		}
		functions[meta.Name] = shimFn
	}
	return functions, nil
}

func fromFunctionDefinition(ctx context.Context, def function.Definition) (shim.Function, error) {
	if def.Return == nil {
		return shim.Function{}, fmt.Errorf("definition has no return type")
	}

	parameters := make([]shim.FunctionParameter, len(def.Parameters))
	for i, p := range def.Parameters {
		parameters[i] = fromFunctionParameter(ctx, p)
	}
	var variadic *shim.FunctionParameter
	if def.VariadicParameter != nil {
		v := fromFunctionParameter(ctx, def.VariadicParameter)
		variadic = &v
	}

	return shim.Function{
		Parameters:         parameters,
		VariadicParameter:  variadic,
		Return:             def.Return.GetType().TerraformType(ctx),
		Summary:            def.Summary,
		Description:        markdownOrPlain(def.MarkdownDescription, def.Description),
		DeprecationMessage: def.DeprecationMessage,
	}, nil
}

func fromFunctionParameter(ctx context.Context, p function.Parameter) shim.FunctionParameter {
	return shim.FunctionParameter{
		Name:           p.GetName(),
		Type:           p.GetType().TerraformType(ctx),
		AllowNullValue: p.GetAllowNullValue(),
		Description:    markdownOrPlain(p.GetMarkdownDescription(), p.GetDescription()),
	}
}

// markdownOrPlain matches the Plugin Framework protocol encoding, which prefers the
// Markdown variant of a description when both are set.
func markdownOrPlain(markdown, plain string) string {
	if markdown != "" {
		return markdown
	}
	return plain
}
