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
	"time"

	"github.com/apparentlymart/go-versions/versions"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	regaddr "github.com/opentofu/registry-address/v2"
	disco "github.com/opentofu/svchost/disco"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v5shim "github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/protov5"
	v6shim "github.com/pulumi/pulumi-terraform-bridge/v3/dynamic/internal/shim/protov6"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/addrs"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/getproviders"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/logging"
	tfplugin "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/plugin"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/providercache"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin5"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6"
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
	p, err := regaddr.ParseProviderSource(key)
	if err != nil {
		return nil, fmt.Errorf("invalid provider name: %w", err)
	}

	v, err := getproviders.ParseVersionConstraints(version)
	if err != nil {
		return nil, fmt.Errorf("could not parse version constraint %q: %w", version, err)
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
		Provider:   addrs.Provider{Type: name},
		Version:    versions.Version{},
		PackageDir: dir,
	})
}

// etxtbsyMaxAttempts is the maximum number of times startPluginClient will
// retry plugin start in the face of ETXTBSY. Worst-case total wait between
// attempts with the default backoff is 350ms.
const etxtbsyMaxAttempts = 4

// etxtbsyBackoff is the sleep before the (attempt+1)'th attempt; attempt 0 has
// no preceding sleep. It is a package variable so tests can override it.
var etxtbsyBackoff = func(attempt int) time.Duration {
	return time.Duration(50*(1<<(attempt-1))) * time.Millisecond
}

// startPluginClient launches the cached provider via go-plugin, retrying when
// the kernel returns ETXTBSY ("text file busy") at fork+exec.
//
// On Linux, execve fails with ETXTBSY if any process holds the binary open for
// write at the moment of exec. The dynamic bridge writes each provider binary
// to its cache directory and then immediately exec's it; under concurrent
// pulumi install (parallel >= 2 packages) the install path occasionally leaves
// a brief open-for-write window that overlaps the exec, surfacing as
//
//	fork/exec <provider>: text file busy
//
// see https://github.com/pulumi/pulumi-terraform-bridge/issues/3425. The race
// is sub-millisecond, so a small bounded retry resolves it cleanly. macOS does
// not enforce ETXTBSY, so this path is effectively a no-op there.
//
// plugin.ClientConfig must be rebuilt on each attempt because exec.Cmd is
// single-use after Start, and plugin.NewClient takes ownership of the Cmd.
func startPluginClient(
	ctx context.Context, meta *providercache.CachedProvider, execFile string,
) (*plugin.Client, plugin.ClientProtocol, error) {
	newConfig := func() *plugin.ClientConfig {
		return &plugin.ClientConfig{
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
			GRPCDialOptions: []grpc.DialOption{
				grpc.WithUnaryInterceptor(includePanic),
			},
		}
	}

	return retryOnTextFileBusy(ctx, meta.Provider.String(), func() (*plugin.Client, plugin.ClientProtocol, error) {
		client := plugin.NewClient(newConfig())
		rpcClient, err := client.Client()
		if err != nil {
			client.Kill()
			return nil, nil, err
		}
		return client, rpcClient, nil
	})
}

// pluginStarter constructs a fresh plugin.Client and attempts to start it,
// returning the running client + protocol on success, or an error.
type pluginStarter func() (*plugin.Client, plugin.ClientProtocol, error)

// retryOnTextFileBusy invokes start in a bounded loop, retrying only when
// start returns an ETXTBSY error. All other errors are returned immediately.
//
// Split out from startPluginClient so the retry policy can be exercised by
// unit tests without spinning up real plugin processes.
func retryOnTextFileBusy(
	ctx context.Context, providerName string, start pluginStarter,
) (*plugin.Client, plugin.ClientProtocol, error) {
	var lastErr error
	for attempt := 0; attempt < etxtbsyMaxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(etxtbsyBackoff(attempt))
		}
		client, rpcClient, err := start()
		if err == nil {
			return client, rpcClient, nil
		}
		if !isTextFileBusy(err) {
			return nil, nil, err
		}
		lastErr = err
		slog.InfoContext(ctx, "provider exec hit ETXTBSY, retrying",
			slog.String("provider", providerName),
			slog.Int("attempt", attempt+1),
			slog.String("error", err.Error()))
	}
	return nil, nil, fmt.Errorf("provider %s exec failed with ETXTBSY after %d attempts: %w",
		providerName, etxtbsyMaxAttempts, lastErr)
}

// isTextFileBusy reports whether err originated from execve returning ETXTBSY.
// We match on the string because go-plugin wraps the underlying *exec.Error
// from cmd.Start, losing the typed syscall errno.
func isTextFileBusy(err error) bool {
	return err != nil && strings.Contains(err.Error(), "text file busy")
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
	ctx context.Context, addr addrs.Provider, version getproviders.VersionConstraints,
	registryDisco *disco.Disco,
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

	source := getproviders.NewRegistrySource(ctx, registryDisco, nil, getproviders.LocationConfig{})

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

	_, err = systemCache.InstallPackage(ctx, meta, nil, true)
	if err != nil {
		return nil, err
	}

	p := systemCache.ProviderVersion(addr, desiredVersion)
	contract.Assertf(p != nil, "We just downloaded (%s,%s) so it should be in the cache", addr, desiredVersion)

	return runProvider(ctx, p)
}

func includePanic(
	ctx context.Context, method string,
	req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	err := invoker(ctx, method, req, reply, cc, opts...)
	if status.Code(err) != codes.Unavailable {
		return nil
	}

	panics := logging.PluginPanics()
	if len(panics) == 0 {
		return err
	}

	return fmt.Errorf("%w:\n%s", err, strings.Join(panics, "\n"))
}

// runProvider produces a provider factory that runs up the executable
// file in the given cache package and uses go-plugin to implement
// providers.Interface against it.
func runProvider(ctx context.Context, meta *providercache.CachedProvider) (Provider, error) {
	execFile, err := meta.ExecutableFile()
	if err != nil {
		return nil, err
	}

	client, rpcClient, err := startPluginClient(ctx, meta, execFile)
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
		return provider{
			v6,
			meta.Provider.Type, meta.Version.String(), meta.Provider.String(),
			rpcClient.Close,
		}, nil
	case 6:
		p := tfplugin6.NewProviderClient(rpcClient.(*plugin.GRPCClient).Conn)
		return provider{
			v6shim.New(p),
			meta.Provider.Type, meta.Version.String(), meta.Provider.String(),
			rpcClient.Close,
		}, nil
	default:
		panic("unsupported protocol version")
	}
}
