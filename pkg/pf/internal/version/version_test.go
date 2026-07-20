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

// TestValidate covers Validate's own logic (the empty-version special case,
// and that a parse error or success from the underlying semver library is
// surfaced with the expected message and the offending value included). It
// deliberately does not enumerate the many strings blang/semver.ParseTolerant
// accepts or rejects (e.g. "v" prefixes, build metadata, leading zeros,
// partial versions) since that is the library's behavior to test, not ours;
// one representative valid and one representative invalid case are enough to
// confirm Validate delegates to it and wraps the result correctly.
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
		{name: "non-semver version is rejected", version: "not-a-version", wantErr: true},
		{name: "valid semver is accepted", version: "1.2.3", wantErr: false},
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
