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
	elidedAttributes       int

	unexpectedSnippets            int
	hclAllLangsConversionFailures int // examples that failed to convert in any language

	// examples that failed to convert in one, but not all, languages. This is less severe impact because users will
	// at least have code in another language to reference:
	hclGoPartialConversionFailures         int
	hclPythonPartialConversionFailures     int
	hclTypeScriptPartialConversionFailures int
	hclCSharpPartialConversionFailures     int

	// Arguments metrics:
	totalArgumentsFromDocs int
	// See comment in getNestedDescriptionFromParsedDocs for why we track this behavior:
	argumentDescriptionsFromAttributes int

	// General metrics:
	entitiesMissingDocs int

	schemaStats schemaTools.PulumiSchemaStats
)

// printDocStats outputs metrics relating to document parsing and conversion
func printDocStats() {
	fmt.Println("")

	fmt.Println("General metrics:")
	fmt.Printf("\t%d total resources containing %d total inputs.\n",
		schemaStats.Resources.TotalResources, schemaStats.Resources.TotalInputProperties)
	fmt.Printf("\t%d total functions.\n", schemaStats.Functions.TotalFunctions)
	fmt.Printf("\t%d entities are missing docs entirely because they could not be found in the upstream provider.\n",
		entitiesMissingDocs)
	fmt.Println("")

	fmt.Println("Description metrics:")
	fmt.Printf("\t%d entity descriptions contained an <elided> reference and were dropped, including examples.\n",
		elidedDescriptions)
	fmt.Printf("\t%d entity descriptions contained an <elided> reference and were dropped, but examples were preserved.\n",
		elidedDescriptionsOnly)
	fmt.Println("")

	fmt.Println("Example conversion metrics:")
	fmt.Printf("\t%d HCL examples failed to convert in all languages\n", hclAllLangsConversionFailures)
	fmt.Printf("\t%d HCL examples were converted in at least one language but failed to convert to TypeScript\n",
		hclTypeScriptPartialConversionFailures)
	fmt.Printf("\t%d HCL examples were converted in at least one language but failed to convert to Python\n",
		hclPythonPartialConversionFailures)
	fmt.Printf("\t%d HCL examples were converted in at least one language but failed to convert to Go\n",
		hclGoPartialConversionFailures)
	fmt.Printf("\t%d HCL examples were converted in at least one language but failed to convert to C#\n",
		hclCSharpPartialConversionFailures)
	fmt.Printf("\t%d entity document sections contained unexpected HCL code snippets. Examples will be converted, "+
		"but may not display correctly in the registry, e.g. lacking tabs.\n", unexpectedSnippets)
	fmt.Println("")

	fmt.Println("Argument metrics:")
	fmt.Printf("\t%d argument descriptions were parsed from the upstream docs\n", totalArgumentsFromDocs)
	fmt.Printf("\t%d top-level input property descriptions came from an upstream attribute (as opposed to an argument). "+
		"Nested arguments are not included in this count.\n", argumentDescriptionsFromAttributes)
	fmt.Printf("\t%d arguments contained an <elided> reference and had their descriptions dropped.\n",
		elidedArguments)
	fmt.Printf("\t%d nested arguments contained an <elided> reference and had their descriptions dropped.\n",
		elidedNestedArguments)
	//nolint:lll
	fmt.Printf("\t%d of %d resource inputs (%.2f%%) are missing descriptions in the schema\n",
		schemaStats.Resources.InputPropertiesMissingDescriptions, schemaStats.Resources.TotalInputProperties,
		float64(schemaStats.Resources.InputPropertiesMissingDescriptions)/float64(schemaStats.Resources.TotalInputProperties)*100)
	fmt.Println("")

	fmt.Println("Attribute metrics:")
	fmt.Printf("\t%d attributes contained an <elided> reference and had their descriptions dropped.\n",
		elidedAttributes)
	fmt.Println("")
}
