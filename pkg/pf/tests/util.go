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

package tfbridgetests

import (
	"context"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func newProviderServer(t *testing.T, info info.Provider) (pulumirpc.ResourceProviderServer, error) {
	ctx := context.Background()
	meta, err := genMetadata(t, info)
	if err != nil {
		return nil, err
	}
	srv, err := tfbridge.NewProviderServer(ctx, logging.NewTestingSink(t), info, meta)
	require.NoError(t, err)
	return srv, nil
}

func newMuxedProviderServer(t *testing.T, info info.Provider) pulumirpc.ResourceProviderServer {
	ctx := context.Background()
	meta := genSDKSchema(t, info)
	p, err := tfbridge.MakeMuxedServer(ctx, info.Name, info, meta)(nil)
	require.NoError(t, err)
	return p
}
