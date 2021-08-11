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

// This file implements the methods used by the Coverage Tracker in order
// to export the data it collected into various JSON formats.

package tfgen

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The export utility's main structure, where it stores the desired output directory
// and a reference to the CoverageTracker that created it
type CoverageExportUtil struct {
	CT *CoverageTracker // Reference to the Coverage Tracker that wants to turn its data into a file
}

func newCoverageExportUtil(coverageTracker *CoverageTracker) CoverageExportUtil {
	return CoverageExportUtil{coverageTracker}
}

// The entire export utility interface. Will attempt to export the Coverage Tracker's data into the
// specified output directory, and will panic if an error is encountered along the way
func (CE *CoverageExportUtil) tryExport(outputDirectory string) {
	CE.exportUploadableResults(outputDirectory, "summary.json")
	CE.exportSummarizedResults(outputDirectory, "concise.json")
}

// Three different ways to export coverage data:
// The first mode, using a large provider > example map
func (CE *CoverageExportUtil) exportFullResults(outputDirectory string, fileName string) {

	// The Coverage Tracker data structure remains identical, the only thing added in the file is the name of the provider
	ProviderNameToExamplesMap := map[string]map[string]*GeneralExampleInfo{CE.CT.ProviderName: CE.CT.EncounteredExamples}

	jsonOutputLocation := createJsonOutputLocation(outputDirectory, fileName)
	marshalAndWriteJson(ProviderNameToExamplesMap, jsonOutputLocation)
}

// The second mode, similar to existing Pulumi coverage Json files uploadable to redshift
func (CE *CoverageExportUtil) exportUploadableResults(outputDirectory string, fileName string) {

	// The Coverage Tracker data structure is flattened down to the example level, and they all
	// get individually written to the file in order to not have the "{ }" brackets at the start and end
	type SingleExampleResult struct {
		ProviderName    string
		ProviderVersion string
		ExampleName     string
		_originalHCL    string
		IsDuplicated    bool
		FailedLanguages []LanguageConversionResult
	}

	jsonOutputLocation := createJsonOutputLocation(outputDirectory, fileName)

	// All the examples in the map are iterated by key and marshalled into one large byte array
	// separated by \n, making the end result look like a bunch of Json files that got concatenated
	var result = []byte{}
	for _, exampleInMap := range CE.CT.EncounteredExamples {
		singleExample := SingleExampleResult{
			ProviderName:    CE.CT.ProviderName,
			ProviderVersion: CE.CT.ProviderVersion,
			ExampleName:     exampleInMap.Name,
			_originalHCL:    "",
			FailedLanguages: []LanguageConversionResult{},
		}

		for _, conversionResult := range exampleInMap.LanguagesConvertedTo {
			if conversionResult.FailureSeverity > 0 {
				singleExample._originalHCL = exampleInMap.OriginalHCL
				singleExample.FailedLanguages = append(singleExample.FailedLanguages, *conversionResult)
			}
			singleExample.IsDuplicated = singleExample.IsDuplicated || conversionResult.MultipleTranslations
		}
		marshalledExample, err := json.MarshalIndent(singleExample, "", "\t")
		panicIfError(err, "Failed to MarshalIndent JSON file")
		result = append(append(result, marshalledExample...), uint8('\n'))
	}
	err2 := ioutil.WriteFile(jsonOutputLocation, result, 0600)
	panicIfError(err2, "Failed to write JSON file")
}

// The third mode, meant for exporting broad information such as total number of examples,
// and what percentage of the total each failure severity makes up
func (CE *CoverageExportUtil) exportSummarizedResults(outputDirectory string, fileName string) {

	// The Coverage Tracker data structure is used to gather general statistics about the examples
	type NumPct struct {
		Number int
		Pct    float32
	}

	type ErrorMessage struct {
		Reason string
		Count  int
	}

	type LanguageStatistic struct {
		NoErrors        NumPct
		LowSevErrors    NumPct
		HighSevErrors   NumPct
		Fatal           NumPct
		Total           int
		_errorHistogram map[string]int
		FrequentErrors  []ErrorMessage
	}

	// Main map for holding all the language conversion statistics
	var allLanguageStatistics = make(map[string]*LanguageStatistic)

	// All the conversion attempts for each example are iterated by language name and
	// their results are added to the main map
	for _, exampleInMap := range CE.CT.EncounteredExamples {
		for _, conversionResult := range exampleInMap.LanguagesConvertedTo {
			var language *LanguageStatistic
			if val, ok := allLanguageStatistics[conversionResult.TargetLanguage]; ok {

				// The main map already contains the language entry
				language = val
			} else {

				// The main map doesn't yet contain this language, and it needs to be added
				allLanguageStatistics[conversionResult.TargetLanguage] = &LanguageStatistic{NumPct{0, 0.0}, NumPct{0, 0.0}, NumPct{0, 0.0}, NumPct{0, 0.0}, 0, make(map[string]int), []ErrorMessage{}}
				language = allLanguageStatistics[conversionResult.TargetLanguage]
			}

			// The language's entry in the summarized results is updated and any
			// error messages are saved
			language.Total += 1
			if conversionResult.FailureSeverity == 0 {
				language.NoErrors.Number += 1
			} else {

				// A failure occured during conversion so we take the failure info, trim
				// it, and add it to the histogram
				abbreviatedReason := strings.Split(conversionResult.FailureInfo, "\n")[0]
				language._errorHistogram[abbreviatedReason] += 1

				if conversionResult.FailureSeverity == 1 {
					language.LowSevErrors.Number += 1
				} else if conversionResult.FailureSeverity == 2 {
					language.HighSevErrors.Number += 1
				} else {
					language.Fatal.Number += 1
				}
			}
		}
	}

	for _, language := range allLanguageStatistics {

		// Calculating error percentages for all languages that were found
		language.NoErrors.Pct = float32(language.NoErrors.Number) / float32(language.Total) * 100.0
		language.LowSevErrors.Pct = float32(language.LowSevErrors.Number) / float32(language.Total) * 100.0
		language.HighSevErrors.Pct = float32(language.HighSevErrors.Number) / float32(language.Total) * 100.0
		language.Fatal.Pct = float32(language.Fatal.Number) / float32(language.Total) * 100.0

		// Appending and sorting conversion errors by their frequency
		for reason, count := range language._errorHistogram {
			language.FrequentErrors = append(language.FrequentErrors, ErrorMessage{reason, count})
		}
		sort.Slice(language.FrequentErrors, func(index1, index2 int) bool {
			return language.FrequentErrors[index1].Count > language.FrequentErrors[index2].Count
		})
	}

	jsonOutputLocation := createJsonOutputLocation(outputDirectory, fileName)
	marshalAndWriteJson(allLanguageStatistics, jsonOutputLocation)
}

// Minor helper functions to assist with exporting results
func createJsonOutputLocation(outputDirectory string, fileName string) string {
	jsonOutputLocation := filepath.Join(outputDirectory, fileName)
	err := os.MkdirAll(outputDirectory, 0700)
	panicIfError(err, "Failed to create output directory for JSON file")
	return jsonOutputLocation
}

func marshalAndWriteJson(unmarshalledData interface{}, finalDestination string) {
	jsonBytes, err := json.MarshalIndent(unmarshalledData, "", "\t")
	panicIfError(err, "Failed to MarshalIndent JSON file")
	err2 := ioutil.WriteFile(finalDestination, jsonBytes, 0600)
	panicIfError(err2, "Failed to write JSON file")

}

func panicIfError(err error, reason string) {
	if err != nil {
		panic(reason)
	}
}
