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

package tfbridge

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"google.golang.org/grpc/status"

	"github.com/golang/glog"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	pbstruct "github.com/golang/protobuf/ptypes/struct"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi/pkg/v2/resource/provider"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/rpcutil/rpcerror"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"

	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
)

// Provider implements the Pulumi resource provider operations for any Terraform plugin.
type Provider struct {
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
func (res *Resource) runTerraformImporter(id string, provider *Provider) (shim.InstanceState, error) {
	contract.Assert(res.TF.Importer() != nil)

	// Run the importer defined in the Terraform resource schema
	states, err := res.TF.Importer()(res.TFName, id, provider.tf.Meta())
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

// NewProvider creates a new Pulumi RPC server wired up to the given host and wrapping the given Terraform provider.
func NewProvider(ctx context.Context, host *provider.HostClient, module string, version string,
	tf shim.Provider, info ProviderInfo, pulumiSchema []byte) *Provider {
	p := &Provider{
		host:         host,
		module:       module,
		version:      version,
		tf:           tf,
		info:         info,
		config:       tf.Schema(),
		pulumiSchema: pulumiSchema,
	}
	p.setLoggingContext(ctx)
	p.initResourceMaps()
	return p
}

var _ pulumirpc.ResourceProviderServer = (*Provider)(nil)

func (p *Provider) pkg() tokens.Package          { return tokens.Package(p.module) }
func (p *Provider) baseConfigMod() tokens.Module { return tokens.Module(p.pkg() + ":config") }
func (p *Provider) baseDataMod() tokens.Module   { return tokens.Module(p.pkg() + ":data") }
func (p *Provider) configMod() tokens.Module     { return p.baseConfigMod() + "/vars" }

func (p *Provider) setLoggingContext(ctx context.Context) {
	if p.host != nil {
		log.SetOutput(&LogRedirector{
			writers: map[string]func(string) error{
				tfTracePrefix: func(msg string) error { return p.host.Log(ctx, diag.Debug, "", msg) },
				tfDebugPrefix: func(msg string) error { return p.host.Log(ctx, diag.Debug, "", msg) },
				tfInfoPrefix:  func(msg string) error { return p.host.Log(ctx, diag.Info, "", msg) },
				tfWarnPrefix:  func(msg string) error { return p.host.Log(ctx, diag.Warning, "", msg) },
				tfErrorPrefix: func(msg string) error { return p.host.Log(ctx, diag.Error, "", msg) },
			},
		})
	}
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
			tok = tokens.Type(string(p.pkg()) + ":" + camelName + ":" + pascalName)
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
			tok = tokens.ModuleMember(string(p.baseDataMod()) + ":" + camelName)
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
		"Expected all Terraform resources in this module to have a '%v' prefix", prefix)
	name = name[len(prefix):]
	return TerraformToPulumiName(name, nil, nil, false),
		TerraformToPulumiName(name, nil, nil, true)
}

func convertStringToPropertyValue(s string, typ shim.ValueType) (resource.PropertyValue, error) {
	// If the schema expects a string, we can just return this as-is.
	if typ == shim.TypeString {
		return resource.NewStringProperty(s), nil
	}

	// Otherwise, we will attempt to deserialize the input string as JSON and convert the result into a Pulumi
	// property. If the input string is empty, we will return an appropriate zero value.
	if s == "" {
		switch typ {
		case shim.TypeBool:
			return resource.NewPropertyValue(false), nil
		case shim.TypeInt, shim.TypeFloat:
			return resource.NewPropertyValue(0), nil
		case shim.TypeList, shim.TypeSet:
			return resource.NewPropertyValue([]interface{}{}), nil
		default:
			return resource.NewPropertyValue(map[string]interface{}{}), nil
		}
	}

	var jsonValue interface{}
	if err := json.Unmarshal([]byte(s), &jsonValue); err != nil {
		return resource.PropertyValue{}, err
	}
	return resource.NewPropertyValue(jsonValue), nil
}

// GetSchema returns the JSON-encoded schema for this provider's package.
func (p *Provider) GetSchema(ctx context.Context,
	req *pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error) {

	if v := req.GetVersion(); v > 1 {
		return nil, errors.Errorf("unsupported schema version %v", v)
	}
	return &pulumirpc.GetSchemaResponse{
		Schema: string(p.pulumiSchema),
	}, nil
}

// CheckConfig validates the configuration for this Terraform provider.
func (p *Provider) CheckConfig(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CheckConfig is not yet implemented")

	// TO_DO - revert this comment!!
	//urn := resource.URN(req.GetUrn())
	//label := fmt.Sprintf("%s.CheckConfig(%s)", p.label(), urn)
	//glog.V(9).Infof("%s executing", label)
	//
	//news, validationErrors := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{
	//	Label:        fmt.Sprintf("%s.news", label),
	//	KeepUnknowns: true,
	//	SkipNulls:    true,
	//	RejectAssets: true,
	//})
	//if validationErrors != nil {
	//	return nil, errors.Wrap(validationErrors, "CheckConfig failed because of malformed resource inputs")
	//}
	//
	//config, validationErrors := buildTerraformConfig(p, news)
	//if validationErrors != nil {
	//	return nil, errors.Wrap(validationErrors, "could not marshal config state")
	//}
	//
	//if p.info.PreConfigureCallback != nil {
	//	if validationErrors = p.info.PreConfigureCallback(news, config); validationErrors != nil {
	//		return nil, validationErrors
	//	}
	//}
	//
	//// This replicates the flow in the validateProviderConfig func where we check for missingKeys first
	//missingKeys, validationErrors := validateProviderConfig(ctx, p, config)
	//if len(missingKeys) > 0 {
	//	return &pulumirpc.CheckResponse{Inputs: req.GetNews(), Failures: missingKeys}, nil
	//}
	//if validationErrors != nil {
	//	return nil, validationErrors
	//}
	//
	//return &pulumirpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func buildTerraformConfig(p *Provider, vars resource.PropertyMap) (shim.ResourceConfig, error) {
	tfVars := make(resource.PropertyMap)
	for k, v := range vars {
		// we need to skip the version as adding that will cause the provider validation to fail
		if string(k) == "version" {
			continue
		}
		if _, has := p.info.ExtraConfig[string(k)]; !has {
			tfVars[k] = v
		}
	}

	inputs, _, err := MakeTerraformInputs(nil, tfVars, nil, nil, p.config, p.info.Config)
	if err != nil {
		return nil, err
	}

	return MakeTerraformConfigFromInputs(p.tf, inputs), nil
}

func validateProviderConfig(ctx context.Context, p *Provider, config shim.ResourceConfig) (
	[]*pulumirpc.ConfigureErrorMissingKeys_MissingKey, error) {

	var missingKeys []*pulumirpc.ConfigureErrorMissingKeys_MissingKey
	p.config.Range(func(key string, meta shim.Schema) bool {
		if meta.Required() && !config.IsSet(key) {
			name := TerraformToPulumiName(key, meta, nil, false)
			fullyQualifiedName := tokens.NewModuleToken(p.pkg(), tokens.ModuleName(name))

			// TF descriptions often have newlines in inopportune positions. This makes them present
			// a little better in our console output.
			descriptionWithoutNewlines := strings.Replace(meta.Description(), "\n", " ", -1)
			missingKeys = append(missingKeys, &pulumirpc.ConfigureErrorMissingKeys_MissingKey{
				Name:        fullyQualifiedName.String(),
				Description: descriptionWithoutNewlines,
			})
		}
		return true
	})

	if len(missingKeys) > 0 {
		return missingKeys, nil
	}

	// Perform validation of the config state so we can offer nice errors.
	warns, errs := p.tf.Validate(config)
	for _, warn := range warns {
		if err := p.host.Log(ctx, diag.Warning, "", fmt.Sprintf("provider config warning: %v", warn)); err != nil {
			return nil, err
		}
	}

	if len(errs) > 0 {
		return nil, errors.Wrap(multierror.Append(nil, errs...), "could not validate provider configuration")
	}

	return nil, nil
}

// DiffConfig diffs the configuration for this Terraform provider.
func (p *Provider) DiffConfig(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DiffConfig is not yet implemented")

	// TO_DO - revert this comment!!
	//urn := resource.URN(req.GetUrn())
	//label := fmt.Sprintf("%s.DiffConfig(%s)", p.label(), urn)
	//glog.V(9).Infof("%s executing", label)
	//
	//// There is no logic in the TF provider that suggests that provider level config
	//// should force a new provider. Therefore, we are going to do this based on our
	//// own schema overrides. We should use ForceNew as part of the SchemaInfo to do this
	//
	//// Create a Resource Schema from the config
	//r := &schema.Resource{Schema: p.tf.Schema}
	//
	//var olds resource.PropertyMap
	//var err error
	//if req.GetOlds() != nil {
	//	olds, err = plugin.UnmarshalProperties(req.GetOlds(), plugin.MarshalOptions{
	//		Label: fmt.Sprintf("%s.olds", label), KeepUnknowns: true})
	//	if err != nil {
	//		return nil, err
	//	}
	//}
	//
	//attrs, meta, err := MakeTerraformAttributes(r, olds, r.Schema, p.info.Config, p.configValues, false)
	//if err != nil {
	//	return nil, errors.Wrapf(err, "preparing %s's old property state", urn)
	//}
	//state := &terraform.InstanceState{ID: req.GetId(), Attributes: attrs, Meta: meta}
	//
	//// Create a resource Config for the new configuration
	//news, err := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{
	//	Label: fmt.Sprintf("%s.news", label), KeepUnknowns: true, SkipNulls: true})
	//if err != nil {
	//	return nil, err
	//}
	//config, err := MakeTerraformConfig(nil, news, r.Schema, p.info.Config, nil, p.configValues, false)
	//if err != nil {
	//	return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	//}
	//diff, err := r.Diff(state, config, nil)
	//if err != nil {
	//	return nil, errors.Wrapf(err, "diffing %s", urn)
	//}
	//
	//detailedDiff := makeDetailedDiff(r.Schema, p.info.Config, olds, news, diff)
	//
	//var replaces []string
	//var changes pulumirpc.DiffResponse_DiffChanges
	//var properties []string
	//hasChanges := len(detailedDiff) > 0
	//if hasChanges {
	//	changes = pulumirpc.DiffResponse_DIFF_SOME
	//	for k := range detailedDiff {
	//		// Turn the attribute name into a top-level property name by trimming everything after the first dot.
	//		if firstSep := strings.IndexAny(k, ".["); firstSep != -1 {
	//			k = k[:firstSep]
	//		}
	//		properties = append(properties, k)
	//		if p.info.Config != nil && p.info.Config[k] != nil {
	//			config := p.info.Config[k]
	//			// now is it a ForceNew or not
	//			if config.ForceNew != nil && *config.ForceNew {
	//				replaces = append(replaces, k)
	//			} else {
	//				properties = append(properties, k)
	//			}
	//		}
	//	}
	//} else {
	//	changes = pulumirpc.DiffResponse_DIFF_NONE
	//}
	//
	//return &pulumirpc.DiffResponse{
	//	Changes:         changes,
	//	Replaces:        replaces,
	//	Diffs:           properties,
	//	DetailedDiff:    detailedDiff,
	//	HasDetailedDiff: true,
	//}, nil
}

// Configure configures the underlying Terraform provider with the live Pulumi variable state.
func (p *Provider) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {

	if req.AcceptSecrets {
		p.supportsSecrets = true
	}

	p.setLoggingContext(ctx)
	// Fetch the map of tokens to values.  It will be in the form of fully qualified tokens, so
	// we will need to translate into simply the configuration variable names.
	vars := make(resource.PropertyMap)
	for k, v := range req.GetVariables() {
		mm, err := tokens.ParseModuleMember(k)
		if err != nil {
			return nil, errors.Wrapf(err, "malformed configuration token '%v'", k)
		}
		if mm.Module() != p.baseConfigMod() && mm.Module() != p.configMod() {
			continue
		}

		typ := shim.TypeString
		_, sch, _ := getInfoFromPulumiName(resource.PropertyKey(mm.Name()), p.config, p.info.Config, false)
		if sch != nil {
			typ = sch.Type()
		}
		pv, err := convertStringToPropertyValue(v, typ)
		if err != nil {
			return nil, errors.Wrapf(err, "malformed configuration value '%v'", v)
		}
		vars[resource.PropertyKey(mm.Name())] = pv
	}

	// Store the config values with their Pulumi names and values, before translation. This lets us fetch
	// them later on for purposes of (e.g.) config-based defaults.
	p.configValues = vars

	config, err := buildTerraformConfig(p, vars)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal config state")
	}

	if req.Variables == nil {
		// We only follow this path if the CLI hasbn't already called CheckConfig
		if p.info.PreConfigureCallback != nil {
			if err = p.info.PreConfigureCallback(vars, config); err != nil {
				return nil, err
			}
		}

		missingKeys, validationErrors := validateProviderConfig(ctx, p, config)
		if len(missingKeys) > 0 {
			err = rpcerror.WithDetails(
				rpcerror.New(codes.InvalidArgument, "required configuration keys were missing"),
				&pulumirpc.ConfigureErrorMissingKeys{MissingKeys: missingKeys})
			return nil, err
		}
		if validationErrors != nil {
			return nil, validationErrors
		}
	}

	// Now actually attempt to do the configuring and return its resulting error (if any).
	if err = p.tf.Configure(config); err != nil {
		return nil, err
	}

	return &pulumirpc.ConfigureResponse{}, nil
}

// Parse the TF error of a missing field:
// https://github.com/hashicorp/terraform/blob/7f5ffbfe9027c34c4ce1062a42b6e8d80b5504e0/helper/schema/schema.go#L1356
var requiredFieldRegex = regexp.MustCompile("\"(.*?)\": required field is not set")

func (p *Provider) formatFailureReason(res Resource, reason string) string {
	// Translate the name in missing-required-field error from TF to Pulumi naming scheme
	parts := requiredFieldRegex.FindStringSubmatch(reason)
	if len(parts) == 2 {
		schema := getSchema(res.TF.Schema(), parts[1])
		info := res.Schema.Fields[parts[1]]
		if schema != nil {
			name := TerraformToPulumiName(parts[1], schema, info, false)
			message := fmt.Sprintf("Missing required property '%s'", name)
			// If a required field is missing and the value can be set via config,
			// extend the error with a hint to set the proper config value
			field := res.Schema.Fields[name]
			if field != nil && field.Default != nil {
				if configKey := field.Default.Config; configKey != "" {
					format := "%s. Either set it explicitly or configure it with 'pulumi config set %s:%s <value>'."
					return fmt.Sprintf(format, message, p.module, configKey)
				}
			}
			return message
		}
	}

	return reason
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *Provider) Check(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	p.setLoggingContext(ctx)
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Check): %s", t)
	}

	label := fmt.Sprintf("%s.Check(%s/%s)", p.label(), urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// Unmarshal the old and new properties.
	var olds resource.PropertyMap
	var err error
	if req.GetOlds() != nil {
		olds, err = plugin.UnmarshalProperties(req.GetOlds(), plugin.MarshalOptions{
			Label: fmt.Sprintf("%s.olds", label), KeepUnknowns: true})
		if err != nil {
			return nil, err
		}
	}

	news, err := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{
		Label: fmt.Sprintf("%s.news", label), KeepUnknowns: true, SkipNulls: true})
	if err != nil {
		return nil, err
	}

	// Now fetch the default values so that (a) we can return them to the caller and (b) so that validation
	// includes the default values.  Otherwise, the provider wouldn't be presented with its own defaults.
	tfname := res.TFName
	inputs, assets, err := MakeTerraformInputs(
		&PulumiResource{URN: urn, Properties: news}, p.configValues, olds, news, res.TF.Schema(), res.Schema.Fields)
	if err != nil {
		return nil, err
	}

	// Now check with the resource provider to see if the values pass muster.
	rescfg := MakeTerraformConfigFromInputs(p.tf, inputs)
	warns, errs := p.tf.ValidateResource(tfname, rescfg)
	for _, warn := range warns {
		if err = p.host.Log(ctx, diag.Warning, urn, fmt.Sprintf("%v verification warning: %v", urn, warn)); err != nil {
			return nil, err
		}
	}

	// Now produce a return value of any properties that failed verification.
	var failures []*pulumirpc.CheckFailure
	for _, err := range errs {
		failures = append(failures, &pulumirpc.CheckFailure{
			Reason: p.formatFailureReason(res, err.Error()),
		})
	}

	// After all is said and done, we need to go back and return only what got populated as a diff from the origin.
	pinputs := MakeTerraformOutputs(p.tf, inputs, res.TF.Schema(), res.Schema.Fields, assets, false, p.supportsSecrets)
	minputs, err := plugin.MarshalProperties(pinputs, plugin.MarshalOptions{
		Label: fmt.Sprintf("%s.inputs", label), KeepUnknowns: true})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.CheckResponse{Inputs: minputs, Failures: failures}, nil
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *Provider) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	p.setLoggingContext(ctx)
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Diff): %s", urn)
	}

	label := fmt.Sprintf("%s.Diff(%s/%s)", p.label(), urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// To figure out if we have a replacement, perform the diff and then look for RequiresNew flags.
	olds, err := plugin.UnmarshalProperties(req.GetOlds(),
		plugin.MarshalOptions{Label: fmt.Sprintf("%s.olds", label), SkipNulls: true})
	if err != nil {
		return nil, err
	}
	state, err := MakeTerraformState(res, req.GetId(), olds)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshaling %s's instance state", urn)
	}

	news, err := plugin.UnmarshalProperties(req.GetNews(),
		plugin.MarshalOptions{Label: fmt.Sprintf("%s.news", label), KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	config, _, err := MakeTerraformConfig(p, news, res.TF.Schema(), res.Schema.Fields)
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	}

	diff, err := p.tf.Diff(res.TFName, state, config)
	if err != nil {
		return nil, errors.Wrapf(err, "diffing %s", urn)
	}

	doIgnoreChanges(res.TF.Schema(), res.Schema.Fields, olds, news, req.GetIgnoreChanges(), diff)
	detailedDiff := makeDetailedDiff(res.TF.Schema(), res.Schema.Fields, olds, news, diff)

	// If there were changes in this diff, check to see if we have a replacement.
	var replaces []string
	var replaced map[string]bool
	var changes pulumirpc.DiffResponse_DiffChanges
	var properties []string
	hasChanges := len(detailedDiff) > 0
	if hasChanges {
		changes = pulumirpc.DiffResponse_DIFF_SOME
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
	} else {
		changes = pulumirpc.DiffResponse_DIFF_NONE
	}

	// For all properties that are ForceNew, but didn't change, assume they are stable.  Also recognize
	// overlays that have requested that we treat specific properties as stable.
	var stables []string
	res.TF.Schema().Range(func(k string, sch shim.Schema) bool {
		name, _, cust := getInfoFromTerraformName(k, res.TF.Schema(), res.Schema.Fields, false)
		if !replaced[string(name)] &&
			(sch.ForceNew() || (cust != nil && cust.Stable != nil && *cust.Stable)) {
			stables = append(stables, string(name))
		}
		return true
	})

	deleteBeforeReplace := len(replaces) > 0 &&
		(res.Schema.DeleteBeforeReplace || nameRequiresDeleteBeforeReplace(news, res.TF.Schema(), res.Schema.Fields))

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
	p.setLoggingContext(ctx)
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Create): %s", t)
	}

	label := fmt.Sprintf("%s.Create(%s/%s)", p.label(), urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// To get Terraform to create a new resource, the ID must be blank and existing state must be empty (since the
	// resource does not exist yet), and the diff object should have no old state and all of the new state.
	config, assets, err := UnmarshalTerraformConfig(
		p, req.GetProperties(), res.TF.Schema(), res.Schema.Fields,
		fmt.Sprintf("%s.news", label))
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	}

	diff, err := p.tf.Diff(res.TFName, nil, config)
	if err != nil {
		return nil, errors.Wrapf(err, "diffing %s", urn)
	}

	// To populate default timeouts, we take the timeouts from the resource schema and insert them into the diff
	timeouts, err := res.TF.DecodeTimeouts(config)
	if err != nil {
		return nil, errors.Errorf("error decoding timeout: %s", err)
	}
	if err = diff.EncodeTimeouts(timeouts); err != nil {
		return nil, errors.Errorf("error setting default timeouts to diff: %s", err)
	}

	// If a custom timeout has been set for this method, overwrite the default timeout
	if req.Timeout != 0 {
		diff.SetTimeout(req.Timeout, shim.TimeoutCreate)
	}

	newstate, err := p.tf.Apply(res.TFName, nil, diff)
	if newstate == nil {
		if err == nil {
			return nil, fmt.Errorf("expected non-nil error with nil state during Create of %s", urn)
		}
		return nil, err
	}

	if newstate.ID() == "" {
		return nil, fmt.Errorf("expected non-empty ID for new state during Create of %s", urn)
	}
	reasons := make([]string, 0)
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "creating %s", urn).Error())
	}

	// Create the ID and property maps and return them.
	props, err := MakeTerraformResult(p.tf, newstate, res.TF.Schema(), res.Schema.Fields, assets, p.supportsSecrets)
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "converting result for %s", urn).Error())
	}

	mprops, err := plugin.MarshalProperties(props, plugin.MarshalOptions{Label: fmt.Sprintf("%s.outs", label)})
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
	p.setLoggingContext(ctx)
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Read): %s", t)
	}

	id := req.GetId()
	label := fmt.Sprintf("%s.Read(%s, %s/%s)", p.label(), id, urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// Manufacture Terraform attributes and state with the provided properties, in preparation for reading.
	oldInputs, err := plugin.UnmarshalProperties(req.GetInputs(), plugin.MarshalOptions{
		Label: fmt.Sprintf("%s.inputs", label), KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	state, err := UnmarshalTerraformState(res, id, req.GetProperties(), fmt.Sprintf("%s.state", label))
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshaling %s's instance state", urn)
	}

	// If we are in a "get" rather than a "refresh", we should call the Terraform importer, if one is defined.
	isRefresh := len(req.GetProperties().GetFields()) != 0
	if !isRefresh && res.TF.Importer() != nil {
		glog.V(9).Infof("%s has TF Importer", res.TFName)

		state, err = res.runTerraformImporter(id, p)
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

	newstate, err := p.tf.Refresh(res.TFName, state)
	if err != nil {
		return nil, errors.Wrapf(err, "refreshing %s", urn)
	}

	// Store the ID and properties in the output.  The ID *should* be the same as the input ID, but in the case
	// that the resource no longer exists, we will simply return the empty string and an empty property map.
	if newstate != nil {
		props, err := MakeTerraformResult(p.tf, newstate, res.TF.Schema(), res.Schema.Fields, nil, p.supportsSecrets)
		if err != nil {
			return nil, err
		}

		mprops, err := plugin.MarshalProperties(props, plugin.MarshalOptions{Label: label + ".state"})
		if err != nil {
			return nil, err
		}

		inputs, err := extractInputsFromOutputs(oldInputs, props, res.TF.Schema(), res.Schema.Fields, isRefresh)
		if err != nil {
			return nil, err
		}
		minputs, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{Label: label + ".inputs"})
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
	p.setLoggingContext(ctx)
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Update): %s", t)
	}

	label := fmt.Sprintf("%s.Update(%s/%s)", p.label(), urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// In order to perform the update, we first need to calculate the Terraform view of the diff.
	olds, err := plugin.UnmarshalProperties(req.GetOlds(),
		plugin.MarshalOptions{Label: fmt.Sprintf("%s.olds", label), SkipNulls: true})
	if err != nil {
		return nil, err
	}
	state, err := MakeTerraformState(res, req.GetId(), olds)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshaling %s's instance state", urn)
	}

	news, err := plugin.UnmarshalProperties(req.GetNews(),
		plugin.MarshalOptions{Label: fmt.Sprintf("%s.news", label), KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	config, assets, err := MakeTerraformConfig(p, news, res.TF.Schema(), res.Schema.Fields)
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	}

	diff, err := p.tf.Diff(res.TFName, state, config)
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
	contract.Assertf(!diff.Destroy() && !diff.RequiresNew(),
		"Expected diff to not require deletion or replacement during Update of %s", urn)

	doIgnoreChanges(res.TF.Schema(), res.Schema.Fields, olds, news, req.GetIgnoreChanges(), diff)

	if req.Timeout != 0 {
		diff.SetTimeout(req.Timeout, shim.TimeoutUpdate)
	}

	newstate, err := p.tf.Apply(res.TFName, state, diff)
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
	reasons := make([]string, 0)
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "updating %s", urn).Error())
	}

	props, err := MakeTerraformResult(p.tf, newstate, res.TF.Schema(), res.Schema.Fields, assets, p.supportsSecrets)
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "converting result for %s", urn).Error())
	}
	mprops, err := plugin.MarshalProperties(props, plugin.MarshalOptions{
		Label: fmt.Sprintf("%s.outs", label)})
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
	p.setLoggingContext(ctx)
	urn := resource.URN(req.GetUrn())
	t := urn.Type()
	res, has := p.resources[t]
	if !has {
		return nil, errors.Errorf("unrecognized resource type (Delete): %s", t)
	}

	label := fmt.Sprintf("%s.Delete(%s/%s)", p.label(), urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// Fetch the resource attributes since many providers need more than just the ID to perform the delete.
	state, err := UnmarshalTerraformState(res, req.GetId(), req.GetProperties(), label)
	if err != nil {
		return nil, err
	}

	// Create a new destroy diff.
	diff := p.tf.NewDestroyDiff()
	if req.Timeout != 0 {
		diff.SetTimeout(req.Timeout, shim.TimeoutDelete)
	}

	if _, err := p.tf.Apply(res.TFName, state, diff); err != nil {
		return nil, errors.Wrapf(err, "deleting %s", urn)
	}
	return &pbempty.Empty{}, nil
}

// Construct creates a new instance of the provided component resource and returns its state.
func (p *Provider) Construct(context.Context, *pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Construct is not yet implemented")
}

// Invoke dynamically executes a built-in function in the provider.
func (p *Provider) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	p.setLoggingContext(ctx)
	tok := tokens.ModuleMember(req.GetTok())
	ds, has := p.dataSources[tok]
	if !has {
		return nil, errors.Errorf("unrecognized data function (Invoke): %s", tok)
	}

	label := fmt.Sprintf("%s.Invoke(%s)", p.label(), tok)
	glog.V(9).Infof("%s executing", label)

	// Unmarshal the arguments.
	args, err := plugin.UnmarshalProperties(req.GetArgs(), plugin.MarshalOptions{
		Label: fmt.Sprintf("%s.args", label), KeepUnknowns: true, SkipNulls: true})
	if err != nil {
		return nil, err
	}

	// First, create the inputs.
	tfname := ds.TFName
	inputs, _, err := MakeTerraformInputs(
		&PulumiResource{Properties: args}, p.configValues, nil, args, ds.TF.Schema(), ds.Schema.Fields)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't prepare resource %v input state", tfname)
	}

	// Next, ensure the inputs are valid before actually performing the invoaction.
	rescfg := MakeTerraformConfigFromInputs(p.tf, inputs)
	warns, errs := p.tf.ValidateDataSource(tfname, rescfg)
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
		diff, err := p.tf.ReadDataDiff(tfname, rescfg)
		if err != nil {
			return nil, errors.Wrapf(err, "reading data source diff for %s", tok)
		}

		invoke, err := p.tf.ReadDataApply(tfname, diff)
		if err != nil {
			return nil, errors.Wrapf(err, "invoking %s", tok)
		}

		// Add the special "id" attribute if it wasn't listed in the schema
		props, err := MakeTerraformResult(p.tf, invoke, ds.TF.Schema(), ds.Schema.Fields, nil, p.supportsSecrets)
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
	req *pulumirpc.InvokeRequest, server pulumirpc.ResourceProvider_StreamInvokeServer) error {

	tok := tokens.ModuleMember(req.GetTok())
	return errors.Errorf("unrecognized data function (StreamInvoke): %s", tok)
}

// GetPluginInfo implements an RPC call that returns the version of this plugin.
func (p *Provider) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: p.version,
	}, nil
}

// Cancel requests that the provider cancel all ongoing RPCs. For TF, this is a no-op.
func (p *Provider) Cancel(ctx context.Context, req *pbempty.Empty) (*pbempty.Empty, error) {
	return &pbempty.Empty{}, nil
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
	legacyModule string, newModule string, info *ResourceInfo) {

	resourcePrefix := p.Name + "_"
	legacyResourceName := resourceName + "_legacy"
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

func (p *ProviderInfo) RenameDataSource(resourceName string, legacyTok tokens.ModuleMember, newTok tokens.ModuleMember,
	legacyModule string, newModule string, info *DataSourceInfo) {

	resourcePrefix := p.Name + "_"
	legacyResourceName := resourceName + "_legacy"
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

// SetAutonaming will loop all resources with a name property, and will add an auto-name property.  It will skip
// those that already have a name mapping entry, since those may have custom overrides set in the resource
// declaration (e.g., for length).
func (p *ProviderInfo) SetAutonaming(maxLength int, separator string) {
	const nameProperty = "name"
	for resname, res := range p.Resources {
		if schema := p.P.ResourcesMap().Get(resname); schema != nil {
			// Only apply auto-name to input properties (Optional || Required) named `name`
			if sch := schema.Schema().Get(nameProperty); sch != nil && (sch.Optional() || sch.Required()) {
				if _, hasfield := res.Fields[nameProperty]; !hasfield {
					if res.Fields == nil {
						res.Fields = make(map[string]*SchemaInfo)
					}
					res.Fields[nameProperty] = AutoName(nameProperty, maxLength, separator)
				}
			}
		}
	}
}
