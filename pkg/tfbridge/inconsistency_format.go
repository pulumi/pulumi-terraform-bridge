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
	"fmt"
	"strings"
)

// FormatTerraformStyleInconsistency formats inconsistency errors
// exactly like Terraform does for consistency with upstream behavior.
//
// The errors parameter should come from objchange.AssertObjectCompatible
// which returns []error containing cty.PathError values with descriptive
// messages about what changed.
func FormatTerraformStyleInconsistency(resourceType string, errs []error) string {
	if len(errs) == 0 {
		return ""
	}

	var lines []string
	for _, err := range errs {
		// Errors from AssertObjectCompatible are already formatted with
		// paths and descriptions like ".attribute_name: was X, but now Y"
		lines = append(lines, fmt.Sprintf("  %s", err.Error()))
	}

	return fmt.Sprintf(
		"Provider produced inconsistent result after apply for %s:\n%s\n\n"+
			"This is a bug in the provider, which should be reported in the provider's own issue tracker.",
		resourceType,
		strings.Join(lines, "\n"),
	)
}
