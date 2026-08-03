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
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	regaddr "github.com/opentofu/registry-address/v2"
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

// serveReattachProvider serves an in-process provider in go-plugin debug mode
// over the given plugin protocol version (5 or 6) and returns its reattach
// entry.
func serveReattachProvider(t *testing.T, key string, protocolVersion int) reattachEntry {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	name := "registry.opentofu.org/" + key
	reattachCh := make(chan *plugin.ReattachConfig)
	closeCh := make(chan struct{})
	go func() {
		var err error
		switch protocolVersion {
		case 5:
			err = tf5server.Serve(
				name,
				providerserver.NewProtocol5(reattachTestProvider{}),
				tf5server.WithGoPluginLogger(hclog.NewNullLogger()),
				tf5server.WithDebug(ctx, reattachCh, closeCh),
				tf5server.WithoutLogStderrOverride(),
			)
		case 6:
			err = tf6server.Serve(
				name,
				providerserver.NewProtocol6(reattachTestProvider{}),
				tf6server.WithGoPluginLogger(hclog.NewNullLogger()),
				tf6server.WithDebug(ctx, reattachCh, closeCh),
				tf6server.WithoutLogStderrOverride(),
			)
		default:
			t.Errorf("unsupported protocol version %d", protocolVersion)
			return
		}
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
	return entry
}

func reattachEnv(t *testing.T, entries map[string]reattachEntry) string {
	t.Helper()
	env, err := json.Marshal(entries)
	require.NoError(t, err)
	return string(env)
}

func TestNamedProviderReattach(t *testing.T) {
	entry := serveReattachProvider(t, "test/mini", 6)
	t.Setenv(envReattachProviders, reattachEnv(t, map[string]reattachEntry{"test/mini": entry}))

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

// A protocol 5 server must be upgraded to protocol 6 over the reattached
// connection.
func TestNamedProviderReattachProtocol5(t *testing.T) {
	entry := serveReattachProvider(t, "test/mini", 5)
	t.Setenv(envReattachProviders, reattachEnv(t, map[string]reattachEntry{"test/mini": entry}))

	ctx := context.Background()
	p, err := NamedProvider(ctx, "test/mini", "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, p.Close()) })

	resp, err := p.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.Provider)
}

// A fully-qualified request must match a bare env key after address
// normalization.
func TestNamedProviderReattachNormalizesAddresses(t *testing.T) {
	entry := serveReattachProvider(t, "test/mini", 6)
	t.Setenv(envReattachProviders, reattachEnv(t, map[string]reattachEntry{"test/mini": entry}))

	p, err := NamedProvider(context.Background(), "registry.opentofu.org/test/mini", "")
	require.NoError(t, err)
	require.NoError(t, p.Close())
}

func TestNamedProviderReattachInvalidEnv(t *testing.T) {
	t.Setenv(envReattachProviders, "not json")

	_, err := NamedProvider(context.Background(), "test/mini", "")
	require.ErrorContains(t, err, envReattachProviders)
}

// A malformed key rejects the env var deterministically, even when a valid
// matching entry is also present.
func TestNamedProviderReattachMalformedSiblingKey(t *testing.T) {
	entry := serveReattachProvider(t, "test/mini", 6)
	t.Setenv(envReattachProviders, reattachEnv(t, map[string]reattachEntry{
		"test/mini": entry,
		"bad key!":  {},
	}))

	_, err := NamedProvider(context.Background(), "test/mini", "")
	require.ErrorContains(t, err, `provider key "bad key!"`)
}

func TestReattachProviderRejectsUnsupportedEntries(t *testing.T) {
	t.Parallel()

	addr, err := regaddr.ParseProviderSource("test/mini")
	require.NoError(t, err)

	grpcUnix := func(network, protocol string) reattachEntry {
		entry := reattachEntry{Protocol: protocol, ProtocolVersion: 6}
		entry.Addr.Network = network
		entry.Addr.String = "/tmp/plugin.sock"
		return entry
	}

	tests := []struct {
		name    string
		entry   reattachEntry
		wantErr string
	}{
		{"unsupported network", grpcUnix("pipe", "grpc"), `unsupported network "pipe"`},
		{"unsupported protocol", grpcUnix("unix", "netrpc"), `unsupported protocol "netrpc"`},
		{"empty protocol", grpcUnix("unix", ""), `unsupported protocol ""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := reattachProvider(context.Background(), addr, tt.entry)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
