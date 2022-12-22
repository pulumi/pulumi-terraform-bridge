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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestFlattenedListEncoder(t *testing.T) {
	enc := &encoding{nil, nil}
	encoder, err := enc.newPropertyEncoder("p", schema.PropertySpec{
		TypeSpec: schema.TypeSpec{
			Type: "string",
		},
	}, tftypes.List{ElementType: tftypes.String})
	require.NoError(t, err)

	t.Run("singleton-list", func(t *testing.T) {
		actual, err := encoder.FromPropertyValue(resource.NewStringProperty("foo"))
		require.NoError(t, err)
		expected := tftypes.NewValue(
			tftypes.List{ElementType: tftypes.String},
			[]tftypes.Value{
				tftypes.NewValue(tftypes.String, "foo"),
			})
		assert.Equal(t, expected, actual)
	})

	t.Run("empty-list", func(t *testing.T) {
		actual, err := encoder.FromPropertyValue(resource.NewNullProperty())
		require.NoError(t, err)
		expected := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{})
		assert.Equal(t, expected, actual)
	})
}

func TestFlattenedListDecoder(t *testing.T) {
	enc := &encoding{nil, nil}
	encoder, err := enc.newPropertyDecoder("p", schema.PropertySpec{
		TypeSpec: schema.TypeSpec{
			Type: "string",
		},
	}, tftypes.List{ElementType: tftypes.String})
	require.NoError(t, err)

	t.Run("singleton-list", func(t *testing.T) {
		tfValue := tftypes.NewValue(
			tftypes.List{ElementType: tftypes.String},
			[]tftypes.Value{
				tftypes.NewValue(tftypes.String, "foo"),
			})
		actual, err := encoder.ToPropertyValue(tfValue)
		require.NoError(t, err)
		expected := resource.NewStringProperty("foo")
		assert.Equal(t, expected, actual)
	})

	t.Run("empty-list", func(t *testing.T) {
		tfValue := tftypes.NewValue(
			tftypes.List{ElementType: tftypes.String},
			[]tftypes.Value{})
		actual, err := encoder.ToPropertyValue(tfValue)
		require.NoError(t, err)
		expected := resource.NewNullProperty()
		assert.Equal(t, expected, actual)
	})
}
