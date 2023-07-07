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

package schemav6

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

const (
	alias = "alias"
	count = "count"
	x     = "x"
)

func TestProviderSchemaExtraction(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		s := pschema.Schema{
			Attributes: map[string]pschema.Attribute{
				x: pschema.StringAttribute{Optional: true},
			},
		}
		es, err := ProviderSchema(s)
		assert.NoError(t, err)

		found := false
		for _, a := range es.Block.Attributes {
			if a.Name != x {
				continue
			}
			assert.Equal(t, true, a.Optional)
			found = true
		}
		assert.True(t, found)
	})

	t.Run("invalid", func(t *testing.T) {
		s := pschema.Schema{
			Attributes: map[string]pschema.Attribute{
				alias: pschema.StringAttribute{Optional: true},
			},
		}
		_, err := ProviderSchema(s)
		assertReservedError(t, alias, err)
	})
}

func TestResoureSchemaExtraction(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		s := rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				x: rschema.StringAttribute{Optional: true},
			},
		}
		es, err := ResourceSchema(s)
		assert.NoError(t, err)

		found := false
		for _, a := range es.Block.Attributes {
			if a.Name != x {
				continue
			}
			assert.Equal(t, true, a.Optional)
			found = true
		}
		assert.True(t, found)
	})

	t.Run("invalid", func(t *testing.T) {
		s := rschema.Schema{
			Attributes: map[string]rschema.Attribute{
				count: rschema.StringAttribute{Optional: true},
			},
		}
		_, err := ResourceSchema(s)
		assertReservedError(t, count, err)
	})
}

func TestDataSourceSchemaExtraction(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		s := dschema.Schema{
			Attributes: map[string]dschema.Attribute{
				x: dschema.StringAttribute{Optional: true},
			},
		}
		es, err := DataSourceSchema(s)
		assert.NoError(t, err)

		found := false
		for _, a := range es.Block.Attributes {
			if a.Name != x {
				continue
			}
			assert.Equal(t, true, a.Optional)
			found = true
		}
		assert.True(t, found)
	})

	t.Run("invalid", func(t *testing.T) {
		s := dschema.Schema{
			Attributes: map[string]dschema.Attribute{
				count: dschema.StringAttribute{Optional: true},
			},
		}
		_, err := DataSourceSchema(s)
		assertReservedError(t, count, err)
	})
}

func assertReservedError(t *testing.T, field string, err error) {
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `1 unexpected diagnostic(s):`)
	line2 := fmt.Sprintf(`- ERROR at AttributeName("%s"). Schema Using Reserved Field Name:`, field)
	assert.Contains(t, err.Error(), line2)
	line3 := fmt.Sprintf(`"%s" is a reserved field name`, field)
	assert.Contains(t, err.Error(), line3)
}
