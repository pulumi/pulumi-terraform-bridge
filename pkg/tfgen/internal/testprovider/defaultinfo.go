// Copyright 2016-2022, Pulumi Corporation.
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

package testprovider

import (
	testproviderdata "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func ProviderDefaultInfo() info.Provider {
	return info.Provider{
		P:           shimv2.NewProvider(testproviderdata.ProviderDefaultInfo()),
		Name:        "default-info",
		Description: "",
		Keywords:    []string{"pulumi", "random"},
		License:     "Apache-2.0",
		Homepage:    "https://pulumi.io",
		Repository:  "",
		Config: map[string]*info.Schema{
			"project": {
				Default: &info.Default{
					Value: []string{"default_project"},
				},
			},
		},
		Resources: map[string]*info.Resource{
			"default_ruleset": {
				Tok: tfbridge.MakeResource("cloudflare", "index", "Ruleset"),
				Fields: map[string]*info.Schema{
					"rules": {
						Elem: &info.Schema{
							Fields: map[string]*info.Schema{
								"id": {
									Default: &info.Default{
										Value: []string{"default_id"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
