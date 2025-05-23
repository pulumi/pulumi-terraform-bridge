// Copyright 2016-2018, Pulumi Corporation.
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

package il

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// ProviderInfoSource abstracts the ability to fetch tfbridge information for a Terraform provider. This is abstracted
// primarily for testing purposes.
type ProviderInfoSource interface {
	// GetProviderInfo returns the tfbridge information for the indicated Terraform provider.
	GetProviderInfo(registry, namespace, name, version string) (*tfbridge.ProviderInfo, error)
}

// mapperProviderInfoSource wraps a convert.Mapper to return tfbridge.ProviderInfo
type mapperProviderInfoSource struct {
	mapper convert.Mapper
}

func NewMapperProviderInfoSource(mapper convert.Mapper) ProviderInfoSource {
	return &mapperProviderInfoSource{mapper: mapper}
}

func (mapper *mapperProviderInfoSource) GetProviderInfo(
	registryName, namespace, name, version string,
) (*tfbridge.ProviderInfo, error) {
	data, err := mapper.mapper.GetMapping(context.TODO(), name, &convert.MapperPackageHint{
		PluginName: GetPulumiProviderName(name),
	})
	if err != nil {
		return nil, err
	}
	// Might be nil or []
	if len(data) == 0 {
		message := fmt.Sprintf("could not find mapping information for provider %s", name)
		message += "; try installing a pulumi plugin that supports this terraform provider"
		return nil, errors.New(message)
	}

	var info *tfbridge.MarshallableProviderInfo
	err = json.Unmarshal(data, &info)
	if err != nil {
		return nil, fmt.Errorf("could not decode schema information for provider %s: %w", name, err)
	}
	return info.Unmarshal(), nil
}

// CachingProviderInfoSource wraps a ProviderInfoSource in a cache for faster access.
type CachingProviderInfoSource struct {
	m sync.RWMutex

	source  ProviderInfoSource
	entries map[string]*tfbridge.ProviderInfo
}

func (cache *CachingProviderInfoSource) cacheKey(registry, namespace, name, version string) string {
	return fmt.Sprintf("%s/%s/%s@%s",
		url.PathEscape(registry), url.PathEscape(namespace), url.PathEscape(name), url.PathEscape(version))
}

func (cache *CachingProviderInfoSource) getProviderInfo(key string) (*tfbridge.ProviderInfo, bool) {
	cache.m.RLock()
	defer cache.m.RUnlock()

	info, ok := cache.entries[key]
	return info, ok
}

// GetProviderInfo returns the tfbridge information for the indicated Terraform provider as well as the name of the
// corresponding Pulumi resource provider.
func (cache *CachingProviderInfoSource) GetProviderInfo(
	registryName, namespace, name, version string,
) (*tfbridge.ProviderInfo, error) {
	key := cache.cacheKey(registryName, namespace, name, version)

	if info, ok := cache.getProviderInfo(key); ok {
		return info, nil
	}

	cache.m.Lock()
	defer cache.m.Unlock()

	info, err := cache.source.GetProviderInfo(registryName, namespace, name, version)
	if err != nil {
		return nil, err
	}
	cache.entries[key] = info
	return info, nil
}

// NewCachingProviderInfoSource creates a new CachingProviderInfoSource that wraps the given ProviderInfoSource.
func NewCachingProviderInfoSource(source ProviderInfoSource) *CachingProviderInfoSource {
	return &CachingProviderInfoSource{
		source:  source,
		entries: map[string]*tfbridge.ProviderInfo{},
	}
}

type multiProviderInfoSource []ProviderInfoSource

func NewMultiProviderInfoSource(sources ...ProviderInfoSource) ProviderInfoSource {
	return multiProviderInfoSource(sources)
}

func (s multiProviderInfoSource) GetProviderInfo(
	registryName, namespace, name, version string,
) (*tfbridge.ProviderInfo, error) {
	for _, s := range s {
		if s != nil {
			if info, err := s.GetProviderInfo(registryName, namespace, name, version); err == nil && info != nil {
				return info, nil
			}
		}
	}

	return nil, getMissingPluginError(name)
}

type pluginProviderInfoSource struct{}

// PluginProviderInfoSource is the ProviderInfoSource that retrieves tfbridge information by loading and interrogating
// the Pulumi resource provider that corresponds to a Terraform provider.
var PluginProviderInfoSource = ProviderInfoSource(pluginProviderInfoSource{})

var pulumiNames = map[string]string{
	"azurerm":  "azure",
	"bigip":    "f5bigip",
	"google":   "gcp",
	"template": "terraform-template",
}

// HasPulumiProviderName returns true if the given Terraform provider has a corresponding Pulumi provider name.
func HasPulumiProviderName(terraformProviderName string) bool {
	_, hasPulumiName := pulumiNames[terraformProviderName]
	return hasPulumiName
}

// GetPulumiProviderName returns the Pulumi name for the given Terraform provider. In most cases the two names will be
// identical.
func GetPulumiProviderName(terraformProviderName string) string {
	if pulumiName, hasPulumiName := pulumiNames[terraformProviderName]; hasPulumiName {
		return pulumiName
	}
	return terraformProviderName
}

// GetTerraformProviderName returns the canonical Terraform provider name for the given provider info.
func GetTerraformProviderName(info tfbridge.ProviderInfo) string {
	if info.Name == "google-beta" {
		return "google"
	}
	return info.Name
}

// GetProviderInfo returns the tfbridge information for the indicated Terraform provider as well as the name of the
// corresponding Pulumi resource provider.
func (pluginProviderInfoSource) GetProviderInfo(
	registryName, namespace, name, version string,
) (*tfbridge.ProviderInfo, error) {
	tfProviderName := name
	pluginName := GetPulumiProviderName(tfProviderName)

	diag := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
		Color: colors.Never,
	})
	ctx := context.Background()
	path, err := workspace.GetPluginPath(ctx, diag, workspace.PluginSpec{
		Kind: apitype.ResourcePlugin,
		Name: pluginName,
	}, nil)
	if err != nil {
		return nil, err
	} else if path == "" {
		return nil, getMissingPluginError(name)
	}

	// Run the plugin and decode its provider config.
	//nolint:gas
	cmd := exec.Command(path, "-get-provider-info")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load plugin %s for provider %s", pluginName, tfProviderName)
	}
	if err = cmd.Start(); err != nil {
		return nil, errors.Wrapf(err, "failed to load plugin %s for provider %s", pluginName, tfProviderName)
	}

	var info *tfbridge.MarshallableProviderInfo
	err = jsoniter.NewDecoder(out).Decode(&info)

	if cErr := cmd.Wait(); cErr != nil {
		return nil, errors.Wrapf(err, "failed to run plugin %s for provider %s", pluginName, tfProviderName)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "could not decode schema information for provider %s", tfProviderName)
	}

	return info.Unmarshal(), nil
}

// getMissingPluginError returns an error that informs the user that a plugin for a Terraform provider cannot be found,
// and how to go about acquiring it if it is hosted on Pulumi.com.
func getMissingPluginError(providerName string) error {
	pluginName := GetPulumiProviderName(providerName)

	message := fmt.Sprintf("could not find plugin %s for provider %s", pluginName, providerName)
	latest := getLatestPluginVersion(pluginName)
	if latest != "" {
		message += fmt.Sprintf("; try running 'pulumi plugin install resource %s %s'", pluginName, latest)
	}
	return errors.New(message)
}

// getLatestPluginVersion returns the version number for the latest released version of the indicated plugin by
// querying the value of the `latest` tag in the plugin's corresponding NPM package.
func getLatestPluginVersion(pluginName string) string {
	resp, err := http.Get("https://registry.npmjs.org/@pulumi/" + pluginName)
	if err != nil {
		return ""
	}
	defer contract.IgnoreClose(resp.Body)

	// The structure of the response to the above call is documented here:
	// - https://github.com/npm/registry/blob/master/docs/responses/package-metadata.md
	var packument struct {
		DistTags map[string]string `json:"dist-tags"`
	}
	if err = jsoniter.NewDecoder(resp.Body).Decode(&packument); err != nil {
		return ""
	}
	return packument.DistTags["latest"]
}
