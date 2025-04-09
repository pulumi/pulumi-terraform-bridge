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

package reservedkeys

const (
	// The __meta key is reserved for storing private resource state and schema version.
	Meta = "__meta"

	// The __defaultsKey is the name of the input property that is used to track which property keys were populated
	// using default values from the resource's schema. This information is used to inform which input properties
	// should be populated using old defaults in subsequent updates. When populating the default value for an input
	// property, the property's old value will only be used as the default if the property's key is present in the
	// defaults list for the old property bag.
	Defaults = "__defaults"
)

func IsBridgeReservedKey(name string) bool {
	if name == Meta || name == Defaults {
		return true
	}
	return false
}
