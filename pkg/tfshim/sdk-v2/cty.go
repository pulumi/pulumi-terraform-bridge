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

package sdkv2

import (
	"github.com/golang/glog"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// makeResourceRawConfig converts the decoded Go values in a terraform.ResourceConfig into a cty.Value that is
// appropriate for Instance{Diff,State}.RawConfig.
func makeResourceRawConfig(config *terraform.ResourceConfig, resource *schema.Resource) cty.Value {
	// The method JSONMapToStateValue has State in its name but the implementation is generic, and fits the use case
	// here. What it does is simply parsing raw map[string]interface{} data into a Object value with a schema that
	// corresponds to the Resource schema.
	value, err := schema.JSONMapToStateValue(config.Raw, resource.CoreConfigSchema())
	if err == nil {
		return value
	} else {
		// This should never happen in practice, but following the original design of this method error recovery
		// is attempted by using approximate methods as it might be better to proceed than to fail fast.
		glog.V(9).Infof("failed to recover resource config value from data, "+
			"falling back to approximate methods: %v", err)
	}

	// Unlike schema.JSONMapToStateValue, schema.HCL2ValueFromConfigValue is an approximate method as it does not
	// consult the type of the resource. This causes problems such as lists being decoded as Tuple when the schema
	// wants a Set. The problems cause CoerceValue to fail.
	original := schema.HCL2ValueFromConfigValue(config.Raw)
	coerced, err := resource.CoreConfigSchema().CoerceValue(original)
	if err != nil {
		// Once more, choosing to proceed with a slightly incorrect value rather than fail fast.
		glog.V(9).Infof("failed to coerce config: %v", err)
		return original
	}
	return coerced
}
