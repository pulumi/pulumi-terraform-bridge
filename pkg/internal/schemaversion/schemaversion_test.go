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

package schemaversion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		schema  string
		version string
		want    string
	}{
		{
			name:    "missing version is filled in",
			schema:  `{"name":"cosmic","resources":{"cosmic:index:Res":{"description":"<b>&</b>"}}}`,
			version: "2.44.3",
			want:    `{"name":"cosmic","resources":{"cosmic:index:Res":{"description":"<b>&</b>"}},"version":"2.44.3"}`,
		},
		{
			name:    "empty version is filled in",
			schema:  `{"name":"cosmic","version":""}`,
			version: "2.44.3",
			want:    `{"name":"cosmic","version":"2.44.3"}`,
		},
		{
			name:    "leading v is dropped",
			schema:  `{"name":"cosmic"}`,
			version: "v2.44.3-alpha.1+dev",
			want:    `{"name":"cosmic","version":"2.44.3-alpha.1+dev"}`,
		},
		{
			name:    "existing version is kept",
			schema:  `{"name": "cosmic", "version": "1.0.0"}`,
			version: "2.44.3",
			want:    `{"name": "cosmic", "version": "1.0.0"}`,
		},
		{
			name:    "no binary version",
			schema:  `{"name": "cosmic"}`,
			version: "",
			want:    `{"name": "cosmic"}`,
		},
		{
			name:    "invalid binary version",
			schema:  `{"name": "cosmic"}`,
			version: "not-a-version",
			want:    `{"name": "cosmic"}`,
		},
		{
			name:    "no schema",
			schema:  ``,
			version: "2.44.3",
			want:    ``,
		},
		{
			name:    "schema is not an object",
			schema:  `null`,
			version: "2.44.3",
			want:    `null`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, string(Inject([]byte(tt.schema), tt.version)))
		})
	}
}
