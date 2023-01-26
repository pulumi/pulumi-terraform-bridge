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

package tfpfbridge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blang/semver"
	pfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
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

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/pfutils"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
)

// Provider implements the Pulumi resource provider operations for any
// Terraform plugin built with Terraform Plugin Framework.
//
// https://www.terraform.io/plugin/framework
type provider struct {
	tfProvider    pfprovider.Provider
	tfServer      tfprotov6.ProviderServer
	info          ProviderInfo
	resources     pfutils.Resources
	datasources   pfutils.DataSources
	pulumiSchema  []byte
	packageSpec   pschema.PackageSpec
	encoding      convert.Encoding
	propertyNames convert.PropertyNames
	diagSink      diag.Sink
	configEncoder convert.Encoder
	configType    tftypes.Object
	version       semver.Version
}

var _ plugin.Provider = &provider{}

// Adaptes a provider to Pulumi. Most users do not need to call this directly but instead use Main to build a fully
// functional binary.
func NewProvider(ctx context.Context, info ProviderInfo, meta ProviderMetadata) (plugin.Provider, error) {
	p := info.NewProvider()
	server6, err := newProviderServer6(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("Fatal failure starting a provider server: %w", err)
	}
	resources, err := pfutils.GatherResources(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("Fatal failure gathering resource metadata: %w", err)
	}

	datasources, err := pfutils.GatherDatasources(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("Fatal failure gathering datasource metadata: %w", err)
	}

	var thePackageSpec pschema.PackageSpec
	if err := json.Unmarshal(meta.PackageSchema, &thePackageSpec); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal PackageSpec: %w", err)
	}

	var renames tfgen.Renames
	if err := json.Unmarshal(meta.BridgeMetadata, &renames); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal BridgeMetadata: %w", err)
	}

	propertyNames := newPrecisePropertyNames(renames)
	enc := convert.NewEncoding(packageSpec{&thePackageSpec}, propertyNames)

	schemaResponse := &pfprovider.SchemaResponse{}
	p.Schema(ctx, pfprovider.SchemaRequest{}, schemaResponse)
	schema, diags := schemaResponse.Schema, schemaResponse.Diagnostics
	if diags.HasError() {
		return nil, fmt.Errorf("Schema() returned diagnostics with HasError")
	}

	providerConfigType := schema.Type().TerraformType(ctx).(tftypes.Object)

	configEncoder, err := enc.NewConfigEncoder(providerConfigType)
	if err != nil {
		return nil, fmt.Errorf("NewConfigEncoder failed: %w", err)
	}

	semverVersion, err := semver.ParseTolerant(info.Version)
	if err != nil {
		return nil, fmt.Errorf("ProviderInfo needs a semver-compatible version string, got info.Version=%q",
			info.Version)
	}

	return &provider{
		tfProvider:    p,
		tfServer:      server6,
		info:          info,
		resources:     resources,
		datasources:   datasources,
		pulumiSchema:  meta.PackageSchema,
		packageSpec:   thePackageSpec,
		propertyNames: propertyNames,
		encoding:      enc,
		configEncoder: configEncoder,
		configType:    providerConfigType,
		version:       semverVersion,
	}, nil
}

func newProviderServer(ctx context.Context,
	info ProviderInfo, meta ProviderMetadata) (pulumirpc.ResourceProviderServer, error) {
	p, err := NewProvider(ctx, info, meta)
	if err != nil {
		return nil, err
	}
	return plugin.NewProviderServer(p), nil
}

// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
func (p *provider) Close() error {
	panic("TODO")
}

// Pkg fetches this provider's package.
func (p *provider) Pkg() tokens.Package {
	panic("TODO")
}

// GetSchema returns the schema for the provider.
func (p *provider) GetSchema(version int) ([]byte, error) {
	return p.pulumiSchema, nil
}

// GetPluginInfo returns this plugin's information.
func (p *provider) GetPluginInfo() (workspace.PluginInfo, error) {
	info := workspace.PluginInfo{
		Name:    p.info.Name,
		Version: &p.version,
		Kind:    workspace.ResourcePlugin,
	}
	return info, nil
}

// SignalCancellation asks all resource providers to gracefully shut down and abort any ongoing operations. Operation
// aborted in this way will return an error (e.g., `Update` and `Create` will either a creation error or an
// initialization error. SignalCancellation is advisory and non-blocking; it is up to the host to decide how long to
// wait after SignalCancellation is called before (e.g.) hard-closing any gRPC connection.
func (p *provider) SignalCancellation() error {
	return nil // TODO proper handling
}

func (p *provider) terraformResourceName(resourceToken tokens.Type) (string, error) {
	for tfname, v := range p.info.Resources {
		if v.Tok == resourceToken {
			return tfname, nil
		}
	}
	return "", fmt.Errorf("[tfpfbridge] unknown resource token: %v", resourceToken)
}

func (p *provider) terraformDatasourceName(functionToken tokens.ModuleMember) (string, error) {
	for tfname, v := range p.info.DataSources {
		if v.Tok == functionToken {
			return tfname, nil
		}
	}
	return "", fmt.Errorf("[tfpfbridge] unknown datasource token: %v", functionToken)
}

// NOT IMPLEMENTED: Call dynamically executes a method in the provider associated with a component resource.
func (p *provider) Call(tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions) (plugin.CallResult, error) {
	return plugin.CallResult{},
		fmt.Errorf("Call is not implemented for Terraform Plugin Framework bridged providers")
}

// NOT IMPLEMENTED: Construct creates a new component resource.
func (p *provider) Construct(info plugin.ConstructInfo, typ tokens.Type, name tokens.QName, parent resource.URN,
	inputs resource.PropertyMap, options plugin.ConstructOptions) (plugin.ConstructResult, error) {
	return plugin.ConstructResult{},
		fmt.Errorf("Construct is not implemented for Terraform Plugin Framework bridged providers")
}

func newProviderServer6(ctx context.Context, p pfprovider.Provider) (tfprotov6.ProviderServer, error) {
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

func (p *provider) GetMapping(key string) ([]byte, string, error) {
	return []byte{}, "", nil
}

type packageSpec struct {
	spec *pschema.PackageSpec
}

var _ convert.PackageSpec = (*packageSpec)(nil)

func (p packageSpec) Config() *pschema.ConfigSpec {
	return &p.spec.Config
}

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
