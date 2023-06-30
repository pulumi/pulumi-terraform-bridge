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

import b "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"

// A strategy that assigns all tokens to the same module.
//
// For example:
//
//	rStrat, dStrat := SingleModule("pkgName_", "index", finalize)
//
// The above example would transform "pkgName_foo" into "pkgName:index:Foo".
func SingleModule(
	tfPackagePrefix, moduleName string, finalize Make,
) b.Strategy {
	return KnownModules(tfPackagePrefix, moduleName, nil, finalize)
}
