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
		// wantErrValue is the exact string the error message is expected to
		// echo back (via info.Version=%q). Defaults to tt.version when empty;
		// set explicitly for the empty-version case so the assertion is not
		// vacuous (asserting a substring of "" is trivially true).
		wantErrValue string
	}{
		{name: "empty version is rejected", version: "", wantErr: true, wantErrValue: `info.Version=""`},
		{name: "whitespace-only version is rejected", version: "   ", wantErr: true},
		{name: "non-semver version is rejected", version: "not-a-version", wantErr: true},
		{name: "extra dotted segment is rejected", version: "1.2.3.4", wantErr: true},
		{name: "plain semver is accepted", version: "1.2.3", wantErr: false},
		{name: "semver with v prefix is accepted", version: "v1.2.3", wantErr: false},
		{name: "semver with prerelease is accepted", version: "1.2.3-alpha.1", wantErr: false},
		{name: "semver with build metadata is accepted", version: "1.2.3+build.1", wantErr: false},
		{name: "semver with leading zeros is rejected", version: "01.02.03", wantErr: true},
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
				want := tt.wantErrValue
				if want == "" {
					want = tt.version
				}
				assert.Contains(t, err.Error(), want)
				return
			}
			require.NoError(t, err)
		})
	}
}
