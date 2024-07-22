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

package run

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/apparentlymart/go-versions/versions"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	disco "github.com/hashicorp/terraform-svchost/disco"
	"github.com/opentofu/opentofu/internal/getproviders"
	"github.com/opentofu/opentofu/internal/logging"
	tfplugin "github.com/opentofu/opentofu/internal/plugin"
	"github.com/opentofu/opentofu/internal/providercache"
	"github.com/opentofu/opentofu/internal/tfplugin5"
	"github.com/opentofu/opentofu/internal/tfplugin6"
	v5shim "github.com/opentofu/opentofu/shim/protov5"
	v6shim "github.com/opentofu/opentofu/shim/protov6"
	tfaddr "github.com/opentofu/registry-address"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// envPluginCache allows users to override where we cache TF providers used by
// pulumi-terraform-bridge's dynamic providers.
//
// It defaults to `$PULUMI_HOME/dynamic_tf_plugins`.
const envPluginCache = "PULUMI_DYNAMIC_TF_PLUGIN_CACHE_DIR"

// A loaded and running provider.
//
// You must call Close on any Provider that has been created.
type Provider interface {
	tfprotov6.ProviderServer
	io.Closer

	Name() string
	URL() string
	Version() string
}

// NamedProvider loads a TF provider with key and version specified.
//
// If version is "", then whatever version is currently installed will be used. If no
// version is installed then the latest version can be used.
//
// `=`, `<=`, `>=` sigils can be used just like in TF.
func NamedProvider(ctx context.Context, key, version string) (Provider, error) {

	p, err := tfaddr.ParseProviderSource(key)
	if err != nil {
		return nil, fmt.Errorf("invalid provider name: %w", err)
	}

	v, err := getproviders.ParseVersionConstraints(version)
	if err != nil {
		return nil, err
	}

	return getProviderServer(ctx, p, v, disco.New())
}

// LocalProvider runs a provider by it's path.
func LocalProvider(ctx context.Context, path string) (Provider, error) {
	dir, name, ok := cutLast(path, "terraform-provider-")
	if !ok {
		return nil, fmt.Errorf("expected path to end with %q", "terraform-provider-${NAME}")
	}

	return runProvider(ctx, &providercache.CachedProvider{
		Provider:   tfaddr.Provider{Type: name},
		Version:    versions.Version{},
		PackageDir: dir,
	})
}

func cutLast(s, sep string) (string, string, bool) {
	if i := strings.LastIndex(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}

	return s, "", false
}

func getPluginCache() (string, error) {
	if dir := os.Getenv(envPluginCache); dir != "" {
		return dir, nil
	}

	return workspace.GetPulumiPath("dynamic_tf_plugins")
}

type provider struct {
	tfprotov6.ProviderServer

	name, version, url string

	close func() error
}

func (p provider) Name() string { return p.name }

func (p provider) Version() string { return p.version }

func (p provider) URL() string { return p.url }

func (p provider) Close() error { return p.close() }

func getProviderServer(
	ctx context.Context, addr tfaddr.Provider, version getproviders.VersionConstraints,
	registrySource *disco.Disco,
) (Provider, error) {
	cacheDir, err := getPluginCache()
	if err != nil {
		return nil, err
	}

	systemCache := providercache.NewDir(cacheDir)

	// Look through existing packages to see if the package we want is already downloaded.
	if packages, ok := systemCache.AllAvailablePackages()[addr]; ok {

		// packages is sorted by precedence, so the first cached result is safe to
		// use.
		acceptable := versions.MeetingConstraints(version)
		for _, p := range packages {
			if acceptable.Has(p.Version) {
				slog.InfoContext(ctx, "Found cached provider",
					slog.Any("addr", addr.String()),
					slog.Any("version", p.Version.String()))
				p := p
				return runProvider(ctx, &p)
			}
		}
	}

	// We have not found a package that fits our constraints, so we need to download
	// one.

	source := getproviders.NewRegistrySource(registrySource)

	availableVersions, warnings, err := source.AvailableVersions(ctx, addr)
	for _, w := range warnings {
		tfbridge.GetLogger(ctx).Warn(w)
	}
	if err != nil {
		// TODO Handle error kinds with distinct error messages
		return nil, err
	}

	desiredVersion := availableVersions.NewestInSet(versions.MeetingConstraints(version))
	if desiredVersion == versions.Unspecified {
		return nil, fmt.Errorf("Could not resolve a version from %s: %s", addr, version)
	}

	meta, err := source.PackageMeta(ctx, addr, desiredVersion, getproviders.CurrentPlatform)
	if err != nil {
		return nil, err
	}

	_, err = systemCache.InstallPackage(ctx, meta, nil)
	if err != nil {
		return nil, err
	}

	p := systemCache.ProviderVersion(addr, desiredVersion)
	contract.Assertf(p != nil, "We just downloaded (%s,%s) so it should be in the cache", addr, desiredVersion)

	return runProvider(ctx, p)
}

// runProvider produces a provider factory that runs up the executable
// file in the given cache package and uses go-plugin to implement
// providers.Interface against it.
func runProvider(ctx context.Context, meta *providercache.CachedProvider) (Provider, error) {
	execFile, err := meta.ExecutableFile()
	if err != nil {
		return nil, err
	}

	config := &plugin.ClientConfig{
		HandshakeConfig:  tfplugin.Handshake,
		Logger:           logging.NewProviderLogger(""),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Managed:          true,
		// We intentionally use [context.Background] so the lifetime of the
		// provider can escape the lifetime of the parameterize call.
		Cmd:              exec.CommandContext(context.Background(), execFile),
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

	switch client.NegotiatedVersion() {
	case 5:
		p := raw.(*tfplugin.GRPCProvider)
		p.PluginClient = client
		p.Addr = meta.Provider

		slog.Info("Found v5 provider.. upgrading to v6")

		v6, err := tf5to6server.UpgradeServer(ctx, func() tfprotov5.ProviderServer {
			return v5shim.New(tfplugin5.NewProviderClient(rpcClient.(*plugin.GRPCClient).Conn))
		})
		if err != nil {
			return nil, err
		}
		return provider{v6,
			meta.Provider.Type, meta.Version.String(), meta.Provider.String(),
			rpcClient.Close,
		}, nil
	case 6:
		p := tfplugin6.NewProviderClient(rpcClient.(*plugin.GRPCClient).Conn)
		return provider{v6shim.New(p),
			meta.Provider.Type, meta.Version.String(), meta.Provider.String(),
			rpcClient.Close,
		}, nil
	default:
		panic("unsupported protocol version")
	}
}
