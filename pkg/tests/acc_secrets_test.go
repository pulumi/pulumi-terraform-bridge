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

package tests

import (
	"testing"

	"encoding/json"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/assert"
)

func TestAccProviderSecrets(t *testing.T) {
	opts := accTestOptions(t).With(integration.ProgramTestOptions{
		Dir: "provider-secrets",

		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			bytes, err := json.MarshalIndent(stack.Deployment, "", "  ")
			assert.NoError(t, err)
			assert.NotContainsf(t, string(bytes), "SECRET",
				"Secret data leaked into the state")
		},
	})
	integration.ProgramTest(t, &opts)
}
