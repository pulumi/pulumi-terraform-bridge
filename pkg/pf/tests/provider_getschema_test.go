// Copyright 2016-2026, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
)

// tfgen omits the version from schema.json, so GetSchema must fill it in from the provider version.
func TestGetSchemaInjectsVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	info := testprovider.RandomProvider()
	meta, err := genMetadata(t, info)
	require.NoError(t, err)

	var spec map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(meta.PackageSchema, &spec))
	delete(spec, "version")
	meta.PackageSchema, err = json.Marshal(spec)
	require.NoError(t, err)

	server, err := tfbridge.NewProviderServer(ctx, logging.NewTestingSink(t), info, meta)
	require.NoError(t, err)
	resp, err := server.GetSchema(ctx, &pulumirpc.GetSchemaRequest{})
	require.NoError(t, err)

	var got struct {
		Version string `json:"version"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp.GetSchema()), &got))
	assert.Equal(t, info.Version, got.Version)
}
