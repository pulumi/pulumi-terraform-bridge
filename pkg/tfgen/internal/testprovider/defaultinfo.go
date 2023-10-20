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
	"unicode"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shimv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	testproviderdata "github.com/pulumi/pulumi-terraform-bridge/v3/internal/testprovider"
)

func ProviderDefaultInfo() tfbridge.ProviderInfo {

	member := func(mod string, mem string) tokens.ModuleMember {
		return tokens.ModuleMember("cloudflare" + ":" + mod + ":" + mem)
	}

	typ := func(mod string, typ string) tokens.Type {
		return tokens.Type(member(mod, typ))
	}

	resource := func(mod string, res string) tokens.Type {
		fn := string(unicode.ToLower(rune(res[0]))) + res[1:]
		return typ(mod+"/"+fn, res)
	}

	return tfbridge.ProviderInfo{
		P:           shimv2.NewProvider(testproviderdata.ProviderDefaultInfo()),
		Name:        "default-info",
		Description: "",
		Keywords:    []string{"pulumi", "random"},
		License:     "Apache-2.0",
		Homepage:    "https://pulumi.io",
		Repository:  "",
		Config: map[string]*tfbridge.SchemaInfo{
			"project": {
				Default: &tfbridge.DefaultInfo{
					Value: []string{"default_project"},
				},
			},
		},
		Resources: map[string]*tfbridge.ResourceInfo{
			"default_ruleset": {
				Tok: resource("index", "Ruleset"),
				Fields: map[string]*tfbridge.SchemaInfo{
					"rules": {
						Elem: &tfbridge.SchemaInfo{
							Fields: map[string]*tfbridge.SchemaInfo{
								"id": {
									Default: &tfbridge.DefaultInfo{
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
