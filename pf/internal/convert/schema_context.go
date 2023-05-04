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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	twalk "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/walk"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type schemaMapContext struct {
	schemaPath  walk.SchemaPath
	schemaMap   shim.SchemaMap
	schemaInfos map[string]*tfbridge.SchemaInfo
}

var _ LocalPropertyNames = &schemaMapContext{}

func newSchemaMapContext(schemaMap shim.SchemaMap, schemaInfos map[string]*tfbridge.SchemaInfo) *schemaMapContext {
	return &schemaMapContext{
		schemaPath:  walk.NewSchemaPath(),
		schemaMap:   schemaMap,
		schemaInfos: schemaInfos,
	}
}

func (sc *schemaMapContext) PropertyKey(tfname TerraformPropertyName, _ tftypes.Type) resource.PropertyKey {
	return sc.ToPropertyKey(tfname)
}

func (sc *schemaMapContext) ToPropertyKey(tfname TerraformPropertyName) resource.PropertyKey {
	n := tfbridge.TerraformToPulumiNameV2(tfname, sc.schemaMap, sc.schemaInfos)
	return resource.PropertyKey(n)
}

func (sc *schemaMapContext) GetAttr(tfname TerraformPropertyName) *schemaPropContext {
	step := walk.NewSchemaPath().GetAttr(tfname)
	s, err := walk.LookupSchemaMapPath(step, sc.schemaMap)
	if err != nil {
		panic(err) /* TODO proper error handling */
	}
	sinfo := twalk.LookupSchemaInfoMapPath(step, sc.schemaInfos)
	return &schemaPropContext{
		schemaPath: sc.schemaPath.GetAttr(tfname),
		schema:     s,
		schemaInfo: sinfo,
	}
}

type schemaPropContext struct {
	schemaPath walk.SchemaPath
	schema     shim.Schema
	schemaInfo *tfbridge.SchemaInfo
}

func (pc *schemaPropContext) Element() *schemaPropContext {
	return pc
}

func (pc *schemaPropContext) Object() *schemaMapContext {
	panic("TODO")
}

func (pc *schemaPropContext) IsMaxItemsOne(collection tftypes.Type) (tftypes.Type, bool) {
	return nil, false
}
