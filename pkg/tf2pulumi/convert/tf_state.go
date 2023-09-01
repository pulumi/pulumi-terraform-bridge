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

package convert

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/terraform/pkg/states/statefile"
)

func TranslateState(info il.ProviderInfoSource, path string) (*plugin.ConvertStateResponse, error) {
	stateFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	file, err := statefile.Read(stateFile)
	if err != nil {
		return nil, err
	}

	state := file.State
	var resources []plugin.ResourceImport
	for _, mod := range state.Modules {
		for _, resource := range mod.Resources {
			// TODO: Currently we just expect one instance
			instance := resource.Instances[nil]
			if instance.HasCurrent() {
				current := instance.Current

				// We assume AttrsJSON is set, this will be true for all recent tfstate files
				var obj map[string]interface{}
				err := json.Unmarshal(current.AttrsJSON, &obj)
				if err != nil {
					return nil, err
				}
				// We only care about the id value
				id, ok := obj["id"]
				if !ok {
					return nil, fmt.Errorf("failed to find id attribute in %s", resource.Addr.Resource)
				}
				// And we expect id to be a string
				idStr, ok := id.(string)
				if !ok {
					return nil, fmt.Errorf("id attribute for %s was not a string", resource.Addr.Resource)
				}

				// Try to grab the info for this resource type
				tfType := resource.Addr.Resource.Type
				provider := impliedProvider(tfType)
				providerInfo, err := info.GetProviderInfo("", "", provider, "")
				if err != nil {
					return nil, fmt.Errorf("failed to get provider info for %q: %v", tfType, err)
				}

				// Get the pulumi type of this resource
				pulumiType := impliedToken(tfType)
				if providerInfo != nil {
					pulumiType = providerInfo.Resources[tfType].Tok.String()
				}

				resources = append(resources, plugin.ResourceImport{
					Type: pulumiType,
					Name: resource.Addr.Resource.Name,
					ID:   idStr,
				})
			}
		}
	}

	return &plugin.ConvertStateResponse{
		Resources: resources,
	}, nil
}
