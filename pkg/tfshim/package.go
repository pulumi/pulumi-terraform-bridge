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

// Package tfshim implements an abstraction layer for TF bridge backends.
//
// Concrete backends include:
//
//   - sdk-v1 for https://github.com/hashicorp/terraform-plugin-sdk
//   - sdk-v2 for https://github.com/hashicorp/terraform-plugin-sdk (v2)
//
// The tfplugin5 backend is experimental and is not as of time of this writing to build production
// providers by Pulumi.
//
// Note that providers built with the Plugin Framework do not currently conform to the backend
// interface and are handled separately, see github.com/pulumi/pulumi-terraform-bridge/pf
//
// This package is internal. While we avoid unnecessary breaking changes, this package may accept
// technically breaking changes between major version releases of the bridge.
package shim
