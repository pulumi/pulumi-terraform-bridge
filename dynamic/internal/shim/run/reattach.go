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
	"fmt"
	"net"
	"os"

	plugin "github.com/hashicorp/go-plugin"
	regaddr "github.com/opentofu/registry-address/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/addrs"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/logging"
	tfplugin "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/plugin"
)

// envReattachProviders instructs the dynamic provider to connect to an
// already-running TF provider server instead of downloading and launching the
// provider binary. Its value uses the same JSON format Terraform and OpenTofu
// accept in TF_REATTACH_PROVIDERS: a map from provider source address to the
// reattach information printed by a provider running in debug mode (or served
// with go-plugin's test mode).
//
// Providers matched here report version "0.0.0"; any version constraint
// requested for them is ignored, since the caller controls the running server.
const envReattachProviders = "PULUMI_BRIDGE_REATTACH_PROVIDERS"

// reattachProviderVersion is the version reported for reattached providers,
// which have no registry version.
const reattachProviderVersion = "0.0.0"

type reattachEntry struct {
	Protocol        string
	ProtocolVersion int
	Pid             int
	Test            bool
	Addr            struct {
		Network string
		String  string
	}
}

// reattachEntryFor returns the reattach entry for addr, if
// PULUMI_BRIDGE_REATTACH_PROVIDERS is set and contains one. Keys are matched
// as provider source addresses, so "simple" and "registry.opentofu.org/-/simple"
// style spellings compare after normalization.
func reattachEntryFor(addr addrs.Provider) (reattachEntry, bool, error) {
	raw := os.Getenv(envReattachProviders)
	if raw == "" {
		return reattachEntry{}, false, nil
	}

	var entries map[string]reattachEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return reattachEntry{}, false, fmt.Errorf("invalid %s: %w", envReattachProviders, err)
	}

	for key, entry := range entries {
		keyAddr, err := regaddr.ParseProviderSource(key)
		if err != nil {
			return reattachEntry{}, false, fmt.Errorf("invalid %s: provider key %q: %w", envReattachProviders, key, err)
		}
		if keyAddr.Equals(addr) {
			return entry, true, nil
		}
	}
	return reattachEntry{}, false, nil
}

// reattachProvider connects to the running provider server described by entry
// instead of launching a provider binary.
func reattachProvider(ctx context.Context, addr addrs.Provider, entry reattachEntry) (Provider, error) {
	var netAddr net.Addr
	switch entry.Addr.Network {
	case "unix":
		netAddr = &net.UnixAddr{Name: entry.Addr.String, Net: "unix"}
	case "tcp":
		var err error
		netAddr, err = net.ResolveTCPAddr("tcp", entry.Addr.String)
		if err != nil {
			return nil, fmt.Errorf("%s: resolving %q for %s: %w", envReattachProviders, entry.Addr.String, addr, err)
		}
	default:
		return nil, fmt.Errorf("%s: unsupported network %q for %s", envReattachProviders, entry.Addr.Network, addr)
	}

	if p := plugin.Protocol(entry.Protocol); p != plugin.ProtocolGRPC {
		return nil, fmt.Errorf("%s: unsupported protocol %q for %s (only %q is supported)",
			envReattachProviders, entry.Protocol, addr, plugin.ProtocolGRPC)
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  tfplugin.Handshake,
		Logger:           logging.NewProviderLogger(""),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Reattach: &plugin.ReattachConfig{
			Protocol:        plugin.ProtocolGRPC,
			ProtocolVersion: entry.ProtocolVersion,
			Addr:            netAddr,
			Pid:             entry.Pid,
			Test:            entry.Test,
		},
	})
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("%s: connecting to %s: %w", envReattachProviders, addr, err)
	}
	grpcClient, ok := rpcClient.(*plugin.GRPCClient)
	if !ok {
		contract.IgnoreClose(rpcClient)
		return nil, fmt.Errorf("%s: expected a gRPC plugin client for %s, got %T", envReattachProviders, addr, rpcClient)
	}

	// The handshake-based negotiation in runProvider does not happen when
	// reattaching, so the protocol version comes from the reattach entry.
	server, err := protov6ServerFromConn(ctx, grpcClient.Conn, entry.ProtocolVersion)
	if err != nil {
		contract.IgnoreClose(grpcClient.Conn)
		return nil, fmt.Errorf("%s: %s: %w", envReattachProviders, addr, err)
	}

	return provider{
		server,
		grpcClient.Conn,
		addr.Type, reattachProviderVersion, addr.String(),
	}, nil
}
