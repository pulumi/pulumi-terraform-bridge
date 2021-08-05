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
// and .db format for uploading and further processing.

package tfgen

// Main overarching structure for storing coverage data on how many examples were processed,
// how many failed, and for what reason
type CoverageTracker struct {
	CurrentExampleName  string                        // Name of current example that is being processed
	EncounteredExamples map[string]GeneralExampleInfo // Mapping example names to their general information
}

// General information about an example, and how successful it was at being converted to different languages
type GeneralExampleInfo struct {
	name                         string
	originalHCL                  string
	languagesConvertedTo         map[string]LanguageConversionResult // Mapping language names to their conversion diagnostics
	nameEncounteredMultipleTimes bool                                // Current name has already been encountered before
}

// Individual language information concerning how successfully an example was converted to Pulumi
type LanguageConversionResult struct {
	targetLanguage            string
	failureSeverity           int    // [None, Medium, High, Fatal]
	failureInfo               string // Additional in-depth information
	examplePossiblyDuplicated bool   // !! Current example name has already been converted for this specific language before. Either the example is duplicated, or a bug is present !!
}

// Failure severity values
const (
	None   = 0
	Medium = 1
	High   = 2
	Fatal  = 3
)

func newCoverageTracker() CoverageTracker {
	return CoverageTracker{"", make(map[string]GeneralExampleInfo)}
}

//========================== Coverage Tracker Interface ===========================
// At different stages, the code translator notifies the tracker of what is going on.
// Notifications are treated as an ordered stream of events: foundExample must be first.

// Used when: generator has found a new example with a convertible block of HCL
func (CT *CoverageTracker) foundExample(exampleName string, hcl string) {
	CT.CurrentExampleName = exampleName
	if val, ok := CT.EncounteredExamples[exampleName]; ok {
		val.nameEncounteredMultipleTimes = true
	}
	//TODO: NYI
}

// Current example has been successfully converted to a certain language
func (CT *CoverageTracker) languageConversionSuccess(targetLanguage string) {
	//TODO: NYI
}

// Generator has failed to convert the current example to a certain language
func (CT *CoverageTracker) languageConversionFailure(conversionFailOpts ConversionFailOpts) {
	//TODO: NYI
}

// Information about failed conversion
type ConversionFailOpts struct {
	targetLanguage  string
	failureSeverity string
	failureInfo     string
}

// Generator ncountered a fatal error when trying to convert the current example to a certain language
func (CT *CoverageTracker) languageConversionPanic(targetLanguage string, panicInfo string) {
	//TODO: NYI
}

//=================================================================================

// Adding a language conversion result to the current example. If a conversion result with the same
// target language already exists, keep the lowest severity one and mark the example as possibly duplicated
func (CT *CoverageTracker) insertLanguageConversionResult(conversionResult LanguageConversionResult) {
	//TODO: NYI
}

// Exporting the coverage results
func (CT *CoverageTracker) exportResults(filePath string) {
	//TODO: NYI
}
