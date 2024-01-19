// Copyright 2016-2024, Pulumi Corporation.
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
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

func extractID(
	ctx context.Context,
	resname string,
	info *tfbridge.ResourceInfo,
	state resource.PropertyMap,
) (resource.ID, error) {
	if info != nil && info.ComputeID != nil {
		return info.ComputeID(ctx, state)
	}

	errSuffix := fmt.Sprintf("This is an error in the provider. Special identity handling may "+
		"be needed. Consider setting ResourceInfo.ComputeID for the %s resource", resname)

	idValue, gotID := state["id"]
	contract.Assertf(gotID, "Resource state did not contain an 'id' property. %s", errSuffix)

	secret := idValue.ContainsSecrets()
	contract.Assertf(!secret, "Cannot support secrets in 'id' property. %s", errSuffix)

	computed := idValue.IsComputed()
	contract.Assertf(!computed, "Unexpected computed PropertyValue in state. %s", errSuffix)

	contract.Assertf(idValue.IsString(),
		"Resource state 'id' property expected to be a string but %v was given. %s",
		idValue, errSuffix)

	return resource.ID(idValue.StringValue()), nil
}
