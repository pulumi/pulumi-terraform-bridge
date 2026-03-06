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
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/addrs"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/httpclient"
)

// NetworkMirrorSource implements the Terraform Network Mirror Protocol.
// See: https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol
type NetworkMirrorSource struct {
	baseURL    string
	httpClient *retryablehttp.Client
}

var _ Source = (*NetworkMirrorSource)(nil)

// NewNetworkMirrorSource creates a NetworkMirrorSource that fetches providers from the
// given mirror base URL (e.g. "https://mirror.example.com/providers").
func NewNetworkMirrorSource(ctx context.Context, baseURL string, httpClient *retryablehttp.Client) *NetworkMirrorSource {
	if httpClient == nil {
		httpClient = httpclient.NewForRegistryRequests(ctx, 1, 10*time.Second)
	}
	return &NetworkMirrorSource{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

// AvailableVersions queries the mirror for all available versions of the given provider.
// Endpoint: GET {baseURL}/{hostname}/{namespace}/{type}/index.json
// Response: {"versions": {"1.0.0": {}, "1.2.3": {}}}
func (s *NetworkMirrorSource) AvailableVersions(ctx context.Context, provider addrs.Provider) (VersionList, Warnings, error) {
	indexURL := fmt.Sprintf("%s/%s/%s/%s/index.json",
		s.baseURL,
		provider.Hostname.ForDisplay(),
		provider.Namespace,
		provider.Type,
	)

	var result struct {
		Versions map[string]json.RawMessage `json:"versions"`
	}
	if err := s.getJSON(ctx, indexURL, &result); err != nil {
		return nil, nil, ErrQueryFailed{Provider: provider, Wrapped: err}
	}

	versions := make(VersionList, 0, len(result.Versions))
	for vstr := range result.Versions {
		v, err := ParseVersion(vstr)
		if err != nil {
			return nil, nil, ErrQueryFailed{
				Provider: provider,
				Wrapped:  fmt.Errorf("mirror response includes invalid version string %q: %w", vstr, err),
			}
		}
		versions = append(versions, v)
	}
	versions.Sort()
	return versions, nil, nil
}

// PackageMeta queries the mirror for metadata about a specific provider version and platform.
// Endpoint: GET {baseURL}/{hostname}/{namespace}/{type}/{version}.json
// Response: {"archives": {"linux_amd64": {"url": "...", "hashes": ["zh:..."]}}}
func (s *NetworkMirrorSource) PackageMeta(ctx context.Context, provider addrs.Provider, version Version, target Platform) (PackageMeta, error) {
	versionURL := fmt.Sprintf("%s/%s/%s/%s/%s.json",
		s.baseURL,
		provider.Hostname.ForDisplay(),
		provider.Namespace,
		provider.Type,
		version,
	)

	var result struct {
		Archives map[string]struct {
			URL    string   `json:"url"`
			Hashes []string `json:"hashes"`
		} `json:"archives"`
	}
	if err := s.getJSON(ctx, versionURL, &result); err != nil {
		return PackageMeta{}, ErrQueryFailed{Provider: provider, Wrapped: err}
	}

	platformKey := target.String()
	archive, ok := result.Archives[platformKey]
	if !ok {
		return PackageMeta{}, ErrPlatformNotSupported{
			Provider: provider,
			Version:  version,
			Platform: target,
		}
	}

	// The archive URL may be relative; resolve it against the version JSON URL.
	pkgURL, err := resolveURL(versionURL, archive.URL)
	if err != nil {
		return PackageMeta{}, ErrQueryFailed{
			Provider: provider,
			Wrapped:  fmt.Errorf("invalid archive URL %q from mirror: %w", archive.URL, err),
		}
	}

	httpClient := s.httpClient
	meta := PackageMeta{
		Provider:       provider,
		Version:        version,
		TargetPlatform: target,
		Location: PackageHTTPURL{
			URL: pkgURL,
			ClientBuilder: func(ctx context.Context) *retryablehttp.Client {
				return httpClient
			},
		},
	}

	if len(archive.Hashes) > 0 {
		hashes := make([]Hash, 0, len(archive.Hashes))
		for _, h := range archive.Hashes {
			hash, err := ParseHash(h)
			if err != nil {
				// Non-fatal: skip unrecognized hash formats.
				continue
			}
			hashes = append(hashes, hash)
		}
		if len(hashes) > 0 {
			meta.Authentication = NewPackageHashAuthentication(target, hashes)
		}
	}

	return meta, nil
}

func (s *NetworkMirrorSource) ForDisplay(provider addrs.Provider) string {
	return fmt.Sprintf("network mirror %s", s.baseURL)
}

func (s *NetworkMirrorSource) getJSON(ctx context.Context, rawURL string, out interface{}) error {
	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return fmt.Errorf("invalid request URL %q: %w", rawURL, err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unsuccessful request to %s: %s", rawURL, resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("invalid JSON response from %s: %w", rawURL, err)
	}
	return nil
}

func resolveURL(base, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("empty URL")
	}
	// If ref already has a scheme, it's absolute.
	if strings.Contains(ref, "://") {
		return ref, nil
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(refURL).String(), nil
}
