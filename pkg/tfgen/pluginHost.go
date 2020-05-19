package tfgen

import (
	"fmt"
	"sync"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
)

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
	key := fmt.Sprintf("%v@%v", pkg, version)
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
