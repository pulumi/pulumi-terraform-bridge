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

package run

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	pfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reattachTestProvider struct{}

func (reattachTestProvider) Metadata(
	_ context.Context, _ pfprovider.MetadataRequest, resp *pfprovider.MetadataResponse,
) {
	resp.TypeName = "mini"
}

func (reattachTestProvider) Schema(context.Context, pfprovider.SchemaRequest, *pfprovider.SchemaResponse) {
}

func (reattachTestProvider) Configure(context.Context, pfprovider.ConfigureRequest, *pfprovider.ConfigureResponse) {
}

func (reattachTestProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

func (reattachTestProvider) Resources(context.Context) []func() resource.Resource { return nil }

// serveReattachProvider serves an in-process protocol 6 provider in go-plugin
// debug mode and returns the PULUMI_BRIDGE_REATTACH_PROVIDERS value that
// points key at it.
func serveReattachProvider(t *testing.T, key string) string {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	reattachCh := make(chan *plugin.ReattachConfig)
	closeCh := make(chan struct{})
	go func() {
		err := tf6server.Serve(
			"registry.opentofu.org/"+key,
			providerserver.NewProtocol6(reattachTestProvider{}),
			tf6server.WithGoPluginLogger(hclog.NewNullLogger()),
			tf6server.WithDebug(ctx, reattachCh, closeCh),
			tf6server.WithoutLogStderrOverride(),
		)
		assert.NoError(t, err)
	}()
	rc := <-reattachCh

	entry := reattachEntry{
		Protocol:        string(rc.Protocol),
		ProtocolVersion: rc.ProtocolVersion,
		Pid:             rc.Pid,
		Test:            rc.Test,
	}
	entry.Addr.Network = rc.Addr.Network()
	entry.Addr.String = rc.Addr.String()
	env, err := json.Marshal(map[string]reattachEntry{key: entry})
	require.NoError(t, err)
	return string(env)
}

func TestNamedProviderReattach(t *testing.T) {
	t.Setenv(envReattachProviders, serveReattachProvider(t, "test/mini"))

	ctx := context.Background()
	p, err := NamedProvider(ctx, "test/mini", "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, p.Close()) })

	assert.Equal(t, "mini", p.Name())
	assert.Equal(t, reattachProviderVersion, p.Version())
	assert.Equal(t, "registry.opentofu.org/test/mini", p.URL())

	resp, err := p.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.Provider)
}

// A fully-qualified request must match a bare env key after address
// normalization.
func TestNamedProviderReattachNormalizesAddresses(t *testing.T) {
	t.Setenv(envReattachProviders, serveReattachProvider(t, "test/mini"))

	p, err := NamedProvider(context.Background(), "registry.opentofu.org/test/mini", "")
	require.NoError(t, err)
	require.NoError(t, p.Close())
}

func TestNamedProviderReattachInvalidEnv(t *testing.T) {
	t.Setenv(envReattachProviders, "not json")

	_, err := NamedProvider(context.Background(), "test/mini", "")
	require.ErrorContains(t, err, envReattachProviders)
}
