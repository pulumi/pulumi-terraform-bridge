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
	"fmt"
	"os"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/pulumi/terraform/pkg/states/statefile"
	"github.com/zclconf/go-cty/cty"
)

func TranslateState(info il.ProviderInfoSource, path string) (*pulumirpc.ConvertStateResponse, error) {
	stateFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	file, err := statefile.Read(stateFile)
	if err != nil {
		return nil, err
	}

	state := file.State
	var resources []*pulumirpc.ResourceImport
	for _, mod := range state.Modules {
		for _, resource := range mod.Resources {
			// TODO: Currently we just expect one instance
			instance := resource.Instances[nil]
			if instance.HasCurrent() {
				current := instance.Current
				// We only care about the id value
				attrTypes := map[string]cty.Type{
					"id": cty.String,
				}
				typ := cty.Object(attrTypes)
				obj, err := current.Decode(typ)
				if err != nil {
					return nil, err
				}
				id := obj.Value.GetAttr("id")

				// Try to grab the info for this resource type
				tfType := resource.Addr.Resource.Type
				provider := impliedProvider(tfType)
				providerInfo, err := info.GetProviderInfo("", "", provider, "")
				if err != nil {
					return nil, fmt.Errorf("Failed to get provider info for %q: %v", tfType, err)
				}

				// Get the pulumi type of this resource
				pulumiType := impliedToken(tfType)
				if providerInfo != nil {
					pulumiType = providerInfo.Resources[tfType].Tok.String()
				}

				resources = append(resources, &pulumirpc.ResourceImport{
					Type: pulumiType,
					Name: resource.Addr.Resource.Name,
					Id:   id.AsString(),
				})
			}
		}
	}

	return &pulumirpc.ConvertStateResponse{
		Resources: resources,
	}, nil
}
