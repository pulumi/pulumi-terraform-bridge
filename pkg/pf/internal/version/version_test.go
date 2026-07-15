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

package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "empty version is rejected", version: "", wantErr: true},
		{name: "non-semver version is rejected", version: "not-a-version", wantErr: true},
		{name: "plain semver is accepted", version: "1.2.3", wantErr: false},
		{name: "semver with v prefix is accepted", version: "v1.2.3", wantErr: false},
		{name: "semver with prerelease is accepted", version: "1.2.3-alpha.1", wantErr: false},
		{name: "partial version is accepted (tolerant parsing)", version: "1.2", wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Validate(tt.version)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(),
					"ProviderInfo.Version is required for Plugin Framework providers and must be semver-compatible")
				assert.Contains(t, err.Error(), tt.version)
				return
			}
			require.NoError(t, err)
		})
	}
}
