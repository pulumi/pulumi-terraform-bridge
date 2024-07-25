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

package parameterize

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalValue(t *testing.T) {
	t.Parallel()

	t.Run("remote", func(t *testing.T) {
		autogold.Expect(
			`{"remote":{"url":"registry/owner/type","version":"1.2.3"}}`,
		).Equal(t, string(Value{Remote: &RemoteValue{
			URL:     "registry/owner/type",
			Version: "1.2.3",
		}}.Marshal()))
	})

	t.Run("local", func(t *testing.T) {
		autogold.Expect(`{"local":{"path":"./path"}}`).Equal(t, string(Value{Local: &LocalValue{
			Path: "./path",
		}}.Marshal()))
	})

	// Invalid values of Value should panic to help catch bugs early.
	shouldPanicOnMarshal := []struct {
		name string
		v    Value
	}{
		{name: "zero value"},
		{"local missing path", Value{Local: new(LocalValue)}},
		{"remote missing version", Value{Remote: &RemoteValue{URL: "url"}}},
		{"remote missing url", Value{Remote: &RemoteValue{Version: "1.2.3"}}},
		{"remote missing values", Value{Remote: new(RemoteValue)}},
		{"local and remote", Value{Remote: new(RemoteValue), Local: new(LocalValue)}},
	}
	for _, tt := range shouldPanicOnMarshal {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() { tt.v.Marshal() })
		})
	}
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect Value
		err    bool
	}{
		{
			name:  "remote",
			input: `{"remote":{"url":"registry/owner/type","version":"1.2.3"}}`,
			expect: Value{Remote: &RemoteValue{
				URL:     "registry/owner/type",
				Version: "1.2.3",
			}},
		},
		{
			name:  "local",
			input: `{"local":{"path":"./path"}}`,
			expect: Value{Local: &LocalValue{
				Path: "./path",
			}},
		},
		{
			name:  "local and remote",
			input: `{"remote":{"url":"registry/owner/type","version":"1.2.3"},"local":{"path":"./path"}}`,
			err:   true,
		},
		{
			name:  "invalid options",
			input: `{"foo":{"url":"registry/owner/type","version":"1.2.3"}}`,
			err:   true,
		},
		{
			name:  "empty",
			input: `{}`,
			err:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := ParseValue([]byte(tt.input))
			if tt.err {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expect, actual)
			}
		})
	}
}

func TestValueIntoArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value Value
		args  Args
	}{
		{
			name: "remote",
			value: Value{Remote: &RemoteValue{
				URL:     "a/b/c",
				Version: "1.2.3",
			}},
			args: Args{Remote: &RemoteArgs{
				Name:    "a/b/c",
				Version: "1.2.3",
			}},
		},
		{
			name: "local",
			value: Value{Local: &LocalValue{
				Path: "./a/b/c",
			}},
			args: Args{Local: &LocalArgs{
				Path: "./a/b/c",
			}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.value.IntoArgs(), tt.args)
		})
	}
}
