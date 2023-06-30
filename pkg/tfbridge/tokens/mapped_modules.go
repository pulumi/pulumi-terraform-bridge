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

package tokens

import (
	"sort"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	b "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// A strategy for assigning tokens to a hand generated set of modules with an arbitrary
// mapping from TF modules to Pulumi modules.
//
// If defaultModule is "", then the returned strategies will error on not encountering a matching module.
func MappedModules(
	tfPackagePrefix, defaultModule string, modules map[string]string, finalize Make,
) b.Strategy {

	mods := make([]string, 0, len(modules))
	for k := range modules {
		mods = append(mods, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(mods)))

	transform := func(tf string) string {
		s, ok := modules[tf]
		if !ok && tf == defaultModule {
			// We pass through the default module as is, so it might not be in
			// `modules`. We need to catch that and return as is.
			return tf
		}
		assert := "Because any mod selected must be from mods, it is guaranteed to be in modules, got %#v"
		contract.Assertf(ok, assert, tf)
		return s
	}

	return b.Strategy{
		Resource: knownModules(tfPackagePrefix, defaultModule, mods,
			func(mod, tk string, r *b.ResourceInfo) error {
				tk, err := finalize(mod, tk)
				if err != nil {
					return err
				}
				checkedApply(&r.Tok, tokens.Type(tk))
				return nil
			}, transform),
		DataSource: knownModules(tfPackagePrefix, defaultModule, mods,
			func(mod, tk string, d *b.DataSourceInfo) error {
				tk, err := finalize(mod, "get"+tk)
				if err != nil {
					return err
				}
				checkedApply(&d.Tok, tokens.ModuleMember(tk))
				return nil
			}, transform),
	}
}
