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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Compensate for a TF problem causing spurious diffs between null and empty collections highlighted as dirty refresh in
// Pulumi. When applying resource changes TF will call normalizeNullValues with apply=true which may erase the
// distinction between empty and null collections. This makes state returned from Read differ from the state returned by
// Create and Update and written to the state store. While the problem is present in both TF and Pulumi bridged
// providers, it is worse in Pulumi because refresh is emitting a warning that something is changing.
//
// To compensate for this problem, this method corrects the Pulumi Read result during `pulumi refresh` to also erase
// empty collections from the Read result if the old input is missing or null.
//
// This currently only handles top-level properties.
//
// See: https://github.com/pulumi/terraform-plugin-sdk/blob/upstream-v2.33.0/helper/schema/grpc_provider.go#L1514
func normalizeNullValues(
	ctx context.Context,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*info.Schema,
	oldInputs, inputs resource.PropertyMap,
) resource.PropertyMap {
	if oldInputs == nil {
		return inputs
	}
	copy := inputs.Copy()
	for _, k := range inputs.StableKeys() {
		v := inputs[k]
		oldInput, gotOldInput := oldInputs[k]
		if gotOldInput && !oldInput.IsNull() {
			continue
		}
		if !(v.IsArray() && len(v.ArrayValue()) == 0) {
			continue
		}
		tfName := PulumiToTerraformName(string(k), schemaMap, schemaInfos)
		schema, ok := schemaMap.GetOk(tfName)
		if !ok {
			continue
		}
		t := schema.Type()
		isCollection := t == shim.TypeList || t == shim.TypeMap || t == shim.TypeSet
		if !isCollection || IsMaxItemsOne(schema, schemaInfos[tfName]) {
			continue
		}
		// An empty collection (not MaxItems=1) with missing/null oldInput is getting replaced to match.
		if gotOldInput {
			p := string(k)
			msg := fmt.Sprintf("normalizeNullValues: replacing %s=[] with oldInputs.%s=null", p, p)
			GetLogger(ctx).Debug(msg)
			copy[k] = oldInput
		} else {
			msg := fmt.Sprintf("normalizeNullValues: removing %s=[] to match oldInputs", string(k))
			GetLogger(ctx).Debug(msg)
			delete(copy, k)
		}
	}
	return copy
}
