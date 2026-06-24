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

package sdkv2

import (
	"context"
	"testing"
	"unsafe"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinknameSymbolsResolve exercises every //go:linkname target in
// linkname.go. The package failing to link (because an SDK upgrade renamed or
// removed a symbol) shows up as a build failure of this test binary; the
// assertions additionally confirm the linked functions behave as expected,
// which guards the unsafe.Pointer-based call into setWriteOnlyNullValues.
func TestLinknameSymbolsResolve(t *testing.T) {
	t.Parallel()

	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {Type: schema.TypeString, Optional: true},
		},
	}
	block := res.CoreConfigSchema()
	ty := block.ImpliedType()

	t.Run("hcl2ValueFromConfigValue", func(t *testing.T) {
		got := hcl2ValueFromConfigValue(map[string]interface{}{"name": "foo"})
		assert.Equal(t, cty.StringVal("foo"), got.GetAttr("name"))
	})

	t.Run("hcl2ValueFromFlatmap", func(t *testing.T) {
		got, err := hcl2ValueFromFlatmap(map[string]string{"name": "foo"}, ty)
		require.NoError(t, err)
		assert.Equal(t, cty.StringVal("foo"), got.GetAttr("name"))
	})

	t.Run("valuesSDKEquivalent", func(t *testing.T) {
		v := cty.ObjectVal(map[string]cty.Value{"name": cty.StringVal("foo")})
		assert.True(t, valuesSDKEquivalent(v, v))
	})

	t.Run("normalizeNullValues", func(t *testing.T) {
		v := cty.ObjectVal(map[string]cty.Value{"name": cty.StringVal("foo")})
		assert.Equal(t, v, normalizeNullValues(v, v, false))
	})

	t.Run("copyTimeoutValues", func(t *testing.T) {
		v := cty.ObjectVal(map[string]cty.Value{"name": cty.StringVal("foo")})
		assert.Equal(t, v, copyTimeoutValues(v, v))
	})

	t.Run("setWriteOnlyNullValues", func(t *testing.T) {
		v := cty.ObjectVal(map[string]cty.Value{
			"id":   cty.NullVal(cty.String),
			"name": cty.StringVal("foo"),
		})
		// No write-only attributes, so the value is returned unchanged. The
		// function reshapes its input against the block schema, so passing it
		// the full implied-type object confirms the unsafe.Pointer block
		// argument is interpreted correctly.
		assert.Equal(t, v, setWriteOnlyNullValues(v, unsafe.Pointer(block)))
	})

	t.Run("validateConfigNulls", func(t *testing.T) {
		v := cty.ObjectVal(map[string]cty.Value{"name": cty.StringVal("foo")})
		assert.Empty(t, validateConfigNulls(context.Background(), v, nil))
	})
}
