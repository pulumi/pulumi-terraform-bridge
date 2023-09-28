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
	"math/big"
	"os"
	"strings"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/il"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/terraform/pkg/addrs"
	"github.com/pulumi/terraform/pkg/states/statefile"
)

// Looks up a given attribute and returns it as a string. If the attribute is not found, or is not a string, an error is
// returned.
func getString(addr addrs.Resource, obj map[string]interface{}, key string) (string, error) {
	attr, ok := obj[key]
	if !ok {
		return "", fmt.Errorf("failed to find %s attribute in %s", key, addr)
	}
	str, ok := attr.(string)
	if !ok {
		return "", fmt.Errorf("%s attribute for %s was not a string", key, addr)
	}
	return str, nil
}

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
			// We only care about managed resources, we can't import data sources
			if resource.Addr.Resource.Mode != addrs.ManagedResourceMode {
				continue
			}

			// TODO: Currently we just expect one instance
			for instanceAddr, instance := range resource.Instances {
				if instance.HasCurrent() {
					current := instance.Current

					// We assume AttrsJSON is set, this will be true for all recent tfstate files
					var obj map[string]interface{}
					err := json.Unmarshal(current.AttrsJSON, &obj)
					if err != nil {
						return nil, err
					}
					var id string
					// Most resources can be imported by passing their `id`, but a few need to be imported using some
					// other property of the resource.  This table includes any of these exceptions.  If you get errors
					// or warnings about resources not being able to be found or the format of resource ids being
					// incorrect, add a mapping here that constructs the correct id format based on the property values
					// in the Terraform state file.
					//
					// TODO(https://github.com/pulumi/pulumi-terraform-bridge/issues/1406): This table should somehow be
					// expressed via the mapping file, rather than hardcoding for each provider here.
					switch resource.Addr.Resource.Type {
					case "aws_ecs_cluster":
						id, err = getString(resource.Addr.Resource, obj, "name")
						if err != nil {
							return nil, err
						}
					case "aws_ecs_service":
						cluster, err := getString(resource.Addr.Resource, obj, "cluster")
						if err != nil {
							return nil, err
						}
						name, err := getString(resource.Addr.Resource, obj, "name")
						if err != nil {
							return nil, err
						}

						parts := strings.Split(cluster, "/")
						id = fmt.Sprintf("%s/%s", parts[len(parts)-1], name)
					case "aws_ecs_task_definition":
						id, err = getString(resource.Addr.Resource, obj, "arn")
						if err != nil {
							return nil, err
						}
					case "aws_route":
						routeTable, err := getString(resource.Addr.Resource, obj, "route_table_id")
						if err != nil {
							return nil, err
						}
						destinationCidr, err := getString(resource.Addr.Resource, obj, "destination_cidr_block")
						if err != nil {
							return nil, err
						}

						id = fmt.Sprintf("%s_%s", routeTable, destinationCidr)
					case "aws_route_table_association":
						subnet, err := getString(resource.Addr.Resource, obj, "subnet_id")
						if err != nil {
							return nil, err
						}
						routeTable, err := getString(resource.Addr.Resource, obj, "route_table_id")
						if err != nil {
							return nil, err
						}

						id = fmt.Sprintf("%s/%s", subnet, routeTable)
					case "aws_iam_role_policy_attachment":
						role, err := getString(resource.Addr.Resource, obj, "role")
						if err != nil {
							return nil, err
						}
						policy, err := getString(resource.Addr.Resource, obj, "policy_arn")
						if err != nil {
							return nil, err
						}

						id = fmt.Sprintf("%s/%s", role, policy)
					default:
						// We only care about the id value
						id, err = getString(resource.Addr.Resource, obj, "id")
						if err != nil {
							return nil, err
						}
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
						resourceInfo := providerInfo.Resources[tfType]
						if resourceInfo != nil {
							pulumiType = resourceInfo.Tok.String()
						} else {
							return nil, fmt.Errorf("failed to get resource info for %q", tfType)
						}
					}

					// Add a suffix to the name if there is more than one instance
					name := resource.Addr.Resource.Name
					switch instanceAddr.(type) {
					case addrs.IntKey:
						flt := instanceAddr.Value().AsBigFloat()
						i, a := flt.Int64()
						contract.Assertf(a == big.Exact, "expected exact conversion to int64")
						name = fmt.Sprintf("%s-%d", name, i)
					case addrs.StringKey:
						name = fmt.Sprintf("%s-%s", name, instanceAddr.Value().AsString())
					}

					resources = append(resources, plugin.ResourceImport{
						Type: pulumiType,
						Name: name,
						ID:   id,
					})
				}
			}
		}
	}

	return &plugin.ConvertStateResponse{
		Resources: resources,
	}, nil
}
