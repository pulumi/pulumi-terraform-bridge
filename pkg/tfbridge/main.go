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

package tfbridge

import (
	"github.com/hashicorp/terraform/helper/logging"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// Main launches the tfbridge plugin for a given package pkg and provider prov.
func Main(pkg string, version string, prov ProviderInfo) {
	// Initialize Terraform logging.
	logging.SetOutput()

	if err := Serve(pkg, version, prov); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
