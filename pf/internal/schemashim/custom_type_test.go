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

package schemashim

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// This example is taken from aws:resourceexplorer/index:Index "timeouts" property.
func TestCustomTypeEmbeddingObjectType(t *testing.T) {
	type timeoutsType struct {
		types.ObjectType
	}

	raw := schema.SingleNestedBlock{
		Attributes: map[string]schema.Attribute{
			"create": schema.StringAttribute{},
			"read":   schema.StringAttribute{},
			"update": schema.StringAttribute{},
		},
		CustomType: timeoutsType{
			ObjectType: types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"create": types.StringType,
					"read":   types.StringType,
					"update": types.StringType,
				},
			},
		},
	}

	shimmed := &blockSchema{"key", pfutils.FromBlockLike(raw)}
	assert.Equal(t, shim.TypeMap, shimmed.Type())
	assert.NotNil(t, shimmed.Elem())
	_, isPseudoResource := shimmed.Elem().(shim.Resource)
	assert.Truef(t, isPseudoResource, "expected shim.Elem() to be of type shim.Resource, encoding an object type")
}
