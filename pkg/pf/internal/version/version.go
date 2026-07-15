// Copyright 2016-2024, Pulumi Corporation.
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

// Package version provides shared version validation for Plugin Framework based
// bridged providers.
package version

import (
	"fmt"

	"github.com/blang/semver"
)

// Validate checks that version is non-empty and semver-compatible.
//
// Plugin Framework entrypoints do not receive a version argument separately
// from ProviderInfo, unlike the Terraform Plugin SDK v2 entrypoints. An empty
// or invalid ProviderInfo.Version compiles successfully and can generate
// schema and SDKs, but currently only fails when the provider is constructed
// at runtime. Validating with this function at tfgen and runtime entrypoints
// surfaces the problem as early as possible, for example at `make tfgen` /
// build_schema time.
func Validate(version string) error {
	const baseMsg = "ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible"

	if version == "" {
		return fmt.Errorf("%s: got info.Version=%q", baseMsg, version)
	}
	if _, err := semver.ParseTolerant(version); err != nil {
		return fmt.Errorf("%s: got info.Version=%q", baseMsg, version)
	}
	return nil
}
