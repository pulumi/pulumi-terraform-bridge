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

// This file implements a system for collecting data on how many HCL examples were
// attempted to be converted to Pulumi, and what percentage of such conversions
// succeeded. Additionally, it allows these diagnostics to be exported in JSON
// format for uploading and further processing.

package tfgen

import "fmt"

// Main overarching structure for storing coverage data on how many examples were processed,
// how many failed, and for what reason
type CoverageTracker struct {
	ProviderName        string                        // Name of the provider
	ProviderVersion     string                        // Version of the provider
	CurrentExampleName  string                        // Name of current example that is being processed
	EncounteredExamples map[string]GeneralExampleInfo // Mapping example names to their general information
}

// General information about an example, and how successful it was at being converted to different languages
type GeneralExampleInfo struct {
	Name                         string
	OriginalHCL                  string
	LanguagesConvertedTo         map[string]LanguageConversionResult // Mapping language names to their conversion diagnostics
	NameEncounteredMultipleTimes bool                                // Current name has already been encountered before
}

// Individual language information concerning how successfully an example was converted to Pulumi
type LanguageConversionResult struct {
	TargetLanguage            string
	FailureSeverity           int    // [None, Medium, High, Fatal]
	FailureInfo               string // Additional in-depth information
	ExamplePossiblyDuplicated bool   // !! Current example name has already been converted for this specific language before. Either the example is duplicated, or a bug is present !!
}

// Failure severity values
const (
	None   = 0
	Medium = 1
	High   = 2
	Fatal  = 3
)

func newCoverageTracker(ProviderName string, ProviderVersion string) *CoverageTracker {
	return &CoverageTracker{ProviderName, ProviderVersion, "", make(map[string]GeneralExampleInfo)}
}

//========================== Coverage Tracker Interface ===========================
// At different stages, the code translator notifies the tracker of what is going on.
// Notifications are treated as an ordered stream of events: foundExample must be first.

// Used when: generator has found a new example with a convertible block of HCL
func (CT *CoverageTracker) foundExample(exampleName string, hcl string) {
	if CT == nil {
		return
	}
	CT.CurrentExampleName = exampleName
	if val, ok := CT.EncounteredExamples[exampleName]; ok {
		val.NameEncounteredMultipleTimes = true
	} else {
		CT.EncounteredExamples[exampleName] = GeneralExampleInfo{exampleName, hcl, make(map[string]LanguageConversionResult), false}
	}
}

// Current example has been successfully converted to a certain language
func (CT *CoverageTracker) languageConversionSuccess(targetLanguage string) {
	if CT == nil {
		return
	}
	CT.insertLanguageConversionResult(LanguageConversionResult{
		TargetLanguage:            targetLanguage,
		FailureSeverity:           0,
		FailureInfo:               "",
		ExamplePossiblyDuplicated: false,
	})
}

// Generator has failed to convert the current example to a certain language
func (CT *CoverageTracker) languageConversionFailure(conversionFailOpts ConversionFailOpts) {
	if CT == nil {
		return
	}
	CT.insertLanguageConversionResult(LanguageConversionResult{
		TargetLanguage:            conversionFailOpts.targetLanguage,
		FailureSeverity:           conversionFailOpts.failureSeverity,
		FailureInfo:               conversionFailOpts.failureInfo,
		ExamplePossiblyDuplicated: false,
	})
}

// Information about failed conversion
type ConversionFailOpts struct {
	targetLanguage  string
	failureSeverity int
	failureInfo     string
}

// Generator ncountered a fatal error when trying to convert the current example to a certain language
func (CT *CoverageTracker) languageConversionPanic(targetLanguage string, panicInfo string) {
	if CT == nil {
		return
	}
	CT.insertLanguageConversionResult(LanguageConversionResult{
		TargetLanguage:            targetLanguage,
		FailureSeverity:           3,
		FailureInfo:               panicInfo,
		ExamplePossiblyDuplicated: false,
	})
}

//=================================================================================

// Adding a language conversion result to the current example. If a conversion result with the same
// target language already exists, keep the lowest severity one and mark the example as possibly duplicated
func (CT *CoverageTracker) insertLanguageConversionResult(conversionResult LanguageConversionResult) {
	if currentExample, ok := CT.EncounteredExamples[CT.CurrentExampleName]; ok {
		if existingConversionResult, ok := currentExample.LanguagesConvertedTo[conversionResult.TargetLanguage]; ok {
			// Specific language conversion result for this example already exists
			if conversionResult.FailureSeverity < existingConversionResult.FailureSeverity {
				currentExample.LanguagesConvertedTo[conversionResult.TargetLanguage] = conversionResult
			}
			existingConversionResult.ExamplePossiblyDuplicated = true
		} else {
			// A brand new language conversion result is being added for this example
			currentExample.LanguagesConvertedTo[conversionResult.TargetLanguage] = conversionResult
		}
	} else {
		fmt.Println("Error: attempted to log the result of a language conversion without first finding an example")
		fmt.Println(conversionResult)
		panic("")
	}
}

// Exporting the coverage results
func (CT *CoverageTracker) exportResults(outputDirectory string) {
	var coverageExportUtil = newCoverageExportUtil(CT)
	coverageExportUtil.tryExport(outputDirectory)
}
