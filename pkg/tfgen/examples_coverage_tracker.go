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
// how many failed, and for what reason. At different stages, the code translator notifies
// the tracker of what is going on. Notifications are treated as an ordered stream of events.
// INTERFACE:
// foundExample(), languageConversionSuccess(), languageConversionFailure(), languageConversionPanic().
type CoverageTracker struct {
	ProviderName        string                         // Name of the provider
	ProviderVersion     string                         // Version of the provider
	CurrentExampleName  string                         // Name of current example that is being processed
	EncounteredExamples map[string]*GeneralExampleInfo // Mapping example names to their general information
}

// General information about an example, and how successful it was at being converted to different languages
type GeneralExampleInfo struct {
	Name                   string
	OriginalHCL            string
	LanguagesConvertedTo   map[string]*LanguageConversionResult // Mapping language names to their conversion diagnostics
	NameFoundMultipleTimes bool                                 // Current name has already been encountered before
}

// Individual language information concerning how successfully an example was converted to Pulumi
type LanguageConversionResult struct {
	TargetLanguage       string
	FailureSeverity      int    // [None, Low, High, Fatal]
	FailureInfo          string // Additional in-depth information
	MultipleTranslations bool   // !! Current example name has already been converted for this specific language before. Either the example is duplicated, or a bug is present !!
}

// Failure severity values
const (
	None  = 0
	Low   = 1
	High  = 2
	Fatal = 3
)

func newCoverageTracker(ProviderName string, ProviderVersion string) *CoverageTracker {
	return &CoverageTracker{ProviderName, ProviderVersion, "", make(map[string]*GeneralExampleInfo)}
}

// Used when: generator has found a new example with a convertible block of HCL
func (ct *CoverageTracker) foundExample(exampleName string, hcl string) {
	if ct == nil {
		return
	}
	ct.CurrentExampleName = exampleName
	if val, ok := ct.EncounteredExamples[exampleName]; ok {
		val.NameFoundMultipleTimes = true
	} else {
		ct.EncounteredExamples[exampleName] = &GeneralExampleInfo{exampleName, hcl, make(map[string]*LanguageConversionResult), false}
	}
}

// Used when: current example has been successfully converted to a certain language
func (ct *CoverageTracker) languageConversionSuccess(targetLanguage string) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(LanguageConversionResult{
		TargetLanguage:       targetLanguage,
		FailureSeverity:      0,
		FailureInfo:          "",
		MultipleTranslations: false,
	})
}

// Used when: generator has failed to convert the current example to a certain language
func (ct *CoverageTracker) languageConversionFailure(conversionFailOpts ConversionFailOpts) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(LanguageConversionResult{
		TargetLanguage:       conversionFailOpts.targetLanguage,
		FailureSeverity:      conversionFailOpts.failureSeverity,
		FailureInfo:          conversionFailOpts.failureInfo,
		MultipleTranslations: false,
	})
}

// Information about failed conversion
type ConversionFailOpts struct {
	targetLanguage  string
	failureSeverity int
	failureInfo     string
}

// Used when: generator encountered a fatal error when trying to convert the current example to a certain language
func (ct *CoverageTracker) languageConversionPanic(targetLanguage string, panicInfo string) {
	if ct == nil {
		return
	}
	ct.insertLanguageConversionResult(LanguageConversionResult{
		TargetLanguage:       targetLanguage,
		FailureSeverity:      3,
		FailureInfo:          panicInfo,
		MultipleTranslations: false,
	})
}

// Adding a language conversion result to the current example. If a conversion result with the same
// target language already exists, keep the lowest severity one and mark the example as possibly duplicated
func (ct *CoverageTracker) insertLanguageConversionResult(conversionResult LanguageConversionResult) {
	if currentExample, ok := ct.EncounteredExamples[ct.CurrentExampleName]; ok {
		if existingConversionResult, ok := currentExample.LanguagesConvertedTo[conversionResult.TargetLanguage]; ok {

			// If incoming result is of a lower severity, keep it instead of the existing one
			if conversionResult.FailureSeverity < existingConversionResult.FailureSeverity {
				currentExample.LanguagesConvertedTo[conversionResult.TargetLanguage] = &conversionResult
			}
			existingConversionResult.MultipleTranslations = true
		} else {

			// A brand new language conversion result is being added for this example
			currentExample.LanguagesConvertedTo[conversionResult.TargetLanguage] = &conversionResult
		}
	} else {
		fmt.Println("Error: attempted to log the result of a language conversion without first finding an example")
		fmt.Println(conversionResult)
		panic("")
	}
}

// Exporting the coverage results
func (ct *CoverageTracker) exportResults(outputDirectory string) error {
	coverageExportUtil := newCoverageExportUtil(ct)
	return (coverageExportUtil.tryExport(outputDirectory))
}
