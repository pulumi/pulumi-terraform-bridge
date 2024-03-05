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
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/golang/glog"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/hashicorp/go-multierror"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/logging"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
)

// Provider implements the Pulumi resource provider operations for any Terraform plugin.
type Provider struct {
	pulumirpc.UnimplementedResourceProviderServer

	host            *provider.HostClient               // the RPC link back to the Pulumi engine.
	module          string                             // the Terraform module name.
	version         string                             // the plugin version number.
	tf              shim.Provider                      // the Terraform resource provider to use.
	info            ProviderInfo                       // overlaid info about this provider.
	config          shim.SchemaMap                     // the Terraform config schema.
	configValues    resource.PropertyMap               // this package's config values.
	resources       map[tokens.Type]Resource           // a map of Pulumi type tokens to resource info.
	dataSources     map[tokens.ModuleMember]DataSource // a map of Pulumi module tokens to data sources.
	supportsSecrets bool                               // true if the engine supports secret property values
	pulumiSchema    []byte                             // the JSON-encoded Pulumi schema.
	memStats        memStatCollector
}

// MuxProvider defines an interface which must be implemented by providers
// that shall be used as mixins of a wrapped Terraform provider
type MuxProvider interface {
	GetSpec(ctx context.Context,
		name, version string) (schema.PackageSpec, error)
	GetInstance(ctx context.Context,
		name, version string,
		host *provider.HostClient) (pulumirpc.ResourceProviderServer, error)
}

// Resource wraps both the Terraform resource type info plus the overlay resource info.
type Resource struct {
	Schema *ResourceInfo // optional provider overrides.
	TF     shim.Resource // the Terraform resource schema.
	TFName string        // the Terraform resource name.
}

// runTerraformImporter runs the Terraform Importer defined on the Resource for the given
// resource ID, and returns a replacement input map if any resources are matched. A nil map
// with no error should be interpreted by the caller as meaning the resource does not exist,
// but there were no errors in determining this.
func (res *Resource) runTerraformImporter(
	ctx context.Context, id string, provider *Provider,
) (shim.InstanceState, error) {
	contract.Assertf(res.TF.Importer() != nil, "res.TF.Importer() != nil")

	// Run the importer defined in the Terraform resource schema
	states, err := res.TF.Importer()(res.TFName, id, provider.tf.Meta(ctx))
	if err != nil {
		return nil, errors.Wrapf(err, "importing %s", id)
	}

	// No resources were returned. There are a few different ways this can happen - principally
	//  - The resource never existed
	//  - The resource did exist but was deleted
	//
	// The engine is capable of converting an empty response into an appropriate error for the
	// user, so we don't want to disable that behaviour by returning our own (likely different)
	// error up the chain. Instead, we return a nil map _and_ a nil error, and it is the
	// responsibility of the caller to convert this into an appropriate error message.
	//
	// We consider the case in which multiple results are returned from the importer, but none
	// match the ID expected to be an error, and this is handled later in this function.
	if len(states) < 1 {
		return nil, nil
	}

	// A Terraform importer can return multiple ResourceData instances for different resources. For
	// example, an AWS security group will also import the related security group rules as independent
	// resources.
	//
	// Some Terraform importers _change_ the ID of the resource to allow for multiple formats to be
	// specified by a user (for example, an AWS API Gateway Response). In the case that we only have
	// a single ResourceData returned, we will use that ResourceData regardless of whether the ID
	// matches, provided the resource Type does match.
	//
	// If we get multiple ResourceData back, we need to search the results for one which matches both
	// the Type and ID of the resource we were trying to import (the "primary" InstanceState).
	//
	// The Type can be identified by looking at the ephemeral data attached to the InstanceState, since
	// it is not stored in all cases - only for import.
	var candidates []shim.InstanceState
	for _, state := range states {
		if state.Type() == res.TFName {
			candidates = append(candidates, state)
		}
	}

	var primaryInstanceState shim.InstanceState
	if len(candidates) == 1 {
		// Take the only result.
		primaryInstanceState = candidates[0]
	} else {
		// Search for a resource with a matching ID. If one exists, take it.
		for _, result := range candidates {
			if result.ID() == id {
				primaryInstanceState = result
				break
			}
		}
	}

	// No resources were matched - error out
	if primaryInstanceState == nil {
		return nil, errors.Errorf("importer for %s returned no matching resources", id)
	}
	return primaryInstanceState, nil
}

// DataSource wraps both the Terraform data source (resource) type info plus the overlay resource info.
type DataSource struct {
	Schema *DataSourceInfo // optional provider overrides.
	TF     shim.Resource   // the Terraform data source schema.
	TFName string          // the Terraform resource name.
}

type CheckFailureErrorElement struct {
	Reason   string
	Property string
}

// CheckFailureError can be returned from a PreConfigureCallback to indicate that the
// configuration is invalid along with an actionable error message. These will be
// returned as failures in the CheckConfig response instead of errors.
type CheckFailureError struct {
	Failures []CheckFailureErrorElement
}

func (e CheckFailureError) Error() string {
	return fmt.Sprintf("CheckFailureErrors %s", e.Failures)
}

// NewProvider creates a new Pulumi RPC server wired up to the given host and wrapping the given Terraform provider.
func NewProvider(ctx context.Context, host *provider.HostClient, module, version string,
	tf shim.Provider, info ProviderInfo, pulumiSchema []byte,
) pulumirpc.ResourceProviderServer {
	if len(info.MuxWith) > 0 {
		p, err := newMuxWithProvider(ctx, host, module, version, info, pulumiSchema)
		if err != nil {
			panic(err)
		}
		return p
	}
	return newProvider(ctx, host, module, version, tf, info, pulumiSchema)
}

func newProvider(ctx context.Context, host *provider.HostClient,
	module, version string, tf shim.Provider, info ProviderInfo, pulumiSchema []byte,
) *Provider {
	p := &Provider{
		host:         host,
		module:       module,
		version:      version,
		tf:           tf,
		info:         info,
		config:       tf.Schema(),
		pulumiSchema: pulumiSchema,
	}
	p.loggingContext(ctx, "")
	p.initResourceMaps()
	return p
}

func newMuxWithProvider(ctx context.Context, host *provider.HostClient,
	module, version string, info ProviderInfo, pulumiSchema []byte,
) (pulumirpc.ResourceProviderServer, error) {
	var mapping muxer.DispatchTable
	if m, found, err := metadata.Get[muxer.DispatchTable](info.GetMetadata(), "mux"); err != nil {
		return nil, err
	} else if found {
		mapping = m
	} else {
		return nil, fmt.Errorf("missing pre-computed muxer mapping")
	}

	servers := []muxer.Endpoint{{
		Server: func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
			return newProvider(ctx, host, module, version, info.P, info, pulumiSchema), nil
		},
	}}
	for _, f := range info.MuxWith {
		servers = append(servers, muxer.Endpoint{
			Server: func(hc *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
				return f.GetInstance(ctx, module, version, hc)
			},
		})
	}

	return muxer.Main{
		Schema:        pulumiSchema,
		DispatchTable: mapping,
		Servers:       servers,
	}.Server(host, module, version)
}

var _ pulumirpc.ResourceProviderServer = (*Provider)(nil)

func (p *Provider) pkg() tokens.Package {
	return tokens.NewPackageToken(tokens.PackageName(tokens.IntoQName(p.module)))
}

func (p *Provider) baseDataMod() tokens.Module {
	return tokens.NewModuleToken(p.pkg(), tokens.ModuleName("data"))
}

func (p *Provider) Attach(context context.Context, req *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	host, err := provider.NewHostClient(req.GetAddress())
	if err != nil {
		return nil, err
	}
	p.host = host
	return &pbempty.Empty{}, nil
}

func (p *Provider) loggingContext(ctx context.Context, urn resource.URN) context.Context {
	// There is no host in a testing context.
	if p.host == nil {
		// For tests that did not call InitLogging yet, we should call it here so that
		// GetLogger does not panic.
		if ctx.Value(logging.CtxKey) == nil {
			return logging.InitLogging(ctx, logging.LogOptions{
				URN:             urn,
				ProviderName:    p.info.Name,
				ProviderVersion: p.version,
			})
		}

		// Otherwise keep the context as-is.
		return ctx
	}

	log.SetOutput(&LogRedirector{
		writers: map[string]func(string) error{
			tfTracePrefix: func(msg string) error { return p.host.Log(ctx, diag.Debug, "", msg) },
			tfDebugPrefix: func(msg string) error { return p.host.Log(ctx, diag.Debug, "", msg) },
			tfInfoPrefix:  func(msg string) error { return p.host.Log(ctx, diag.Info, "", msg) },
			tfWarnPrefix:  func(msg string) error { return p.host.Log(ctx, diag.Warning, "", msg) },
			tfErrorPrefix: func(msg string) error { return p.host.Log(ctx, diag.Error, "", msg) },
		},
	})

	return logging.InitLogging(ctx, logging.LogOptions{
		LogSink:         p.host,
		URN:             urn,
		ProviderName:    p.info.Name,
		ProviderVersion: p.version,
	})
}

func (p *Provider) label() string {
	return fmt.Sprintf("tf.Provider[%s]", p.module)
}

// initResourceMaps creates maps from Pulumi types and tokens to Terraform resource type.
func (p *Provider) initResourceMaps() {
	// Fetch a list of all resource types handled by this provider and make a map.
	p.resources = make(map[tokens.Type]Resource)
	p.tf.ResourcesMap().Range(func(name string, res shim.Resource) bool {
		var tok tokens.Type

		// See if there is override information for this resource.  If yes, use that to decode the token.
		var schema *ResourceInfo
		if p.info.Resources != nil {
			schema = p.info.Resources[name]
			if schema != nil {
				tok = schema.Tok
			}
		}

		// Otherwise, we default to the standard naming scheme.
		if tok == "" {
			// Manufacture a token with the package, module, and resource type name.
			camelName, pascalName := p.camelPascalPulumiName(name)
			modTok := tokens.NewModuleToken(p.pkg(), tokens.ModuleName(camelName))
			tok = tokens.NewTypeToken(modTok, tokens.TypeName(pascalName))
		}

		p.resources[tok] = Resource{
			TF:     res,
			TFName: name,
			Schema: schema,
		}

		return true
	})

	// Fetch a list of all data source types handled by this provider and make a similar map.
	p.dataSources = make(map[tokens.ModuleMember]DataSource)
	p.tf.DataSourcesMap().Range(func(name string, ds shim.Resource) bool {
		var tok tokens.ModuleMember

		// See if there is override information for this resource.  If yes, use that to decode the token.
		var schema *DataSourceInfo
		if p.info.DataSources != nil {
			schema = p.info.DataSources[name]
			if schema != nil {
				tok = schema.Tok
			}
		}

		// Otherwise, we default to the standard naming scheme.
		if tok == "" {
			// Manufacture a token with the data module and camel-cased name.
			camelName, _ := p.camelPascalPulumiName(name)
			tok = tokens.NewModuleMemberToken(p.baseDataMod(), tokens.ModuleMemberName(camelName))
		}

		p.dataSources[tok] = DataSource{
			TF:     ds,
			TFName: name,
			Schema: schema,
		}

		return true
	})
}

// camelPascalPulumiName returns the camel and pascal cased name for a given terraform name.
func (p *Provider) camelPascalPulumiName(name string) (string, string) {
	prefix := p.info.GetResourcePrefix() + "_"
	contract.Assertf(strings.HasPrefix(name, prefix),
		"Expected all Terraform resources in this module to have a '%v' prefix (%q)", prefix, name)
	name = name[len(prefix):]
	camel := TerraformToPulumiNameV2(name, nil, nil)
	pascal := camel
	if pascal != "" {
		pascal = string(unicode.ToUpper(rune(pascal[0]))) + pascal[1:]
	}
	return camel, pascal
}

// GetSchema returns the JSON-encoded schema for this provider's package.
func (p *Provider) GetSchema(ctx context.Context,
	req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	if v := req.GetVersion(); v > 1 {
		return nil, errors.Errorf("unsupported schema version %v", v)
	}
	return &pulumirpc.GetSchemaResponse{
		Schema: string(p.pulumiSchema),
	}, nil
}

func makeCheckResponseFromCheckErr(err CheckFailureError) *pulumirpc.CheckResponse {
	failures := make([]*pulumirpc.CheckFailure, len(err.Failures))
	for i, failure := range err.Failures {
		failures[i] = &pulumirpc.CheckFailure{
			Reason:   failure.Reason,
			Property: failure.Property,
		}
	}
	return &pulumirpc.CheckResponse{
		Failures: failures,
	}
}

// CheckConfig validates the configuration for this Terraform provider.
func (p *Provider) CheckConfig(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	urn := resource.URN(req.GetUrn())
	ctx = p.loggingContext(ctx, urn)
	label := fmt.Sprintf("%s.CheckConfig(%s)", p.label(), urn)
	glog.V(9).Infof("%s executing", label)

	configEnc := NewConfigEncoding(p.config, p.info.Config)

	news, validationErrors := configEnc.UnmarshalProperties(req.GetNews())
	if validationErrors != nil {
		return nil, errors.Wrap(validationErrors, "CheckConfig failed because of malformed resource inputs")
	}

	checkConfigSpan, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.CheckConfig",
		opentracing.Tag{Key: "provider", Value: p.info.Name},
		opentracing.Tag{Key: "version", Value: p.version},
		opentracing.Tag{Key: "inputs", Value: resource.NewObjectProperty(news).String()},
		opentracing.Tag{Key: "urn", Value: string(urn)},
	)
	defer checkConfigSpan.Finish()
	p.memStats.collectMemStats(ctx, checkConfigSpan)

	config, validationErrors := buildTerraformConfig(ctx, p, news)
	if validationErrors != nil {
		return nil, errors.Wrap(validationErrors, "could not marshal config state")
	}

	// It is currently a breaking change to call PreConfigureCallback with unknown values. The user code does not
	// expect them and may panic.
	//
	// Currently we do not call it at all if there are any unknowns.
	//
	// See pulumi/pulumi-terraform-bridge#1087
	if !news.ContainsUnknowns() {
		if err := p.preConfigureCallback(ctx, news, config); err != nil {
			if failureErr, ok := err.(CheckFailureError); ok {
				return makeCheckResponseFromCheckErr(failureErr), nil
			}
			return nil, err
		}
		if err := p.preConfigureCallbackWithLogger(ctx, news, config); err != nil {
			if failureErr, ok := err.(CheckFailureError); ok {
				return makeCheckResponseFromCheckErr(failureErr), nil
			}
			return nil, err
		}
	}

	checkFailures := validateProviderConfig(ctx, urn, p, config)
	if len(checkFailures) > 0 {
		return &pulumirpc.CheckResponse{
			Failures: checkFailures,
		}, nil
	}

	// Ensure properties marked secret in the schema have secret values.
	secretNews := MarkSchemaSecrets(ctx, p.config, p.info.Config, resource.NewObjectProperty(news)).ObjectValue()

	// In case news was modified by pre-configure callbacks, marshal it again to send out the modified value.
	newsStruct, err := configEnc.MarshalProperties(secretNews)
	if err != nil {
		return nil, err
	}

	return &pulumirpc.CheckResponse{
		Inputs: newsStruct,
	}, nil
}

func (p *Provider) preConfigureCallback(
	ctx context.Context,
	news resource.PropertyMap,
	config shim.ResourceConfig,
) error {
	if p.info.PreConfigureCallback == nil {
		return nil
	}
	span, _ := opentracing.StartSpanFromContext(ctx, "sdkv2.PreConfigureCallback")
	defer span.Finish()
	// NOTE: the user code may modify news in-place.
	return p.info.PreConfigureCallback(news, config)
}

func (p *Provider) preConfigureCallbackWithLogger(
	ctx context.Context,
	news resource.PropertyMap,
	config shim.ResourceConfig,
) error {
	if p.info.PreConfigureCallbackWithLogger == nil {
		return nil
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.PreConfigureCallbackWithLogger")
	defer span.Finish()
	// NOTE: the user code may modify news in-place.
	return p.info.PreConfigureCallbackWithLogger(ctx, p.host, news, config)
}

func buildTerraformConfig(ctx context.Context, p *Provider, vars resource.PropertyMap) (shim.ResourceConfig, error) {
	tfVars := make(resource.PropertyMap)
	ignoredKeys := map[string]bool{"version": true, "pluginDownloadURL": true}
	for k, v := range vars {
		// we need to skip the version as adding that will cause the provider validation to fail
		if ignoredKeys[string(k)] {
			continue
		}
		if _, has := p.info.ExtraConfig[string(k)]; !has {
			tfVars[k] = v
		}
	}

	inputs, _, err := MakeTerraformInputs(ctx, nil, tfVars, nil, tfVars, p.config, p.info.Config)
	if err != nil {
		return nil, err
	}

	return MakeTerraformConfigFromInputs(ctx, p.tf, inputs), nil
}

func validateProviderConfig(
	ctx context.Context,
	urn resource.URN,
	p *Provider,
	config shim.ResourceConfig,
) []*pulumirpc.CheckFailure {
	span, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.ValidateProviderConfig")
	defer span.Finish()

	schemaMap := p.config
	schemaInfos := p.info.GetConfig()

	var missingKeys []*pulumirpc.CheckFailure
	p.config.Range(func(key string, meta shim.Schema) bool {
		if meta.Required() && !config.IsSet(key) {
			pp := NewCheckFailurePath(schemaMap, schemaInfos, key)
			cf := NewCheckFailure(MissingKey, "Missing key", &pp, urn, true /*isProvider*/, p.module,
				schemaMap, schemaInfos)
			checkFailure := pulumirpc.CheckFailure{
				Property: string(cf.Property),
				Reason:   cf.Reason,
			}
			missingKeys = append(missingKeys, &checkFailure)
		}
		return true
	})

	if len(missingKeys) > 0 {
		return missingKeys
	}

	// Perform validation of the config state so we can offer nice errors.
	warns, errs := p.tf.Validate(ctx, config)
	for _, warn := range warns {
		logErr := p.host.Log(ctx, diag.Warning, "", fmt.Sprintf("provider config warning: %v", warn))
		if logErr != nil {
			glog.V(9).Infof("Failed to log to the engine: %v", logErr)
			continue
		}
	}

	return p.adaptCheckFailures(ctx, urn, true /*isProvider*/, p.config, p.info.GetConfig(), errs)
}

// A DiffConfig implementation enables the Pulumi provider to detect when provider configuration is
// changing and suggest a replacement plans when some properties require a replace. Replacing a
// provider then replaces all resources provisioned against this provider. See also the section on
// Explicit Provider Configuration in the docs:
//
// https://www.pulumi.com/docs/concepts/resources/providers/
//
// There is no matching context in Terraform so this remains a Pulumi-level feature.
func (p *Provider) DiffConfig(
	ctx context.Context, req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	urn := resource.URN(req.GetUrn())
	label := fmt.Sprintf("%s.DiffConfig(%s)", p.label(), urn)
	glog.V(9).Infof("%s executing", label)

	return plugin.NewProviderServer(&configDiffer{
		schemaMap:   p.config,
		schemaInfos: p.info.Config,
	}).DiffConfig(ctx, req)
}

// Re-exported to reuse for Plugin Framework based providers.
func DiffConfig(
	config shim.SchemaMap, configInfos map[string]*SchemaInfo,
) func(
	urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	differ := &configDiffer{
		schemaMap:   config,
		schemaInfos: configInfos,
	}
	return differ.DiffConfig
}

type configDiffer struct {
	plugin.UnimplementedProvider
	schemaMap   shim.SchemaMap
	schemaInfos map[string]*SchemaInfo
}

// Schema may specify that changing a property requires a replacement.
func (p *configDiffer) forcesProviderReplace(path resource.PropertyPath) bool {
	schemaPath := PropertyPathToSchemaPath(path, p.schemaMap, p.schemaInfos)
	_, info, err := LookupSchemas(schemaPath, p.schemaMap, p.schemaInfos)
	if err != nil {
		contract.IgnoreError(err)
		return false
	}
	if info != nil && info.ForcesProviderReplace != nil {
		return *info.ForcesProviderReplace
	}
	return false
}

func (p *configDiffer) DiffConfig(
	urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	contract.Assertf(allowUnknowns, "Expected allowUnknowns to always be true for DiffConfig")

	// Seems that DiffIncludeUnknowns only accepts func (PropertyKey) bool to support ignoring
	// changes which is awkward for recursive changes, would be better if it supported
	// func(PropertyPath) bool. Instead of doing this, support IgnoreChanges by copying old
	// values to new values to disable the diff.
	newInputsIC, err := propertyvalue.ApplyIgnoreChanges(oldInputs, newInputs, ignoreChanges)
	if err != nil {
		return plugin.DiffResult{}, fmt.Errorf("Error applying ignoreChanges: %v", err)
	}

	objDiff := oldInputs.DiffIncludeUnknowns(newInputsIC)
	inputDiff := true
	detailedDiff := plugin.NewDetailedDiffFromObjectDiff(objDiff, inputDiff)

	// Ensure that if schema specifies ForceNew, a change becomes a replacement.
	for key, change := range detailedDiff {
		keyPath, err := resource.ParsePropertyPath(key)
		contract.AssertNoErrorf(err, "Unexpected failed parse of PropertyPath %q", key)
		if p.forcesProviderReplace(keyPath) {
			detailedDiff[key] = change.ToReplace()
		}
	}

	return plugin.DiffResult{
		// This is never true for DiffConfig at the provider level, making this explicit.
		DeleteBeforeReplace: false,
		// Everything else will be inferred from DetailedDiff by plugin.NewProviderServer.
		DetailedDiff: detailedDiff,
	}, nil
}

// Configure configures the underlying Terraform provider with the live Pulumi variable state.
//
// NOTE that validation and calling PreConfigureCallbacks are not called here but are called in CheckConfig. Pulumi will
// always call CheckConfig first and call Configure with validated (or extended) results of CheckConfig.
func (p *Provider) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	if req.AcceptSecrets {
		p.supportsSecrets = true
	}

	ctx = p.loggingContext(ctx, "")

	configEnc := NewConfigEncoding(p.config, p.info.Config)

	configMap, err := configEnc.UnmarshalProperties(req.GetArgs())
	if err != nil {
		return nil, err
	}

	configureSpan, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.Configure",
		opentracing.Tag{Key: "provider", Value: p.info.Name},
		opentracing.Tag{Key: "version", Value: p.version},
		opentracing.Tag{Key: "inputs", Value: resource.NewObjectProperty(configMap).String()},
	)
	defer configureSpan.Finish()
	p.memStats.collectMemStats(ctx, configureSpan)

	// Store the config values with their Pulumi names and values, before translation. This lets us fetch
	// them later on for purposes of (e.g.) config-based defaults.
	p.configValues = configMap

	config, err := buildTerraformConfig(ctx, p, configMap)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal config state")
	}

	// Now actually attempt to do the configuring and return its resulting error (if any).
	if err = p.tf.Configure(ctx, config); err != nil {
		replacedErr, replacementError := ReplaceConfigProperties(err.Error(), p.info.Config, p.config)
		if replacementError != nil {
			wrappedErr := errors.Wrapf(
				replacementError,
				"failed to replace config properties in error message",
			)
			return nil, multierror.Append(err, wrappedErr)
		}
		return nil, errors.New(replacedErr)
	}

	return &pulumirpc.ConfigureResponse{
		SupportsPreview: true,
	}, nil
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *Provider) Check(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	ctx = p.loggingContext(ctx, resource.URN(req.GetUrn()))
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Check): %s", t)
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.Check",
		opentracing.Tag{Key: "urn", Value: string(urn)},
	)
	defer span.Finish()
	p.memStats.collectMemStats(ctx, span)

	label := fmt.Sprintf("%s.Check(%s/%s)", p.label(), urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// Unmarshal the old and new properties.
	var olds resource.PropertyMap
	var err error
	if req.GetOlds() != nil {
		olds, err = plugin.UnmarshalProperties(req.GetOlds(), plugin.MarshalOptions{
			Label: fmt.Sprintf("%s.olds", label), KeepUnknowns: true,
		})
		if err != nil {
			return nil, err
		}
		olds, err = transformFromState(ctx, res.Schema, olds)
		if err != nil {
			return nil, err
		}

	}

	news, err := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{
		Label: fmt.Sprintf("%s.news", label), KeepUnknowns: true, SkipNulls: true,
	})
	if err != nil {
		return nil, err
	}

	if check := res.Schema.PreCheckCallback; check != nil {
		news, err = check(ctx, news, p.configValues.Copy())
		if err != nil {
			return nil, err
		}
	}

	// Now fetch the default values so that (a) we can return them to the caller and (b) so that validation
	// includes the default values.  Otherwise, the provider wouldn't be presented with its own defaults.
	tfname := res.TFName
	inputs, _, err := makeTerraformInputsWithoutTFDefaults(ctx,
		&PulumiResource{URN: urn, Properties: news, Seed: req.RandomSeed},
		p.configValues, olds, news, res.TF.Schema(), res.Schema.Fields)
	if err != nil {
		return nil, err
	}

	// Now check with the resource provider to see if the values pass muster.
	rescfg := MakeTerraformConfigFromInputs(ctx, p.tf, inputs)
	warns, errs := p.tf.ValidateResource(ctx, tfname, rescfg)
	for _, warn := range warns {
		warning := fmt.Sprintf("%v verification warning: %v", urn, warn)
		if p.host == nil {
			glog.Warning(warning)
		} else {
			if err = p.host.Log(ctx, diag.Warning, urn, warning); err != nil {
				return nil, err
			}
		}
	}

	// Now produce CheckFalures for any properties that failed verification.
	failures := p.adaptCheckFailures(ctx, urn, false /*isProvider*/, res.TF.Schema(), res.Schema.GetFields(), errs)

	// Now re-generate the inputs WITH the TF defaults
	inputs, assets, err := MakeTerraformInputs(ctx,
		&PulumiResource{URN: urn, Properties: news, Seed: req.RandomSeed},
		p.configValues, olds, news, res.TF.Schema(), res.Schema.Fields)
	if err != nil {
		return nil, err
	}

	// After all is said and done, we need to go back and return only what got populated as a diff from the origin.
	pinputs := MakeTerraformOutputs(
		ctx, p.tf, inputs, res.TF.Schema(), res.Schema.Fields, assets, false, p.supportsSecrets,
	)

	pinputsWithSecrets := MarkSchemaSecrets(ctx, res.TF.Schema(), res.Schema.Fields,
		resource.NewObjectProperty(pinputs)).ObjectValue()

	minputs, err := plugin.MarshalProperties(pinputsWithSecrets, plugin.MarshalOptions{
		Label: fmt.Sprintf("%s.inputs", label), KeepUnknowns: true, KeepSecrets: true,
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.CheckResponse{Inputs: minputs, Failures: failures}, nil
}

// For properties with MaxItemsOne, where the state is still an array
// (i.e. from a previous version without MaxItemsOne)
// we need to mark them for update manually in order to correct the state
// from an array to a flat type.
// The diff is otherwise ignored since MakeTerraformInputs won't touch
// the type if it in the right shape.
func markWronglyTypedMaxItemsOneStateDiff(
	schema shim.SchemaMap, info map[string]*SchemaInfo, olds resource.PropertyMap,
) bool {
	res := False()
	tr := func(pulumiPath resource.PropertyPath, localValue resource.PropertyValue) (resource.PropertyValue, error) {
		schemaPath := PropertyPathToSchemaPath(pulumiPath, schema, info)
		localSchema, info, err := LookupSchemas(schemaPath, schema, info)
		contract.IgnoreError(err)
		if IsMaxItemsOne(localSchema, info) && localValue.IsArray() {
			glog.V(9).Infof("Found type mismatch for %s, flagging for update.", pulumiPath)
			*res = true
		}
		return localValue, nil // don't change just visit
	}
	_, err := propertyvalue.TransformPropertyValue(make(resource.PropertyPath, 0), tr, resource.NewObjectProperty(olds))
	contract.AssertNoErrorf(err, "markWronglyTypedMaxItemsOneStateDiff should not return errors!")
	return *res
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *Provider) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	ctx = p.loggingContext(ctx, resource.URN(req.GetUrn()))
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Diff): %s", urn)
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.Diff",
		opentracing.Tag{Key: "urn", Value: string(urn)},
	)
	defer span.Finish()
	p.memStats.collectMemStats(ctx, span)

	label := fmt.Sprintf("%s.Diff(%s/%s)", p.label(), urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// To figure out if we have a replacement, perform the diff and then look for RequiresNew flags.
	olds, err := plugin.UnmarshalProperties(req.GetOlds(),
		plugin.MarshalOptions{Label: fmt.Sprintf("%s.olds", label), SkipNulls: true})
	if err != nil {
		return nil, err
	}
	olds, err = transformFromState(ctx, res.Schema, olds)
	if err != nil {
		return nil, err
	}

	state, err := MakeTerraformState(ctx, res, req.GetId(), olds)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshaling %s's instance state", urn)
	}

	news, err := plugin.UnmarshalProperties(req.GetNews(),
		plugin.MarshalOptions{Label: fmt.Sprintf("%s.news", label), KeepUnknowns: true})
	if err != nil {
		return nil, err
	}

	schema, fields := res.TF.Schema(), res.Schema.Fields

	config, _, err := MakeTerraformConfig(ctx, p, news, schema, fields)
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	}

	ic := newIgnoreChanges(ctx, schema, fields, olds, news, req.GetIgnoreChanges())

	diff, err := p.tf.Diff(ctx, res.TFName, state, config, shim.DiffOptions{
		IgnoreChanges: ic,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "diffing %s", urn)
	}

	dd := makeDetailedDiffExtra(ctx, schema, fields, olds, news, diff)
	detailedDiff, changes := dd.diffs, dd.changes

	// There are some providers/situations which `makeDetailedDiff` distorts the expected changes, leading
	// to changes being dropped by Pulumi.
	// Until we fix `makeDetailedDiff`, it is safer to refer to the Terraform Diff attribute length for setting
	// the DiffResponse.
	// We will still use `detailedDiff` for diff display purposes.

	// See also https://github.com/pulumi/pulumi-terraform-bridge/issues/1501.
	if p.info.XSkipDetailedDiffForChanges && len(diff.Attributes()) > 0 {
		changes = pulumirpc.DiffResponse_DIFF_SOME
		// Perhaps collectionDiffs can shed some light and locate the changes to the end-user.
		for path, diff := range dd.collectionDiffs {
			detailedDiff[path] = diff
		}
	}

	// If there were changes in this diff, check to see if we have a replacement.
	var replaces []string
	var replaced map[string]bool
	var properties []string

	if changes == pulumirpc.DiffResponse_DIFF_SOME {
		for k, d := range detailedDiff {
			// Turn the attribute name into a top-level property name by trimming everything after the first dot.
			if firstSep := strings.IndexAny(k, ".["); firstSep != -1 {
				k = k[:firstSep]
			}
			properties = append(properties, k)

			switch d.Kind {
			case pulumirpc.PropertyDiff_ADD_REPLACE,
				pulumirpc.PropertyDiff_UPDATE_REPLACE,
				pulumirpc.PropertyDiff_DELETE_REPLACE:

				replaces = append(replaces, k)
				if replaced == nil {
					replaced = make(map[string]bool)
				}
				replaced[k] = true
			}
		}
	}

	// For all properties that are ForceNew, but didn't change, assume they are stable.  Also recognize
	// overlays that have requested that we treat specific properties as stable.
	var stables []string
	schema.Range(func(k string, sch shim.Schema) bool {
		name, _, cust := getInfoFromTerraformName(k, schema, fields, false)
		if !replaced[string(name)] &&
			(sch.ForceNew() || (cust != nil && cust.Stable != nil && *cust.Stable)) {
			stables = append(stables, string(name))
		}
		return true
	})

	deleteBeforeReplace := len(replaces) > 0 &&
		(res.Schema.DeleteBeforeReplace || nameRequiresDeleteBeforeReplace(news, olds, schema, res.Schema))

	// If the upstream diff object indicates a replace is necessary and we have not
	// recorded any replaces, that means that `makeDetailedDiff` failed to translate a
	// property. This is known to happen for computed input properties:
	//
	// https://github.com/pulumi/pulumi-aws/issues/2971
	if (diff.RequiresNew() || diff.Destroy()) &&
		// In theory, we should be safe to set __meta as replaces whenever
		// `diff.RequiresNew() || diff.Destroy()` but by checking replaces we
		// limit the blast radius of this change to diffs that we know will panic
		// later on.
		len(replaces) == 0 {
		replaces = append(replaces, "__meta")
		changes = pulumirpc.DiffResponse_DIFF_SOME
	}

	if changes == pulumirpc.DiffResponse_DIFF_NONE &&
		markWronglyTypedMaxItemsOneStateDiff(schema, fields, olds) {

		changes = pulumirpc.DiffResponse_DIFF_SOME
	}

	return &pulumirpc.DiffResponse{
		Changes:             changes,
		Replaces:            replaces,
		Stables:             stables,
		DeleteBeforeReplace: deleteBeforeReplace,
		Diffs:               properties,
		DetailedDiff:        detailedDiff,
		HasDetailedDiff:     true,
	}, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transactional").
func (p *Provider) Create(ctx context.Context, req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
	ctx = p.loggingContext(ctx, resource.URN(req.GetUrn()))
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Create): %s", t)
	}

	createSpan, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.Create",
		opentracing.Tag{Key: "urn", Value: string(urn)},
	)
	defer createSpan.Finish()
	p.memStats.collectMemStats(ctx, createSpan)

	label := fmt.Sprintf("%s.Create(%s/%s)", p.label(), urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// To get Terraform to create a new resource, the ID must be blank and existing state must be empty (since the
	// resource does not exist yet), and the diff object should have no old state and all of the new state.
	config, assets, err := UnmarshalTerraformConfig(ctx,
		p, req.GetProperties(), res.TF.Schema(), res.Schema.Fields,
		fmt.Sprintf("%s.news", label))
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	}

	// To populate default timeouts, we take the timeouts from the resource schema and insert them into the diff
	timeouts, err := res.TF.DecodeTimeouts(config)
	if err != nil {
		return nil, errors.Errorf("error decoding timeout: %s", err)
	}

	diff, err := p.tf.Diff(ctx, res.TFName, nil, config, shim.DiffOptions{
		TimeoutOptions: shim.TimeoutOptions{
			ResourceTimeout:  timeouts,
			TimeoutOverrides: newTimeoutOverrides(shim.TimeoutCreate, req.Timeout),
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "diffing %s", urn)
	}

	var newstate shim.InstanceState
	var reasons []string
	if !req.GetPreview() {
		newstate, err = p.tf.Apply(ctx, res.TFName, nil, diff)
		if newstate == nil {
			if err == nil {
				return nil, fmt.Errorf("expected non-nil error with nil state during Create of %s", urn)
			}
			return nil, err
		}
		if newstate.ID() == "" {
			return nil, fmt.Errorf("expected non-empty ID for new state during Create of %s", urn)
		}

		if err != nil {
			reasons = append(reasons, errors.Wrapf(err, "creating %s", urn).Error())
		}
	} else {
		newstate, err = diff.ProposedState(res.TF, nil)
		if err != nil {
			return nil, fmt.Errorf("internal error: failed to fetch proposed state during diff (%w)", err)
		}
	}

	// Create the ID and property maps and return them.
	props, err := MakeTerraformResult(ctx, p.tf, newstate, res.TF.Schema(), res.Schema.Fields, assets, p.supportsSecrets)
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "converting result for %s", urn).Error())
	}

	if res.Schema.TransformOutputs != nil {
		var err error
		props, err = res.Schema.TransformOutputs(ctx, props)
		if err != nil {
			return nil, err
		}
	}

	mprops, err := plugin.MarshalProperties(props, plugin.MarshalOptions{
		Label:        fmt.Sprintf("%s.outs", label),
		KeepUnknowns: req.GetPreview(),
		KeepSecrets:  p.supportsSecrets,
	})
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "marshalling %s", urn).Error())
	}

	if len(reasons) != 0 {
		return nil, initializationError(newstate.ID(), mprops, reasons)
	}

	return &pulumirpc.CreateResponse{Id: newstate.ID(), Properties: mprops}, nil
}

// Read the current live state associated with a resource.  Enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource ID, but may also include some properties.
func (p *Provider) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	ctx = p.loggingContext(ctx, resource.URN(req.GetUrn()))
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Read): %s", t)
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.Read",
		opentracing.Tag{Key: "urn", Value: string(urn)},
	)
	defer span.Finish()
	p.memStats.collectMemStats(ctx, span)

	id := req.GetId()
	label := fmt.Sprintf("%s.Read(%s, %s/%s)", p.label(), id, urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// Manufacture Terraform attributes and state with the provided properties, in preparation for reading.
	oldInputs, err := plugin.UnmarshalProperties(req.GetInputs(), plugin.MarshalOptions{
		Label: fmt.Sprintf("%s.inputs", label), KeepUnknowns: true,
	})
	if err != nil {
		return nil, err
	}
	state, err := UnmarshalTerraformState(ctx, res, id, req.GetProperties(), fmt.Sprintf("%s.state", label))
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshaling %s's instance state", urn)
	}

	// If we are in a "get" rather than a "refresh", we should call the Terraform importer, if one is defined.
	isRefresh := len(req.GetProperties().GetFields()) != 0
	if !isRefresh && res.TF.Importer() != nil {
		glog.V(9).Infof("%s has TF Importer", res.TFName)

		state, err = res.runTerraformImporter(ctx, id, p)
		if err != nil {
			// Pass through any error running the importer
			return nil, err
		}
		if state == nil {
			// The resource is gone (or never existed). Return a gRPC response with no
			// resource ID set to indicate this.
			return &pulumirpc.ReadResponse{}, nil
		}
	}

	config, assets, err := MakeTerraformConfig(ctx, p, oldInputs, res.TF.Schema(), res.Schema.Fields)
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	}

	newstate, err := p.tf.Refresh(ctx, res.TFName, state, config)
	if err != nil {
		return nil, errors.Wrapf(err, "refreshing %s", urn)
	}

	// Store the ID and properties in the output.  The ID *should* be the same as the input ID, but in the case
	// that the resource no longer exists, we will simply return the empty string and an empty property map.
	if newstate != nil {
		props, err := MakeTerraformResult(ctx, p.tf, newstate, res.TF.Schema(), res.Schema.Fields, assets, p.supportsSecrets)
		if err != nil {
			return nil, err
		}

		if res.Schema.TransformOutputs != nil {
			var err error
			props, err = res.Schema.TransformOutputs(ctx, props)
			if err != nil {
				return nil, err
			}
		}

		mprops, err := plugin.MarshalProperties(props, plugin.MarshalOptions{
			Label:       label + ".state",
			KeepSecrets: p.supportsSecrets,
		})
		if err != nil {
			return nil, err
		}

		inputs, err := ExtractInputsFromOutputs(oldInputs, props, res.TF.Schema(), res.Schema.Fields, isRefresh)
		if err != nil {
			return nil, err
		}
		minputs, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{
			Label:       label + ".inputs",
			KeepSecrets: p.supportsSecrets,
		})
		if err != nil {
			return nil, err
		}

		return &pulumirpc.ReadResponse{Id: newstate.ID(), Properties: mprops, Inputs: minputs}, nil
	}

	// The resource is gone.
	return &pulumirpc.ReadResponse{}, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *Provider) Update(ctx context.Context, req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
	ctx = p.loggingContext(ctx, resource.URN(req.GetUrn()))
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Update): %s", t)
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.Update",
		opentracing.Tag{Key: "urn", Value: string(urn)},
	)
	defer span.Finish()
	p.memStats.collectMemStats(ctx, span)

	label := fmt.Sprintf("%s.Update(%s/%s)", p.label(), urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// In order to perform the update, we first need to calculate the Terraform view of the diff.
	olds, err := plugin.UnmarshalProperties(req.GetOlds(),
		plugin.MarshalOptions{Label: fmt.Sprintf("%s.olds", label), SkipNulls: true})
	if err != nil {
		return nil, err
	}
	olds, err = transformFromState(ctx, res.Schema, olds)
	if err != nil {
		return nil, err
	}

	state, err := MakeTerraformState(ctx, res, req.GetId(), olds)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshaling %s's instance state", urn)
	}

	news, err := plugin.UnmarshalProperties(req.GetNews(),
		plugin.MarshalOptions{Label: fmt.Sprintf("%s.news", label), KeepUnknowns: true})
	if err != nil {
		return nil, err
	}

	schema, fields := res.TF.Schema(), res.Schema.Fields

	config, assets, err := MakeTerraformConfig(ctx, p, news, schema, fields)
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	}

	ic := newIgnoreChanges(ctx, schema, fields, olds, news, req.GetIgnoreChanges())
	diff, err := p.tf.Diff(ctx, res.TFName, state, config, shim.DiffOptions{
		IgnoreChanges: ic,
		TimeoutOptions: shim.TimeoutOptions{
			TimeoutOverrides: newTimeoutOverrides(shim.TimeoutUpdate, req.Timeout),
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "diffing %s", urn)
	}
	if diff == nil {
		// It is very possible for us to get here with a nil diff: custom diffing behavior, etc. can cause
		// textual/structural changes not to be semantic changes. A better solution would be to change the result of
		// Diff to indicate no change, but that is a slightly riskier change that we'd rather not take at the current
		// moment.
		return &pulumirpc.UpdateResponse{Properties: req.GetOlds()}, nil
	}

	if diff.Destroy() || diff.RequiresNew() {
		return nil, fmt.Errorf("internal: expected diff to not require deletion or replacement"+
			" during Update of %s. Found delete=%t, replace=%t. This indicates a bug in provider.",
			urn, diff.Destroy(), diff.RequiresNew())
	}

	var newstate shim.InstanceState
	var reasons []string
	if !req.GetPreview() {
		newstate, err = p.tf.Apply(ctx, res.TFName, state, diff)
		if newstate == nil {
			if err != nil {
				return nil, err
			}

			return nil, fmt.Errorf("Resource provider reported that the resource did not exist while updating %s.\n\n"+
				"This is usually a result of the resource having been deleted outside of Pulumi, and can often be "+
				"fixed by running `pulumi refresh` before updating.", urn)
		}
		if newstate.ID() == "" {
			return nil, fmt.Errorf("expected non-empty ID for new state during Update of %s", urn)
		}
		if err != nil {
			reasons = append(reasons, errors.Wrapf(err, "updating %s", urn).Error())
		}
	} else {
		newstate, err = diff.ProposedState(res.TF, state)
		if err != nil {
			return nil, fmt.Errorf("internal error: failed to fetch proposed state during diff (%w)", err)
		}
	}

	props, err := MakeTerraformResult(ctx, p.tf, newstate, res.TF.Schema(), res.Schema.Fields, assets, p.supportsSecrets)
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "converting result for %s", urn).Error())
	}

	if res.Schema.TransformOutputs != nil {
		var err error
		props, err = res.Schema.TransformOutputs(ctx, props)
		if err != nil {
			return nil, err
		}
	}

	mprops, err := plugin.MarshalProperties(props, plugin.MarshalOptions{
		Label:        fmt.Sprintf("%s.outs", label),
		KeepUnknowns: req.GetPreview(),
		KeepSecrets:  p.supportsSecrets,
	})
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "marshalling %s", urn).Error())
	}

	if len(reasons) != 0 {
		return nil, initializationError(newstate.ID(), mprops, reasons)
	}
	return &pulumirpc.UpdateResponse{Properties: mprops}, nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *Provider) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*pbempty.Empty, error) {
	ctx = p.loggingContext(ctx, resource.URN(req.GetUrn()))
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Delete): %s", t)
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.Delete",
		opentracing.Tag{Key: "urn", Value: string(urn)},
	)
	defer span.Finish()
	p.memStats.collectMemStats(ctx, span)

	label := fmt.Sprintf("%s.Delete(%s/%s)", p.label(), urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// Fetch the resource attributes since many providers need more than just the ID to perform the delete.
	state, err := UnmarshalTerraformState(ctx, res, req.GetId(), req.GetProperties(), label)
	if err != nil {
		return nil, err
	}

	// Create a new destroy diff.
	diff := p.tf.NewDestroyDiff(ctx, res.TFName, shim.TimeoutOptions{
		TimeoutOverrides: newTimeoutOverrides(shim.TimeoutDelete, req.Timeout),
	})
	if _, err := p.tf.Apply(ctx, res.TFName, state, diff); err != nil {
		return nil, errors.Wrapf(err, "deleting %s", urn)
	}
	return &pbempty.Empty{}, nil
}

// Construct creates a new instance of the provided component resource and returns its state.
func (p *Provider) Construct(context.Context, *pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Construct is not yet implemented")
}

// Call dynamically executes a method in the provider associated with a component resource.
func (p *Provider) Call(ctx context.Context, req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Call is not yet implemented")
}

// Invoke dynamically executes a built-in function in the provider.
func (p *Provider) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	ctx = p.loggingContext(ctx, "")
	tok := tokens.ModuleMember(req.GetTok())
	ds, has := p.dataSources[tok]
	if !has {
		return nil, errors.Errorf("unrecognized data function (Invoke): %s", tok)
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.Invoke",
		opentracing.Tag{Key: "token", Value: string(tok)},
	)
	defer span.Finish()
	p.memStats.collectMemStats(ctx, span)

	label := fmt.Sprintf("%s.Invoke(%s)", p.label(), tok)
	glog.V(9).Infof("%s executing", label)

	// Unmarshal the arguments.
	args, err := plugin.UnmarshalProperties(req.GetArgs(), plugin.MarshalOptions{
		Label: fmt.Sprintf("%s.args", label), KeepUnknowns: true, SkipNulls: true,
	})
	if err != nil {
		return nil, err
	}

	// First, create the inputs.
	tfname := ds.TFName
	inputs, _, err := MakeTerraformInputs(
		ctx,
		&PulumiResource{Properties: args},
		p.configValues,
		nil, args,
		ds.TF.Schema(),
		ds.Schema.Fields)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't prepare resource %v input state", tfname)
	}

	// Next, ensure the inputs are valid before actually performing the invoaction.
	rescfg := MakeTerraformConfigFromInputs(ctx, p.tf, inputs)
	warns, errs := p.tf.ValidateDataSource(ctx, tfname, rescfg)
	for _, warn := range warns {
		if err = p.host.Log(ctx, diag.Warning, "", fmt.Sprintf("%v verification warning: %v", tok, warn)); err != nil {
			return nil, err
		}
	}

	// Now produce a return value of any properties that failed verification.
	var failures []*pulumirpc.CheckFailure
	for _, err := range errs {
		failures = append(failures, &pulumirpc.CheckFailure{
			Reason: err.Error(),
		})
	}

	// If there are no failures in verification, go ahead and perform the invocation.
	var ret *pbstruct.Struct
	if len(failures) == 0 {
		diff, err := p.tf.ReadDataDiff(ctx, tfname, rescfg)
		if err != nil {
			return nil, errors.Wrapf(err, "reading data source diff for %s", tok)
		}

		invoke, err := p.tf.ReadDataApply(ctx, tfname, diff)
		if err != nil {
			return nil, errors.Wrapf(err, "invoking %s", tok)
		}

		// Add the special "id" attribute if it wasn't listed in the schema
		props, err := MakeTerraformResult(ctx, p.tf, invoke, ds.TF.Schema(), ds.Schema.Fields, nil, p.supportsSecrets)
		if err != nil {
			return nil, err
		}
		if _, has := props["id"]; !has && invoke != nil {
			props["id"] = resource.NewStringProperty(invoke.ID())
		}

		ret, err = plugin.MarshalProperties(
			props,
			plugin.MarshalOptions{Label: fmt.Sprintf("%s.returns", label)})
		if err != nil {
			return nil, err
		}
	}

	return &pulumirpc.InvokeResponse{
		Return:   ret,
		Failures: failures,
	}, nil
}

// StreamInvoke dynamically executes a built-in function in the provider. The result is streamed
// back as a series of messages.
func (p *Provider) StreamInvoke(
	req *pulumirpc.InvokeRequest, server pulumirpc.ResourceProvider_StreamInvokeServer,
) error {
	tok := tokens.ModuleMember(req.GetTok())
	return errors.Errorf("unrecognized data function (StreamInvoke): %s", tok)
}

// GetPluginInfo implements an RPC call that returns the version of this plugin.
func (p *Provider) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	getPluginInfoSpan, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.GetPluginInfo",
		opentracing.Tag{Key: "provider", Value: p.info.Name},
		opentracing.Tag{Key: "version", Value: p.version},
	)
	defer getPluginInfoSpan.Finish()
	p.memStats.collectMemStats(ctx, getPluginInfoSpan)

	return &pulumirpc.PluginInfo{
		Version: p.version,
	}, nil
}

// Cancel requests that the provider cancel all ongoing RPCs. For TF, this is a no-op.
func (p *Provider) Cancel(ctx context.Context, req *pbempty.Empty) (*pbempty.Empty, error) {
	return &pbempty.Empty{}, nil
}

func (p *Provider) GetMapping(
	ctx context.Context, req *pulumirpc.GetMappingRequest,
) (*pulumirpc.GetMappingResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "sdkv2.GetMapping",
		opentracing.Tag{Key: "key", Value: req.Key},
	)
	defer span.Finish()
	p.memStats.collectMemStats(ctx, span)

	// The prototype converter used the key "tf", but the new plugin converter uses "terraform". For now
	// support both, eventually we can remove the "tf" key.
	if req.Key == "tf" || req.Key == "terraform" {
		info := MarshalProviderInfo(&p.info)
		mapping, err := json.Marshal(info)
		if err != nil {
			return nil, err
		}
		return &pulumirpc.GetMappingResponse{
			Provider: p.info.Name,
			Data:     mapping,
		}, nil
	}

	// An empty response is valid for GetMapping, it means we don't have a mapping for the given key
	return &pulumirpc.GetMappingResponse{}, nil
}

func initializationError(id string, props *pbstruct.Struct, reasons []string) error {
	contract.Assertf(len(reasons) > 0, "initializationError must be passed at least one reason")
	detail := pulumirpc.ErrorResourceInitFailed{
		Id:         id,
		Properties: props,
		Reasons:    reasons,
	}
	return rpcerror.WithDetails(rpcerror.New(codes.Unknown, reasons[0]), &detail)
}

func (p *ProviderInfo) RenameResourceWithAlias(resourceName string, legacyTok tokens.Type, newTok tokens.Type,
	legacyModule string, newModule string, info *ResourceInfo,
) {
	resourcePrefix := p.Name + "_"
	legacyResourceName := resourceName + RenamedEntitySuffix
	if info == nil {
		info = &ResourceInfo{}
	}
	legacyInfo := *info
	currentInfo := *info

	legacyInfo.Tok = legacyTok
	legacyType := legacyInfo.Tok.String()

	if newTok != "" {
		legacyTok = newTok
	}

	currentInfo.Tok = legacyTok
	currentInfo.Aliases = []AliasInfo{
		{Type: &legacyType},
	}

	if legacyInfo.Docs == nil {
		legacyInfo.Docs = &DocInfo{
			Source: resourceName[len(resourcePrefix):] + ".html.markdown",
		}
	}

	legacyInfo.DeprecationMessage = fmt.Sprintf("%s has been deprecated in favor of %s",
		generateResourceName(legacyInfo.Tok.Module().Package(), strings.ToLower(legacyModule),
			legacyInfo.Tok.Name().String()),
		generateResourceName(currentInfo.Tok.Module().Package(), strings.ToLower(newModule),
			currentInfo.Tok.Name().String()))
	p.Resources[resourceName] = &currentInfo
	p.Resources[legacyResourceName] = &legacyInfo
	p.P.ResourcesMap().Set(legacyResourceName, p.P.ResourcesMap().Get(resourceName))
}

const (
	RenamedEntitySuffix string = "_legacy"
)

func (p *ProviderInfo) RenameDataSource(resourceName string, legacyTok tokens.ModuleMember, newTok tokens.ModuleMember,
	legacyModule string, newModule string, info *DataSourceInfo,
) {
	resourcePrefix := p.Name + "_"
	legacyResourceName := resourceName + RenamedEntitySuffix
	if info == nil {
		info = &DataSourceInfo{}
	}
	legacyInfo := *info
	currentInfo := *info

	legacyInfo.Tok = legacyTok

	if newTok != "" {
		legacyTok = newTok
	}

	currentInfo.Tok = legacyTok

	if legacyInfo.Docs == nil {
		legacyInfo.Docs = &DocInfo{
			Source: resourceName[len(resourcePrefix):] + ".html.markdown",
		}
	}

	legacyInfo.DeprecationMessage = fmt.Sprintf("%s has been deprecated in favor of %s",
		generateResourceName(legacyInfo.Tok.Module().Package(), strings.ToLower(legacyModule),
			legacyInfo.Tok.Name().String()),
		generateResourceName(currentInfo.Tok.Module().Package(), strings.ToLower(newModule),
			currentInfo.Tok.Name().String()))
	p.DataSources[resourceName] = &currentInfo
	p.DataSources[legacyResourceName] = &legacyInfo
	p.P.DataSourcesMap().Set(legacyResourceName, p.P.DataSourcesMap().Get(resourceName))
}

func generateResourceName(packageName tokens.Package, moduleName string, moduleMemberName string) string {
	// We don't want DeprecationMessages that read
	// `postgresql.index.DefaultPrivileg` has been deprecated in favour of `postgresql.index.DefaultPrivileges`
	// we would never use `index` in a reference to the Class. So we should remove this where needed
	if moduleName == "" || moduleName == "index" {
		return fmt.Sprintf("%s.%s", packageName, moduleMemberName)
	}

	return fmt.Sprintf("%s.%s.%s", packageName, moduleName, moduleMemberName)
}

// SetAutonaming auto-names all resource properties that are literally called "name".
//
// The effect is identical to configuring each matching property with [AutoName]. Pulumi will propose an auto-computed
// value for these properties when no value is given by the user program. If a property was required before auto-naming,
// it becomes optional.
//
// The maxLength and separator parameters configure how AutoName generates default values. See [AutoNameOptions].
//
// SetAutonaming will skip properties that already have a [SchemaInfo] entry in [ResourceInfo.Fields], assuming those
// are already customized by the user. If those properties need AutoName functionality, please use AutoName directly to
// populate their SchemaInfo entry.
//
// Note that when constructing a ProviderInfo incrementally, some care is required to make sure SetAutonaming is called
// after [ProviderInfo.Resources] map is fully populated, as it relies on this map to find resources to auto-name.
func (p *ProviderInfo) SetAutonaming(maxLength int, separator string) {
	if p.P == nil {
		glog.Warningln("SetAutonaming found a `ProviderInfo.P` nil. No Autonames were applied.")
		return
	}

	const nameProperty = "name"
	for resname, res := range p.Resources {
		if schema := p.P.ResourcesMap().Get(resname); schema != nil {
			// Only apply auto-name to input properties (Optional || Required)
			// of type `string` named `name`
			if sch, hasName := schema.Schema().GetOk(nameProperty); hasName &&
				(sch.Optional() || sch.Required()) && // Is an input type
				sch.Type() == shim.TypeString { // has type string

				if _, hasfield := res.Fields[nameProperty]; !hasfield {
					ensureMap(&res.Fields)[nameProperty] = AutoName(nameProperty, maxLength, separator)
				}
			}
		}
	}
}

// SetProviderLicense is used to pass a license type to a provider metadata
func SetProviderLicense(license TFProviderLicense) *TFProviderLicense {
	return &license
}

func ref[T any](t T) *T { return &t }

// True is used for interactions in the providers that require a pointer to true
func True() *bool { return ref(true) }

// False is used for interactions in the providers that require a pointer to false
func False() *bool { return ref(false) }

func transformFromState(
	ctx context.Context, res *ResourceInfo, inputs resource.PropertyMap,
) (resource.PropertyMap, error) {
	if res == nil {
		return inputs, nil
	}
	f := res.TransformFromState
	if f == nil {
		return inputs, nil
	}
	o, err := f(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("transforming inputs: %w", err)
	}
	return o, nil
}

// If a custom timeout has been set for this method, overwrite the default timeout.
func newTimeoutOverrides(key shim.TimeoutKey, maybeTimeoutSeconds float64) map[shim.TimeoutKey]time.Duration {
	timeoutOverrides := map[shim.TimeoutKey]time.Duration{}
	if maybeTimeoutSeconds != 0 {
		timeoutOverrides[shim.TimeoutCreate] = time.Duration(maybeTimeoutSeconds * float64(time.Second))
	}
	return timeoutOverrides
}
