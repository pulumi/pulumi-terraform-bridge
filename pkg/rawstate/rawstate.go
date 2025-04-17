// Copyright 2016-2025, Pulumi Corporation.
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

package rawstate

import (
	"encoding/json"
)

// RawState is the raw un-encoded Terraform state, without type information. It is passed as-is for providers to
// upgrade and run migrations on.
//
// The representation matches the format accepted on the gRPC Terraform protocol:
//
//	https://github.com/hashicorp/terraform-plugin-go/blob/v0.26.0/tfprotov5/internal/tfplugin5/tfplugin5.pb.go#L519
//	https://github.com/hashicorp/terraform-plugin-go/blob/v0.26.0/tfprotov6/state.go#L35
type RawState json.RawMessage

// Ensure json.Marshal preserves this data as-is.
func (x RawState) MarshalJSON() ([]byte, error) {
	return x, nil
}

// Ensure json.Unmarshal preserves this data as-is.
func (x *RawState) UnmarshalJSON(raw []byte) error {
	*x = raw
	return nil
}
