// Copyright 2016-2024, Pulumi Corporation.
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

package tfplugin5

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/opentofu/opentofu/internal/tfplugin5"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func GetMetadataRequest(i *tfprotov5.GetMetadataRequest) *tfplugin5.GetMetadata_Request {
	if i == nil {
		return nil
	}
	return &tfplugin5.GetMetadata_Request{}
}

func GetMetadataResponse(i *tfplugin5.GetMetadata_Response) *tfprotov5.GetMetadataResponse {
	if i == nil {
		return nil
	}

	return &tfprotov5.GetMetadataResponse{
		ServerCapabilities: serverCapabilities(i.ServerCapabilities),
		Diagnostics:        diagnostics(i.Diagnostics),
		DataSources:        applyArray(i.DataSources, dataSourceMetadata),
		Functions:          applyArray(i.Functions, functionMetadata),
		Resources:          applyArray(i.Resources, resourceMetadata),
	}
}

func dataSourceMetadata(i *tfplugin5.GetMetadata_DataSourceMetadata) tfprotov5.DataSourceMetadata {
	return tfprotov5.DataSourceMetadata{TypeName: i.GetTypeName()}
}

func functionMetadata(i *tfplugin5.GetMetadata_FunctionMetadata) tfprotov5.FunctionMetadata {
	return tfprotov5.FunctionMetadata{Name: i.GetName()}
}

func resourceMetadata(i *tfplugin5.GetMetadata_ResourceMetadata) tfprotov5.ResourceMetadata {
	return tfprotov5.ResourceMetadata{TypeName: i.GetTypeName()}
}

func serverCapabilities(i *tfplugin5.ServerCapabilities) *tfprotov5.ServerCapabilities {
	if i == nil {
		return nil
	}

	return &tfprotov5.ServerCapabilities{
		GetProviderSchemaOptional: i.GetProviderSchemaOptional,
		MoveResourceState:         i.MoveResourceState,
		PlanDestroy:               i.PlanDestroy,
	}
}

func diagnostics(i []*tfplugin5.Diagnostic) []*tfprotov5.Diagnostic {
	return applyArray(i, diagnostic)
}

func diagnostic(i *tfplugin5.Diagnostic) *tfprotov5.Diagnostic {
	if i == nil {
		return nil
	}

	return &tfprotov5.Diagnostic{
		Severity:  diagnosticSeverity(i.Severity),
		Summary:   i.Summary,
		Detail:    i.Detail,
		Attribute: attributePath(i.Attribute),
	}
}

func diagnosticSeverity(i tfplugin5.Diagnostic_Severity) tfprotov5.DiagnosticSeverity {
	switch i {
	case tfplugin5.Diagnostic_ERROR:
		return tfprotov5.DiagnosticSeverityError
	case tfplugin5.Diagnostic_WARNING:
		return tfprotov5.DiagnosticSeverityWarning
	default:
		return tfprotov5.DiagnosticSeverityInvalid
	}
}

func attributePath(i *tfplugin5.AttributePath) *tftypes.AttributePath {
	if i == nil {
		return nil
	}

	return tftypes.NewAttributePathWithSteps(applyArray(i.Steps, func(v *tfplugin5.AttributePath_Step) tftypes.AttributePathStep {
		switch v := v.Selector.(type) {
		case *tfplugin5.AttributePath_Step_AttributeName:
			return tftypes.AttributeName(v.AttributeName)
		case *tfplugin5.AttributePath_Step_ElementKeyInt:
			return tftypes.ElementKeyInt(v.ElementKeyInt)
		case *tfplugin5.AttributePath_Step_ElementKeyString:
			return tftypes.ElementKeyString(v.ElementKeyString)
		default:
			return nil
		}
	}))
}

func GetProviderSchemaRequest(i *tfprotov5.GetProviderSchemaRequest) *tfplugin5.GetProviderSchema_Request {
	if i == nil {
		return nil
	}

	return &tfplugin5.GetProviderSchema_Request{}
}

func GetProviderSchemaResponse(i *tfplugin5.GetProviderSchema_Response) *tfprotov5.GetProviderSchemaResponse {
	if i == nil {
		return nil
	}

	return &tfprotov5.GetProviderSchemaResponse{
		ServerCapabilities: serverCapabilities(i.ServerCapabilities),
		Provider:           schema(i.Provider),
		ProviderMeta:       schema(i.ProviderMeta),
		ResourceSchemas:    applyMap(i.ResourceSchemas, schema),
		DataSourceSchemas:  applyMap(i.DataSourceSchemas, schema),
		Functions:          applyMap(i.Functions, function),
		Diagnostics:        diagnostics(i.Diagnostics),
	}
}

func schema(i *tfplugin5.Schema) *tfprotov5.Schema {
	if i == nil {
		return nil
	}

	return &tfprotov5.Schema{
		Version: i.Version,
		Block:   schemaBlock(i.Block),
	}
}

func schemaBlock(i *tfplugin5.Schema_Block) *tfprotov5.SchemaBlock {
	if i == nil {
		return nil
	}

	return &tfprotov5.SchemaBlock{
		Version:         i.Version,
		Attributes:      attributes(i.Attributes),
		BlockTypes:      applyArray(i.BlockTypes, nestedBlock),
		Description:     i.Description,
		DescriptionKind: stringKind(i.DescriptionKind),
		Deprecated:      i.Deprecated,
	}
}

func attributes(i []*tfplugin5.Schema_Attribute) []*tfprotov5.SchemaAttribute {
	return applyArray(i, attribute)
}

func attribute(i *tfplugin5.Schema_Attribute) *tfprotov5.SchemaAttribute {
	if i == nil {
		return nil
	}

	return &tfprotov5.SchemaAttribute{
		Name:            i.Name,
		Type:            _type(i.Type),
		Description:     i.Description,
		Required:        i.Required,
		Optional:        i.Optional,
		Computed:        i.Computed,
		Sensitive:       i.Sensitive,
		DescriptionKind: stringKind(i.DescriptionKind),
		Deprecated:      i.Deprecated,
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

func nestedBlock(i *tfplugin5.Schema_NestedBlock) *tfprotov5.SchemaNestedBlock {
	if i == nil {
		return nil
	}

	return &tfprotov5.SchemaNestedBlock{
		TypeName: i.TypeName,
		Block:    block(i.Block),
		Nesting:  schemaBlockNestingMode(i.Nesting),
		MinItems: i.MinItems,
		MaxItems: i.MaxItems,
	}
}

func schemaBlockNestingMode(i tfplugin5.Schema_NestedBlock_NestingMode) tfprotov5.SchemaNestedBlockNestingMode {
	switch i {
	case tfplugin5.Schema_NestedBlock_GROUP:
		return tfprotov5.SchemaNestedBlockNestingModeGroup
	case tfplugin5.Schema_NestedBlock_LIST:
		return tfprotov5.SchemaNestedBlockNestingModeList
	case tfplugin5.Schema_NestedBlock_MAP:
		return tfprotov5.SchemaNestedBlockNestingModeMap
	case tfplugin5.Schema_NestedBlock_SET:
		return tfprotov5.SchemaNestedBlockNestingModeSet
	case tfplugin5.Schema_NestedBlock_SINGLE:
		return tfprotov5.SchemaNestedBlockNestingModeSingle
	default:
		return tfprotov5.SchemaNestedBlockNestingModeInvalid
	}
}

func block(i *tfplugin5.Schema_Block) *tfprotov5.SchemaBlock {
	if i == nil {
		return nil
	}

	return &tfprotov5.SchemaBlock{
		Version:         i.Version,
		Attributes:      attributes(i.Attributes),
		BlockTypes:      applyArray(i.BlockTypes, nestedBlock),
		Description:     i.Description,
		DescriptionKind: stringKind(i.DescriptionKind),
		Deprecated:      i.Deprecated,
	}
}

func stringKind(i tfplugin5.StringKind) tfprotov5.StringKind {
	switch i {
	case tfplugin5.StringKind_PLAIN:
		return tfprotov5.StringKindPlain
	case tfplugin5.StringKind_MARKDOWN:
		return tfprotov5.StringKindMarkdown
	default:
		contract.Failf("Unknown string kind %#v", i)
		return tfprotov5.StringKindMarkdown
	}
}

func function(i *tfplugin5.Function) *tfprotov5.Function {
	if i == nil {
		return nil
	}

	return &tfprotov5.Function{
		Parameters:         applyArray(i.Parameters, functionParameter),
		VariadicParameter:  functionParameter(i.VariadicParameter),
		Return:             functionReturn(i.Return),
		Summary:            i.Summary,
		Description:        i.Description,
		DescriptionKind:    stringKind(i.DescriptionKind),
		DeprecationMessage: i.DeprecationMessage,
	}
}

func functionParameter(i *tfplugin5.Function_Parameter) *tfprotov5.FunctionParameter {
	if i == nil {
		return nil
	}

	return &tfprotov5.FunctionParameter{
		AllowNullValue:     i.AllowNullValue,
		AllowUnknownValues: i.AllowUnknownValues,
		Description:        i.Description,
		DescriptionKind:    stringKind(i.DescriptionKind),
		Name:               i.Name,
		Type:               _type(i.Type),
	}
}

func functionReturn(i *tfplugin5.Function_Return) *tfprotov5.FunctionReturn {
	if i == nil {
		return nil
	}

	return &tfprotov5.FunctionReturn{
		Type: _type(i.Type),
	}
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
