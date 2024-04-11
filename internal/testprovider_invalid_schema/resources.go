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

package tpinvschema

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func ProviderInfo() tfbridge.ProviderInfo {
	prov := tfbridge.ProviderInfo{
		P:    sdkv2.NewProvider(Provider()),
		Name: "tpinvschema",

		PreConfigureCallback: func(vars resource.PropertyMap, config shim.ResourceConfig) error {
			return nil
		},
		Resources: map[string]*tfbridge.ResourceInfo{
			"invalid_res": {
				Tok: tfbridge.MakeResource("tpinvschema", "index", "invalid_res"),
			},
		},
	}

	return prov
}
