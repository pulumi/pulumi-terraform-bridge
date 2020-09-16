// Copyright 2016-2018, Pulumi Corporation.
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

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/resource/provider"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/plugins"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfx/registry"
)

func New(cache *plugins.Cache, host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
	if cache == nil {
		c, err := plugins.DefaultCache()
		if err != nil {
			return nil, err
		}
		cache = c
	}

	cancelContext, cancel := context.WithCancel(context.Background())
	return &tfxProvider{
		cancelContext: cancelContext,
		cancel:        cancel,
		host:          host,
		cache:         cache,
		providers:     map[string]*tfbridge.Provider{},
	}, nil
}

type tfxProvider struct {
	m sync.RWMutex

	cancelContext context.Context
	cancel        func()
	host          *provider.HostClient
	cache         *plugins.Cache
	providers     map[string]*tfbridge.Provider
}

func (p *tfxProvider) getProvider(ref string) (*tfbridge.Provider, bool) {
	p.m.RLock()
	defer p.m.RUnlock()

	provider, ok := p.providers[ref]
	return provider, ok
}

func (p *tfxProvider) loadProvider(ctx context.Context, meta plugins.PluginMeta,
	config map[string]string) (*tfbridge.Provider, error) {

	registry, err := registry.NewClient(meta.RegistryName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to provider registry: %v", err)
	}

	pluginMeta, err := p.cache.GetPlugin(registry, meta.Namespace, meta.Name, meta.Version.EQ)
	if err != nil {
		return nil, fmt.Errorf("failed to open Terraform provider %v", meta)
	}
	if pluginMeta == nil {
		return nil, fmt.Errorf("Terraform provider %v is not installed", meta)
	}

	info, err := StartProvider(p.cancelContext, *pluginMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to start Terraform provider: %w", err)
	}

	provider := tfbridge.NewProvider(p.cancelContext, p.host, pluginMeta.Name, pluginMeta.Version.String(),
		info.P, info, nil)

	if _, err = provider.Configure(ctx, &pulumirpc.ConfigureRequest{
		Variables: config,
	}); err != nil {
		_, cancelErr := provider.Cancel(ctx, &pbempty.Empty{})
		contract.IgnoreError(cancelErr)
		return nil, err
	}

	p.m.Lock()
	defer p.m.Unlock()

	p.providers[meta.String()] = provider
	return provider, nil
}

func (p *tfxProvider) ensureProvider(ctx context.Context, ref string) (*tfbridge.Provider, error) {
	if provider, ok := p.getProvider(ref); ok {
		return provider, nil
	}

	meta, err := plugins.ParsePluginReference(ref)
	if err != nil {
		return nil, fmt.Errorf("malformed provider reference: %w", err)
	}
	if meta.Version == nil {
		return nil, fmt.Errorf("provider reference %v does not include a version", ref)
	}

	return p.loadProvider(ctx, *meta, nil)
}

// GetSchema returns the JSON-encoded schema for this provider's package.
func (p *tfxProvider) GetSchema(ctx context.Context,
	req *pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetSchema is not yet implemented")
}

// CheckConfig validates the configuration for this Terraform provider.
func (p *tfxProvider) CheckConfig(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CheckConfig is not yet implemented")
}

// DiffConfig diffs the configuration for this Terraform provider.
func (p *tfxProvider) DiffConfig(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DiffConfig is not yet implemented")
}

// Configure configures the underlying Terraform provider with the live Pulumi variable state.
func (p *tfxProvider) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {

	// Break the request up into its constituent provider configuration blocks.
	for k, v := range req.GetVariables() {
		mm, err := tokens.ParseModuleMember(k)
		if err != nil {
			return nil, fmt.Errorf("malformed configuration token '%v'", k)
		}

		meta, err := plugins.ParsePluginReference(string(mm.Name()))
		if err != nil {
			return nil, fmt.Errorf("malformed provider reference: %w", err)
		}

		var object map[string]interface{}
		if err := json.Unmarshal([]byte(v), &object); err != nil {
			return nil, fmt.Errorf("%v: '%v' is not a valid configuration object", k, v)
		}

		config := map[string]string{}
		for subkey, v := range object {
			strval := ""
			switch v := v.(type) {
			case []interface{}, map[string]interface{}:
				bytes, err := json.Marshal(v)
				if err != nil {
					return nil, fmt.Errorf("%v: failed to marshal property %v: %w", k, subkey, err)
				}
				strval = string(bytes)
			default:
				strval = fmt.Sprintf("%v", v)
			}

			config[meta.Name+":config:"+subkey] = strval
		}

		if _, err = p.loadProvider(ctx, *meta, config); err != nil {
			return nil, fmt.Errorf("failed to configure Terraform provider %v: %w", meta, err)
		}
	}

	return &pulumirpc.ConfigureResponse{}, nil
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *tfxProvider) Check(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	urn := resource.URN(req.Urn)
	provider, err := p.ensureProvider(ctx, string(urn.Type().Module().Name()))
	if err != nil {
		return nil, err
	}
	return provider.Check(ctx, req)
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *tfxProvider) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	urn := resource.URN(req.Urn)
	provider, err := p.ensureProvider(ctx, string(urn.Type().Module().Name()))
	if err != nil {
		return nil, err
	}
	return provider.Diff(ctx, req)
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transactional").
func (p *tfxProvider) Create(ctx context.Context, req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
	urn := resource.URN(req.Urn)
	provider, err := p.ensureProvider(ctx, string(urn.Type().Module().Name()))
	if err != nil {
		return nil, err
	}
	return provider.Create(ctx, req)
}

// Read the current live state associated with a resource.  Enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource ID, but may also include some properties.
func (p *tfxProvider) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	urn := resource.URN(req.Urn)
	provider, err := p.ensureProvider(ctx, string(urn.Type().Module().Name()))
	if err != nil {
		return nil, err
	}
	return provider.Read(ctx, req)
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *tfxProvider) Update(ctx context.Context, req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
	urn := resource.URN(req.Urn)
	provider, err := p.ensureProvider(ctx, string(urn.Type().Module().Name()))
	if err != nil {
		return nil, err
	}
	return provider.Update(ctx, req)
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *tfxProvider) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*pbempty.Empty, error) {
	urn := resource.URN(req.Urn)
	provider, err := p.ensureProvider(ctx, string(urn.Type().Module().Name()))
	if err != nil {
		return nil, err
	}
	return provider.Delete(ctx, req)
}

// Construct creates a new instance of the provided component resource and returns its state.
func (p *tfxProvider) Construct(context.Context, *pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Construct is not yet implemented")
}

// Invoke dynamically executes a built-in function in the provider.
func (p *tfxProvider) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	token := tokens.ModuleMember(req.Tok)
	provider, err := p.ensureProvider(ctx, string(token.Module().Name()))
	if err != nil {
		return nil, err
	}
	return provider.Invoke(ctx, req)
}

// StreamInvoke dynamically executes a built-in function in the provider. The result is streamed
// back as a series of messages.
func (p *tfxProvider) StreamInvoke(
	req *pulumirpc.InvokeRequest, server pulumirpc.ResourceProvider_StreamInvokeServer) error {

	tok := tokens.ModuleMember(req.GetTok())
	return errors.Errorf("unrecognized data function (StreamInvoke): %s", tok)
}

// GetPluginInfo implements an RPC call that returns the version of this plugin.
func (p *tfxProvider) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: "0.0.1",
	}, nil
}

// Cancel requests that the provider cancel all ongoing RPCs. For TF, this is a no-op.
func (p *tfxProvider) Cancel(ctx context.Context, req *pbempty.Empty) (*pbempty.Empty, error) {
	p.cancel()
	return &pbempty.Empty{}, nil
}
