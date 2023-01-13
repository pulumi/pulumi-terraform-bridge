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
	"encoding/json"
	"fmt"

	"github.com/blang/semver"
	tfsdkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/pfutils"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

// Provider implements the Pulumi resource provider operations for any
// Terraform plugin built with Terraform Plugin Framework.
//
// https://www.terraform.io/plugin/framework
type Provider struct {
	tfProvider    tfsdkprovider.Provider
	tfServer      tfprotov6.ProviderServer
	info          info.ProviderInfo
	resources     pfutils.Resources
	datasources   pfutils.DataSources
	pulumiSchema  []byte
	packageSpec   pschema.PackageSpec
	encoding      convert.Encoding
	propertyNames convert.PropertyNames
	diagSink      diag.Sink
	configEncoder convert.Encoder
	configType    tftypes.Object
}

var _ plugin.Provider = &Provider{}

func NewProvider(info info.ProviderInfo, pulumiSchema []byte, serializedRenames []byte) plugin.Provider {
	ctx := context.TODO()
	p := info.P()
	server6, err := newProviderServer6(ctx, p)
	if err != nil {
		panic(fmt.Errorf("Fatal failure starting a provider server: %w", err))
	}
	resources, err := pfutils.GatherResources(ctx, p)
	if err != nil {
		panic(fmt.Errorf("Fatal failure gathering resource metadata: %w", err))
	}

	datasources, err := pfutils.GatherDatasources(ctx, p)
	if err != nil {
		panic(fmt.Errorf("Fatal failure gathering datasource metadata: %w", err))
	}

	var thePackageSpec pschema.PackageSpec
	if err := json.Unmarshal(pulumiSchema, &thePackageSpec); err != nil {
		panic(fmt.Errorf("Failed to unmarshal PackageSpec: %w", err))
	}

	var renames tfgen.Renames
	if err := json.Unmarshal(serializedRenames, &renames); err != nil {
		panic(fmt.Errorf("Failed to unmarshal Renames: %w", err))
	}

	propertyNames := newPrecisePropertyNames(renames)
	enc := convert.NewEncoding(packageSpec{&thePackageSpec}, propertyNames)

	schema, diags := p.GetSchema(ctx)
	if diags.HasError() {
		panic(fmt.Errorf("GetSchema returned diagnostics with HasError"))
	}

	providerConfigType := schema.Type().TerraformType(ctx).(tftypes.Object)

	configEncoder, err := enc.NewConfigEncoder(providerConfigType)
	if err != nil {
		panic(fmt.Errorf("NewConfigEncoder failed: %w", err))
	}

	return &Provider{
		tfProvider:    p,
		tfServer:      server6,
		info:          info,
		resources:     resources,
		datasources:   datasources,
		pulumiSchema:  pulumiSchema,
		packageSpec:   thePackageSpec,
		propertyNames: propertyNames,
		encoding:      enc,
		configEncoder: configEncoder,
		configType:    providerConfigType,
	}
}

func NewProviderServer(
	info info.ProviderInfo,
	pulumiSchema []byte,
	serializedRenames []byte,
) pulumirpc.ResourceProviderServer {
	return plugin.NewProviderServer(NewProvider(info, pulumiSchema, serializedRenames))
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
	return nil // TODO proper handling
}

func (p *Provider) terraformResourceName(resourceToken tokens.Type) (string, error) {
	for tfname, v := range p.info.Resources {
		if v.Tok == resourceToken {
			return tfname, nil
		}
	}
	return "", fmt.Errorf("[tfpfbridge] unknown resource token: %v", resourceToken)
}

func (p *Provider) terraformDatasourceName(functionToken tokens.ModuleMember) (string, error) {
	for tfname, v := range p.info.DataSources {
		if v.Tok == functionToken {
			return tfname, nil
		}
	}
	return "", fmt.Errorf("[tfpfbridge] unknown datasource token: %v", functionToken)
}

// NOT IMPLEMENTED: Call dynamically executes a method in the provider associated with a component resource.
func (p *Provider) Call(tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions) (plugin.CallResult, error) {
	return plugin.CallResult{},
		fmt.Errorf("Call is not implemented for Terraform Plugin Framework bridged providers")
}

// NOT IMPLEMENTED: Construct creates a new component resource.
func (p *Provider) Construct(info plugin.ConstructInfo, typ tokens.Type, name tokens.QName, parent resource.URN,
	inputs resource.PropertyMap, options plugin.ConstructOptions) (plugin.ConstructResult, error) {
	return plugin.ConstructResult{},
		fmt.Errorf("Construct is not implemented for Terraform Plugin Framework bridged providers")
}

func newProviderServer6(ctx context.Context, p tfsdkprovider.Provider) (tfprotov6.ProviderServer, error) {
	newServer6 := providerserver.NewProtocol6(p)
	server6 := newServer6()

	// Somehow this GetProviderSchema call needs to happen at least once to avoid Resource Type Not Found in the
	// tfServer, to init it properly to remember provider name and compute correct resource names like
	// random_integer instead of _integer (unknown provider name).
	if _, err := server6.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{}); err != nil {
		return nil, err
	}

	return server6, nil
}

func (p *Provider) GetMapping(key string) ([]byte, string, error) {
	return []byte{}, "", nil
}

type packageSpec struct {
	spec *pschema.PackageSpec
}

var _ convert.PackageSpec = (*packageSpec)(nil)

func (p packageSpec) Resource(tok tokens.Type) *pschema.ResourceSpec {
	res, ok := p.spec.Resources[string(tok)]
	if ok {
		return &res
	}
	return nil
}

func (p packageSpec) Type(tok tokens.Type) *pschema.ComplexTypeSpec {
	typ, ok := p.spec.Types[string(tok)]
	if ok {
		return &typ
	}
	return nil
}

func (p packageSpec) Function(tok tokens.ModuleMember) *pschema.FunctionSpec {
	res, ok := p.spec.Functions[string(tok)]
	if ok {
		return &res
	}
	return nil
}
