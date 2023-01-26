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

package tfpfbridge

import (
	pfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// Configures Pulumi provider metadata and bridging options.
type ProviderInfo struct {

	// Inherits the options used for bridging providers built with the Terraform Plugin SDK.
	//
	// One notable exception is P (provider itself). When populating ProviderInfo, property P must be nil. Populate
	// NewProvider instead.
	tfbridge.ProviderInfo

	// Constructs a new instance of the Terraform provider for bridging.
	NewProvider func() pfprovider.Provider
}
