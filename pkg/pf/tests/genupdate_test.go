// Copyright 2016-2022, Pulumi Corporation.
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

	testutils "github.com/pulumi/providertest/replay"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
)

// These tests replay Update gRPC logs to get some unit test coverage for Update.
//
// To re-capture try:
//
//	cd tests/integration
//	PULUMI_DEBUG_GPRC=$PWD/grpc.json go test -run TestUpdateProgram
//	jq -s . grpc.json
func TestGenUpdates(t *testing.T) {
	t.Parallel()
	trace := "testdata/updateprogram.json"

	info := testprovider.SyntheticTestBridgeProvider()
	info.EnableAccurateBridgePreview = false
	server, err := newProviderServer(t, info)
	require.NoError(t, err)
	testutils.ReplayFile(t, server, trace)
}
