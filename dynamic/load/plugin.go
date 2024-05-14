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

package load

import (
	"fmt"
	"os"

	plugin "github.com/hashicorp/go-plugin"
	tfaddr "github.com/opentofu/registry-address"

	"github.com/opentofu/opentofu/shim/providercache"
	"github.com/opentofu/opentofu/shim/providers"
)

// TODO:
//
// - If TF is setup and has already cached a provider, we should try to use that
// provider.
//
// - If not, but TF is setup we should probably contribute to the existing cache.
//
// - If TF is not setup, we should cache within PULUMI_HOME to avoid creating new dirs.
const envPluginCache = "TF_PLUGIN_CACHE_DIR"

func Provider(key, version string) {
	p := tfaddr.Provider{Type: key}
}

func loadProviderServer(addr tfaddr.Provider) (providers.Interface, error) {
	providerCacheDir := os.Getenv(envPluginCache)
	providersMap := providercache.NewDir(providerCacheDir)

	p := providersMap.ProviderLatestVersion(addr)
	if p == nil {
		return nil, fmt.Errorf("provider not found in cache: %v\n", addr)
	}

	return providerfactory(p)
}

// providerFactory produces a provider factory that runs up the executable
// file in the given cache package and uses go-plugin to implement
// providers.Interface against it.
func providerFactory(meta *providercache.CachedProvider) providers.Factory {
	return func() (providers.Interface, error) {
		execFile, err := meta.ExecutableFile()
		if err != nil {
			return nil, err
		}

		config := &plugin.ClientConfig{
			HandshakeConfig:  tfplugin.Handshake,
			Logger:           logging.NewProviderLogger(""),
			AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
			Managed:          true,
			Cmd:              exec.Command(execFile),
			AutoMTLS:         enableProviderAutoMTLS,
			VersionedPlugins: tfplugin.VersionedPlugins,
			SyncStdout:       logging.PluginOutputMonitor(fmt.Sprintf("%s:stdout", meta.Provider)),
			SyncStderr:       logging.PluginOutputMonitor(fmt.Sprintf("%s:stderr", meta.Provider)),
		}

		client := plugin.NewClient(config)
		rpcClient, err := client.Client()
		if err != nil {
			return nil, err
		}

		raw, err := rpcClient.Dispense(tfplugin.ProviderPluginName)
		if err != nil {
			return nil, err
		}

		// store the client so that the plugin can kill the child process
		protoVer := client.NegotiatedVersion()
		switch protoVer {
		case 5:
			p := raw.(*tfplugin.GRPCProvider)
			p.PluginClient = client
			p.Addr = meta.Provider
			return p, nil
		case 6:
			p := raw.(*tfplugin6.GRPCProvider)
			p.PluginClient = client
			p.Addr = meta.Provider
			return p, nil
		default:
			panic("unsupported protocol version")
		}
	}
}
