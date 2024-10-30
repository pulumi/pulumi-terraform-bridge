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

package proto

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func (p Provider) Resources(context.Context) (runtypes.Resources, error) {
	v, err := p.getSchema()
	if err != nil {
		return nil, err
	}
	return resources{collection(v.ResourceSchemas)}, nil
}

func (p Provider) DataSources(context.Context) (runtypes.DataSources, error) {
	v, err := p.getSchema()
	if err != nil {
		return nil, err
	}
	return datasources{collection(v.DataSourceSchemas)}, nil
}

type schema struct{ s *tfprotov6.Schema }

var _ = runtypes.Schema(schema{})

func (s schema) ResourceProtoSchema(ctx context.Context) (*tfprotov6.Schema, error) {
	// Technically this will return non-nil even when s is not a resource schema, but that is OK currently.
	return s.s, nil
}

func (s schema) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (interface{}, error) {
	return s.s.ValueType().ApplyTerraform5AttributePathStep(step)
}

func (s schema) Type(context.Context) tftypes.Type {
	return s.s.ValueType()
}

func (s schema) DeprecationMessage() string { return deprecated(s.s.Block.Deprecated) }

func (s schema) ResourceSchemaVersion() int64 { return s.s.Version }

func (s schema) Shim() shim.SchemaMap {
	return blockMap{s.s.Block}
}

type resources struct{ collection }

func (resources) IsResources() {}

type datasources struct{ collection }

func (datasources) IsDataSources() {}

type collection map[string]*tfprotov6.Schema

func (c collection) All() []runtypes.TypeName {
	arr := make([]runtypes.TypeName, 0, len(c))
	for k := range c {
		arr = append(arr, runtypes.TypeName(k))
	}
	return arr
}

func (c collection) Has(key runtypes.TypeName) bool {
	_, ok := c[string(key)]
	return ok
}

func (c collection) Schema(key runtypes.TypeName) runtypes.Schema {
	s, ok := c[string(key)]
	contract.Assertf(ok, "called Schema on a resource that does not exist")

	return schema{s}
}
