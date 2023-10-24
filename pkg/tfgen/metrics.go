// Copyright 2016-2022, Pulumi Corporation.
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

package tfgen

import (
	"fmt"

	schemaTools "github.com/pulumi/schema-tools/pkg"
)

var (
	ignoredDocHeaders      = make(map[string]int)
	elidedDescriptions     int // i.e., we discard the entire description, including examples
	elidedDescriptionsOnly int // we discarded the description proper, but were able to preserve the examples
	elidedArguments        int
	elidedNestedArguments  int
	unexpectedSnippets     int

	// Arguments metrics:
	totalArgumentsFromDocs int
	// See comment in getNestedDescriptionFromParsedDocs for why we track this behavior:
	argumentDescriptionsFromAttributes int
	entitiesMissingDocs                int

	schemaStats schemaTools.PulumiSchemaStats
)

// printDocStats outputs metrics relating to document parsing and conversion
func printDocStats() {
	fmt.Println("General metrics:")
	fmt.Printf("\t%d total resources containing %d total inputs.\n",
		schemaStats.Resources.TotalResources, schemaStats.Resources.TotalInputProperties)
	fmt.Printf("\t%d total functions.\n", schemaStats.Functions.TotalFunctions)
	if entitiesMissingDocs > 0 {
		fmt.Printf("\t%d entities are missing docs entirely because they could not be found in the upstream provider.\n",
			entitiesMissingDocs)
	}
	if unexpectedSnippets > 0 {
		fmt.Printf("\t%d entity document sections contained unexpected HCL code snippets. Examples will be converted, "+
			"but may not display correctly in the registry, e.g. lacking tabs.\n", unexpectedSnippets)
	}
	fmt.Println("")

	fmt.Println("Argument metrics:")
	fmt.Printf("\t%d argument descriptions were parsed from the upstream docs\n", totalArgumentsFromDocs)
	fmt.Printf("\t%d top-level input property descriptions came from an upstream attribute (as opposed to an argument). "+
		"Nested arguments are not included in this count.\n", argumentDescriptionsFromAttributes)
	if elidedArguments > 0 || elidedNestedArguments > 0 {
		fmt.Printf("\t%d arguments contained an <elided> reference and had their descriptions dropped.\n",
			elidedArguments)
		fmt.Printf("\t%d nested arguments contained an <elided> reference and had their descriptions dropped.\n",
			elidedNestedArguments)
	}
	fmt.Printf(
		"\t%d of %d resource inputs (%.2f%%) are missing descriptions in the schema\n",
		schemaStats.Resources.InputPropertiesMissingDescriptions,
		schemaStats.Resources.TotalInputProperties,
		float64(
			schemaStats.Resources.InputPropertiesMissingDescriptions,
		)/float64(
			schemaStats.Resources.TotalInputProperties,
		)*100,
	)
	fmt.Println("")
}
