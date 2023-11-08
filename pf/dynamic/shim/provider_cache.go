package shim

import (
	"fmt"
	"os"
	"os/exec"

	plugin "github.com/hashicorp/go-plugin"
	svchost "github.com/hashicorp/terraform-svchost"
	"github.com/opentofu/opentofu/internal/logging"
	tfplugin "github.com/opentofu/opentofu/internal/plugin"
	tfplugin6 "github.com/opentofu/opentofu/internal/plugin6"
	"github.com/opentofu/opentofu/internal/providercache"
	"github.com/opentofu/opentofu/internal/providers"
	tfaddr "github.com/opentofu/registry-address"
)

var enableProviderAutoMTLS = os.Getenv("TF_DISABLE_PLUGIN_TLS") == ""

func NewProviderCache(path string) ProviderCache {
	return ProviderCache{
		impl: providercache.NewDir(path),
	}
}

type ProviderCache struct {
	impl *providercache.Dir
}

func (p ProviderCache) AllAvailablePackages() map[tfaddr.Provider][]providercache.CachedProvider {
	return p.impl.AllAvailablePackages()
}

func (p ProviderCache) GetProviderFactory(addr tfaddr.Provider) providers.Factory {
	return providerFactory(p.impl.ProviderLatestVersion(addr))
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

type ProviderAddr struct {
	Type      string
	Namespace string
	Hostname  svchost.Hostname
}
