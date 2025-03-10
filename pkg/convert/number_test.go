// Copyright 2016-2025, Pulumi Corporation.
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
	"github.com/stretchr/testify/require"
)

func Test_numberEncoder_emptyStringToNull(t *testing.T) {
	t.Parallel()
	n := newNumberEncoder()
	v, err := n.fromPropertyValue(resource.NewStringProperty(""))
	require.NoError(t, err)
	require.True(t, v.IsNull())
}

func Test_nubmerEncoder_tryParseNumber_int(t *testing.T) {
	t.Parallel()
	n := newNumberEncoder().(*numberEncoder)
	v, ok := n.tryParseNumber("42")
	require.Equal(t, int64(42), v)
	require.Equal(t, true, ok)
	vv, err := n.fromPropertyValue(resource.NewStringProperty("42"))
	require.NoError(t, err)
	require.Equal(t, tftypes.Number, vv.Type())
	require.True(t, vv.Equal(tftypes.NewValue(tftypes.Number, int64(42))))
}

func Test_nubmerEncoder_tryParseNumber_float(t *testing.T) {
	t.Parallel()
	n := newNumberEncoder().(*numberEncoder)
	v, ok := n.tryParseNumber("42.5")
	require.Equal(t, float64(42.5), v)
	require.Equal(t, true, ok)
	vv, err := n.fromPropertyValue(resource.NewStringProperty("42.5"))
	require.NoError(t, err)
	require.Equal(t, tftypes.Number, vv.Type())
	require.True(t, vv.Equal(tftypes.NewValue(tftypes.Number, float64(42.5))))
}
