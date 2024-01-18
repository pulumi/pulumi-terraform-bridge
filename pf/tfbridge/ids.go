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
	idValue, ok := state["id"]
	c := fmt.Sprintf(". If special identity handling is needed, consider customizing "+
		"ResourceInfo.ComputeID for the %s resource", resname)
	if !ok {
		return "", fmt.Errorf("Resource state did not contain an 'id' property" + c)
	}
	if !idValue.IsString() {
		return "", fmt.Errorf("Resource state 'id' property expected to be a string but "+
			"%v was given"+c, idValue)
	}
	return resource.ID(idValue.StringValue()), nil
}
