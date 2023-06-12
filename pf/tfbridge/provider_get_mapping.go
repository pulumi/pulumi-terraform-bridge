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

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func (p *provider) GetMappingWithContext(ctx context.Context, key string) ([]byte, string, error) {
	// Code and comments follow Provider.GetMapping in pkg/tfbridge/provider.go

	// The prototype converter uses the key "tf", but the new plugin converter uses "terraform". For now support
	// both, eventually we can remove the "tf" key.
	if key == "tf" || key == "terraform" {
		info := p.marshalProviderInfo(ctx)
		mapping, err := json.Marshal(info)
		if err != nil {
			return nil, "", err
		}
		mapped := p.info.ResourcePrefix
		if mapped == "" {
			mapped = p.info.Name
		}
		return mapping, mapped, nil
	}

	// An empty response is valid for GetMapping, it means we don't have a mapping for the given key
	return []byte{}, "", nil
}

func (p *provider) marshalProviderInfo(ctx context.Context) *tfbridge.MarshallableProviderInfo {
	var providerInfoCopy tfbridge.ProviderInfo = p.info.ProviderInfo
	providerInfoCopy.P = schemashim.ShimSchemaOnlyProvider(ctx, p.tfProvider)
	return tfbridge.MarshalProviderInfo(&providerInfoCopy)
}
