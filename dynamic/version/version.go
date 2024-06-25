// Copyright 2016-2018, Pulumi Corporation.
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

package version

// version is initialized by the Go linker to contain the semver of this build.
var version string

// The Version of the provider.
//
// Version always returns valid semver, defaulting to v0.0.0-dev if no version was
// specified during linking.
func Version() string {
	if version == "" {
		return "v0.0.0-dev"
	}
	return version
}
