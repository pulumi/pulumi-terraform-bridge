package tfplugin5

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-cty/cty/msgpack"

	shim "github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfshim/tfplugin5/proto"
)

type provider struct {
	client           proto.ProviderClient
	terraformVersion string

	resources   resourceMap
	dataSources resourceMap
	config      *resource
}

func NewProvider(ctx context.Context, client proto.ProviderClient, terraformVersion string) (shim.Provider, error) {
	schemaResponse, err := client.GetSchema(ctx, &proto.GetProviderSchema_Request{})
	if err != nil {
		return nil, fmt.Errorf("error retrieving schema: %w", err)
	}

	// Default to reporting 0.13.2.
	if terraformVersion == "" {
		terraformVersion = "0.13.2"
	}

	p := &provider{
		client:           client,
		terraformVersion: terraformVersion,
	}

	p.resources, err = unmarshalResourceMap(p, schemaResponse.ResourceSchemas)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling resources: %w", err)
	}

	p.dataSources, err = unmarshalResourceMap(p, schemaResponse.DataSourceSchemas)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling data sources: %w", err)
	}

	p.config, err = unmarshalResource(p, "", schemaResponse.Provider)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling provider config: %w", err)
	}

	return p, nil
}

func (p *provider) decodeState(resource *resource, s *instanceState,
	val cty.Value, meta map[string]interface{}) (shim.InstanceState, error) {

	if !val.Type().IsObjectType() || !val.IsKnown() {
		return nil, fmt.Errorf("internal error: state is not an object or is unknown")
	}

	if val.IsNull() && s == nil {
		return nil, nil
	}

	if s == nil {
		s = &instanceState{resourceType: resource.resourceType}
	}

	if val.IsNull() {
		s.id = ""
		s.object = nil
		return s, nil
	}

	valueMap := val.AsValueMap()
	if idVal := valueMap["id"]; idVal.Type() == cty.String && !idVal.IsNull() && idVal.IsKnown() {
		s.id = idVal.AsString()
	}

	object, err := ctyToGo(val)
	if err != nil {
		return nil, err
	}
	s.object, s.meta = object.(map[string]interface{}), meta
	return s, nil
}

func (p *provider) upgradeResourceState(resource *resource, s *instanceState) (*instanceState, error) {
	if s == nil {
		return nil, nil
	}

	schemaVersion := int64(0)
	if schemaVersionValue, ok := s.meta["schema_version"]; ok {
		if schemaVersionString, ok := schemaVersionValue.(string); ok {
			sv, err := strconv.ParseInt(schemaVersionString, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("could not parse schema version: %v", err)
			}
			schemaVersion = sv
		}
	}

	stateBytes, err := json.Marshal(s.object)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.UpgradeResourceState(context.TODO(), &proto.UpgradeResourceState_Request{
		TypeName: resource.resourceType,
		Version:  schemaVersion,
		RawState: &proto.RawState{Json: stateBytes},
	})
	if err != nil {
		return nil, err
	}
	if err = unmarshalErrors(resp.Diagnostics); err != nil {
		return nil, err
	}

	upgradedVal, err := msgpack.Unmarshal(resp.UpgradedState.Msgpack, resource.ctyType)
	if err != nil {
		return nil, err
	}

	upgradedShim, err := p.decodeState(resource, s, upgradedVal, s.meta)
	upgradedState, _ := upgradedShim.(*instanceState)
	return upgradedState, err
}

func (p *provider) importResourceState(t, id string, _ interface{}) ([]shim.InstanceState, error) {
	resp, err := p.client.ImportResourceState(context.TODO(), &proto.ImportResourceState_Request{
		TypeName: t,
		Id:       id,
	})
	if err != nil {
		return nil, err
	}

	states := make([]shim.InstanceState, len(resp.ImportedResources))
	for i, importedResource := range resp.ImportedResources {
		resource, ok := p.resources[importedResource.TypeName]
		if !ok {
			return nil, fmt.Errorf("unknown resource type %v", importedResource.TypeName)
		}

		stateVal, err := msgpack.Unmarshal(importedResource.State.Msgpack, resource.ctyType)
		if err != nil {
			return nil, err
		}

		var metaVal map[string]interface{}
		if err = json.Unmarshal(importedResource.Private, &metaVal); err != nil {
			return nil, err
		}

		states[i], err = p.decodeState(resource, nil, stateVal, metaVal)
		if err != nil {
			return nil, err
		}
	}
	return states, nil
}

func (p *provider) Schema() shim.SchemaMap {
	return p.config.schema
}

func (p *provider) ResourcesMap() shim.ResourceMap {
	return p.resources
}

func (p *provider) DataSourcesMap() shim.ResourceMap {
	return p.dataSources
}

func (p *provider) Validate(c shim.ResourceConfig) ([]string, []error) {
	config, ok := c.(resourceConfig)
	if !ok {
		return nil, []error{fmt.Errorf("internal error: foreign resource config")}
	}

	val, err := config.marshal(p.config.ctyType)
	if err != nil {
		return nil, []error{err}
	}

	resp, err := p.client.PrepareProviderConfig(context.TODO(), &proto.PrepareProviderConfig_Request{
		Config: &proto.DynamicValue{Msgpack: val},
	})
	if err != nil {
		return nil, []error{err}
	}

	return unmarshalWarningsAndErrors(resp.Diagnostics)
}

func (p *provider) ValidateResource(t string, c shim.ResourceConfig) ([]string, []error) {
	config, ok := c.(resourceConfig)
	if !ok {
		return nil, []error{fmt.Errorf("internal error: foreign resource config")}
	}

	resource, ok := p.resources[t]
	if !ok {
		return nil, []error{fmt.Errorf("unknown resource type %v", t)}
	}

	val, err := config.marshal(resource.ctyType)
	if err != nil {
		return nil, []error{err}
	}

	resp, err := p.client.ValidateResourceTypeConfig(context.TODO(), &proto.ValidateResourceTypeConfig_Request{
		TypeName: t,
		Config:   &proto.DynamicValue{Msgpack: val},
	})
	if err != nil {
		return nil, []error{err}
	}

	return unmarshalWarningsAndErrors(resp.Diagnostics)
}

func (p *provider) ValidateDataSource(t string, c shim.ResourceConfig) ([]string, []error) {
	config, ok := c.(resourceConfig)
	if !ok {
		return nil, []error{fmt.Errorf("internal error: foreign resource config")}
	}

	dataSource, ok := p.dataSources[t]
	if !ok {
		return nil, []error{fmt.Errorf("unknown data source %v", t)}
	}

	val, err := config.marshal(dataSource.ctyType)
	if err != nil {
		return nil, []error{err}
	}

	resp, err := p.client.ValidateDataSourceConfig(context.TODO(), &proto.ValidateDataSourceConfig_Request{
		TypeName: t,
		Config:   &proto.DynamicValue{Msgpack: val},
	})
	if err != nil {
		return nil, []error{err}
	}

	return unmarshalWarningsAndErrors(resp.Diagnostics)
}

func (p *provider) Configure(c shim.ResourceConfig) error {
	config, ok := c.(resourceConfig)
	if !ok {
		return fmt.Errorf("internal error: foreign resource config")
	}

	val, err := config.marshal(p.config.ctyType)
	if err != nil {
		return err
	}

	resp, err := p.client.Configure(context.TODO(), &proto.Configure_Request{
		TerraformVersion: p.terraformVersion,
		Config:           &proto.DynamicValue{Msgpack: val},
	})
	if err != nil {
		return err
	}

	return unmarshalErrors(resp.Diagnostics)
}

func (p *provider) Diff(t string, s shim.InstanceState, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	state, ok := s.(*instanceState)
	if s != nil && !ok {
		return nil, fmt.Errorf("internal error: foreign resource state")
	}
	config, ok := c.(resourceConfig)
	if !ok {
		return nil, fmt.Errorf("internal error: foreign resource config")
	}

	resource, ok := p.resources[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource type %v", t)
	}

	state, err := p.upgradeResourceState(resource, state)
	if err != nil {
		return nil, err
	}

	stateVal, err := goToCty(state.getObject(), resource.ctyType)
	if err != nil {
		return nil, err
	}
	configVal, err := goToCty(config, resource.ctyType)
	if err != nil {
		return nil, err
	}

	stateBytes, err := msgpack.Marshal(stateVal, resource.ctyType)
	if err != nil {
		return nil, err
	}
	var metaBytes []byte
	if state != nil {
		m, err := json.Marshal(state.meta)
		if err != nil {
			return nil, err
		}
		metaBytes = m
	}
	configBytes, err := msgpack.Marshal(configVal, resource.ctyType)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.PlanResourceChange(context.TODO(), &proto.PlanResourceChange_Request{
		TypeName:         resource.resourceType,
		PriorState:       &proto.DynamicValue{Msgpack: stateBytes},
		ProposedNewState: &proto.DynamicValue{Msgpack: configBytes},
		PriorPrivate:     metaBytes,
	})
	if err != nil {
		return nil, err
	}

	plannedVal, err := msgpack.Unmarshal(resp.PlannedState.Msgpack, resource.ctyType)
	if err != nil {
		return nil, err
	}

	var plannedMeta map[string]interface{}
	if err = json.Unmarshal(resp.PlannedPrivate, &plannedMeta); err != nil {
		return nil, err
	}

	return newInstanceDiff(stateVal, plannedVal, plannedMeta, resp.RequiresReplace), nil
}

func (p *provider) Apply(t string, s shim.InstanceState, d shim.InstanceDiff) (shim.InstanceState, error) {
	state, ok := s.(*instanceState)
	if s != nil && !ok {
		return nil, fmt.Errorf("internal error: foreign resource state")
	}
	diff, ok := d.(*instanceDiff)
	if !ok {
		return nil, fmt.Errorf("internal error: foreign instance diff")
	}

	resource, ok := p.resources[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource type %v", t)
	}

	state, err := p.upgradeResourceState(resource, state)
	if err != nil {
		return nil, err
	}

	stateBytes, err := state.marshal(resource.ctyType)
	if err != nil {
		return nil, err
	}
	if diff.planned == (cty.Value{}) {
		diff.planned = cty.NullVal(resource.ctyType)
	}
	plannedStateBytes, err := msgpack.Marshal(diff.planned, resource.ctyType)
	if err != nil {
		return nil, err
	}
	plannedMetaBytes, err := json.Marshal(diff.meta)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.ApplyResourceChange(context.TODO(), &proto.ApplyResourceChange_Request{
		TypeName:       resource.resourceType,
		PriorState:     &proto.DynamicValue{Msgpack: stateBytes},
		PlannedState:   &proto.DynamicValue{Msgpack: plannedStateBytes},
		PlannedPrivate: plannedMetaBytes,
	})
	if err != nil {
		return nil, err
	}

	newStateVal, err := msgpack.Unmarshal(resp.NewState.Msgpack, resource.ctyType)
	if err != nil {
		return nil, err
	}

	var newMetaVal map[string]interface{}
	if len(resp.Private) != 0 {
		if err = json.Unmarshal(resp.Private, &newMetaVal); err != nil {
			return nil, err
		}
	}

	newState, err := p.decodeState(resource, state, newStateVal, newMetaVal)
	if err != nil {
		return nil, err
	}

	return newState, unmarshalErrors(resp.Diagnostics)
}

func (p *provider) Refresh(t string, s shim.InstanceState) (shim.InstanceState, error) {
	state, ok := s.(*instanceState)
	if s != nil && !ok {
		return nil, fmt.Errorf("internal error: foreign resource state")
	}

	resource, ok := p.resources[t]
	if !ok {
		return nil, fmt.Errorf("unknown resource type %v", t)
	}

	state, err := p.upgradeResourceState(resource, state)
	if err != nil {
		return nil, err
	}

	stateBytes, err := state.marshal(resource.ctyType)
	if err != nil {
		return nil, err
	}
	metaBytes, err := json.Marshal(state.meta)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.ReadResource(context.TODO(), &proto.ReadResource_Request{
		TypeName:     resource.resourceType,
		CurrentState: &proto.DynamicValue{Msgpack: stateBytes},
		Private:      metaBytes,
	})
	if err != nil {
		return nil, err
	}

	newStateVal, err := msgpack.Unmarshal(resp.NewState.Msgpack, resource.ctyType)
	if err != nil {
		return nil, err
	}

	var newMetaVal map[string]interface{}
	if len(resp.Private) != 0 {
		if err = json.Unmarshal(resp.Private, &newMetaVal); err != nil {
			return nil, err
		}
	}

	newState, err := p.decodeState(resource, state, newStateVal, newMetaVal)
	if err != nil {
		return nil, err
	}

	return newState, unmarshalErrors(resp.Diagnostics)
}

func (p *provider) ReadDataDiff(t string, c shim.ResourceConfig) (shim.InstanceDiff, error) {
	dataSource, ok := p.dataSources[t]
	if !ok {
		return nil, fmt.Errorf("unknown data source %v", t)
	}

	planned, err := goToCty(c, dataSource.ctyType)
	if err != nil {
		return nil, err
	}

	return &instanceDiff{planned: planned}, nil
}

func (p *provider) ReadDataApply(t string, d shim.InstanceDiff) (shim.InstanceState, error) {
	diff, ok := d.(*instanceDiff)
	if d != nil && !ok {
		return nil, fmt.Errorf("internal error: foreign instance diff")
	}

	dataSource, ok := p.dataSources[t]
	if !ok {
		return nil, fmt.Errorf("unknown data source %v", t)
	}

	configBytes, err := msgpack.Marshal(diff.planned, dataSource.ctyType)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.ReadDataSource(context.TODO(), &proto.ReadDataSource_Request{
		TypeName: t,
		Config:   &proto.DynamicValue{Msgpack: configBytes},
	})
	if err != nil {
		return nil, err
	}

	stateVal, err := msgpack.Unmarshal(resp.State.Msgpack, dataSource.ctyType)
	if err != nil {
		return nil, err
	}

	return p.decodeState(dataSource, nil, stateVal, nil)
}

func (p *provider) Meta() interface{} {
	return nil
}

func (p *provider) Stop() error {
	resp, err := p.client.Stop(context.TODO(), &proto.Stop_Request{})
	switch {
	case err != nil:
		return err
	case resp.Error != "":
		return fmt.Errorf("%s", err)
	default:
		return nil
	}
}

func (p *provider) InitLogging() {
	// Nothing to do.
}

func (p *provider) NewDestroyDiff() shim.InstanceDiff {
	return &instanceDiff{destroy: true}
}

func (p *provider) NewResourceConfig(object map[string]interface{}) shim.ResourceConfig {
	return resourceConfig(object)
}

func (p *provider) IsSet(v interface{}) ([]interface{}, bool) {
	val, ok := v.(cty.Value)
	if !ok {
		return nil, false
	}
	if !val.Type().IsSetType() {
		return nil, false
	}

	result := make([]interface{}, 0, val.LengthInt())
	iter := val.ElementIterator()
	for iter.Next() {
		v, _ := iter.Element()
		gv, err := ctyToGo(v)
		if err != nil {
			// NOTE: this might be worthy of a panic.
			return nil, false
		}
		result = append(result, gv)
	}
	return result, true
}
