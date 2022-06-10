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
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"strings"
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

	nestedArgSectionsMultipleMatches int
	nestedArgsWithNoPreviousMatch    int

	// General metrics:
	entitiesMissingDocs int

	schemaStats pulumiSchemaStats
)

type pulumiSchemaStats struct {
	totalResources            int
	totalResourceInputs       int
	resourceInputsMissingDesc int
	totalFunctions            int
}

func countStats(sch pschema.PackageSpec) pulumiSchemaStats {
	// This code is adapted from https://github.com/mikhailshilkov/schema-tools. If we make schema-tools more robust and
	// portable, we should consider unifying these codebases. (We elected not to do this upfront due to unknown downstream
	// effects of changing schema-tools as it's used in all of our GH Actions and is not pinned to a version.)
	stats := pulumiSchemaStats{}

	uniques := codegen.NewStringSet()
	visitedTypes := codegen.NewStringSet()

	var propCount func(string) (int, int)
	propCount = func(typeName string) (totalProperties int, propertiesMissingDesc int) {
		if visitedTypes.Has(typeName) {
			return 0, 0
		}

		visitedTypes.Add(typeName)

		t := sch.Types[typeName]

		totalProperties = len(t.Properties)
		propertiesMissingDesc = 0

		for _, p := range t.Properties {
			if p.Description == "" {
				propertiesMissingDesc++
			}

			if p.Ref != "" {
				tn := strings.TrimPrefix(p.Ref, "#/types/")
				nestedTotalProps, nestedPropsMissingDesc := propCount(tn)
				totalProperties += nestedTotalProps
				propertiesMissingDesc += nestedPropsMissingDesc
			}
		}
		return totalProperties, propertiesMissingDesc
	}

	for n, r := range sch.Resources {
		baseName := versionlessName(n)
		if uniques.Has(baseName) {
			continue
		}
		uniques.Add(baseName)
		stats.totalResourceInputs += len(r.InputProperties)
		for _, p := range r.InputProperties {
			if p.Description == "" {
				stats.resourceInputsMissingDesc++
			}

			if p.Ref != "" {
				typeName := strings.TrimPrefix(p.Ref, "#/types/")
				nestedTotalProps, nestedPropsMissingDesc := propCount(typeName)
				stats.totalResourceInputs += nestedTotalProps
				stats.resourceInputsMissingDesc += nestedPropsMissingDesc
			}
		}
	}

	stats.totalResources = len(uniques)
	stats.totalFunctions = len(sch.Functions)

	return stats
}

func versionlessName(name string) string {
	// This code is adapted from https://github.com/mikhailshilkov/schema-tools. See comment in countStats().
	parts := strings.Split(name, ":")
	mod := parts[1]
	modParts := strings.Split(mod, "/")
	if len(modParts) == 2 {
		mod = modParts[0]
	}
	return fmt.Sprintf("%s:%s", mod, parts[2])
}

// printDocStats outputs metrics relating to document parsing and conversion
func printDocStats() {
	fmt.Println("")

	fmt.Println("General metrics:")
	fmt.Printf("\t%d total resources containing %d total inputs.\n",
		schemaStats.totalResources, schemaStats.totalResourceInputs)
	fmt.Printf("\t%d total functions.\n", schemaStats.totalFunctions)
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
	fmt.Printf("\t%d of %d resource inputs (%.2f%%) are missing descriptions in the schema\n",
		schemaStats.resourceInputsMissingDesc, schemaStats.totalResourceInputs,
		float64(schemaStats.resourceInputsMissingDesc)/float64(schemaStats.totalResourceInputs)*100)
	fmt.Printf("\t%d arg doc sections were partially processed because they contained a nested block that matched multiple previously mentioned arguments. The affected resources and functions will have missing argument descriptions.", nestedArgSectionsMultipleMatches)
	fmt.Printf("\"%d arg doc sections have a nested block with no previously mentioned argument. The affected resources and functions may have incorrect argument descriptions.\", nestedArgsWithNoPreviousMatch")
	fmt.Println("")

	fmt.Println("Attribute metrics:")
	fmt.Printf("\t%d attributes contained an <elided> reference and had their descriptions dropped.\n",
		elidedAttributes)
	fmt.Println("")
}
