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

package tokens

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// The Pulumi module that provider-defined functions map into.
//
// Terraform provider-defined functions are named without the provider prefix and live in
// a namespace separate from resources and data sources, so module-splitting heuristics
// based on name prefixes do not apply to them. Every token strategy maps functions into
// the top-level module.
const functionModule = "index"

// topLevelFunction maps a provider-defined function into the top-level module.
//
// The Terraform name is camelCased and used directly, without a "get" prefix: a
// Terraform function "parse_arn" becomes "pkg:index/parseArn:parseArn".
func topLevelFunction(finalize Make) info.FunctionStrategy {
	return func(tfToken string, fn *info.Function) error {
		if fn.Tok != "" {
			return nil
		}
		tk, err := finalize(functionModule, camelCase(tfToken))
		if err != nil {
			return err
		}
		fn.Tok = tokens.ModuleMember(tk)
		return nil
	}
}
