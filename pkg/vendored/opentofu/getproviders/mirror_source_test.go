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

package getproviders

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opentofu/svchost"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/addrs"
)

func newTestProvider(hostname, namespace, typ string) addrs.Provider {
	return addrs.Provider{
		Hostname:  svchost.Hostname(hostname),
		Namespace: namespace,
		Type:      typ,
	}
}

func TestMirrorSource_AvailableVersions(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/registry.terraform.io/hashicorp/random/index.json", func(w http.ResponseWriter, r *http.Request) {
		resp := mirrorVersionsResponse{
			Versions: map[string]json.RawMessage{
				"3.5.0": json.RawMessage(`{}`),
				"3.6.3": json.RawMessage(`{}`),
				"3.4.0": json.RawMessage(`{}`),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/registry.terraform.io/hashicorp/unknown/index.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx := context.Background()
	source, err := NewMirrorSource(ctx, server.URL, nil, LocationConfig{})
	require.NoError(t, err)

	t.Run("lists versions sorted", func(t *testing.T) {
		t.Parallel()
		provider := newTestProvider("registry.terraform.io", "hashicorp", "random")
		versions, warnings, err := source.AvailableVersions(ctx, provider)
		require.NoError(t, err)
		assert.Empty(t, warnings)
		require.Len(t, versions, 3)
		// Sorted by precedence (lowest first)
		assert.Equal(t, "3.4.0", versions[0].String())
		assert.Equal(t, "3.5.0", versions[1].String())
		assert.Equal(t, "3.6.3", versions[2].String())
	})

	t.Run("unknown provider returns ErrRegistryProviderNotKnown", func(t *testing.T) {
		t.Parallel()
		provider := newTestProvider("registry.terraform.io", "hashicorp", "unknown")
		_, _, err := source.AvailableVersions(ctx, provider)
		require.Error(t, err)
		var notKnown ErrRegistryProviderNotKnown
		assert.ErrorAs(t, err, &notKnown)
	})

	t.Run("ForDisplay", func(t *testing.T) {
		t.Parallel()
		provider := newTestProvider("registry.terraform.io", "hashicorp", "random")
		display := source.ForDisplay(provider)
		assert.Contains(t, display, "mirror")
	})
}

func TestMirrorSource_PackageMeta(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/registry.terraform.io/hashicorp/random/3.6.3.json", func(w http.ResponseWriter, r *http.Request) {
		resp := mirrorPackageResponse{
			Archives: map[string]mirrorArchive{
				"linux_amd64": {
					URL:    "terraform-provider-random_3.6.3_linux_amd64.zip",
					Hashes: []string{"zh:1234567890abcdef"},
				},
				"darwin_arm64": {
					URL: "terraform-provider-random_3.6.3_darwin_arm64.zip",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/registry.terraform.io/hashicorp/random/9.9.9.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx := context.Background()
	source, err := NewMirrorSource(ctx, server.URL, nil, LocationConfig{})
	require.NoError(t, err)

	provider := newTestProvider("registry.terraform.io", "hashicorp", "random")

	t.Run("returns package meta for available platform", func(t *testing.T) {
		t.Parallel()
		version := MustParseVersion("3.6.3")
		meta, err := source.PackageMeta(ctx, provider, version, Platform{OS: "linux", Arch: "amd64"})
		require.NoError(t, err)
		assert.Equal(t, provider, meta.Provider)
		assert.Equal(t, version, meta.Version)
		assert.Equal(t, Platform{OS: "linux", Arch: "amd64"}, meta.TargetPlatform)
		assert.Equal(t, "terraform-provider-random_3.6.3_linux_amd64.zip", meta.Filename)
		assert.Contains(t, meta.Location.String(), "terraform-provider-random_3.6.3_linux_amd64.zip")
	})

	t.Run("returns ErrPlatformNotSupported for unavailable platform", func(t *testing.T) {
		t.Parallel()
		version := MustParseVersion("3.6.3")
		_, err := source.PackageMeta(ctx, provider, version, Platform{OS: "windows", Arch: "386"})
		require.Error(t, err)
		var platformErr ErrPlatformNotSupported
		assert.ErrorAs(t, err, &platformErr)
	})

	t.Run("returns error for unknown version", func(t *testing.T) {
		t.Parallel()
		version := MustParseVersion("9.9.9")
		_, err := source.PackageMeta(ctx, provider, version, Platform{OS: "linux", Arch: "amd64"})
		require.Error(t, err)
	})
}

func TestMirrorSource_AbsoluteArchiveURL(t *testing.T) {
	t.Parallel()

	absoluteURL := "https://releases.example.com/terraform-provider-random_3.6.3_linux_amd64.zip"

	mux := http.NewServeMux()
	mux.HandleFunc("/registry.terraform.io/hashicorp/random/3.6.3.json", func(w http.ResponseWriter, r *http.Request) {
		resp := mirrorPackageResponse{
			Archives: map[string]mirrorArchive{
				"linux_amd64": {
					URL: absoluteURL,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx := context.Background()
	source, err := NewMirrorSource(ctx, server.URL, nil, LocationConfig{})
	require.NoError(t, err)

	provider := newTestProvider("registry.terraform.io", "hashicorp", "random")
	version := MustParseVersion("3.6.3")
	meta, err := source.PackageMeta(ctx, provider, version, Platform{OS: "linux", Arch: "amd64"})
	require.NoError(t, err)
	assert.Equal(t, absoluteURL, meta.Location.String())
}

func TestNewMirrorSource_Validation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("rejects non-http schemes", func(t *testing.T) {
		t.Parallel()
		_, err := NewMirrorSource(ctx, "ftp://mirror.example.com/", nil, LocationConfig{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "http or https")
	})

	t.Run("accepts http", func(t *testing.T) {
		t.Parallel()
		s, err := NewMirrorSource(ctx, "http://mirror.example.com/providers/", nil, LocationConfig{})
		require.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("accepts https", func(t *testing.T) {
		t.Parallel()
		s, err := NewMirrorSource(ctx, "https://mirror.example.com/providers/", nil, LocationConfig{})
		require.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("adds trailing slash if missing", func(t *testing.T) {
		t.Parallel()
		s, err := NewMirrorSource(ctx, "https://mirror.example.com/providers", nil, LocationConfig{})
		require.NoError(t, err)
		assert.Equal(t, "https://mirror.example.com/providers/", s.baseURL.String())
	})
}

func TestMirrorSource_AvailableVersions_Unauthorized(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/registry.terraform.io/hashicorp/random/index.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx := context.Background()
	source, err := NewMirrorSource(ctx, server.URL, nil, LocationConfig{})
	require.NoError(t, err)

	provider := newTestProvider("registry.terraform.io", "hashicorp", "random")
	_, _, err = source.AvailableVersions(ctx, provider)
	require.Error(t, err)
	var unauthorizedErr ErrUnauthorized
	assert.ErrorAs(t, err, &unauthorizedErr)
}
