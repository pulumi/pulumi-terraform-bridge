package tfgen

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
)

type cachingProviderHost struct {
	plugin.Host

	cache map[string]plugin.Provider
}

func (host *cachingProviderHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	key := fmt.Sprintf("%v@%v", pkg, version)
	provider, ok := host.cache[key]
	if !ok {
		prov, err := host.Host.Provider(pkg, version)
		if err != nil {
			return nil, err
		}
		host.cache[key], provider = prov, prov
	}
	return provider, nil
}
