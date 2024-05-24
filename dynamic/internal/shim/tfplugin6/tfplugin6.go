package tfplugin6

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/opentofu/opentofu/internal/tfplugin6"
)

func GetMetadataRequest(i *tfprotov6.GetMetadataRequest) *tfplugin6.GetMetadata_Request {
	if i == nil {
		return nil
	}
	return &tfplugin6.GetMetadata_Request{}
}

func GetMetadataResult(i *tfplugin6.GetMetadata_Response) *tfprotov6.GetMetadataResponse {
	if i == nil {
		return nil
	}
	return &tfprotov6.GetMetadataResponse{
		ServerCapabilities: serverCapabilities(i.ServerCapabilities),
		Diagnostics:        diagnostics(i.Diagnostics),
		DataSources:        applyArray(i.DataSources, dataSourceMetadata),
		Functions:          applyArray(i.Functions, functionMetadata),
		Resources:          applyArray(i.Resources, resourceMetadata),
	}
}

func dataSourceMetadata(i *tfplugin6.GetMetadata_DataSourceMetadata) tfprotov6.DataSourceMetadata {
	return tfprotov6.DataSourceMetadata{TypeName: i.GetTypeName()}
}

func functionMetadata(i *tfplugin6.GetMetadata_FunctionMetadata) tfprotov6.FunctionMetadata {
	return tfprotov6.FunctionMetadata{Name: i.GetName()}
}

func resourceMetadata(i *tfplugin6.GetMetadata_ResourceMetadata) tfprotov6.ResourceMetadata {
	return tfprotov6.ResourceMetadata{TypeName: i.GetTypeName()}
}

func applyArray[From, To any, F func(From) To](arr []From, f F) []To {
	to := make([]To, len(arr))
	for i, v := range arr {
		to[i] = f(v)
	}
	return to
}

func applyMap[K comparable, From, To any, F func(From) To](m map[K]From, f F) map[K]To {
	out := make(map[K]To, len(m))
	for k, v := range m {
		out[k] = f(v)
	}
	return out
}

func serverCapabilities(i *tfplugin6.ServerCapabilities) *tfprotov6.ServerCapabilities {
	if i == nil {
		return nil
	}
	return &tfprotov6.ServerCapabilities{
		GetProviderSchemaOptional: i.GetProviderSchemaOptional,
		MoveResourceState:         i.MoveResourceState,
		PlanDestroy:               i.PlanDestroy,
	}
}

func diagnostics(i []*tfplugin6.Diagnostic) []*tfprotov6.Diagnostic {
	return applyArray(i, diagnostic)
}

func diagnostic(i *tfplugin6.Diagnostic) *tfprotov6.Diagnostic {
	if i == nil {
		return nil
	}
	return &tfprotov6.Diagnostic{
		Severity:  diagnosticSeverity(i.Severity),
		Summary:   i.Summary,
		Detail:    i.Detail,
		Attribute: attributePath(i.Attribute),
	}
}
func diagnosticSeverity(i tfplugin6.Diagnostic_Severity) tfprotov6.DiagnosticSeverity {
	switch i {
	case tfplugin6.Diagnostic_ERROR:
		return tfprotov6.DiagnosticSeverityError
	case tfplugin6.Diagnostic_WARNING:
		return tfprotov6.DiagnosticSeverityWarning
	default:
		return tfprotov6.DiagnosticSeverityInvalid
	}
}

func attributePath(i *tfplugin6.AttributePath) *tftypes.AttributePath {
	if i == nil {
		return nil
	}
	path := make([]tftypes.AttributePathStep, len(i.Steps))
	for i, step := range i.Steps {
		switch s := step.Selector.(type) {
		case *tfplugin6.AttributePath_Step_AttributeName:
			path[i] = tftypes.AttributeName(s.AttributeName)
		case *tfplugin6.AttributePath_Step_ElementKeyInt:
			path[i] = tftypes.ElementKeyInt(s.ElementKeyInt)
		case *tfplugin6.AttributePath_Step_ElementKeyString:
			path[i] = tftypes.ElementKeyString(s.ElementKeyString)
		}
	}

	return tftypes.NewAttributePathWithSteps(path)
}

func GetProviderSchemaRequest(i *tfprotov6.GetProviderSchemaRequest) *tfplugin6.GetProviderSchema_Request {
	if i == nil {
		return nil
	}
	return &tfplugin6.GetProviderSchema_Request{}
}

func GetProviderSchemaResponse(i *tfplugin6.GetProviderSchema_Response) *tfprotov6.GetProviderSchemaResponse {
	if i == nil {
		return nil
	}
	return &tfprotov6.GetProviderSchemaResponse{
		ServerCapabilities: serverCapabilities(i.ServerCapabilities),
		Provider:           schema(i.Provider),
		ProviderMeta:       schema(i.ProviderMeta),
		ResourceSchemas:    applyMap(i.ResourceSchemas, schema),
		DataSourceSchemas:  applyMap(i.DataSourceSchemas, schema),
		Functions:          applyMap(i.Functions, function),
		Diagnostics:        diagnostics(i.Diagnostics),
	}
}

func schema(i *tfplugin6.Schema) *tfprotov6.Schema {
	if i == nil {
		return nil
	}
	return &tfprotov6.Schema{
		Version: i.Version,
		Block:   schemaBlock(i.Block),
	}
}

func schemaBlock(i *tfplugin6.Schema_Block) *tfprotov6.SchemaBlock {
	if i == nil {
		return nil
	}
	return &tfprotov6.SchemaBlock{
		Version:         i.Version,
		Attributes:      applyArray(i.Attributes, schemaAttribute),
		BlockTypes:      applyArray(i.BlockTypes, schemaNestedBlock),
		Description:     i.Description,
		DescriptionKind: stringKind(i.DescriptionKind),
		Deprecated:      i.Deprecated,
	}
}

func stringKind(i tfplugin6.StringKind) tfprotov6.StringKind {
	switch i {
	case tfplugin6.StringKind_MARKDOWN:
		return tfprotov6.StringKindMarkdown
	case tfplugin6.StringKind_PLAIN:
		return tfprotov6.StringKindPlain
	default:
		return tfprotov6.StringKindPlain
	}
}

func _type(i []byte) tftypes.Type {
	if i == nil {
		return nil
	}
	// This function isn't really deprecated, but it is supposed to be private.
	t, err := tftypes.ParseJSONType(i)
	if err != nil {
		panic(err) // TODO: Handle invalid type
	}
	return t
}

func schemaAttribute(i *tfplugin6.Schema_Attribute) *tfprotov6.SchemaAttribute {
	if i == nil {
		return nil
	}

	return &tfprotov6.SchemaAttribute{
		Name:            i.Name,
		Type:            _type(i.Type),
		NestedType:      schemaObject(i.NestedType),
		Description:     i.Description,
		Required:        i.Required,
		Optional:        i.Optional,
		Computed:        i.Computed,
		Sensitive:       i.Sensitive,
		DescriptionKind: stringKind(i.DescriptionKind),
		Deprecated:      i.Deprecated,
	}
}

func schemaObject(i *tfplugin6.Schema_Object) *tfprotov6.SchemaObject {
	if i == nil {
		return nil
	}
	return &tfprotov6.SchemaObject{
		Attributes: applyArray(i.Attributes, schemaAttribute),
		Nesting:    schemaObjectNestingModel(i.Nesting),
	}
}

func schemaObjectNestingModel(i tfplugin6.Schema_Object_NestingMode) tfprotov6.SchemaObjectNestingMode {
	switch i {
	case tfplugin6.Schema_Object_LIST:
		return tfprotov6.SchemaObjectNestingModeList
	case tfplugin6.Schema_Object_MAP:
		return tfprotov6.SchemaObjectNestingModeMap
	case tfplugin6.Schema_Object_SET:
		return tfprotov6.SchemaObjectNestingModeSet
	case tfplugin6.Schema_Object_SINGLE:
		return tfprotov6.SchemaObjectNestingModeSingle
	default:
		return tfprotov6.SchemaObjectNestingModeInvalid
	}
}

func schemaNestedBlock(i *tfplugin6.Schema_NestedBlock) *tfprotov6.SchemaNestedBlock {
	if i == nil {
		return nil
	}
	return &tfprotov6.SchemaNestedBlock{
		TypeName: i.TypeName,
		Block:    schemaBlock(i.Block),
		Nesting:  tfprotov6.SchemaNestedBlockNestingMode(i.Nesting),
		MinItems: i.MinItems,
		MaxItems: i.MaxItems,
	}
}

func schemaNestedBlockNestingMode(i tfplugin6.Schema_NestedBlock_NestingMode) tfprotov6.SchemaNestedBlockNestingMode {
	switch i {
	case tfplugin6.Schema_NestedBlock_GROUP:
		return tfprotov6.SchemaNestedBlockNestingModeGroup
	case tfplugin6.Schema_NestedBlock_LIST:
		return tfprotov6.SchemaNestedBlockNestingModeList
	case tfplugin6.Schema_NestedBlock_MAP:
		return tfprotov6.SchemaNestedBlockNestingModeMap
	case tfplugin6.Schema_NestedBlock_SET:
		return tfprotov6.SchemaNestedBlockNestingModeSet
	case tfplugin6.Schema_NestedBlock_SINGLE:
		return tfprotov6.SchemaNestedBlockNestingModeSingle
	default:
		return tfprotov6.SchemaNestedBlockNestingModeInvalid
	}
}

func function(i *tfplugin6.Function) *tfprotov6.Function {
	if i == nil {
		return nil
	}
	return &tfprotov6.Function{
		Parameters:         applyArray(i.Parameters, funcionParameter),
		VariadicParameter:  funcionParameter(i.VariadicParameter),
		Return:             functionReturn(i.Return),
		Summary:            i.Summary,
		Description:        i.Description,
		DescriptionKind:    stringKind(i.DescriptionKind),
		DeprecationMessage: i.DeprecationMessage,
	}
}

func funcionParameter(i *tfplugin6.Function_Parameter) *tfprotov6.FunctionParameter {
	if i == nil {
		return nil
	}
	return &tfprotov6.FunctionParameter{
		AllowNullValue:     i.AllowNullValue,
		AllowUnknownValues: i.AllowUnknownValues,
		Description:        i.Description,
		DescriptionKind:    stringKind(i.DescriptionKind),
		Name:               i.Name,
		Type:               _type(i.Type),
	}
}

func functionReturn(i *tfplugin6.Function_Return) *tfprotov6.FunctionReturn {
	if i == nil {
		return nil
	}
	return &tfprotov6.FunctionReturn{
		Type: _type(i.Type),
	}
}

func ValidateProviderConfigRequest(i *tfprotov6.ValidateProviderConfigRequest) *tfplugin6.ValidateProviderConfig_Request {
	if i == nil {
		return nil
	}
	return &tfplugin6.ValidateProviderConfig_Request{
		Config: dynamicValueRequest(i.Config),
	}
}

func ValidateProviderConfigResponse(i *tfplugin6.ValidateProviderConfig_Response) *tfprotov6.ValidateProviderConfigResponse {
	if i == nil {
		return nil
	}
	return &tfprotov6.ValidateProviderConfigResponse{
		// This field is filled in where ValidateProviderConfigResponse is called.
		PreparedConfig: nil,
		Diagnostics:    diagnostics(i.Diagnostics),
	}
}

func dynamicValueRequest(i *tfprotov6.DynamicValue) *tfplugin6.DynamicValue {
	if i == nil {
		return nil
	}
	return &tfplugin6.DynamicValue{
		Msgpack: i.MsgPack,
		Json:    i.JSON,
	}
}

func ConfigureProviderRequest(i *tfprotov6.ConfigureProviderRequest) *tfplugin6.ConfigureProvider_Request {
	if i == nil {
		return nil
	}
	return &tfplugin6.ConfigureProvider_Request{
		TerraformVersion: i.TerraformVersion,
		Config:           dynamicValueRequest(i.Config),
	}
}

func ConfigureProviderResponse(i *tfplugin6.ConfigureProvider_Response) *tfprotov6.ConfigureProviderResponse {
	if i == nil {
		return nil
	}
	return &tfprotov6.ConfigureProviderResponse{
		Diagnostics: diagnostics(i.Diagnostics),
	}
}

func StopProviderRequest(i *tfprotov6.StopProviderRequest) *tfplugin6.StopProvider_Request {
	if i == nil {
		return nil
	}

	return &tfplugin6.StopProvider_Request{}
}

func StopProviderResponse(i *tfplugin6.StopProvider_Response) *tfprotov6.StopProviderResponse {
	if i == nil {
		return nil
	}

	return &tfprotov6.StopProviderResponse{Error: i.Error}
}

func ValidateResourceConfigRequest(i *tfprotov6.ValidateResourceConfigRequest) *tfplugin6.ValidateResourceConfig_Request {
	if i == nil {
		return nil
	}

	return &tfplugin6.ValidateResourceConfig_Request{
		TypeName: i.TypeName,
		Config:   dynamicValueRequest(i.Config),
	}
}

func ValidateResourceConfigResponse(i *tfplugin6.ValidateResourceConfig_Response) *tfprotov6.ValidateResourceConfigResponse {
	if i == nil {
		return nil
	}

	return &tfprotov6.ValidateResourceConfigResponse{
		Diagnostics: diagnostics(i.Diagnostics),
	}
}

func ReadResourceRequest(i *tfprotov6.ReadResourceRequest) *tfplugin6.ReadResource_Request {
	if i == nil {
		return nil
	}

	return &tfplugin6.ReadResource_Request{
		TypeName:     i.TypeName,
		CurrentState: dynamicValueRequest(i.CurrentState),
		Private:      i.Private,
		ProviderMeta: dynamicValueRequest(i.ProviderMeta),
	}
}

func ReadResourceResponse(i *tfplugin6.ReadResource_Response) *tfprotov6.ReadResourceResponse {
	if i == nil {
		return nil
	}

	return &tfprotov6.ReadResourceResponse{
		NewState:    dynamicValueResponse(i.NewState),
		Diagnostics: diagnostics(i.Diagnostics),
		Private:     i.Private,
		Deferred:    nil, // The provider won't specify a read as deferred.
	}
}

func dynamicValueResponse(i *tfplugin6.DynamicValue) *tfprotov6.DynamicValue {
	if i == nil {
		return nil
	}
	return &tfprotov6.DynamicValue{
		MsgPack: i.Msgpack,
		JSON:    i.Json,
	}
}

func UpgradeResourceStateRequest(i *tfprotov6.UpgradeResourceStateRequest) *tfplugin6.UpgradeResourceState_Request {
	if i == nil {
		return nil
	}

	return &tfplugin6.UpgradeResourceState_Request{
		TypeName: i.TypeName,
		Version:  i.Version,
		RawState: rawStateRequest(i.RawState),
	}
}

func UpgradeResourceStateResponse(i *tfplugin6.UpgradeResourceState_Response) *tfprotov6.UpgradeResourceStateResponse {
	if i == nil {
		return nil
	}

	return &tfprotov6.UpgradeResourceStateResponse{
		UpgradedState: dynamicValueResponse(i.UpgradedState),
		Diagnostics:   diagnostics(i.Diagnostics),
	}
}

func rawStateRequest(i *tfprotov6.RawState) *tfplugin6.RawState {
	if i == nil {
		return nil
	}

	return &tfplugin6.RawState{
		Json:    i.JSON,
		Flatmap: i.Flatmap,
	}
}
