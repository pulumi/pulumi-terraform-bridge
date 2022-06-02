// Copyright 2016-2021, Pulumi Corporation.
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

/*
This file implements a system for collecting data on how many HCL examples were
attempted to be converted to Pulumi, and what percentage of such conversions
succeeded. Additionally, it allows these diagnostics to be exported in JSON
format for uploading and further processing.

The tracker records the results of individual translation attempts: a provider
with 100 examples would have 400 attempts (four languages for each example).
How many of these attempts succeeded is what the percentages reference. These
400 attempts can either be exported as a whole, or be grouped by language into
four categories of 100 attempts, with each corresponding to one example.
*/

package tfgen

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

/*
Main overarching structure for storing coverage data on how many examples were processed,
how many failed, and for what reason. At different stages, the code translator notifies
the tracker of what is going on. Notifications are treated as an ordered sequence of events.

NOTIFICATION INTERFACE:

foundExample(pageName, hcl)

languageConversionSuccess(targetLanguage)

languageConversionWarning(targetLanguage, warningDiagnostics)

languageConversionFailure(targetLanguage, failureDiagnostics)

languageConversionPanic(targetLanguage, panicInfo)
*/
type CoverageTracker struct {
	ProviderName     string                        // Name of the provider
	ProviderVersion  string                        // Version of the provider
	currentPageName  string                        // Name of current page that is being processed
	EncounteredPages map[string]*DocumentationPage // Map linking page IDs to their data
}

// A structure encompassing a single page, which contains one or more examples.
// This closely resembles the web pages seen in Pulumi/Terraform documentation.
type DocumentationPage struct {
	Name     string    // The unique ID of this documentation page
	Examples []Example // This page's examples, stored in the order they were found
}

// Contains information about a single example, consisting of a block of HCL and
// one or more language conversion results.
type Example struct {
	OriginalHCL       string                               // Original HCL code that the example was found with
	ConversionResults map[string]*LanguageConversionResult // Mapping language names to their conversion results
}

// Individual language information concerning how successfully an example was converted to Pulumi
type LanguageConversionResult struct {
	FailureSeverity int    // This conversion's outcome: [Success, Warning, Failure, Fatal]
	FailureInfo     string // Additional in-depth information

	// How many times this example has been converted to this language.
	// It is expected that this will be equal to 1.
	TranslationCount int
}

// Conversion outcome severity values
const (
	Success = 0
	Warning = 1
	Failure = 2
	Fatal   = 3
)

func newCoverageTracker(ProviderName string, ProviderVersion string) *CoverageTracker {
	return &CoverageTracker{ProviderName, ProviderVersion, "",
		make(map[string]*DocumentationPage)}
}

// Used when: generator has found a brand new example, with a convertible block
// of HCL that hasn't been encountered before.
func (ct *CoverageTracker) foundExample(pageName string, hcl string) {
	if ct == nil {
		return
	}
	ct.currentPageName = pageName

	if existingPage, ok := ct.EncounteredPages[pageName]; ok {
		// This example's page already exists. Appending example to it.
		existingPage.Examples = append(existingPage.Examples, Example{hcl, make(map[string]*LanguageConversionResult)})
	} else {
		// Initializing a page for this example.
		ct.EncounteredPages[pageName] = &DocumentationPage{
			pageName,
			[]Example{Example{hcl, make(map[string]*LanguageConversionResult)}},
		}
	}
}

// Used when: current example has been successfully converted to a certain language
func (ct *CoverageTracker) languageConversionSuccess(languageName string) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(languageName, LanguageConversionResult{
		FailureSeverity:  Success,
		FailureInfo:      "",
		TranslationCount: 1,
	})
}

//nolint
// Used when: generator has successfully converted current example, but threw out some warnings
func (ct *CoverageTracker) languageConversionWarning(languageName string, warningDiagnostics hcl.Diagnostics) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(languageName, LanguageConversionResult{
		FailureSeverity:  Warning,
		FailureInfo:      formatDiagnostics(warningDiagnostics),
		TranslationCount: 1,
	})
}

// Used when: generator has failed to convert the current example to a certain language
func (ct *CoverageTracker) languageConversionFailure(languageName string, failureDiagnostics hcl.Diagnostics) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(languageName, LanguageConversionResult{
		FailureSeverity:  Failure,
		FailureInfo:      formatDiagnostics(failureDiagnostics),
		TranslationCount: 1,
	})
}

// Used when: generator encountered a fatal internal error when trying to convert the
// current example to a certain language
func (ct *CoverageTracker) languageConversionPanic(languageName string, panicInfo string) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(languageName, LanguageConversionResult{
		FailureSeverity:  Fatal,
		FailureInfo:      panicInfo,
		TranslationCount: 1,
	})
}

// Adding a language conversion result to the current example. If a conversion result with the same
// target language already exists, keep the lowest severity one and mark the example as possibly duplicated
func (ct *CoverageTracker) insertLanguageConversionResult(languageName string,
	newConversionResult LanguageConversionResult) {
	if currentPage, ok := ct.EncounteredPages[ct.currentPageName]; ok {
		lastExample := currentPage.lastExample()

		if existingConversionResult, ok := lastExample.ConversionResults[languageName]; ok {
			// Example already has this language conversion attempt. Replace if new one has a lower severity
			if newConversionResult.FailureSeverity < existingConversionResult.FailureSeverity {
				lastExample.ConversionResults[languageName] = &newConversionResult
			}
			existingConversionResult.TranslationCount += 1
		} else {
			// The new language conversion result is added for this example
			lastExample.ConversionResults[languageName] = &newConversionResult
		}
	} else {
		// Check that foundExample() is called before all other Coverage Tracker interface methods
		fmt.Println("Error: attempted to log an example language conversion result without first finding its page")
		fmt.Println(newConversionResult)
		panic("")
	}
}

// Turning the hcl.Diagnostics provided during warnings or failures into a brief explanation of
// why the converter didn't succeed. If the diagnostics have details available, they are included.
func formatDiagnostics(diagnostics hcl.Diagnostics) string {
	results := []string{}

	// Helper method to check if results already have one of this diagnostic
	contains := func(result []string, target string) bool {
		for _, diag := range result {
			if diag == target {
				return true
			}
		}
		return false
	}

	for i := 0; i < len(diagnostics); i++ {
		formattedDiagnostic := diagnostics[i].Summary

		// Include diagnostic details if suitable
		if diagnostics[i].Detail != "" && diagnostics[i].Detail != formattedDiagnostic {
			formattedDiagnostic += ": " + diagnostics[i].Detail
		}

		// Append formatted diagnostic if results don't have it
		if !contains(results, formattedDiagnostic) {
			results = append(results, formattedDiagnostic)
		}
	}

	// Returning all the formatted diagnostics as a single string
	return strings.Join(results[:], "; ")
}

// Returning the page's last example, to which conversion results will be added.
func (Page *DocumentationPage) lastExample() *Example {
	return &Page.Examples[len(Page.Examples)-1]
}

// Exporting the coverage results
func (ct *CoverageTracker) exportResults(outputDirectory string) error {
	coverageExportUtil := newCoverageExportUtil(ct)
	return (coverageExportUtil.tryExport(outputDirectory))
}
