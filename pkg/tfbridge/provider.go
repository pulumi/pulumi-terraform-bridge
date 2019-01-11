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
	"strings"

	"github.com/golang/glog"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/resource/provider"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/rpcutil/rpcerror"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// Provider implements the Pulumi resource provider operations for any Terraform plugin.
type Provider struct {
	host        *provider.HostClient               // the RPC link back to the Pulumi engine.
	module      string                             // the Terraform module name.
	version     string                             // the plugin version number.
	tf          *schema.Provider                   // the Terraform resource provider to use.
	info        ProviderInfo                       // overlaid info about this provider.
	config      map[string]*schema.Schema          // the Terraform config schema.
	resources   map[tokens.Type]Resource           // a map of Pulumi type tokens to resource info.
	dataSources map[tokens.ModuleMember]DataSource // a map of Pulumi module tokens to data sources.
}

// Resource wraps both the Terraform resource type info plus the overlay resource info.
type Resource struct {
	Schema *ResourceInfo    // optional provider overrides.
	TF     *schema.Resource // the Terraform resource schema.
	TFName string           // the Terraform resource name.
}

// runTerraformImporter runs the Terraform Importer defined on the Resource for the given
// resource ID, and returns a replacement input map if any resources are matched. A nil map
// with no error should be interpreted by the caller as meaning the resource does not exist,
// but there were no errors in determining this.
func (res *Resource) runTerraformImporter(resourceID resource.ID, provider *Provider) (map[string]string, error) {
	// There is nothing to do here if the resource doesn't have an importer defined in the
	// Terraform schema.
	if res.TF.Importer == nil {
		return nil, nil
	}

	glog.V(9).Infof("%s has TF Importer", res.TFName)

	id := resourceID.String()

	// Prepare a Terraform ResourceData for the importer
	data := res.TF.Data(nil)
	data.SetId(id)
	data.SetType(res.TFName)

	// Run the importer defined in the Terraform resource schema
	results, err := res.TF.Importer.State(data, provider.tf.Meta())
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
	if len(results) < 1 {
		return nil, nil
	}

	// Allow constructing an error in the case that we have a nil InstanceState returned from
	// Terraform, which is always a programming error.
	makeNilStateError := func(badResourceID string) error {
		return errors.Errorf("importer for %s returned a empty resource state. This is always "+
			"the result of a bug in the resource provider - please report this "+
			"as a bug in the Pulumi provider repository.", badResourceID)
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
	var primaryInstanceState *terraform.InstanceState

	if len(results) == 1 {
		// Take the only result, assuming the Type matches
		state := results[0].State()
		if state == nil {
			return nil, makeNilStateError(id)
		}
		if state.Ephemeral.Type == res.TFName {
			primaryInstanceState = state
		}
	} else {
		// Search for a Type+ID match, and use the first (if any)
		for _, result := range results {
			if result.Id() != id {
				continue
			}

			state := result.State()
			if state == nil {
				return nil, makeNilStateError(id)
			}

			if state.Ephemeral.Type != res.TFName {
				continue
			}

			primaryInstanceState = state
			break
		}
	}

	// No resources were matched - error out
	if primaryInstanceState == nil {
		return nil, errors.Errorf("importer for %s returned no matching resources", id)
	}
	return primaryInstanceState.Attributes, nil
}

// DataSource wraps both the Terraform data source (resource) type info plus the overlay resource info.
type DataSource struct {
	Schema *DataSourceInfo  // optional provider overrides.
	TF     *schema.Resource // the Terraform data source schema.
	TFName string           // the Terraform resource name.
}

// NewProvider creates a new Pulumi RPC server wired up to the given host and wrapping the given Terraform provider.
func NewProvider(ctx context.Context, host *provider.HostClient, module string, version string,
	tf *schema.Provider, info ProviderInfo) *Provider {
	p := &Provider{
		host:    host,
		module:  module,
		version: version,
		tf:      tf,
		info:    info,
		config:  tf.Schema,
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

func (p *Provider) label() string {
	return fmt.Sprintf("tf.Provider[%s]", p.module)
}

// initResourceMaps creates maps from Pulumi types and tokens to Terraform resource type.
func (p *Provider) initResourceMaps() {
	// Fetch a list of all resource types handled by this provider and make a map.
	p.resources = make(map[tokens.Type]Resource)
	for _, res := range p.tf.Resources() {
		var tok tokens.Type

		// See if there is override information for this resource.  If yes, use that to decode the token.
		var schema *ResourceInfo
		if p.info.Resources != nil {
			schema = p.info.Resources[res.Name]
			if schema != nil {
				tok = schema.Tok
			}
		}

		// Otherwise, we default to the standard naming scheme.
		if tok == "" {
			// Manufacture a token with the package, module, and resource type name.
			camelName, pascalName := p.camelPascalPulumiName(res.Name)
			tok = tokens.Type(string(p.pkg()) + ":" + camelName + ":" + pascalName)
		}

		p.resources[tok] = Resource{
			TF:     p.tf.ResourcesMap[res.Name],
			TFName: res.Name,
			Schema: schema,
		}
	}

	// Fetch a list of all data source types handled by this provider and make a similar map.
	p.dataSources = make(map[tokens.ModuleMember]DataSource)
	for _, ds := range p.tf.DataSources() {
		var tok tokens.ModuleMember

		// See if there is override information for this resource.  If yes, use that to decode the token.
		var schema *DataSourceInfo
		if p.info.DataSources != nil {
			schema = p.info.DataSources[ds.Name]
			if schema != nil {
				tok = schema.Tok
			}
		}

		// Otherwise, we default to the standard naming scheme.
		if tok == "" {
			// Manufacture a token with the data module and camel-cased name.
			camelName, _ := p.camelPascalPulumiName(ds.Name)
			tok = tokens.ModuleMember(string(p.baseDataMod()) + ":" + camelName)
		}

		p.dataSources[tok] = DataSource{
			TF:     p.tf.DataSourcesMap[ds.Name],
			TFName: ds.Name,
			Schema: schema,
		}
	}
}

// camelPascalPulumiName returns the camel and pascal cased name for a given terraform name.
func (p *Provider) camelPascalPulumiName(name string) (string, string) {
	// Strip off the module prefix (e.g., "aws_") and then return the camel- and Pascal-cased names.
	prefix := p.info.Name + "_" // all resources will have this prefix.
	contract.Assertf(strings.HasPrefix(name, prefix),
		"Expected all Terraform resources in this module to have a '%v' prefix", prefix)
	name = name[len(prefix):]
	return TerraformToPulumiName(name, nil, false), TerraformToPulumiName(name, nil, true)
}

func convertStringToPropertyValue(s string, typ schema.ValueType) (resource.PropertyValue, error) {
	// If the schema expects a string, we can just return this as-is.
	if typ == schema.TypeString {
		return resource.NewStringProperty(s), nil
	}

	// Otherwise, we will attempt to deserialize the input string as JSON and convert the result into a Pulumi
	// property. If the input string is empty, we will return an appropriate zero value.
	if s == "" {
		switch typ {
		case schema.TypeBool:
			return resource.NewPropertyValue(false), nil
		case schema.TypeInt, schema.TypeFloat:
			return resource.NewPropertyValue(0), nil
		case schema.TypeList, schema.TypeSet:
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

// Configure configures the underlying Terraform provider with the live Pulumi variable state.
func (p *Provider) Configure(ctx context.Context, req *pulumirpc.ConfigureRequest) (*pbempty.Empty, error) {
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

		typ := schema.TypeString
		_, sch, _ := getInfoFromPulumiName(resource.PropertyKey(mm.Name()), p.config, p.info.Config, false)
		if sch != nil {
			typ = sch.Type
		}
		pv, err := convertStringToPropertyValue(v, typ)
		if err != nil {
			return nil, errors.Wrapf(err, "malformed configuration value '%v'", v)
		}
		vars[resource.PropertyKey(mm.Name())] = pv
	}

	// First make a Terraform config map out of the variables. We do this before checking for missing properties
	// s.t. we can pull any defaults out of the TF schema.
	config, err := MakeTerraformConfig(nil, vars, p.config, p.info.Config, true)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal config state")
	}

	if p.info.PreConfigureCallback != nil {
		if err = p.info.PreConfigureCallback(vars, config); err != nil {
			return nil, err
		}
	}

	// So we can provide better error messages, do a quick scan of required configs for this
	// schema and report any that haven't been supplied.
	var missingKeys []*pulumirpc.ConfigureErrorMissingKeys_MissingKey
	for key, meta := range p.config {
		if meta.Required && !config.IsSet(key) {
			name := TerraformToPulumiName(key, meta, false)
			fullyQualifiedName := tokens.NewModuleToken(p.pkg(), tokens.ModuleName(name))

			// TF descriptions often have newlines in inopportune positions. This makes them present
			// a little better in our console output.
			descriptionWithoutNewlines := strings.Replace(meta.Description, "\n", " ", -1)
			missingKeys = append(missingKeys, &pulumirpc.ConfigureErrorMissingKeys_MissingKey{
				Name:        fullyQualifiedName.String(),
				Description: descriptionWithoutNewlines,
			})
		}
	}

	if len(missingKeys) > 0 {
		// Clients of our RPC endpoint will be looking for this detail in order to figure out
		// which keys need descriptive error messages.
		err = rpcerror.WithDetails(
			rpcerror.New(codes.InvalidArgument, "required configuration keys were missing"),
			&pulumirpc.ConfigureErrorMissingKeys{MissingKeys: missingKeys})
		return nil, err
	}

	// Perform validation of the config state so we can offer nice errors.
	warns, errs := p.tf.Validate(config)
	for _, warn := range warns {
		if err = p.host.Log(ctx, diag.Warning, "", fmt.Sprintf("provider config warning: %v", warn)); err != nil {
			return nil, err
		}
	}

	if len(errs) > 0 {
		return nil, errors.Wrap(multierror.Append(nil, errs...), "could not validate provider configuration")
	}

	// Now actually attempt to do the configuring and return its resulting error (if any).
	if err = p.tf.Configure(config); err != nil {
		return nil, err
	}
	return &pbempty.Empty{}, nil
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
			Label: fmt.Sprintf("%s.olds", label), KeepUnknowns: true, SkipNulls: true})
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
	assets := make(AssetTable)
	inputs, err := MakeTerraformInputs(
		&PulumiResource{URN: urn, Properties: news},
		olds, news, res.TF.Schema, res.Schema.Fields, assets, true, false)
	if err != nil {
		return nil, err
	}

	// Now check with the resource provider to see if the values pass muster.
	rescfg, err := MakeTerraformConfigFromInputs(inputs)
	if err != nil {
		return nil, err
	}
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
			Reason: err.Error(),
		})
	}

	// After all is said and done, we need to go back and return only what got populated as a diff from the origin.
	pinputs := MakeTerraformOutputs(inputs, res.TF.Schema, res.Schema.Fields, assets, false)
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
	inputs, meta, err := MakeTerraformAttributesFromRPC(
		res.TF, req.GetOlds(), res.TF.Schema, res.Schema.Fields, false, false, fmt.Sprintf("%s.olds", label))
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's old property state", urn)
	}
	info := &terraform.InstanceInfo{Type: res.TFName}
	state := &terraform.InstanceState{ID: req.GetId(), Attributes: inputs, Meta: meta}
	config, err := MakeTerraformConfigFromRPC(
		nil, req.GetNews(), res.TF.Schema, res.Schema.Fields, true, false, fmt.Sprintf("%s.news", label))
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	}
	diff, err := p.tf.Diff(info, state, config)
	if err != nil {
		return nil, errors.Wrapf(err, "diffing %s", urn)
	}

	// If there were changes in this diff, check to see if we have a replacement.
	var replaces []string
	var replaced map[resource.PropertyKey]bool
	var changes pulumirpc.DiffResponse_DiffChanges
	hasChanges := diff != nil && len(diff.Attributes) > 0
	if hasChanges {
		changes = pulumirpc.DiffResponse_DIFF_SOME
		for k, attr := range diff.Attributes {
			if attr.RequiresNew {
				name, _, _ := getInfoFromTerraformName(k, res.TF.Schema, res.Schema.Fields, false)
				replaces = append(replaces, string(name))
				if replaced == nil {
					replaced = make(map[resource.PropertyKey]bool)
				}
				replaced[name] = true
			}
		}
	} else {
		changes = pulumirpc.DiffResponse_DIFF_NONE
	}

	// For all properties that are ForceNew, but didn't change, assume they are stable.  Also recognize
	// overlays that have requested that we treat specific properties as stable.
	var stables []string
	for k, sch := range res.TF.Schema {
		name, _, cust := getInfoFromTerraformName(k, res.TF.Schema, res.Schema.Fields, false)
		if !replaced[name] &&
			(sch.ForceNew || (cust != nil && cust.Stable != nil && *cust.Stable)) {
			stables = append(stables, string(name))
		}
	}

	return &pulumirpc.DiffResponse{
		Changes:             changes,
		Replaces:            replaces,
		Stables:             stables,
		DeleteBeforeReplace: len(replaces) > 0 && res.Schema.DeleteBeforeReplace,
	}, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
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
	info := &terraform.InstanceInfo{Type: res.TFName}
	state := &terraform.InstanceState{}
	config, err := MakeTerraformConfigFromRPC(
		nil, req.GetProperties(), res.TF.Schema, res.Schema.Fields, true, false, fmt.Sprintf("%s.news", label))
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	}
	diff, err := p.tf.Diff(info, state, config)
	if err != nil {
		return nil, errors.Wrapf(err, "diffing %s", urn)
	}

	newstate, err := p.tf.Apply(info, state, diff)
	if newstate == nil {
		contract.Assertf(err != nil, "expected non-nil error with nil state during Create")
		return nil, err
	}

	contract.Assertf(newstate.ID != "", "Expected non-empty ID for new state during Create")
	reasons := make([]string, 0)
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "creating %s", urn).Error())
	}

	// Create the ID and property maps and return them.
	props := MakeTerraformResult(newstate, res.TF.Schema, res.Schema.Fields)
	mprops, err := plugin.MarshalProperties(props, plugin.MarshalOptions{Label: fmt.Sprintf("%s.outs", label)})
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "marshalling %s", urn).Error())
	}

	if len(reasons) != 0 {
		return nil, initializationError(newstate.ID, mprops, reasons)
	}
	return &pulumirpc.CreateResponse{Id: newstate.ID, Properties: mprops}, nil
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

	id := resource.ID(req.GetId())
	label := fmt.Sprintf("%s.Read(%s, %s/%s)", p.label(), id, urn, res.TFName)
	glog.V(9).Infof("%s executing", label)

	// Manufacture Terraform attributes and state with the provided properties, in preparation for reading.
	inputs, meta, err := MakeTerraformAttributesFromRPC(
		res.TF, req.GetProperties(), res.TF.Schema, res.Schema.Fields, false, false, fmt.Sprintf("%s.state", label))
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's property state", urn)
	}

	// If we are in a "get" rather than a "refresh", we should call the Terraform importer, if one is defined.
	if len(req.GetProperties().GetFields()) == 0 {
		inputs, err = res.runTerraformImporter(id, p)
		if err != nil {
			// Pass through any error running the importer
			return nil, err
		}
		if inputs == nil {
			// The resource is gone (or never existed). Return a gRPC response with no
			// resource ID set to indicate this.
			return &pulumirpc.ReadResponse{}, nil
		}
	}

	info := &terraform.InstanceInfo{Type: res.TFName}
	state := &terraform.InstanceState{ID: req.GetId(), Attributes: inputs, Meta: meta}
	newstate, err := p.tf.Refresh(info, state)
	if err != nil {
		return nil, errors.Wrapf(err, "refreshing %s", urn)
	}

	// Store the ID and properties in the output.  The ID *should* be the same as the input ID, but in the case
	// that the resource no longer exists, we will simply return the empty string and an empty property map.
	if newstate != nil {
		props := MakeTerraformResult(newstate, res.TF.Schema, res.Schema.Fields)
		mprops, err := plugin.MarshalProperties(props, plugin.MarshalOptions{
			Label: fmt.Sprintf("%s.newstate", label)})
		if err != nil {
			return nil, err
		}
		return &pulumirpc.ReadResponse{Id: newstate.ID, Properties: mprops}, nil
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
	inputs, meta, err := MakeTerraformAttributesFromRPC(
		res.TF, req.GetOlds(), res.TF.Schema, res.Schema.Fields, false, false, fmt.Sprintf("%s.olds", label))
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's old property state", urn)
	}
	info := &terraform.InstanceInfo{Type: res.TFName}
	state := &terraform.InstanceState{ID: req.GetId(), Attributes: inputs, Meta: meta}
	config, err := MakeTerraformConfigFromRPC(
		nil, req.GetNews(), res.TF.Schema, res.Schema.Fields, true, false, fmt.Sprintf("%s.news", label))
	if err != nil {
		return nil, errors.Wrapf(err, "preparing %s's new property state", urn)
	}
	diff, err := p.tf.Diff(info, state, config)
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

	newstate, err := p.tf.Apply(info, state, diff)
	if newstate == nil {
		contract.Assertf(err != nil, "expected non-nil error with nil state during Update")
		return nil, err
	}

	contract.Assertf(newstate.ID != "", "Expected non-empty ID for new state during Update")
	reasons := make([]string, 0)
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "updating %s", urn).Error())
	}

	props := MakeTerraformResult(newstate, res.TF.Schema, res.Schema.Fields)
	mprops, err := plugin.MarshalProperties(props, plugin.MarshalOptions{
		Label: fmt.Sprintf("%s.outs", label)})
	if err != nil {
		reasons = append(reasons, errors.Wrapf(err, "marshalling %s", urn).Error())
	}

	if len(reasons) != 0 {
		return nil, initializationError(newstate.ID, mprops, reasons)
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
	attrs, meta, err := MakeTerraformAttributesFromRPC(
		res.TF, req.GetProperties(), res.TF.Schema, res.Schema.Fields, false, false, label)
	if err != nil {
		return nil, err
	}

	// Create a new state, with no diff, that is missing an ID.  Terraform will interpret this as a create operation.
	info := &terraform.InstanceInfo{Type: res.TFName}
	state := &terraform.InstanceState{ID: req.GetId(), Attributes: attrs, Meta: meta}
	if _, err := p.tf.Apply(info, state, &terraform.InstanceDiff{Destroy: true}); err != nil {
		return nil, errors.Wrapf(err, "deleting %s", urn)
	}
	return &pbempty.Empty{}, nil
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
	inputs, err := MakeTerraformInputs(
		&PulumiResource{Properties: args}, nil, args, ds.TF.Schema, ds.Schema.Fields, nil, true, false)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't prepare resource %v input state", tfname)
	}

	// Next, ensure the inputs are valid before actually performing the invoaction.
	info := &terraform.InstanceInfo{Type: tfname}
	rescfg, err := MakeTerraformConfigFromInputs(inputs)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't make config for %v validation", tfname)
	}
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
		diff, err := p.tf.ReadDataDiff(info, rescfg)
		if err != nil {
			return nil, errors.Wrapf(err, "reading data source diff for %s", tok)
		}

		invoke, err := p.tf.ReadDataApply(info, diff)
		if err != nil {
			return nil, errors.Wrapf(err, "invoking %s", tok)
		}

		// Add the special "id" attribute if it wasn't listed in the schema
		props := MakeTerraformResult(invoke, ds.TF.Schema, ds.Schema.Fields)
		if _, has := props["id"]; !has {
			props["id"] = resource.NewStringProperty(invoke.ID)
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
