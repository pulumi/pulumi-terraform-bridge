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
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type datasourceHandle struct {
	token                   tokens.ModuleMember
	makeDataSource          func() datasource.DataSource
	terraformDataSourceName string
	schema                  pfutils.Schema
	encoder                 convert.Encoder
	decoder                 convert.Decoder
	schemaOnlyShim          shim.Resource
	pulumiDataSourceInfo    *tfbridge.DataSourceInfo // optional
}

func (p *provider) datasourceHandle(ctx context.Context, token tokens.ModuleMember) (datasourceHandle, error) {
	dsName, err := p.terraformDatasourceName(token)
	if err != nil {
		return datasourceHandle{}, err
	}

	typeName := pfutils.TypeName(dsName)
	schema := p.datasources.Schema(typeName)

	makeDataSource := func() datasource.DataSource {
		return p.datasources.DataSource(typeName)
	}

	typ := schema.Type().TerraformType(ctx).(tftypes.Object)

	encoder, err := p.encoding.NewDataSourceEncoder(dsName, typ)
	if err != nil {
		return datasourceHandle{}, err
	}

	decoder, err := p.encoding.NewDataSourceDecoder(dsName, typ)
	if err != nil {
		return datasourceHandle{}, err
	}

	shim, _ := p.schemaOnlyProvider.DataSourcesMap().GetOk(dsName)

	result := datasourceHandle{
		token:                   token,
		makeDataSource:          makeDataSource,
		terraformDataSourceName: dsName,
		schema:                  schema,
		encoder:                 encoder,
		decoder:                 decoder,
		schemaOnlyShim:          shim,
	}

	if info, ok := p.info.DataSources[dsName]; ok {
		result.pulumiDataSourceInfo = info
	}

	return result, nil
}
