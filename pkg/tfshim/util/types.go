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

package util

import (
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

// isOfTypeMap detects schemas indicating a map[string,X] type. Due to a quirky encoding of
// single-nested Terraform blocks, it is insufficient to just check for tfs.Type() == shim.TypeMap.
// See [shim.Schema.Elem()] comment for all the details of the encoding.
func IsOfTypeMap(tfs shim.Schema) bool {
	if tfs == nil || tfs.Type() != shim.TypeMap {
		return false
	}
	_, hasResourceElem := tfs.Elem().(shim.Resource)
	return !hasResourceElem
}
