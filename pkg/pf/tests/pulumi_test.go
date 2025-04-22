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

package tfbridgetests

import (
	"testing"

	"github.com/pulumi/providertest/pulumitest"
	"github.com/stretchr/testify/require"

	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/pulcheck"
)

// Quick setup for integration-testing PF-based providers.
func newPulumiTest(t *testing.T, providerBuilder *pb.Provider, testProgramYAML string) *pulumitest.PulumiTest {
	if providerBuilder.TypeName == "" {
		providerBuilder.TypeName = "testprovider"
	}
	providerInfo := providerBuilder.ToProviderInfo()
	providerInfo.EnableZeroDefaultSchemaVersion = true
	pt, err := pulcheck.PulCheck(t, providerInfo, testProgramYAML)
	require.NoError(t, err)
	return pt
}
