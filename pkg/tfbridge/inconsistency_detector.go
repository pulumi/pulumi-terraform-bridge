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

// InconsistencyDetail describes a single detected inconsistency
type InconsistencyDetail struct {
	// Path is the attribute path where the inconsistency was found
	Path string

	// PlannedValue is what was expected from the plan
	PlannedValue interface{}

	// ActualValue is what was actually returned after apply
	ActualValue interface{}

	// Description provides a human-readable explanation
	Description string
}

// InconsistencyResult contains the detected inconsistencies
type InconsistencyResult struct {
	// Detected indicates if inconsistencies were found
	Detected bool

	// ResourceType is the type of resource checked
	ResourceType string

	// Inconsistencies holds the list of detected inconsistencies
	Inconsistencies []InconsistencyDetail

	// RawPlanned holds the raw planned state (for debug log levels)
	RawPlanned interface{}

	// RawActual holds the raw actual state (for debug log levels)
	RawActual interface{}
}

// We don't declare interfaces here, as we'll use the ones from tfshim package
