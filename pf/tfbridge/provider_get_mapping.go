// Copyright 2016-2023, Pulumi Corporation.
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

package tfbridge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func (p *provider) GetMappingWithContext(ctx context.Context, key, provider string) ([]byte, string, error) {
	// Code and comments follow Provider.GetMapping in pkg/tfbridge/provider.go

	mapped := p.info.ResourcePrefix
	if mapped == "" {
		mapped = p.info.Name
	}

	// The prototype converter uses the key "tf", but the new plugin converter uses "terraform". For now support
	// both, eventually we can remove the "tf" key.
	if key == "tf" || key == "terraform" {

		// The provider key should either be empty (old engines) or the name of the provider we support (new engines)
		if provider != "" && provider != mapped {
			return nil, "", fmt.Errorf("unknown provider %q", provider)
		}

		info := p.marshalProviderInfo(ctx)
		mapping, err := json.Marshal(info)
		if err != nil {
			return nil, "", err
		}
		return mapping, mapped, nil
	}

	// An empty response is valid for GetMapping, it means we don't have a mapping for the given key
	return []byte{}, "", nil
}

func (p *provider) marshalProviderInfo(ctx context.Context) *tfbridge.MarshallableProviderInfo {
	var providerInfoCopy tfbridge.ProviderInfo = p.info

	return tfbridge.MarshalProviderInfo(&providerInfoCopy)
}

func (p *provider) GetMappingsWithContext(ctx context.Context, key string) ([]string, error) {
	// Code and comments follow Provider.GetMapping in pkg/tfbridge/provider.go
	if key == "tf" || key == "terraform" {
		mapped := p.info.ResourcePrefix
		if mapped == "" {
			mapped = p.info.Name
		}
		return []string{mapped}, nil
	}
	// An empty response is valid for GetMappings, it means we don't have a mapping for the given key
	return nil, nil
}
