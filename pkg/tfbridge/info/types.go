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

package info

// InconsistencyFilter defines an interface for filtering out known inconsistencies
// between planned and applied resource state
type InconsistencyFilter interface {
	// ShouldIgnoreAttribute returns true if inconsistencies for this attribute should be ignored
	ShouldIgnoreAttribute(resourceType, attrName string) bool

	// ShouldIgnoreValueChange returns true if changes between these values should be ignored
	ShouldIgnoreValueChange(resourceType, attrName string, plannedVal, actualVal interface{}) bool
}
