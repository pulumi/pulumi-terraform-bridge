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

package tpsdkv2

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"

	sdkv2 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/sdk-v2"
)

func ProviderInfo() tfbridge.ProviderInfo {
	return tfbridge.ProviderInfo{
		P:    sdkv2.NewProvider(Provider()),
		Name: "tpsdkv2",
	}
}
