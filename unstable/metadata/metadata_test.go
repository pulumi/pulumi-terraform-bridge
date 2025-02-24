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

package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshal(t *testing.T) {
	t.Parallel()
	data, err := New(nil)
	require.NoError(t, err)

	err = Set(data, "hi", []string{"hello", "world"})
	assert.NoError(t, err)

	marshalled := data.MarshalIndent()
	assert.Equal(t, `{
    "hi": [
        "hello",
        "world"
    ]
}`, string(marshalled))

	parsed, err := New(marshalled)
	assert.NoError(t, err)
	read, _, err := Get[[]string](parsed, "hi")
	assert.NoError(t, err)
	assert.Equal(t, []string{"hello", "world"}, read)
}

func TestMarshalIndent(t *testing.T) {
	t.Parallel()
	data, err := New(nil)
	require.NoError(t, err)

	err = Set(data, "hi", []string{"hello", "world"})
	assert.NoError(t, err)

	marshalled := data.Marshal()
	assert.Equal(t, `{"hi":["hello","world"]}`, string(marshalled))

	parsed, err := New(marshalled)
	assert.NoError(t, err)
	read, _, err := Get[[]string](parsed, "hi")
	assert.NoError(t, err)
	assert.Equal(t, []string{"hello", "world"}, read)
}
