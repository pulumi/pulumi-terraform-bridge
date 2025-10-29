// Copyright 2025-2025, Pulumi Corporation.
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
	"os"
	"path/filepath"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
)

func TestEphemeralCreateDelete(t *testing.T) {
	t.Parallel()
	server, err := newProviderServer(t, testprovider.SyntheticTestBridgeProvider())
	require.NoError(t, err)

	properties, err := structpb.NewStruct(map[string]any{
		"statedir": "/tmp",
	})
	require.NoError(t, err)
	resp, err := server.Create(t.Context(), &pulumirpc.CreateRequest{
		Urn:        "urn:pulumi:test-stack::basicprogram::testbridge:index/ephemeral:Testeph::r1",
		Properties: properties,
	})
	require.NoError(t, err)
	path := filepath.Join("/tmp", resp.Id+".bin")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "\xa8statedir\xa4/tmp")

	_, err = server.Delete(t.Context(), &pulumirpc.DeleteRequest{
		Urn: "urn:pulumi:test-stack::basicprogram::testbridge:index/ephemeral:Testeph::r1",
		Id:  resp.Id,
	})
	require.NoError(t, err)
	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err))
}
