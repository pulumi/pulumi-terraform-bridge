// Copyright 2016-2026, Pulumi Corporation.
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
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestNonUTF8StringRoundTrip(outerT *testing.T) {
	outerT.Parallel()

	converters := []struct {
		name string
		enc  Encoder
		dec  Decoder
	}{
		{"string", newStringEncoder(), newStringDecoder()},
		{"dynamic", newDynamicEncoder(), newDynamicDecoder()},
	}

	for _, c := range converters {
		outerT.Run(c.name, func(outerT *testing.T) {
			outerT.Parallel()
			rapid.Check(outerT, func(t *rapid.T) {
				b := rapid.SliceOf(rapid.Byte()).Draw(t, "bytes")
				// The marker-prefixed variant exercises the collision escape.
				for _, s := range []string{string(b), nonUTF8StringSig + string(b)} {
					tf := tftypes.NewValue(tftypes.String, s)

					prop, err := c.dec.toPropertyValue(tf)
					require.NoError(t, err)
					require.True(t, utf8.ValidString(prop.StringValue()))

					if utf8.ValidString(s) && !strings.HasPrefix(s, nonUTF8StringSig) {
						assert.Equal(t, s, prop.StringValue())
					}

					back, err := c.enc.fromPropertyValue(prop)
					require.NoError(t, err)
					require.Equal(t, tf, back)
				}
			})
		})
	}
}

// The escaped form is persisted in Pulumi state files, so it must not change between
// bridge versions.
func TestNonUTF8StringRepresentation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		tf   string
		prop string
	}{
		{
			name: "plain string is unchanged",
			tf:   "hello",
			prop: "hello",
		},
		{
			name: "invalid UTF-8 (gzip magic)",
			tf:   "\x1f\x8b\x08\x00binary",
			prop: nonUTF8StringSig + "H4sIAGJpbmFyeQ==",
		},
		{
			name: "valid string colliding with the marker",
			tf:   nonUTF8StringSig + "looks escaped but is user data",
			prop: nonUTF8StringSig +
				"X19wdWx1bWlfbm9uX3V0Zjhfc3RyaW5nX2FlYmM1ZmE1ODM3NDRkOTdhMGVlOWJiNmUwYzBlOWMyOmxv" +
				"b2tzIGVzY2FwZWQgYnV0IGlzIHVzZXIgZGF0YQ==",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			prop, err := newStringDecoder().toPropertyValue(
				tftypes.NewValue(tftypes.String, c.tf))
			require.NoError(t, err)
			require.Equal(t, resource.NewStringProperty(c.prop), prop)
		})
	}
}

func TestNonUTF8StringInvalidBase64(t *testing.T) {
	t.Parallel()

	_, err := newStringEncoder().fromPropertyValue(
		resource.NewStringProperty(nonUTF8StringSig + "!!! not base64 !!!"))
	require.ErrorContains(t, err, "not valid base64")
}
