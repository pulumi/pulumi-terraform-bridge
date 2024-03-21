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

	"github.com/blang/semver"

	pfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	pl "github.com/pulumi/pulumi-terraform-bridge/pf/internal/plugin"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
)

// Provider implements the Pulumi resource provider operations for any
// Terraform plugin built with Terraform Plugin Framework.
//
// https://www.terraform.io/plugin/framework
type provider struct {
	tfProvider    pfprovider.Provider
	tfServer      tfprotov6.ProviderServer
	info          tfbridge.ProviderInfo
	resources     pfutils.Resources
	datasources   pfutils.DataSources
	pulumiSchema  []byte
	encoding      convert.Encoding
	diagSink      diag.Sink
	configEncoder convert.Encoder
	configType    tftypes.Object
	version       semver.Version
	logSink       logging.Sink

	// Used by CheckConfig to remember the current Provider configuration so that it can be recalled and used for
	// populating defaults specified via DefaultInfo.Config.
	lastKnownProviderConfig resource.PropertyMap

	schemaOnlyProvider shim.Provider
}

var _ pl.ProviderWithContext = &provider{}

// Adapts a provider to Pulumi. Most users do not need to call this directly but instead use Main to build a fully
// functional binary.
//
// info.P must be constructed with ShimProvider or ShimProviderWithContext.
func NewProvider(ctx context.Context, info tfbridge.ProviderInfo, meta ProviderMetadata) (plugin.Provider, error) {
	pwc, err := newProviderWithContext(ctx, info, meta)
	if err != nil {
		return nil, err
	}
	return pl.NewProvider(ctx, pwc), nil
}

// Wrap a PF Provider in a shim.Provider.
func ShimProvider(p pfprovider.Provider) shim.Provider {
	return ShimProviderWithContext(context.Background(), p)
}

// Wrap a PF Provider in a shim.Provider with the given context.Context.
func ShimProviderWithContext(ctx context.Context, p pfprovider.Provider) shim.Provider {
	return schemashim.ShimSchemaOnlyProvider(ctx, p)
}

func newProviderWithContext(ctx context.Context, info tfbridge.ProviderInfo,
	meta ProviderMetadata) (pl.ProviderWithContext, error) {
	const infoPErrMSg string = "info.P must be constructed with ShimProvider or ShimProviderWithContext"
	if info.P == nil {
		return nil, fmt.Errorf("%s: cannot be nil", infoPErrMSg)
	}
	schemaOnlyProvider, ok := info.P.(*schemashim.SchemaOnlyProvider)
	if !ok {
		return nil, fmt.Errorf("%s: found non-conforming type %T", infoPErrMSg, info.P)
	}
	p := schemaOnlyProvider.PfProvider()

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

	if info.MetadataInfo == nil {
		return nil, fmt.Errorf("[pf/tfbridge] ProviderInfo.BridgeMetadata is required but is nil")
	}

	enc := convert.NewEncoding(schemaOnlyProvider, &info)

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
		encoding:      enc,
		configEncoder: configEncoder,
		configType:    providerConfigType,
		version:       semverVersion,

		schemaOnlyProvider: schemaOnlyProvider,
	}, nil
}

// Internal. The signature of this function can change between major releases. Exposed to facilitate testing.
func NewProviderServer(
	ctx context.Context,
	logSink logging.Sink,
	info tfbridge.ProviderInfo,
	meta ProviderMetadata,
) (pulumirpc.ResourceProviderServer, error) {
	p, err := newProviderWithContext(ctx, info, meta)
	if err != nil {
		return nil, err
	}
	pp := p.(*provider)

	pp.logSink = logSink
	configEnc := tfbridge.NewConfigEncoding(pp.schemaOnlyProvider.Schema(), pp.info.Config)
	return pl.NewProviderServerWithContext(p, configEnc), nil
}

// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
func (p *provider) Close() error {
	return nil
}

// Pkg fetches this provider's package.
func (p *provider) PkgWithContext(_ context.Context) tokens.Package {
	return tokens.Package(p.info.Name)
}

// GetSchema returns the schema for the provider.
func (p *provider) GetSchemaWithContext(_ context.Context, version int) ([]byte, error) {
	return p.pulumiSchema, nil
}

// GetPluginInfo returns this plugin's information.
func (p *provider) GetPluginInfoWithContext(_ context.Context) (workspace.PluginInfo, error) {
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
func (p *provider) SignalCancellationWithContext(_ context.Context) error {
	// Some improvements are possible here to gracefully shut down.
	return nil
}

func (p *provider) terraformResourceName(resourceToken tokens.Type) (string, error) {
	for tfname, v := range p.info.Resources {
		if v.Tok == resourceToken {
			return tfname, nil
		}
	}
	return "", fmt.Errorf("[pf/tfbridge] unknown resource token: %v", resourceToken)
}

func (p *provider) terraformDatasourceName(functionToken tokens.ModuleMember) (string, error) {
	for tfname, v := range p.info.DataSources {
		if v.Tok == functionToken {
			return tfname, nil
		}
	}
	return "", fmt.Errorf("[pf/tfbridge] unknown datasource token: %v", functionToken)
}

// NOT IMPLEMENTED: Call dynamically executes a method in the provider associated with a component resource.
func (p *provider) CallWithContext(_ context.Context,
	tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions) (plugin.CallResult, error) {
	return plugin.CallResult{},
		fmt.Errorf("Call is not implemented for Terraform Plugin Framework bridged providers")
}

// NOT IMPLEMENTED: Construct creates a new component resource.
func (p *provider) ConstructWithContext(_ context.Context,
	info plugin.ConstructInfo, typ tokens.Type, name tokens.QName, parent resource.URN,
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
