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
	"fmt"

	pafprovider "github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Provider implements the Pulumi resource provider operations for any
// Terraform plugin built with Terraform Plugin Framework.
//
// https://www.terraform.io/plugin/framework
type Provider struct {
	tfProvider pafprovider.Provider
}

var _ plugin.Provider = &Provider{}

func NewProvider(tfProvider pafprovider.Provider) plugin.Provider {
	return &Provider{
		tfProvider: tfProvider,
	}
}

func NewProviderServer(tfProvider pafprovider.Provider) pulumirpc.ResourceProviderServer {
	return plugin.NewProviderServer(NewProvider(tfProvider))
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
	panic("TODO")
}

// CheckConfig validates the configuration for this resource provider.
func (p *Provider) CheckConfig(urn resource.URN,
	olds, news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {
	panic("TODO")
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (p *Provider) DiffConfig(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {
	panic("TODO")
}

// Configure configures the resource provider with "globals" that control its behavior.
func (p *Provider) Configure(inputs resource.PropertyMap) error {
	panic("TODO")
}

// Check validates that the given property bag is valid for a resource of the given type and returns the inputs that
// should be passed to successive calls to Diff, Create, or Update for this resource.
func (p *Provider) Check(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool, randomSeed []byte) (resource.PropertyMap, []plugin.CheckFailure, error) {
	panic("TODO")
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *Provider) Diff(urn resource.URN, id resource.ID, olds resource.PropertyMap,
	news resource.PropertyMap, allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {
	panic("TODO")
}

// Create allocates a new instance of the provided resource and returns its unique resource.ID.
func (p *Provider) Create(urn resource.URN, news resource.PropertyMap,
	timeout float64, preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {
	panic("TODO")
}

// Read the current live state associated with a resource. Enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource ID, but may also include some properties. If the resource
// is missing (for instance, because it has been deleted), the resulting property map will be nil.
func (p *Provider) Read(urn resource.URN, id resource.ID,
	inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
	panic("TODO")
}

// Update updates an existing resource with new values.
func (p *Provider) Update(urn resource.URN, id resource.ID, olds resource.PropertyMap, news resource.PropertyMap,
	timeout float64, ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {
	panic("TODO")
}

func (p *Provider) Delete(urn resource.URN, id resource.ID,
	props resource.PropertyMap, timeout float64) (resource.Status, error) {
	panic("TODO")
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
	panic("TODO")
}

// SignalCancellation asks all resource providers to gracefully shut down and abort any ongoing operations. Operation
// aborted in this way will return an error (e.g., `Update` and `Create` will either a creation error or an
// initialization error. SignalCancellation is advisory and non-blocking; it is up to the host to decide how long to
// wait after SignalCancellation is called before (e.g.) hard-closing any gRPC connection.
func (p *Provider) SignalCancellation() error {
	return nil
}
