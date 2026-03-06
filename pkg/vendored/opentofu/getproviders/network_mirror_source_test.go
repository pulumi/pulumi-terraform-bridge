// Copyright 2016-2025, Pulumi Corporation.
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

	regaddr "github.com/opentofu/registry-address/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustProvider(source string) regaddr.Provider {
	p, err := regaddr.ParseProviderSource(source)
	if err != nil {
		panic(err)
	}
	return p
}

func TestNetworkMirrorSource_AvailableVersions(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry.terraform.io/hashicorp/random/index.json":
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"versions": map[string]interface{}{
					"3.5.1": map[string]interface{}{},
					"3.6.0": map[string]interface{}{},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	provider := mustProvider("registry.terraform.io/hashicorp/random")

	ctx := context.Background()
	source := NewNetworkMirrorSource(ctx, srv.URL, nil)
	versions, warnings, err := source.AvailableVersions(ctx, provider)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, versions, 2)
	assert.Equal(t, "3.5.1", versions[0].String())
	assert.Equal(t, "3.6.0", versions[1].String())
}

func TestNetworkMirrorSource_PackageMeta(t *testing.T) {
	t.Parallel()

	archiveSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake zip content")) //nolint:errcheck
	}))
	defer archiveSrv.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry.terraform.io/hashicorp/random/3.5.1.json":
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"archives": map[string]interface{}{
					"linux_amd64": map[string]interface{}{
						"url":    archiveSrv.URL + "/terraform-provider-random_3.5.1_linux_amd64.zip",
						"hashes": []string{},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	provider := mustProvider("registry.terraform.io/hashicorp/random")
	version := MustParseVersion("3.5.1")
	platform := Platform{OS: "linux", Arch: "amd64"}

	ctx := context.Background()
	source := NewNetworkMirrorSource(ctx, srv.URL, nil)
	meta, err := source.PackageMeta(ctx, provider, version, platform)
	require.NoError(t, err)
	assert.Equal(t, provider, meta.Provider)
	assert.Equal(t, version, meta.Version)
	assert.Equal(t, platform, meta.TargetPlatform)

	httpLoc, ok := meta.Location.(PackageHTTPURL)
	require.True(t, ok, "expected PackageHTTPURL location")
	assert.Equal(t, archiveSrv.URL+"/terraform-provider-random_3.5.1_linux_amd64.zip", httpLoc.URL)
}

func TestNetworkMirrorSource_PackageMeta_PlatformNotSupported(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"archives": map[string]interface{}{
				"linux_amd64": map[string]interface{}{
					"url": "https://example.com/provider.zip",
				},
			},
		})
	}))
	defer srv.Close()

	provider := mustProvider("registry.terraform.io/hashicorp/random")
	version := MustParseVersion("3.5.1")
	platform := Platform{OS: "windows", Arch: "amd64"}

	ctx := context.Background()
	source := NewNetworkMirrorSource(ctx, srv.URL, nil)
	_, err := source.PackageMeta(ctx, provider, version, platform)
	require.Error(t, err)
	var notSupported ErrPlatformNotSupported
	require.ErrorAs(t, err, &notSupported)
}

func TestNetworkMirrorSource_RelativeURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry.terraform.io/hashicorp/random/3.5.1.json":
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"archives": map[string]interface{}{
					"linux_amd64": map[string]interface{}{
						"url":    "../files/terraform-provider-random_3.5.1_linux_amd64.zip",
						"hashes": []string{},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	provider := mustProvider("registry.terraform.io/hashicorp/random")
	version := MustParseVersion("3.5.1")
	platform := Platform{OS: "linux", Arch: "amd64"}

	ctx := context.Background()
	source := NewNetworkMirrorSource(ctx, srv.URL, nil)
	meta, err := source.PackageMeta(ctx, provider, version, platform)
	require.NoError(t, err)

	httpLoc, ok := meta.Location.(PackageHTTPURL)
	require.True(t, ok)
	// Relative URL should be resolved against the version JSON URL.
	assert.Contains(t, httpLoc.URL, "terraform-provider-random_3.5.1_linux_amd64.zip")
}
