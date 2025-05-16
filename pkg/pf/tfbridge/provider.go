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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver"
	pfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/configencoding"
	pl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/plugin"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/providerserver"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/valueshim"
)

type providerOptions struct {
	enableAccuratePFBridgePreview bool
}

type providerOption func(providerOptions) (providerOptions, error)

func withAccuratePFBridgePreview() providerOption {
	return func(opts providerOptions) (providerOptions, error) {
		opts.enableAccuratePFBridgePreview = true
		return opts, nil
	}
}

func getProviderOptions(opts []providerOption) (providerOptions, error) {
	res := providerOptions{}
	for _, o := range opts {
		var err error
		res, err = o(res)
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

// Provider implements the Pulumi resource provider operations for any
// Terraform plugin built with Terraform Plugin Framework.
//
// https://www.terraform.io/plugin/framework
type provider struct {
	tfServer      tfprotov6.ProviderServer
	info          tfbridge.ProviderInfo
	resources     runtypes.Resources
	datasources   runtypes.DataSources
	pulumiSchema  func(context.Context, plugin.GetSchemaRequest) ([]byte, error)
	encoding      convert.Encoding
	diagSink      diag.Sink
	configEncoder convert.Encoder
	configType    tftypes.Object
	version       semver.Version
	logSink       logging.Sink

	parameterize func(context.Context, plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error)

	// Used by CheckConfig to remember the current Provider configuration so that it can be recalled and used for
	// populating defaults specified via DefaultInfo.Config.
	lastKnownProviderConfig resource.PropertyMap

	schemaOnlyProvider shim.Provider
	providerOpts       []providerOption
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
	return pl.NewProvider(pwc), nil
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
	meta ProviderMetadata,
) (configencoding.Provider[*provider], error) {
	const infoPErrMSg string = "info.P must be constructed with ShimProvider or ShimProviderWithContext"

	if info.P == nil {
		return nil, fmt.Errorf("%s: cannot be nil", infoPErrMSg)
	}

	pfServer, ok := info.P.(pf.ShimProvider)
	if !ok {
		return nil, fmt.Errorf("Unknown inner type for info.P: %T, %s", info.P, infoPErrMSg)
	}
	server6, err := pfServer.Server(ctx)
	if err != nil {
		return nil, err
	}
	resources, err := pfServer.Resources(ctx)
	if err != nil {
		return nil, err
	}
	datasources, err := pfServer.DataSources(ctx)
	if err != nil {
		return nil, err
	}

	if info.MetadataInfo == nil {
		return nil, fmt.Errorf("[pf/tfbridge] ProviderInfo.BridgeMetadata is required but is nil")
	}

	providerConfigType, err := pfServer.Config(ctx)
	if err != nil {
		return nil, err
	}
	enc := convert.NewEncoding(info.P, &info)
	configEncoder, err := enc.NewConfigEncoder(providerConfigType)
	if err != nil {
		return nil, fmt.Errorf("NewConfigEncoder failed: %w", err)
	}

	semverVersion, err := semver.ParseTolerant(info.Version)
	if err != nil {
		return nil, fmt.Errorf("ProviderInfo needs a semver-compatible version string, got info.Version=%q",
			info.Version)
	}

	contract.Assertf((meta.PackageSchema == nil) != (meta.XGetSchema == nil),
		"Exactly one of PackageSchema or XGetSchema should be specified.")

	schema := meta.XGetSchema
	if meta.XGetSchema == nil {
		schema = func(context.Context, plugin.GetSchemaRequest) ([]byte, error) {
			return meta.PackageSchema, nil
		}
	}

	opts := []providerOption{}
	if info.EnableAccuratePFBridgePreview || cmdutil.IsTruthy(os.Getenv("PULUMI_TF_BRIDGE_ACCURATE_PF_BRIDGE_PREVIEW")) {
		opts = append(opts, withAccuratePFBridgePreview())
	}

	p := &provider{
		tfServer:           server6,
		info:               info,
		resources:          resources,
		datasources:        datasources,
		pulumiSchema:       schema,
		encoding:           enc,
		configEncoder:      configEncoder,
		configType:         providerConfigType,
		version:            semverVersion,
		schemaOnlyProvider: info.P,
		parameterize:       meta.XParamaterize,
		providerOpts:       opts,
	}

	return configencoding.New(p), nil
}

func (p *provider) GetConfigEncoding(context.Context) *tfbridge.ConfigEncoding {
	return tfbridge.NewConfigEncoding(p.schemaOnlyProvider.Schema(), p.info.Config)
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

	p.Unwrap().logSink = logSink
	srv := pl.NewProviderServerWithContext(p)

	return providerserver.NewPanicRecoveringProviderServer(&providerserver.PanicRecoveringProviderServerOptions{
		Logger:                 logSink,
		ResourceProviderServer: srv,
		ProviderName:           info.Name,
		ProviderVersion:        info.Version,
	}), nil
}

// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
func (p *provider) Close() error {
	return nil
}

// Pkg fetches this provider's package.
func (p *provider) Pkg() tokens.Package {
	return tokens.Package(p.info.Name)
}

type xResetProviderKey struct{}

type xParameterizeResetProviderFunc = func(context.Context, tfbridge.ProviderInfo, ProviderMetadata) error

// XParameterizeResetProvider resets the enclosing PF provider with a new info and meta combination.
//
// XParameterizeResetProvider is an unstable method and may change in any bridge
// release. It is intended only for internal use.
func XParameterizeResetProvider(ctx context.Context, info tfbridge.ProviderInfo, meta ProviderMetadata) error {
	return ctx.Value(xResetProviderKey{}).(xParameterizeResetProviderFunc)(ctx, info, meta)
}

func (p *provider) ParameterizeWithContext(
	ctx context.Context, req plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	ctx = p.initLogging(ctx, p.logSink, "")
	if p.parameterize == nil {
		return (&plugin.UnimplementedProvider{}).Parameterize(ctx, req)
	}

	ctx = context.WithValue(ctx, xResetProviderKey{},
		func(ctx context.Context, info tfbridge.ProviderInfo, meta ProviderMetadata) error {
			pp, err := newProviderWithContext(ctx, info, meta)
			if err != nil {
				return err
			}
			pp.Unwrap().logSink = p.logSink // Preserve the log sink from p

			*p = *pp.Unwrap()

			return nil
		})

	return p.parameterize(ctx, req)
}

// GetSchema returns the schema for the provider.
func (p *provider) GetSchemaWithContext(ctx context.Context, req plugin.GetSchemaRequest) ([]byte, error) {
	ctx = p.initLogging(ctx, p.logSink, "")
	return p.pulumiSchema(ctx, req)
}

// GetPluginInfo returns this plugin's information.
func (p *provider) GetPluginInfoWithContext(_ context.Context) (workspace.PluginInfo, error) {
	info := workspace.PluginInfo{
		Name:    p.info.Name,
		Version: &p.version,
		Kind:    apitype.ResourcePlugin,
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

func (p *provider) terraformResourceNameOrRenamedEntity(resourceToken tokens.Type) (string, error) {
	for tfname, v := range p.info.Resources {
		if v.Tok == resourceToken {
			return tfname, nil
		}
	}
	return "", fmt.Errorf("[pf/tfbridge] unknown resource token: %v", resourceToken)
}

func (p *provider) terraformDatasourceNameOrRenamedEntity(functionToken tokens.ModuleMember) (string, error) {
	for tfname, v := range p.info.DataSources {
		if v.Tok == functionToken {
			return tfname, nil
		}
	}
	return "", fmt.Errorf("[pf/tfbridge] unknown datasource token: %v", functionToken)
}

func (p *provider) returnTerraformConfig() (resource.PropertyMap, error) {
	// Get the current configuration
	config, err := convert.EncodePropertyMapToDynamic(p.configEncoder, p.configType, p.lastKnownProviderConfig)
	if err != nil {
		return nil, fmt.Errorf("error encoding property map: %v", err)
	}
	tfConfigValue, err := config.Unmarshal(p.configType)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling protov6.DynamicValue: %v", err)
	}

	if !tfConfigValue.IsFullyKnown() {
		msg := fmt.Sprintf("It looks like you're trying to use the provider's terraformConfig function. " +
			"The result of this function is meant for use as the config value of a required provider for a " +
			"Pulumi Terraform Module. All inputs to provider configuration must be known for this feature to work.")
		return nil, errors.New(msg)
	}
	// use valueshim package to marshal tfConfigValue into raw json,
	// which can be unmarshaled into a map[string]interface{}
	value := valueshim.FromTValue(tfConfigValue)
	configJSONMessage, err := value.Marshal(value.Type())
	if err != nil {
		return nil, fmt.Errorf("error marshaling into raw JSON message: %v", err)
	}

	jsonConfigMap := map[string]any{}
	err = json.Unmarshal(configJSONMessage, &jsonConfigMap)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %v", err)
	}
	originalMap := resource.NewPropertyMapFromMap(jsonConfigMap)
	configsMap := resource.PropertyMap{}
	// Make the map entries secret to avoid leaking sensitive information
	for key, val := range originalMap {
		configsMap[key] = resource.MakeSecret(val)
	}
	resultMap := resource.PropertyMap{
		"result": resource.NewObjectProperty(configsMap),
	}

	return resultMap, nil
}

func (p *provider) CallWithContext(ctx context.Context,
	tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions,
) (plugin.CallResult, error) {
	_, functionName, found := strings.Cut(tok.Name().String(), "/")
	if !found {
		return plugin.CallResult{}, fmt.Errorf("error getting method name from token name %q", tok)
	}

	switch functionName {
	case "terraformConfig":
		returnMap, err := p.returnTerraformConfig()
		if err != nil {
			return plugin.CallResult{}, err
		}
		return plugin.CallResult{
			Return: returnMap,
		}, nil
	default:
		return plugin.CallResult{}, fmt.Errorf("unknown method %v", tok)
	}
}

// NOT IMPLEMENTED: Construct creates a new component resource.
func (p *provider) ConstructWithContext(_ context.Context,
	info plugin.ConstructInfo, typ tokens.Type, name tokens.QName, parent resource.URN,
	inputs resource.PropertyMap, options plugin.ConstructOptions,
) (plugin.ConstructResult, error) {
	return plugin.ConstructResult{},
		fmt.Errorf("Construct is not implemented for Terraform Plugin Framework bridged providers")
}
