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

package shim

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	plugin "github.com/hashicorp/go-plugin"
	disco "github.com/hashicorp/terraform-svchost/disco"
	"github.com/opentofu/opentofu/internal/depsfile"
	"github.com/opentofu/opentofu/internal/getproviders"
	"github.com/opentofu/opentofu/internal/logging"
	tfplugin "github.com/opentofu/opentofu/internal/plugin"
	tfplugin6 "github.com/opentofu/opentofu/internal/plugin6"
	"github.com/opentofu/opentofu/internal/providercache"
	"github.com/opentofu/opentofu/internal/providers"
	tfaddr "github.com/opentofu/registry-address"
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

// A loaded and running provider.
//
// You must call Close on any Provider that has been created.
type Provider interface {
	providers.Interface

	Name() string
	Version() string
}

// Load a TF provider with key and version specified.
//
// If version is "", then whatever version is currently installed will be used. If no
// version is installed then the latest version can be used.
//
// `=`, `<=`, `>=` sigils can be used just like in TF.
func LoadProvider(ctx context.Context, key, version string) (Provider, error) {
	p := tfaddr.Provider{
		Type: key,

		// We assume that all providers are hosted in the registry.
		//
		// TODO: We will need to support providers at a specified path at minimum.
		//
		// TODO: If we ever relax the requirement to have one host, we will need
		// to find some way to keep `key & version => provider` true.
		Hostname:  tfaddr.DefaultProviderRegistryHost,
		Namespace: "opentofu",
	}

	v, err := getproviders.ParseVersionConstraints(version)
	if err != nil {
		return nil, err
	}

	return loadProviderServer(ctx, p, v)
}

func getPluginCache() (string, error) {
	if dir := os.Getenv(envPluginCache); dir != "" {
		return dir, nil
	}

	pulumiHome := os.Getenv("PULUMI_HOME")
	if pulumiHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		pulumiHome = filepath.Join(home, ".pulumi")
	}

	return filepath.Join(pulumiHome, "dynamic_tf_plugins"), nil
}

func loadLockFile(path string) (*depsfile.Locks, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return depsfile.NewLocks(), nil
	}

	l, diags := depsfile.LoadLocksFromFile(path)
	if diags.HasErrors() {
		return l, diags.Err()
	}
	// TODO: Don't swallow warnings
	return l, nil
}

type provider struct {
	providers.Interface

	name, version string
}

func (p provider) Name() string { return p.name }

func (p provider) Version() string { return p.version }

func loadProviderServer(
	ctx context.Context, addr tfaddr.Provider, version getproviders.VersionConstraints,
) (Provider, error) {
	cacheDir, err := getPluginCache()
	if err != nil {
		return nil, err
	}
	providersMap := providercache.NewDir(cacheDir)

	// Check if we have addr at version already.
	//
	// If we do, we don't need to invoke the Install machinery. Calling
	// providersMap.AllAvailablePackages()

	installer := providercache.NewInstaller(providersMap, getproviders.NewRegistrySource(disco.New()))

	const lockFile = "/Users/ianwahbe/go/src/github.com/pulumi/pulumi-terraform-bridge/dynamic/test.lock"
	lock, err := loadLockFile(lockFile)
	if err != nil {
		return nil, err
	}

	lock, err = installer.EnsureProviderVersions(ctx, lock, getproviders.Requirements{
		addr: version,
	}, providercache.InstallUpgrades)
	if err != nil {
		return nil, err
	}

	diags := depsfile.SaveLocksToFile(lock, lockFile)
	if diags.HasErrors() {
		return nil, diags.Err()
	}

	p := providersMap.ProviderLatestVersion(addr)
	if p == nil {
		return nil, fmt.Errorf("provider not found in cache: %v", addr)
	}

	i, err := runProvider(p)
	if err != nil {
		return nil, err
	}

	return provider{i, addr.Type, p.Version.String()}, nil
}

// runProvider produces a provider factory that runs up the executable
// file in the given cache package and uses go-plugin to implement
// providers.Interface against it.
func runProvider(meta *providercache.CachedProvider) (providers.Interface, error) {
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
		AutoMTLS:         true,
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
