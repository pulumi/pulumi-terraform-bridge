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

package tfbridge

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

func TestParseResourceStateFromTFInner(t *testing.T) {
    t.Parallel()
	ctx := context.Background()

	ty := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id": tftypes.String,
			"x":  tftypes.String,
		},
	}

	t.Run("parses physical nil", func(t *testing.T) {
		up, err := parseResourceStateFromTFInner(ctx, ty, 0, nil, nil)
		require.NoError(t, err)
		require.Equal(t, `tftypes.Object["id":tftypes.String, "x":tftypes.String]<null>`, up.state.Value.String())
	})

	t.Run("parses logical nil", func(t *testing.T) {
		dv, err := makeDynamicValue(tftypes.NewValue(ty, nil))
		require.NoError(t, err)
		up, err := parseResourceStateFromTFInner(ctx, ty, 0, &dv, nil)
		require.NoError(t, err)
		require.Equal(t, `tftypes.Object["id":tftypes.String, "x":tftypes.String]<null>`, up.state.Value.String())
	})

	t.Run("parses a valid object", func(t *testing.T) {
		dv, err := makeDynamicValue(tftypes.NewValue(ty, map[string]tftypes.Value{
			"id": tftypes.NewValue(tftypes.String, "id1"),
			"x":  tftypes.NewValue(tftypes.String, nil),
		}))
		require.NoError(t, err)
		up, err := parseResourceStateFromTFInner(ctx, ty, 0, &dv, nil)
		require.NoError(t, err)
		//nolint:lll
		require.Equal(t, `tftypes.Object["id":tftypes.String, "x":tftypes.String]<"id":tftypes.String<"id1">, "x":tftypes.String<null>>`, up.state.Value.String())
	})

	t.Run("fails to parse a malformed object", func(t *testing.T) {
		dv, err := makeDynamicValue(tftypes.NewValue(tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id": tftypes.String,
			},
		}, map[string]tftypes.Value{
			"id": tftypes.NewValue(tftypes.String, "id1"),
		}))
		require.NoError(t, err)
		up, err := parseResourceStateFromTFInner(ctx, ty, 0, &dv, nil)
		require.Nil(t, up)
		require.Equal(t, "error decoding object; expected 2 attributes, got 1", err.Error())
		t.Log(err)
		require.Error(t, err)
	})
}
