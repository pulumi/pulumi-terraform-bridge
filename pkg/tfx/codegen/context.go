package codegen

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfgen"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/plugins"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/provider"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/registry"
)

type schemaProvider struct {
	plugin.Provider

	schema []byte
}

func (p *schemaProvider) GetSchema(version int) ([]byte, error) {
	if version >= 1 {
		return nil, fmt.Errorf("unsupported version %v", version)
	}
	return p.schema, nil
}

type Context struct {
	plugin.Host

	context    context.Context
	cache      *plugins.Cache
	progress   plugins.ProgressFunc
	pluginMeta map[string]plugins.PluginMeta
}

type ContextOptions struct {
	Host       plugin.Host
	Progress   plugins.ProgressFunc
	PluginMeta map[string]plugins.PluginMeta
}

func NewContext(ctx context.Context, cache *plugins.Cache, opts ContextOptions) (*Context, error) {
	host := opts.Host
	if host == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		ctx, err := plugin.NewContext(nil, nil, nil, nil, cwd, nil, nil)
		if err != nil {
			return nil, err
		}
		host = ctx.Host
	}
	if cache == nil {
		c, err := plugins.DefaultCache()
		if err != nil {
			return nil, fmt.Errorf("could not open cache: %w", err)
		}
		cache = c
	}
	return &Context{
		Host:       host,
		context:    ctx,
		cache:      cache,
		progress:   opts.Progress,
		pluginMeta: opts.PluginMeta,
	}, nil
}

func (c *Context) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	meta, err := plugins.ParsePluginReference(string(pkg))
	if err != nil {
		return c.Host.Provider(pkg, version)
	}

	reg, err := registry.NewClient(meta.RegistryName)
	if err != nil {
		return nil, fmt.Errorf("could not connect to registry %v: %w", meta.RegistryName, err)
	}

	pluginMeta, err := c.cache.GetPlugin(reg, meta.Namespace, meta.Name, meta.Version.EQ)
	if err != nil {
		return nil, fmt.Errorf("could not get or install plugin %v: %w", meta, err)
	}

	cancelContext, cancel := context.WithCancel(c.context)
	defer cancel()

	info, err := provider.StartProvider(cancelContext, *pluginMeta)
	if err != nil {
		return nil, fmt.Errorf("could not start provider %v: %w", pluginMeta, err)
	}

	// Rewrite tokens for codegen. Normally, the tokens are of the form "tfx:<provider ref>:<member name>" so that the
	// engine will load the correct plugin, but for codegen we want them to be of the form
	// "<provider name>::<member name>" so that we generate more correct code.
	for _, r := range info.Resources {
		r.Tok = tokens.Type(fmt.Sprintf("%v::%v", meta.Name, r.Tok.Name()))
	}
	for _, d := range info.DataSources {
		d.Tok = tokens.ModuleMember(fmt.Sprintf("%v::%v", meta.Name, d.Tok.Name()))
	}

	sink := diag.DefaultSink(ioutil.Discard, ioutil.Discard, diag.FormatOptions{
		Color: colors.Never,
	})
	schema, err := tfgen.GenerateSchema(info, sink)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema for %v: %w", meta, err)
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema for %v: %w", meta, err)
	}

	return &schemaProvider{schema: schemaBytes}, nil
}

func (c *Context) GetProviderInfo(registryName, namespace, name, version string) (*tfbridge.ProviderInfo, error) {
	meta, ok := c.pluginMeta[name]
	if !ok {
		meta := plugins.PluginMeta{
			RegistryName: registry.DefaultName,
			Namespace:    plugins.DefaultNamespace,
			Name:         name,
		}
		if registryName != "" {
			meta.RegistryName = registryName
		}
		if namespace != "" {
			meta.Namespace = namespace
		}
		if version != "" {
			sv, err := semver.ParseTolerant(version)
			if err != nil {
				return nil, fmt.Errorf("failed to parse version: %w", err)
			}
			meta.Version = &sv
		}
	}

	reg, err := registry.NewClient(meta.RegistryName)
	if err != nil {
		return nil, fmt.Errorf("could not connect to registry %v: %w", meta.RegistryName, err)
	}

	versionRange := func(v semver.Version) bool { return true }
	if meta.Version != nil {
		versionRange = meta.Version.EQ
	}

	pluginMeta, err := c.cache.EnsurePlugin(reg, meta.Namespace, meta.Name, versionRange, c.progress)
	if err != nil {
		return nil, fmt.Errorf("could not get or install plugin %v: %w", meta, err)
	}

	info, err := provider.StartProvider(c.context, *pluginMeta)
	if err != nil {
		return nil, fmt.Errorf("could not start provider %v: %w", pluginMeta, err)
	}

	// Rewrite tokens for Context.Provider. Normally, the tokens are of the form "tfx:<provider ref>:<member name>" so
	// that the engine will load the correct plugin, but for Context.Provider we want them to be of the form
	// "<provider ref>::<member name>" so that we can distinguish TFX providers from other providers.
	for _, r := range info.Resources {
		r.Tok = tokens.Type(fmt.Sprintf("%v::%v", meta, r.Tok.Name()))
	}
	for _, d := range info.DataSources {
		d.Tok = tokens.ModuleMember(fmt.Sprintf("%v::%v", meta, d.Tok.Name()))
	}

	return &info, nil
}
