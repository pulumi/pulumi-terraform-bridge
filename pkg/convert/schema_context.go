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

package convert

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type schemaMapContext struct {
	schemaPath  walk.SchemaPath
	schemaMap   shim.SchemaMap
	schemaInfos map[string]*tfbridge.SchemaInfo
}

var _ localPropertyNames = &schemaMapContext{}

func newSchemaMapContext(schemaMap shim.SchemaMap, schemaInfos map[string]*tfbridge.SchemaInfo) *schemaMapContext {
	return &schemaMapContext{
		schemaPath:  walk.NewSchemaPath(),
		schemaMap:   schemaMap,
		schemaInfos: schemaInfos,
	}
}

func newResourceSchemaMapContext(
	resource string,
	schemaOnlyProvider shim.Provider,
	providerInfo *tfbridge.ProviderInfo,
) *schemaMapContext {
	r := schemaOnlyProvider.ResourcesMap().Get(resource)
	contract.Assertf(r != nil, "no resource %q found in ResourceMap", resource)
	sm := r.Schema()
	var fields map[string]*tfbridge.SchemaInfo
	if providerInfo != nil {
		fields = providerInfo.Resources[resource].GetFields()
	}
	return newSchemaMapContext(sm, fields)
}

func newDataSourceSchemaMapContext(
	dataSource string,
	schemaOnlyProvider shim.Provider,
	providerInfo *tfbridge.ProviderInfo,
) *schemaMapContext {
	r := schemaOnlyProvider.DataSourcesMap().Get(dataSource)
	contract.Assertf(r != nil, "no data source %q found in DataSourcesMap", dataSource)
	sm := r.Schema()
	var fields map[string]*tfbridge.SchemaInfo
	if providerInfo != nil {
		fields = providerInfo.DataSources[dataSource].GetFields()
	}
	return newSchemaMapContext(sm, fields)
}

func (sc *schemaMapContext) PropertyKey(tfname terraformPropertyName, _ tftypes.Type) resource.PropertyKey {
	return sc.ToPropertyKey(tfname)
}

func (sc *schemaMapContext) ToPropertyKey(tfname terraformPropertyName) resource.PropertyKey {
	n := tfbridge.TerraformToPulumiNameV2(tfname, sc.schemaMap, sc.schemaInfos)
	return resource.PropertyKey(n)
}

func (sc *schemaMapContext) GetAttr(tfname terraformPropertyName) (*schemaPropContext, error) {
	step := walk.NewSchemaPath().GetAttr(tfname)
	s, err := walk.LookupSchemaMapPath(step, sc.schemaMap)
	if err != nil {
		return nil, err
	}
	sinfo := tfbridge.LookupSchemaInfoMapPath(step, sc.schemaInfos)
	return &schemaPropContext{
		schemaPath: sc.schemaPath.GetAttr(tfname),
		schema:     s,
		schemaInfo: sinfo,
	}, nil
}

type schemaPropContext struct {
	schemaPath walk.SchemaPath
	schema     shim.Schema
	schemaInfo *tfbridge.SchemaInfo
}

func (pc *schemaPropContext) Secret() bool {
	if pc.schemaInfo != nil && pc.schemaInfo.Secret != nil {
		return *pc.schemaInfo.Secret
	}
	if pc.schema != nil {
		return pc.schema.Sensitive()
	}
	return false
}

func (pc *schemaPropContext) Element() (*schemaPropContext, error) {
	step := walk.NewSchemaPath().Element()
	var s shim.Schema
	if pc.schema != nil {
		var err error
		s, err = walk.LookupSchemaPath(step, pc.schema)
		if err != nil {
			return nil, fmt.Errorf("when deriving converters for an element of a collection: %w", err)
		}
	}
	sinfo := tfbridge.LookupSchemaInfoPath(step, pc.schemaInfo)
	return &schemaPropContext{
		schemaPath: pc.schemaPath.Element(),
		schema:     s,
		schemaInfo: sinfo,
	}, nil
}

func (pc *schemaPropContext) TupleElement(position int) (*schemaPropContext, error) {
	mctx, err := pc.Object()
	if err != nil {
		return nil, fmt.Errorf("when deriving converters for a tuple element at position %d %w", position, err)
	}
	return mctx.GetAttr(tuplePropertyName(position))
}

func (pc *schemaPropContext) Object() (*schemaMapContext, error) {
	if pc.schema != nil {
		switch elem := pc.schema.Elem().(type) {
		case shim.Resource:
			var fields map[string]*tfbridge.SchemaInfo
			if pc.schemaInfo != nil {
				fields = pc.schemaInfo.Fields
			}
			return &schemaMapContext{
				schemaPath:  pc.schemaPath,
				schemaMap:   elem.Schema(),
				schemaInfos: fields,
			}, nil
		}
	}
	return nil, fmt.Errorf("expected an object type schema at %s",
		pc.schemaPath.GoString())
}

func (pc *schemaPropContext) IsMaxItemsOne(collection tftypes.Type) (tftypes.Type, bool) {
	switch c := collection.(type) {
	case tftypes.List:
		if tfbridge.IsMaxItemsOne(pc.schema, pc.schemaInfo) {
			return c.ElementType, true
		}
	case tftypes.Set:
		if tfbridge.IsMaxItemsOne(pc.schema, pc.schemaInfo) {
			return c.ElementType, true
		}
	}
	return nil, false
}
