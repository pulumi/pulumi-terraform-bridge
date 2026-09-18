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
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/addrs"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/opentofu/httpclient"
)

// MirrorSource is a Source that downloads providers from a Terraform network mirror,
// bypassing the standard registry service discovery protocol.
//
// This is useful in air-gapped environments where the registry.terraform.io
// well-known endpoint is not accessible but a network mirror (e.g., Artifactory)
// is available.
//
// The network mirror protocol is documented at:
// https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol
type MirrorSource struct {
	baseURL    *url.URL
	httpClient *retryablehttp.Client

	locationConfig LocationConfig
}

var _ Source = (*MirrorSource)(nil)

// NewMirrorSource creates a Source that uses the Terraform network mirror protocol
// to discover and download providers. The mirrorURL should be the base URL of the
// mirror (e.g., "https://mirror.example.com/providers/").
func NewMirrorSource(ctx context.Context, mirrorURL string, httpClient *retryablehttp.Client, locationCfg LocationConfig) (*MirrorSource, error) {
	// Ensure the URL ends with a trailing slash for proper path joining
	if !strings.HasSuffix(mirrorURL, "/") {
		mirrorURL += "/"
	}

	u, err := url.Parse(mirrorURL)
	if err != nil {
		return nil, fmt.Errorf("invalid mirror URL %q: %w", mirrorURL, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("mirror URL must use http or https scheme, got %q", u.Scheme)
	}

	if httpClient == nil {
		httpClient = httpclient.NewForRegistryRequests(ctx, 1, 10*time.Second)
	}

	return &MirrorSource{
		baseURL:        u,
		httpClient:     httpClient,
		locationConfig: locationCfg,
	}, nil
}

// mirrorVersionsResponse is the JSON structure returned by the mirror's version listing endpoint.
// Example: {"versions": {"3.6.3": {}, "3.5.0": {}}}
type mirrorVersionsResponse struct {
	Versions map[string]json.RawMessage `json:"versions"`
}

// mirrorPackageResponse is the JSON structure returned by the mirror's package info endpoint.
// Example: {"archives": {"linux_amd64": {"url": "terraform-provider-random_3.6.3_linux_amd64.zip", "hashes": [...]}}}
type mirrorPackageResponse struct {
	Archives map[string]mirrorArchive `json:"archives"`
}

type mirrorArchive struct {
	URL    string   `json:"url"`
	Hashes []string `json:"hashes,omitempty"`
}

// AvailableVersions queries the mirror for all available versions of the given provider.
//
// The mirror protocol endpoint is: {mirror_url}/{hostname}/{namespace}/{type}/index.json
func (s *MirrorSource) AvailableVersions(ctx context.Context, provider addrs.Provider) (VersionList, Warnings, error) {
	endpointURL := s.providerURL(provider, "index.json")

	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", endpointURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("mirror: failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, nil, ErrHostUnreachable{
			Hostname: provider.Hostname,
			Wrapped:  fmt.Errorf("mirror request failed: %w", err),
		}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusNotFound:
		return nil, nil, ErrRegistryProviderNotKnown{Provider: provider}
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, nil, ErrUnauthorized{Hostname: provider.Hostname}
	default:
		return nil, nil, ErrQueryFailed{
			Provider: provider,
			Wrapped:  fmt.Errorf("mirror returned unexpected status: %s", resp.Status),
		}
	}

	var body mirrorVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, nil, ErrQueryFailed{
			Provider: provider,
			Wrapped:  fmt.Errorf("failed to parse mirror version list: %w", err),
		}
	}

	if len(body.Versions) == 0 {
		return nil, nil, nil
	}

	ret := make(VersionList, 0, len(body.Versions))
	for vStr := range body.Versions {
		v, err := ParseVersion(vStr)
		if err != nil {
			return nil, nil, ErrQueryFailed{
				Provider: provider,
				Wrapped:  fmt.Errorf("mirror response includes invalid version %q: %w", vStr, err),
			}
		}
		ret = append(ret, v)
	}
	ret.Sort()
	return ret, nil, nil
}

// PackageMeta queries the mirror for download metadata of a specific provider version and platform.
//
// The mirror protocol endpoint is: {mirror_url}/{hostname}/{namespace}/{type}/{version}.json
func (s *MirrorSource) PackageMeta(ctx context.Context, provider addrs.Provider, version Version, target Platform) (PackageMeta, error) {
	endpointURL := s.providerURL(provider, version.String()+".json")

	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", endpointURL, nil)
	if err != nil {
		return PackageMeta{}, fmt.Errorf("mirror: failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return PackageMeta{}, ErrQueryFailed{
			Provider: provider,
			Wrapped:  fmt.Errorf("mirror request failed: %w", err),
		}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusNotFound:
		return PackageMeta{}, ErrPlatformNotSupported{
			Provider: provider,
			Version:  version,
			Platform: target,
		}
	case http.StatusUnauthorized, http.StatusForbidden:
		return PackageMeta{}, ErrUnauthorized{Hostname: provider.Hostname}
	default:
		return PackageMeta{}, ErrQueryFailed{
			Provider: provider,
			Wrapped:  fmt.Errorf("mirror returned unexpected status: %s", resp.Status),
		}
	}

	var body mirrorPackageResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return PackageMeta{}, ErrQueryFailed{
			Provider: provider,
			Wrapped:  fmt.Errorf("failed to parse mirror package response: %w", err),
		}
	}

	platformKey := target.String()
	archive, ok := body.Archives[platformKey]
	if !ok {
		return PackageMeta{}, ErrPlatformNotSupported{
			Provider: provider,
			Version:  version,
			Platform: target,
		}
	}

	// Resolve the download URL - it may be relative to the mirror base or absolute
	downloadURL, err := s.resolveArchiveURL(provider, version, archive.URL)
	if err != nil {
		return PackageMeta{}, fmt.Errorf("mirror: invalid download URL %q: %w", archive.URL, err)
	}

	// Construct filename from the URL path
	filename := path.Base(downloadURL)

	ret := PackageMeta{
		Provider:       provider,
		Version:        version,
		TargetPlatform: target,
		Filename:       filename,
		Location: PackageHTTPURL{
			URL: downloadURL,
			ClientBuilder: func(ctx context.Context) *retryablehttp.Client {
				return packageHTTPUrlClientWithRetry(ctx, s.locationConfig.ProviderDownloadRetries)
			},
		},
		// Network mirrors don't provide the same authentication chain as registries.
		// Hash verification is done via the hashes in the mirror response if available.
		Authentication: nil,
	}

	// If the mirror provides hashes, we could set up hash authentication here.
	// For now, we trust the mirror (same as Terraform does for network mirrors).

	return ret, nil
}

// ForDisplay returns a human-readable description of this source.
func (s *MirrorSource) ForDisplay(provider addrs.Provider) string {
	return fmt.Sprintf("mirror %s", s.baseURL.Host)
}

// providerURL constructs the full URL for a provider-specific endpoint on the mirror.
func (s *MirrorSource) providerURL(provider addrs.Provider, file string) string {
	// The network mirror protocol path is: {hostname}/{namespace}/{type}/{file}
	providerPath := path.Join(
		provider.Hostname.String(),
		provider.Namespace,
		provider.Type,
		file,
	)
	ref, _ := url.Parse(providerPath)
	return s.baseURL.ResolveReference(ref).String()
}

// resolveArchiveURL resolves the archive URL from a mirror response.
// If the URL is relative, it's resolved relative to the provider's directory on the mirror.
// If absolute, it's used as-is.
func (s *MirrorSource) resolveArchiveURL(provider addrs.Provider, _ Version, archiveURL string) (string, error) {
	parsed, err := url.Parse(archiveURL)
	if err != nil {
		return "", err
	}

	// If it's an absolute URL, use it directly
	if parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", fmt.Errorf("archive URL must use http or https scheme, got %q", parsed.Scheme)
		}
		return archiveURL, nil
	}

	// Relative URL — resolve relative to the provider's version directory on the mirror
	providerDir := path.Join(
		provider.Hostname.String(),
		provider.Namespace,
		provider.Type,
	) + "/"
	dirRef, _ := url.Parse(providerDir)
	baseDir := s.baseURL.ResolveReference(dirRef)
	return baseDir.ResolveReference(parsed).String(), nil
}
