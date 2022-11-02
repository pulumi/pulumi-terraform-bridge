// Copyright 2016-2022, Pulumi Corporation.
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

package tfbridge

import (
	"context"
	"fmt"
	"sync"

	"github.com/blang/semver"
	tfsdkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	tfsdkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
)

// Provider implements the Pulumi resource provider operations for any
// Terraform plugin built with Terraform Plugin Framework.
//
// https://www.terraform.io/plugin/framework
type Provider struct {
	tfProvider      tfsdkprovider.Provider
	tfServer        tfprotov6.ProviderServer
	resourcesByType resourcesByType
	info            info.ProviderInfo
	resourcesCache  resources
	resourcesOnce   sync.Once
	pulumiSchema    []byte
}

var _ plugin.Provider = &Provider{}

func NewProvider(info info.ProviderInfo, pulumiSchema []byte) plugin.Provider {
	p := info.P()
	server6 := providerserver.NewProtocol6(p)
	return &Provider{
		tfProvider:   p,
		tfServer:     server6(),
		info:         info,
		pulumiSchema: pulumiSchema,
	}
}

func NewProviderServer(info info.ProviderInfo, pulumiSchema []byte) pulumirpc.ResourceProviderServer {
	return plugin.NewProviderServer(NewProvider(info, pulumiSchema))
}

// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
func (p *Provider) Close() error {
	panic("TODO")
}

// Pkg fetches this provider's package.
func (p *Provider) Pkg() tokens.Package {
	panic("TODO")
}

// GetSchema returns the schema for the provider.
func (p *Provider) GetSchema(version int) ([]byte, error) {
	return p.pulumiSchema, nil
}

// CheckConfig validates the configuration for this resource provider.
func (p *Provider) CheckConfig(urn resource.URN,
	olds, news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {
	// TODO proper implementation here.
	return news, []plugin.CheckFailure{}, nil
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (p *Provider) DiffConfig(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {

	// TODO proper implementation here.
	return plugin.DiffResult{}, nil
}

// Configure configures the resource provider with "globals" that control its behavior.
func (p *Provider) Configure(inputs resource.PropertyMap) error {
	// TODO actually configure
	return nil
}

// Construct creates a new component resource.
func (p *Provider) Construct(info plugin.ConstructInfo, typ tokens.Type, name tokens.QName, parent resource.URN,
	inputs resource.PropertyMap, options plugin.ConstructOptions) (plugin.ConstructResult, error) {
	return plugin.ConstructResult{},
		fmt.Errorf("Construct is not implemented for Terraform Plugin Framework bridged providers")
}

// Invoke dynamically executes a built-in function in the provider.
func (p *Provider) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
	panic("TODO")
}

// StreamInvoke dynamically executes a built-in function in the provider, which returns a stream of responses.
func (p *Provider) StreamInvoke(tok tokens.ModuleMember, args resource.PropertyMap,
	onNext func(resource.PropertyMap) error) ([]plugin.CheckFailure, error) {
	panic("TODO")
}

// Call dynamically executes a method in the provider associated with a component resource.
func (p *Provider) Call(tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions) (plugin.CallResult, error) {
	return plugin.CallResult{},
		fmt.Errorf("Call is not implemented for Terraform Plugin Framework bridged providers")
}

// GetPluginInfo returns this plugin's information.
func (p *Provider) GetPluginInfo() (workspace.PluginInfo, error) {
	ver, err := semver.Parse(p.info.Version)
	if err != nil {
		return workspace.PluginInfo{}, err
	}
	info := workspace.PluginInfo{
		Name:    p.info.Name,
		Version: &ver,
		Kind:    workspace.ResourcePlugin,
	}
	return info, nil
}

// SignalCancellation asks all resource providers to gracefully shut down and abort any ongoing operations. Operation
// aborted in this way will return an error (e.g., `Update` and `Create` will either a creation error or an
// initialization error. SignalCancellation is advisory and non-blocking; it is up to the host to decide how long to
// wait after SignalCancellation is called before (e.g.) hard-closing any gRPC connection.
func (p *Provider) SignalCancellation() error {
	return nil
}

func (p *Provider) terraformResourceName(resourceToken tokens.Type) (string, error) {
	for tfname, v := range p.info.Resources {
		if v.Tok == resourceToken {
			return tfname, nil
		}
	}
	return "", fmt.Errorf("Unkonwn resource: %v", resourceToken)
}

// func (p *Provider) findResource(ctx context.Context, token tokens.Type) {
// 	for _, makeResource := range p.tfProvider.Resources(ctx) {
// 		res := makeResource()
// 		schema := res.GetSchema(ctx)
// 		schema.
// 	}
// }

type resourcesByType map[tokens.Type]tfsdkresource.Resource

func (rbt resourcesByType) ByURN(urn resource.URN) (tfsdkresource.Resource, error) {
	r, ok := rbt[urn.Type()]
	if !ok {
		return nil, fmt.Errorf("unrecognized resource type: %s", urn.Type())
	}
	return r, nil
}

func newResourcesByType(ctx context.Context, prov *tfsdkprovider.Provider) resourcesByType {
	panic("TODO")
}
