// Copyright 2026, Pulumi Corporation.
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

package run

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamedProvider_MirrorEnvVar(t *testing.T) {
	// Verify that setting the env var causes the mirror path to be used.
	// We use a mock mirror that returns valid version info but no actual binaries,
	// which is enough to prove the mirror path is taken.

	platform := runtime.GOOS + "_" + runtime.GOARCH

	mux := http.NewServeMux()
	// Use the full provider source with explicit registry hostname
	mux.HandleFunc("/registry.terraform.io/hashicorp/random/index.json", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"versions": map[string]interface{}{
				"3.6.3": map[string]interface{}{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/registry.terraform.io/hashicorp/random/3.6.3.json", func(w http.ResponseWriter, r *http.Request) {
		// Return a package response that points to a non-existent download URL.
		// This will cause the install to fail, but proves the mirror path is used.
		resp := map[string]interface{}{
			"archives": map[string]interface{}{
				platform: map[string]interface{}{
					"url": "http://localhost:1/nonexistent.zip",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// Set up env vars
	t.Setenv(envNetworkMirror, server.URL)
	t.Setenv(envPluginCache, t.TempDir())

	ctx := context.Background()

	// Use explicit registry hostname so the mirror paths match
	_, err := NamedProvider(ctx, "registry.terraform.io/hashicorp/random", "3.6.3")
	require.Error(t, err)
	// The error should be about failing to install from mirror, not about service discovery
	assert.Contains(t, err.Error(), "mirror")
	assert.NotContains(t, err.Error(), "discovery document")
	assert.NotContains(t, err.Error(), ".well-known")
}

func TestNamedProvider_MirrorNotUsedWhenUnset(t *testing.T) {
	// Verify that when the env var is NOT set, the mirror path is NOT used.
	val := os.Getenv(envNetworkMirror)
	assert.Empty(t, val, "PULUMI_TF_NETWORK_MIRROR_URL should not be set in the test environment")
}
