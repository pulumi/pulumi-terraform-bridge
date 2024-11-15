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

package testing

import (
	pkgtesting "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/x/testing"
)

// Deprecated: Use github.com/pulumi/pulumi-terraform-bridge/v3/pkg/x/testing.JSONMatchOption instead.
type JSONMatchOption = pkgtesting.JSONMatchOption

// Deprecated: Use github.com/pulumi/pulumi-terraform-bridge/v3/pkg/x/testing.WithUnorderedArrayPaths instead.
var WithUnorderedArrayPaths = pkgtesting.WithUnorderedArrayPaths

// Deprecated: Use github.com/pulumi/pulumi-terraform-bridge/v3/pkg/x/testing.AssertJSONMatchesPattern instead.
var AssertJSONMatchesPattern = pkgtesting.AssertJSONMatchesPattern
