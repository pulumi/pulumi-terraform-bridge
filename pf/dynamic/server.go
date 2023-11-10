package dynamic

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/opentofu/opentofu/shim"
	"github.com/zclconf/go-cty/cty"
)

func NewDynamicServer(providerInterface shim.Interface) *dynamicPfServer {
	return &dynamicPfServer{
		impl: providerInterface,
	}
}

type dynamicPfServer struct {
	impl   shim.Interface
	schema *shim.GetProviderSchemaResponse
}

func (ps *dynamicPfServer) reloadSchema() {
	schema := ps.impl.GetProviderSchema()
	ps.schema = &schema
}

func (ps *dynamicPfServer) maybeReloadSchema() {
	if ps.schema == nil {
		ps.reloadSchema()
	}
}

func (ps *dynamicPfServer) getProviderType() cty.Type {
	ps.maybeReloadSchema()
	return ps.schema.Provider.Block.ImpliedType()
}

func (ps *dynamicPfServer) getProviderMetaType() cty.Type {
	ps.maybeReloadSchema()
	return ps.schema.ProviderMeta.Block.ImpliedType()
}

func (ps *dynamicPfServer) getResourceType(typeName string) cty.Type {
	ps.maybeReloadSchema()
	return ps.schema.ResourceTypes[typeName].Block.ImpliedType()
}

func (ps *dynamicPfServer) getDataSource(typeName string) cty.Type {
	ps.maybeReloadSchema()
	return ps.schema.DataSources[typeName].Block.ImpliedType()
}

var _ tfprotov6.ProviderServer = &dynamicPfServer{}

func (ps *dynamicPfServer) GetMetadata(context.Context, *tfprotov6.GetMetadataRequest) (*tfprotov6.GetMetadataResponse, error) {
	panic("UNUSED")
}

func (ps *dynamicPfServer) GetProviderSchema(context.Context, *tfprotov6.GetProviderSchemaRequest) (*tfprotov6.GetProviderSchemaResponse, error) {
	ps.reloadSchema()
	pResp := ps.schema
	resp := &tfprotov6.GetProviderSchemaResponse{
		ServerCapabilities: convertServerCapabilities(pResp.ServerCapabilities),
		Provider:           convertSchema(pResp.Provider),
		ProviderMeta:       convertSchema(pResp.ProviderMeta),
		ResourceSchemas:    convertSchemas(pResp.ResourceTypes),
		DataSourceSchemas:  convertSchemas(pResp.DataSources),
		Diagnostics:        convertDiagnostics(pResp.Diagnostics),
	}
	return resp, nil
}

func (ps *dynamicPfServer) ValidateProviderConfig(c context.Context, req *tfprotov6.ValidateProviderConfigRequest) (*tfprotov6.ValidateProviderConfigResponse, error) {
	config, err := unmarshalFromDV(req.Config, ps.getProviderType())
	if err != nil {
		return nil, err
	}
	pReq := shim.ValidateProviderConfigRequest{
		Config: config,
	}
	pResp := ps.impl.ValidateProviderConfig(pReq)
	return &tfprotov6.ValidateProviderConfigResponse{
		Diagnostics: convertDiagnostics(pResp.Diagnostics),
	}, nil
}

func (ps *dynamicPfServer) ConfigureProvider(c context.Context, req *tfprotov6.ConfigureProviderRequest) (*tfprotov6.ConfigureProviderResponse, error) {
	config, err := unmarshalFromDV(req.Config, ps.getProviderType())
	if err != nil {
		return nil, err
	}
	pReq := shim.ConfigureProviderRequest{
		TerraformVersion: req.TerraformVersion,
		Config:           config,
	}
	pResp := ps.impl.ConfigureProvider(pReq)
	return &tfprotov6.ConfigureProviderResponse{
		Diagnostics: convertDiagnostics(pResp.Diagnostics),
	}, nil
}

func (ps *dynamicPfServer) StopProvider(context.Context, *tfprotov6.StopProviderRequest) (*tfprotov6.StopProviderResponse, error) {
	err := ps.impl.Stop()
	return &tfprotov6.StopProviderResponse{Error: fmt.Sprintf("%v", err)}, nil
}

func (ps *dynamicPfServer) ValidateResourceConfig(c context.Context, req *tfprotov6.ValidateResourceConfigRequest) (*tfprotov6.ValidateResourceConfigResponse, error) {
	config, err := unmarshalFromDV(req.Config, ps.getResourceType(req.TypeName))
	if err != nil {
		return nil, err
	}
	pReq := shim.ValidateResourceConfigRequest{
		TypeName: req.TypeName,
		Config:   config,
	}
	pResp := ps.impl.ValidateResourceConfig(pReq)
	return &tfprotov6.ValidateResourceConfigResponse{
		Diagnostics: convertDiagnostics(pResp.Diagnostics),
	}, nil
}

func (ps *dynamicPfServer) UpgradeResourceState(c context.Context, req *tfprotov6.UpgradeResourceStateRequest) (*tfprotov6.UpgradeResourceStateResponse, error) {
	pReq := shim.UpgradeResourceStateRequest{
		TypeName:        req.TypeName,
		Version:         req.Version,
		RawStateJSON:    req.RawState.JSON,
		RawStateFlatmap: req.RawState.Flatmap,
	}
	pResp := ps.impl.UpgradeResourceState(pReq)

	upgradedState, err := marshalToDV(&pResp.UpgradedState)
	if err != nil {
		return nil, err
	}

	return &tfprotov6.UpgradeResourceStateResponse{
		UpgradedState: upgradedState,
		Diagnostics:   convertDiagnostics(pResp.Diagnostics),
	}, nil
}

func (ps *dynamicPfServer) ReadResource(c context.Context, req *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	currentState, err := unmarshalFromDV(req.CurrentState, ps.getResourceType(req.TypeName))
	if err != nil {
		return nil, err
	}

	meta, err := unmarshalFromDV(req.ProviderMeta, ps.getProviderMetaType())
	if err != nil {
		return nil, err
	}

	pReq := shim.ReadResourceRequest{
		TypeName:     req.TypeName,
		Private:      req.Private,
		PriorState:   currentState,
		ProviderMeta: meta,
	}
	pResp := ps.impl.ReadResource(pReq)
	newState, err := marshalToDV(&pResp.NewState)
	if err != nil {
		return nil, err
	}
	return &tfprotov6.ReadResourceResponse{
		NewState:    newState,
		Private:     pResp.Private,
		Diagnostics: convertDiagnostics(pResp.Diagnostics),
	}, nil
}

func (ps *dynamicPfServer) PlanResourceChange(c context.Context, req *tfprotov6.PlanResourceChangeRequest) (*tfprotov6.PlanResourceChangeResponse, error) {
	priorState, err := unmarshalFromDV(req.PriorState, ps.getResourceType(req.TypeName))
	if err != nil {
		return nil, err
	}

	proposedNewState, err := unmarshalFromDV(req.ProposedNewState, ps.getResourceType(req.TypeName))
	if err != nil {
		return nil, err
	}

	config, err := unmarshalFromDV(req.Config, ps.getResourceType(req.TypeName))
	if err != nil {
		return nil, err
	}

	meta, err := unmarshalFromDV(req.ProviderMeta, ps.getProviderMetaType())
	if err != nil {
		return nil, err
	}

	pReq := shim.PlanResourceChangeRequest{
		TypeName:         req.TypeName,
		PriorPrivate:     req.PriorPrivate,
		PriorState:       priorState,
		ProposedNewState: proposedNewState,
		Config:           config,
		ProviderMeta:     meta,
	}
	pResp := ps.impl.PlanResourceChange(pReq)
	plannedState, err := marshalToDV(&pResp.PlannedState)
	if err != nil {
		return nil, err
	}

	return &tfprotov6.PlanResourceChangeResponse{
		RequiresReplace:             pathsToAttributePaths(pResp.RequiresReplace),
		PlannedState:                plannedState,
		PlannedPrivate:              pResp.PlannedPrivate,
		Diagnostics:                 convertDiagnostics(pResp.Diagnostics),
		UnsafeToUseLegacyTypeSystem: pResp.LegacyTypeSystem,
	}, nil
}

func (ps *dynamicPfServer) ApplyResourceChange(c context.Context, req *tfprotov6.ApplyResourceChangeRequest) (*tfprotov6.ApplyResourceChangeResponse, error) {

	priorState, err := unmarshalFromDV(req.PriorState, ps.getResourceType(req.TypeName))
	if err != nil {
		return nil, err
	}

	plannedState, err := unmarshalFromDV(req.PlannedState, ps.getResourceType(req.TypeName))
	if err != nil {
		return nil, err
	}

	config, err := unmarshalFromDV(req.Config, ps.getResourceType(req.TypeName))
	if err != nil {
		return nil, err
	}

	meta, err := unmarshalFromDV(req.ProviderMeta, ps.getProviderMetaType())
	if err != nil {
		return nil, err
	}

	pReq := shim.ApplyResourceChangeRequest{
		TypeName:       req.TypeName,
		PlannedPrivate: req.PlannedPrivate,
		PriorState:     priorState,
		PlannedState:   plannedState,
		Config:         config,
		ProviderMeta:   meta,
	}
	pResp := ps.impl.ApplyResourceChange(pReq)
	newState, err := marshalToDV(&pResp.NewState)
	if err != nil {
		return nil, err
	}

	return &tfprotov6.ApplyResourceChangeResponse{
		NewState:                    newState,
		Private:                     pResp.Private,
		Diagnostics:                 convertDiagnostics(pResp.Diagnostics),
		UnsafeToUseLegacyTypeSystem: pResp.LegacyTypeSystem,
	}, nil
}

func (ps *dynamicPfServer) ImportResourceState(c context.Context, req *tfprotov6.ImportResourceStateRequest) (*tfprotov6.ImportResourceStateResponse, error) {
	pReq := shim.ImportResourceStateRequest{
		TypeName: req.TypeName,
		ID:       req.ID,
	}
	pResp := ps.impl.ImportResourceState(pReq)
	irs, err := convertImportedResources(pResp.ImportedResources)
	if err != nil {
		return nil, err
	}

	return &tfprotov6.ImportResourceStateResponse{
		ImportedResources: irs,
		Diagnostics:       convertDiagnostics(pResp.Diagnostics),
	}, nil
}

func (ps *dynamicPfServer) ValidateDataResourceConfig(c context.Context, req *tfprotov6.ValidateDataResourceConfigRequest) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	config, err := unmarshalFromDV(req.Config, ps.getDataSource(req.TypeName))
	if err != nil {
		return nil, err
	}
	pReq := shim.ValidateDataResourceConfigRequest{
		TypeName: req.TypeName,
		Config:   config,
	}
	pResp := ps.impl.ValidateDataResourceConfig(pReq)
	return &tfprotov6.ValidateDataResourceConfigResponse{
		Diagnostics: convertDiagnostics(pResp.Diagnostics),
	}, nil
}

func (ps *dynamicPfServer) ReadDataSource(c context.Context, req *tfprotov6.ReadDataSourceRequest) (*tfprotov6.ReadDataSourceResponse, error) {
	config, err := unmarshalFromDV(req.Config, ps.getDataSource(req.TypeName))
	if err != nil {
		return nil, err
	}

	meta, err := unmarshalFromDV(req.ProviderMeta, ps.getProviderMetaType())
	if err != nil {
		return nil, err
	}

	pReq := shim.ReadDataSourceRequest{
		TypeName:     req.TypeName,
		Config:       config,
		ProviderMeta: meta,
	}
	pResp := ps.impl.ReadDataSource(pReq)
	state, err := marshalToDV(&pResp.State)
	if err != nil {
		return nil, err
	}

	return &tfprotov6.ReadDataSourceResponse{
		State:       state,
		Diagnostics: convertDiagnostics(pResp.Diagnostics),
	}, nil
}

func convertImportedResources(irs []shim.ImportedResource) ([]*tfprotov6.ImportedResource, error) {
	res := make([]*tfprotov6.ImportedResource, 0, len(irs))
	for _, ir := range irs {
		converted, err := convertImportedResource(ir)
		if err != nil {
			return nil, err
		}
		res = append(res, converted)
	}
	return res, nil
}

func convertImportedResource(ir shim.ImportedResource) (*tfprotov6.ImportedResource, error) {
	state, err := marshalToDV(&ir.State)
	if err != nil {
		return nil, err
	}

	return &tfprotov6.ImportedResource{
		TypeName: ir.TypeName,
		State:    state,
		Private:  ir.Private,
	}, nil
}

func convertSchemas(scm map[string]shim.Schema) map[string]*tfprotov6.Schema {
	res := make(map[string]*tfprotov6.Schema, len(scm))
	for key, schema := range scm {
		res[key] = convertSchema(schema)
	}
	return res
}

func convertSchema(sc shim.Schema) *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: sc.Version,
		Block:   convertSchemaBlock(sc.Block, sc.Version),
	}
}

func convertSchemaBlock(sb *shim.SchemaBlock, version int64) *tfprotov6.SchemaBlock {
	if sb == nil {
		return nil
	}
	return &tfprotov6.SchemaBlock{
		Version:         version,
		Attributes:      convertSchemaAttributes(sb.Attributes),
		BlockTypes:      convertNestedBlocks(sb.BlockTypes, version),
		Description:     sb.Description,
		DescriptionKind: convertStringKind(sb.DescriptionKind),
		Deprecated:      sb.Deprecated,
	}
}

func convertNestedBlocks(nbs map[string]*shim.NestedBlock, version int64) []*tfprotov6.SchemaNestedBlock {
	res := make([]*tfprotov6.SchemaNestedBlock, 0, len(nbs))
	for name, blk := range nbs {
		res = append(res, convertNestedBlock(blk, name, version))
	}
	return res
}

func convertNestedBlock(nb *shim.NestedBlock, name string, version int64) *tfprotov6.SchemaNestedBlock {
	return &tfprotov6.SchemaNestedBlock{
		TypeName: name,
		Block:    convertSchemaBlock(&nb.Block, version),
		Nesting:  convertSchemaNestedBlockNestingMode(nb.Nesting),
		MinItems: int64(nb.MinItems),
		MaxItems: int64(nb.MaxItems),
	}
}

func convertSchemaNestedBlockNestingMode(m shim.NestingMode) tfprotov6.SchemaNestedBlockNestingMode {
	switch m {
	case shim.NestingSingle:
		return tfprotov6.SchemaNestedBlockNestingModeSingle
	case shim.NestingList:
		return tfprotov6.SchemaNestedBlockNestingModeList
	case shim.NestingSet:
		return tfprotov6.SchemaNestedBlockNestingModeSet
	case shim.NestingMap:
		return tfprotov6.SchemaNestedBlockNestingModeMap
	default:
		return tfprotov6.SchemaNestedBlockNestingModeInvalid
	}
}

func convertSchemaAttributes(as map[string]*shim.SchemaAttribute) []*tfprotov6.SchemaAttribute {
	res := make([]*tfprotov6.SchemaAttribute, 0, len(as))
	for name, attr := range as {
		res = append(res, convertSchemaAttribute(attr, name))
	}
	return res
}

func convertSchemaAttribute(a *shim.SchemaAttribute, name string) *tfprotov6.SchemaAttribute {
	tftype := tftypeFromCtyType(a.Type)
	return &tfprotov6.SchemaAttribute{
		Name:            name,
		Type:            tftype,
		NestedType:      convertSchemaObject(a.NestedType),
		Description:     a.Description,
		Required:        a.Required,
		Optional:        a.Optional,
		Computed:        a.Computed,
		Sensitive:       a.Sensitive,
		DescriptionKind: convertStringKind(a.DescriptionKind),
		Deprecated:      a.Deprecated,
	}
}

func convertSchemaObject(o *shim.SchemaObject) *tfprotov6.SchemaObject {
	if o == nil {
		return nil
	}
	return &tfprotov6.SchemaObject{
		Attributes: convertSchemaAttributes(o.Attributes),
		Nesting:    convertSchemaObjectNestingMode(o.Nesting),
	}
}

func convertSchemaObjectNestingMode(m shim.NestingMode) tfprotov6.SchemaObjectNestingMode {
	switch m {
	case shim.NestingSingle:
		return tfprotov6.SchemaObjectNestingModeSingle
	case shim.NestingList:
		return tfprotov6.SchemaObjectNestingModeList
	case shim.NestingSet:
		return tfprotov6.SchemaObjectNestingModeSet
	case shim.NestingMap:
		return tfprotov6.SchemaObjectNestingModeMap
	default:
		return tfprotov6.SchemaObjectNestingModeInvalid
	}
}

func convertStringKind(sk shim.SchemaStringKind) tfprotov6.StringKind {
	switch sk {
	case shim.SchemaStringMarkdown:
		return tfprotov6.StringKindMarkdown
	case shim.SchemaStringPlain:
		return tfprotov6.StringKindPlain
	default:
		return tfprotov6.StringKindPlain
	}
}

func convertServerCapabilities(sc shim.ServerCapabilities) *tfprotov6.ServerCapabilities {
	return &tfprotov6.ServerCapabilities{
		GetProviderSchemaOptional: sc.GetProviderSchemaOptional,
		PlanDestroy:               sc.PlanDestroy,
	}
}

func convertDiagnostics(ds []shim.Diagnostic) []*tfprotov6.Diagnostic {
	ret := make([]*tfprotov6.Diagnostic, 0, len(ds))

	for _, d := range ds {
		ret = append(ret, convertDiagnostic(d))
	}
	return ret
}

func convertDiagnostic(d shim.Diagnostic) *tfprotov6.Diagnostic {
	path := pathToAttributePath(shim.GetAttribute(d))
	return &tfprotov6.Diagnostic{
		Summary:   d.Description().Summary,
		Detail:    d.Description().Detail,
		Severity:  convertDiagnosticSeverity(d.Severity()),
		Attribute: path,
	}
}

func convertDiagnosticSeverity(s shim.Severity) tfprotov6.DiagnosticSeverity {
	switch s {
	case shim.SeverityError:
		return tfprotov6.DiagnosticSeverityError
	case shim.SeverityWarning:
		return tfprotov6.DiagnosticSeverityWarning
	default:
		return tfprotov6.DiagnosticSeverityInvalid
	}
}
