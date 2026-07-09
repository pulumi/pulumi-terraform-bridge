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

package shim

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Function describes a provider-defined function.
//
// Provider-defined functions are a Terraform Plugin Framework feature (protocol 6.5+,
// Terraform 1.8+). They are pure, offline computations: no side effects and no access to
// provider configuration. In Terraform they live in a namespace separate from resources
// and data sources; names are registered without the provider prefix (e.g. "parse_arn",
// not "aws_parse_arn") and called as provider::<local-name>::<function-name>(...).
//
// See https://developer.hashicorp.com/terraform/plugin/framework/functions/concepts.
type Function struct {
	// Parameters describes the ordered, positional parameters of the function.
	Parameters []FunctionParameter

	// VariadicParameter, if non-nil, describes a final parameter that accepts zero or
	// more trailing arguments of its type.
	VariadicParameter *FunctionParameter

	// Return is the type of the function result.
	Return tftypes.Type

	// Summary is a short, single-line description of the function.
	Summary string

	// Description is a longer description of the function, possibly in Markdown.
	Description string

	// DeprecationMessage is non-empty if the function is deprecated.
	DeprecationMessage string
}

// FunctionParameter describes a single parameter of a provider-defined function.
type FunctionParameter struct {
	// Name is the parameter name. Terraform treats parameter names as
	// documentation-only: they may be empty or duplicate other parameter names.
	Name string

	// Type is the type constraint of the parameter.
	Type tftypes.Type

	// AllowNullValue is true if the parameter accepts a null argument.
	AllowNullValue bool

	// Description is a description of the parameter, possibly in Markdown.
	Description string
}
