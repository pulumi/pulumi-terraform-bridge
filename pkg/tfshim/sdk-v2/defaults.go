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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func withPatchedDefaults(s *schema.Schema) *schema.Schema {
	if hasInvalidDefault(s) {
		var schema schema.Schema = *s
		schema.Default = nil
		return &schema
	}
	return s
}

func hasInvalidDefault(s *schema.Schema) bool {
	if s.Default == nil && s.ValidateFunc == nil {
		return false
	}
	_, errors := s.ValidateFunc(s.Default, "field")
	for _, err := range errors {
		glog.V(9).Infof("ignoring a Default value %v that does not validate with ValidateFunc: %v",
			s.Default,
			err)
	}
	return len(errors) > 0
}
