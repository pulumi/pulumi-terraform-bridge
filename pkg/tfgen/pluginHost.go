package tfgen

import (
	"fmt"
	"sync"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

type inmemoryProvider struct {
	plugin.Provider

	name   string
	schema []byte
	info   tfbridge.ProviderInfo
}

func newInMemoryProvider(name string, schema []byte, info tfbridge.ProviderInfo) *inmemoryProvider {
	// Round-trip the info through a marshaler to normalize the types to the schema shim.
	return &inmemoryProvider{
		name:   name,
		schema: schema,
		info:   *tfbridge.MarshalProviderInfo(&info).Unmarshal(),
	}
}

func (p *inmemoryProvider) Pkg() tokens.Package {
	return tokens.Package(p.name)
}

func (p *inmemoryProvider) GetSchema(version int) ([]byte, error) {
	return p.schema, nil
}

func (p *inmemoryProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	var version *semver.Version
	if p.info.Version != "" {
		v, err := semver.ParseTolerant(p.info.Version)
		if err != nil {
			return workspace.PluginInfo{}, fmt.Errorf("failed to parse pkg version: %w", err)
		}
		version = &v
	}
	return workspace.PluginInfo{
		Name:              p.info.Name,
		Kind:              workspace.ResourcePlugin,
		Version:           version,
		PluginDownloadURL: p.info.PluginDownloadURL,
	}, nil
}

type inmemoryProviderHost struct {
	plugin.Host
	il.ProviderInfoSource

	provider *inmemoryProvider
}

func (host *inmemoryProviderHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	if pkg == host.provider.Pkg() {
		return host.provider, nil
	}
	return host.Host.Provider(pkg, version)
}

func (host *inmemoryProviderHost) ResolvePlugin(kind workspace.PluginKind, name string, version *semver.Version) (*workspace.PluginInfo, error) {
	if name == host.provider.name {
		info, err := host.provider.GetPluginInfo()
		if err != nil {
			return nil, err
		}
		return &info, nil
	}
	return host.Host.ResolvePlugin(kind, name, version)
}

func (host *inmemoryProviderHost) GetProviderInfo(
	registryName, namespace, name, version string) (*tfbridge.ProviderInfo, error) {

	if name == il.GetTerraformProviderName(host.provider.info) {
		return &host.provider.info, nil
	}
	return host.ProviderInfoSource.GetProviderInfo(registryName, namespace, name, version)
}

type cachingProviderHost struct {
	plugin.Host

	m     sync.RWMutex
	cache map[string]plugin.Provider
}

func (host *cachingProviderHost) getProvider(key string) (plugin.Provider, bool) {
	host.m.RLock()
	defer host.m.RUnlock()

	provider, ok := host.cache[key]
	return provider, ok
}

func (host *cachingProviderHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	key := string(pkg) + "@"
	if version != nil {
		key = version.String()
	}
	if provider, ok := host.getProvider(key); ok {
		return provider, nil
	}

	host.m.Lock()
	defer host.m.Unlock()

	provider, err := host.Host.Provider(pkg, version)
	if err != nil {
		return nil, err
	}
	host.cache[key] = provider
	return provider, nil
}
