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
	"strings"

	"github.com/hashicorp/hcl/v2"
)

/*
Main overarching structure for storing coverage data on how many examples were processed,
how many failed, and for what reason. At different stages, the code translator notifies
the tracker of what is going on. Notifications are treated as an ordered sequence of events.

NOTIFICATION INTERFACE:

example := getOrCreateExample(pageName, hcl)

languageConversionSuccess(example, targetLanguage)

languageConversionWarning(example, targetLanguage, warningDiagnostics)

languageConversionFailure(example, targetLanguage, failureDiagnostics)

languageConversionPanic(example, targetLanguage, panicInfo)
*/
type CoverageTracker struct {
	ProviderName     string                        // Name of the provider
	ProviderVersion  string                        // Version of the provider
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
	Program         string // Converted program

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
	return &CoverageTracker{ProviderName, ProviderVersion, make(map[string]*DocumentationPage)}
}

// Find example by pageName and raw HCL source.
func (ct *CoverageTracker) getExample(pageName string, hcl string) *Example {
	if ct == nil {
		return nil
	}
	page, ok := ct.EncounteredPages[pageName]
	if !ok {
		return nil
	}
	for _, e := range page.Examples {
		if e.OriginalHCL == hcl {
			return &e
		}
	}
	return nil
}

// Similar to getExample, but instead of returning nil when not found, creates an appropriate page
// and Example object and registers it.
func (ct *CoverageTracker) getOrCreateExample(pageName string, hcl string) *Example {
	if ct == nil {
		return nil
	}
	if e := ct.getExample(pageName, hcl); e != nil {
		return e
	}
	if existingPage, ok := ct.EncounteredPages[pageName]; ok {
		// This example's page already exists. Appending example to it.
		example := Example{hcl, make(map[string]*LanguageConversionResult)}
		existingPage.Examples = append(existingPage.Examples, example)
		return &example
	} else {
		// Initializing a page for this example.
		example := Example{hcl, make(map[string]*LanguageConversionResult)}
		examples := []Example{example}
		ct.EncounteredPages[pageName] = &DocumentationPage{pageName, examples}
		return &example
	}
}

// Used when: current example has been successfully converted to a certain language
func (ct *CoverageTracker) languageConversionSuccess(
	e *Example, languageName string, program string,
) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(e, languageName, LanguageConversionResult{
		FailureSeverity:  Success,
		FailureInfo:      "",
		TranslationCount: 1,
		Program:          program,
	})
}

// Used when: generator has successfully converted current example, but threw out some warnings
//
//nolint:unused
func (ct *CoverageTracker) languageConversionWarning(
	e *Example, languageName string, warningDiagnostics hcl.Diagnostics,
) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(e, languageName, LanguageConversionResult{
		FailureSeverity:  Warning,
		FailureInfo:      formatDiagnostics(warningDiagnostics),
		TranslationCount: 1,
	})
}

// Used when: generator has failed to convert the current example to a certain language
func (ct *CoverageTracker) languageConversionFailure(
	e *Example, languageName string, failureDiagnostics hcl.Diagnostics,
) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(e, languageName, LanguageConversionResult{
		FailureSeverity:  Failure,
		FailureInfo:      formatDiagnostics(failureDiagnostics),
		TranslationCount: 1,
	})
}

// Used when: generator encountered a fatal internal error when trying to convert the
// current example to a certain language
func (ct *CoverageTracker) languageConversionPanic(
	e *Example, languageName string, panicInfo string,
) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(e, languageName, LanguageConversionResult{
		FailureSeverity:  Fatal,
		FailureInfo:      panicInfo,
		TranslationCount: 1,
	})
}

// Adding a language conversion result to the current example. If a conversion result with the same
// target language already exists, keep the lowest severity one and mark the example as possibly
// duplicated
func (ct *CoverageTracker) insertLanguageConversionResult(
	e *Example, languageName string, newConversionResult LanguageConversionResult,
) {
	if existingConversionResult, ok := e.ConversionResults[languageName]; ok {
		// Example already has this language conversion attempt. Replace if new one has a
		// lower severity
		if newConversionResult.FailureSeverity < existingConversionResult.FailureSeverity {
			e.ConversionResults[languageName] = &newConversionResult
		}
		existingConversionResult.TranslationCount++
	} else {
		// The new language conversion result is added for this example
		e.ConversionResults[languageName] = &newConversionResult
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

// Exporting the coverage results
func (ct *CoverageTracker) exportResults(outputDirectory string) error {
	coverageExportUtil := newCoverageExportUtil(ct)
	return coverageExportUtil.tryExport(outputDirectory)
}

func (ct *CoverageTracker) getShortResultSummary() string {
	coverageExportUtil := newCoverageExportUtil(ct)
	return coverageExportUtil.produceHumanReadableSummary()
}
