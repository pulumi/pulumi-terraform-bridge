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

package crosstests

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestConvertResourceValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    resource.PropertyMap
		expected map[string]any
	}{
		{
			input: resource.PropertyMap{
				"a": resource.NewProperty(resource.PropertyMap{}),
			},
			expected: map[string]any{
				"a": map[string]any{},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			actual := convertResourceValue(t, tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
