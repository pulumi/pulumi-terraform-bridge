// Copyright 2016-2022, Pulumi Corporation.
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
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	shimschema "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

func TestFlattenedEncoder(t *testing.T) {
	t.Parallel()
	listEncoder, err := newPropertyEncoder(
		maxItemsOneCollectionPropContext("p", shim.TypeList),
		"p",
		tftypes.List{ElementType: tftypes.String})
	require.NoError(t, err)

	setEncoder, err := newPropertyEncoder(
		maxItemsOneCollectionPropContext("p", shim.TypeSet),
		"p",
		tftypes.Set{ElementType: tftypes.String})
	require.NoError(t, err)

	t.Run("singleton-list", func(t *testing.T) {
		actual, err := listEncoder.fromPropertyValue(resource.NewStringProperty("foo"))
		require.NoError(t, err)
		expected := tftypes.NewValue(tftypes.List{ElementType: tftypes.String},
			[]tftypes.Value{tftypes.NewValue(tftypes.String, "foo")})
		assert.Equal(t, expected, actual)
	})

	t.Run("empty-list", func(t *testing.T) {
		actual, err := listEncoder.fromPropertyValue(resource.NewNullProperty())
		require.NoError(t, err)
		expected := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{})
		assert.Equal(t, expected, actual)
	})

	t.Run("singleton-set", func(t *testing.T) {
		actual, err := setEncoder.fromPropertyValue(resource.NewStringProperty("foo"))
		require.NoError(t, err)
		expected := tftypes.NewValue(tftypes.Set{ElementType: tftypes.String},
			[]tftypes.Value{tftypes.NewValue(tftypes.String, "foo")})
		assert.Equal(t, expected, actual)
	})

	t.Run("empty-set", func(t *testing.T) {
		actual, err := setEncoder.fromPropertyValue(resource.NewNullProperty())
		require.NoError(t, err)
		expected := tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{})
		assert.Equal(t, expected, actual)
	})

	t.Run("error-propagation", func(t *testing.T) {
		_, err := listEncoder.fromPropertyValue(resource.NewObjectProperty(resource.PropertyMap{}))
		require.Error(t, err)
	})
}

func TestFlattenedDecoder(t *testing.T) {
	t.Parallel()
	listDecoder, err := newPropertyDecoder(
		maxItemsOneCollectionPropContext("p", shim.TypeList),
		"p",
		tftypes.List{ElementType: tftypes.String})
	require.NoError(t, err)

	setDecoder, err := newPropertyDecoder(
		maxItemsOneCollectionPropContext("p", shim.TypeSet),
		"p",
		tftypes.Set{ElementType: tftypes.String})
	require.NoError(t, err)

	t.Run("singleton-list", func(t *testing.T) {
		tfValue := tftypes.NewValue(tftypes.List{ElementType: tftypes.String},
			[]tftypes.Value{tftypes.NewValue(tftypes.String, "foo")})
		actual, err := decode(listDecoder, tfValue)
		require.NoError(t, err)
		expected := resource.NewStringProperty("foo")
		assert.Equal(t, expected, actual)
	})

	t.Run("empty-list", func(t *testing.T) {
		tfValue := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{})
		actual, err := decode(listDecoder, tfValue)
		require.NoError(t, err)
		expected := resource.NewNullProperty()
		assert.Equal(t, expected, actual)
	})

	t.Run("singleton-set", func(t *testing.T) {
		tfValue := tftypes.NewValue(tftypes.Set{ElementType: tftypes.String},
			[]tftypes.Value{tftypes.NewValue(tftypes.String, "foo")})
		actual, err := decode(setDecoder, tfValue)
		require.NoError(t, err)
		expected := resource.NewStringProperty("foo")
		assert.Equal(t, expected, actual)
	})

	t.Run("empty-set", func(t *testing.T) {
		tfValue := tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{})
		actual, err := decode(setDecoder, tfValue)
		require.NoError(t, err)
		expected := resource.NewNullProperty()
		assert.Equal(t, expected, actual)
	})

	t.Run("error-propagation", func(t *testing.T) {
		tfValue := tftypes.NewValue(tftypes.String, "mistyped")
		_, err := decode(listDecoder, tfValue)
		require.Error(t, err)
	})

	t.Run("unknown-list-element", func(t *testing.T) {
		tfValue := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
			tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		})
		v, err := decode(listDecoder, tfValue)
		require.NoError(t, err)
		require.Equal(t, unknownProperty(), v)
	})

	t.Run("unknown-set-element", func(t *testing.T) {
		tfValue := tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{
			tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		})
		v, err := decode(setDecoder, tfValue)
		require.NoError(t, err)
		require.Equal(t, unknownProperty(), v)
	})

	t.Run("unknown-list", func(t *testing.T) {
		tfValue := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue)
		v, err := decode(listDecoder, tfValue)
		require.NoError(t, err)
		require.Equal(t, unknownProperty(), v)
	})

	t.Run("unknown-set", func(t *testing.T) {
		tfValue := tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, tftypes.UnknownValue)
		v, err := decode(setDecoder, tfValue)
		require.NoError(t, err)
		require.Equal(t, unknownProperty(), v)
	})
}

func maxItemsOneCollectionPropContext(propName string, collectionType shim.ValueType) *schemaPropContext {
	yes := true
	return &schemaPropContext{
		schemaPath: walk.NewSchemaPath().GetAttr(propName),
		schema: (&shimschema.Schema{
			Type:     collectionType,
			Optional: true,
			Elem: (&shimschema.Schema{
				Type:     shim.TypeString,
				Optional: true,
			}).Shim(),
		}).Shim(),
		schemaInfo: &tfbridge.SchemaInfo{
			MaxItemsOne: &yes,
		},
	}
}
