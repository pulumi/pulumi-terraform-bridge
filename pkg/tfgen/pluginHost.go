package tfgen

import (
	"fmt"
	"sync"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

var _ = (plugin.Provider)((*inmemoryProvider)(nil))

type inmemoryProvider struct {
	name   tokens.Package
	schema []byte
	info   tfbridge.ProviderInfo
}

func newInMemoryProvider(name tokens.Package, schema []byte, info tfbridge.ProviderInfo) *inmemoryProvider {
	// Round-trip the info through a marshaler to normalize the types to the schema shim.
	return &inmemoryProvider{
		name:   name,
		schema: schema,
		info:   *tfbridge.MarshalProviderInfo(&info).Unmarshal(),
	}
}

func (p *inmemoryProvider) Pkg() tokens.Package {
	return p.name
}

func (p *inmemoryProvider) GetSchema(version int) ([]byte, error) {
	return p.schema, nil
}

func (p *inmemoryProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	var version *semver.Version
	if p.info.Version != "" {
		v, err := semver.ParseTolerant(p.info.Version)
		if err != nil {
			return workspace.PluginInfo{}, fmt.Errorf("failed to parse pkg %q version: %w", p.name, err)
		}
		version = &v
	}
	return workspace.PluginInfo{
		Name:    p.info.Name,
		Kind:    workspace.ResourcePlugin,
		Version: version,
	}, nil
}

func (p *inmemoryProvider) Call(tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions) (plugin.CallResult, error) {
	panic("unimplemented")
}

func (p *inmemoryProvider) CheckConfig(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {
	panic("unimplemented")
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (p *inmemoryProvider) DiffConfig(urn resource.URN, olds, news resource.PropertyMap, allowUnknowns bool,
	ignoreChanges []string) (plugin.DiffResult, error) {
	panic("unimplemented")
}

// Configure configures the resource provider with "globals" that control its behavior.
func (p *inmemoryProvider) Configure(inputs resource.PropertyMap) error {
	panic("unimplemented")
}

func (p *inmemoryProvider) Check(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool, randomSeed []byte) (resource.PropertyMap, []plugin.CheckFailure, error) {
	panic("unimplemented")
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *inmemoryProvider) Diff(urn resource.URN, id resource.ID, olds resource.PropertyMap, news resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {
	panic("unimplemented")
}

// Create allocates a new instance of the provided resource and returns its unique resource.ID.
func (p *inmemoryProvider) Create(urn resource.URN, news resource.PropertyMap, timeout float64,
	preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {
	panic("unimplemented")
}

// Read the current live state associated with a resource.  Enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource ID, but may also include some properties.  If the
// resource is missing (for instance, because it has been deleted), the resulting property map will be nil.
func (p *inmemoryProvider) Read(urn resource.URN, id resource.ID,
	inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
	panic("unimplemented")
}

// Update updates an existing resource with new values.
func (p *inmemoryProvider) Update(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap, timeout float64, ignoreChanges []string,
	preview bool) (resource.PropertyMap, resource.Status, error) {
	panic("unimplemented")
}

// Delete tears down an existing resource.
func (p *inmemoryProvider) Delete(urn resource.URN, id resource.ID, props resource.PropertyMap,
	timeout float64) (resource.Status, error) {
	panic("unimplemented")
}

// Construct creates a new component resource.
func (p *inmemoryProvider) Construct(info plugin.ConstructInfo, typ tokens.Type, name tokens.QName,
	parent resource.URN, inputs resource.PropertyMap, options plugin.ConstructOptions) (plugin.ConstructResult, error) {
	panic("unimplemented")
}

// Invoke dynamically executes a built-in function in the provider.
func (p *inmemoryProvider) Invoke(tok tokens.ModuleMember, args resource.PropertyMap) (resource.PropertyMap,
	[]plugin.CheckFailure, error) {
	panic("unimplemented")
}

// StreamInvoke dynamically executes a built-in function in the provider, which returns a stream
// of responses.
func (p *inmemoryProvider) StreamInvoke(
	tok tokens.ModuleMember,
	args resource.PropertyMap,
	onNext func(resource.PropertyMap) error) ([]plugin.CheckFailure, error) {
	panic("unimplemented")
}

func (p *inmemoryProvider) Close() error {
	return nil
}
func (p *inmemoryProvider) SignalCancellation() error {
	return nil
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

// ResolvePlugin resolves a plugin kind, name, and optional semver to a candidate plugin
// to load. inmemoryProviderHost does this by checking if the generating provider is being
// loaded. If it is, it returns it's provider. Otherwise, we defer
// inmemoryProviderHost.Host.
func (host *inmemoryProviderHost) ResolvePlugin(kind workspace.PluginKind, name string,
	version *semver.Version) (*workspace.PluginInfo, error) {
	if name == host.provider.name.String() {
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
	key := pkg.String() + "@"
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
