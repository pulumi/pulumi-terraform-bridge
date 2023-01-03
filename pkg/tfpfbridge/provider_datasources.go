// Copyright 2016-2023, Pulumi Corporation.
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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/pfutils"
)

type datasourceHandle struct {
	makeDataSource          func() datasource.DataSource
	terraformDataSourceName string
	schema                  tfsdk.Schema
	encoder                 convert.Encoder
	decoder                 convert.Decoder
}

func (p *Provider) datasourceHandle(ctx context.Context, token tokens.ModuleMember) (datasourceHandle, error) {
	dsName, err := p.terraformDatasourceName(token)
	if err != nil {
		return datasourceHandle{}, err
	}

	typeName := pfutils.TypeName(dsName)

	schema := p.datasources.Schema(typeName)

	makeDataSource := func() datasource.DataSource {
		return p.datasources.DataSource(typeName)
	}

	// inputs := map[string]attr.Type{}
	// outputs := map[string]attr.Type{}

	// for name, attr := range schema.Attributes {
	// 	if !(attr.IsComputed() && !attr.IsOptional()) {
	// 		inputs[name] = attr.FrameworkType()
	// 	}
	// 	if attr.IsComputed() {
	// 		outputs[name] = attr.FrameworkType()
	// 	}
	// }

	// for name, block := range schema.Blocks {
	// 	inputs[name] = block.Type()
	// }

	// inputAttrType := types.ObjectType{AttrTypes: inputs}
	// outputAttrType := types.ObjectType{AttrTypes: outputs}

	// inputType := inputAttrType.TerraformType(ctx).(tftypes.Object)
	// outputType := outputAttrType.TerraformType(ctx).(tftypes.Object)

	typ := schema.Type().TerraformType(ctx).(tftypes.Object)

	encoder, err := p.encoding.NewDataSourceEncoder(token, typ)
	if err != nil {
		return datasourceHandle{}, err
	}

	decoder, err := p.encoding.NewDataSourceDecoder(token, typ)
	if err != nil {
		return datasourceHandle{}, err
	}

	return datasourceHandle{
		makeDataSource:          makeDataSource,
		terraformDataSourceName: dsName,
		schema:                  schema,
		encoder:                 encoder,
		decoder:                 decoder,
	}, nil
}
