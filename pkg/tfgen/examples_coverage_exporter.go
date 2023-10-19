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
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// The export utility's main structure, where it stores the desired output directory
// and a reference to the CoverageTracker that created it
type coverageExportUtil struct {
	Tracker *CoverageTracker // Reference to the Coverage Tracker that wants to turn its data into a file
}

func newCoverageExportUtil(coverageTracker *CoverageTracker) coverageExportUtil {
	return coverageExportUtil{coverageTracker}
}

// The entire export utility interface. Will attempt to export the Coverage Tracker's data into the
// specified output directory, and will panic if an error is encountered along the way
func (ce *coverageExportUtil) tryExport(outputDirectory string) error {

	// "summary.json" is the file name that other Pulumi coverage trackers use
	var err = ce.exportByExample(outputDirectory, "byExample.json")
	if err != nil {
		return err
	}
	err = ce.exportByLanguage(outputDirectory, "byLanguage.json")
	if err != nil {
		return err
	}

	// `summary.json` & `shortSummary.txt` are magic filenames used by pulumi/ci-mgmt/provider-ci.
	// If it finds these files, `summary.json` gets uploaded to S3 for cloudwatch analysis, and
	// `shortSummary.txt` is read by the terminal to be visible in Github Actions for inspection
	err = ce.exportOverall(outputDirectory, "summary.json")
	if err != nil {
		return err
	}
	err = ce.exportMarkdown(outputDirectory, "summary.md")
	if err != nil {
		return err
	}
	return ce.exportHumanReadable(outputDirectory, "shortSummary.txt")
}

// Four different ways to export coverage data:
// The first mode, which lists each example individually in one big file. This is the most detailed.
func (ce *coverageExportUtil) exportByExample(outputDirectory string, fileName string) error {

	// The Coverage Tracker data structure is flattened down to the example level, and they all
	// get individually written to the file in order to not have the "{ }" brackets at the start and end
	type FlattenedExample struct {
		ExampleName       string
		OriginalHCL       string `json:"OriginalHCL,omitempty"`
		ConversionResults map[string]*LanguageConversionResult
	}

	jsonOutputLocation, err := createEmptyFile(outputDirectory, fileName)
	if err != nil {
		return err
	}

	// All the examples in the tracker are iterated by page ID + index, and marshalled into one large byte
	// array separated by \n, making the end result look like a bunch of Json files that got concatenated
	var result []byte
	for _, page := range ce.Tracker.EncounteredPages {
		for index, example := range page.Examples {
			flattenedName := page.Name
			if len(page.Examples) > 1 {
				flattenedName += fmt.Sprintf("#%d", index)
			}

			flattenedExample := FlattenedExample{
				ExampleName:       flattenedName,
				OriginalHCL:       example.OriginalHCL,
				ConversionResults: example.ConversionResults,
			}

			marshalledExample, err := json.MarshalIndent(flattenedExample, "", "\t")
			if err != nil {
				return err
			}
			result = append(append(result, marshalledExample...), uint8('\n'))
		}
	}
	return os.WriteFile(jsonOutputLocation, result, 0600)
}

// The second mode, which exports information about each language such as total number of
// examples, common failure messages, and failure severity percentages.
func (ce *coverageExportUtil) exportByLanguage(outputDirectory string, fileName string) error {

	// The Coverage Tracker data structure is flattened to gather statistics about each language
	type NumPct struct {
		Number int
		Pct    float64
	}

	type ErrorMessage struct {
		Reason string
		Count  int
	}

	type LanguageStatistic struct {
		Total           int
		Successes       NumPct
		Warnings        NumPct
		Failures        NumPct
		Fatals          NumPct
		_errorHistogram map[string]int
		FrequentErrors  []ErrorMessage
	}

	// Main map for holding all the language conversion statistics
	var allLanguageStatistics = make(map[string]*LanguageStatistic)

	// All the conversion attempts for each example are iterated by language name and
	// their results are added to the main map
	for _, page := range ce.Tracker.EncounteredPages {
		for _, example := range page.Examples {
			for languageName, conversionResult := range example.ConversionResults {

				// Obtaining the current language we will be creating statistics for
				var currentLanguage *LanguageStatistic
				if existingLanguage, ok := allLanguageStatistics[languageName]; ok {

					// Current language already exists in main map
					currentLanguage = existingLanguage
				} else {

					// The main map doesn't yet contain this language, and it needs to be added
					allLanguageStatistics[languageName] = &LanguageStatistic{0,
						NumPct{0, 0.0}, NumPct{0, 0.0},
						NumPct{0, 0.0}, NumPct{0, 0.0},
						make(map[string]int), []ErrorMessage{}}
					currentLanguage = allLanguageStatistics[languageName]
				}

				// The language's entry in the main map is updated and any error messages are saved
				currentLanguage.Total++
				if conversionResult.FailureSeverity == Success {
					currentLanguage.Successes.Number++
				} else {

					// A failure occurred during conversion so we take the failure info
					// and add it to the histogram
					currentLanguage._errorHistogram[conversionResult.FailureInfo]++

					switch conversionResult.FailureSeverity {
					case Warning:
						currentLanguage.Warnings.Number++
					case Failure:
						currentLanguage.Failures.Number++
					default:
						currentLanguage.Fatals.Number++
					}
				}
			}
		}
	}

	for _, language := range allLanguageStatistics {

		// Calculating error percentages for all languages that were found
		if language.Total > 0 {
			language.Successes.Pct = float64(language.Successes.Number) / float64(language.Total) * 100.0
			language.Warnings.Pct = float64(language.Warnings.Number) / float64(language.Total) * 100.0
			language.Failures.Pct = float64(language.Failures.Number) / float64(language.Total) * 100.0
			language.Fatals.Pct = float64(language.Fatals.Number) / float64(language.Total) * 100.0
		}

		// Appending and sorting conversion errors by their frequency
		for reason, count := range language._errorHistogram {
			language.FrequentErrors = append(language.FrequentErrors, ErrorMessage{reason, count})
		}
		sort.Slice(language.FrequentErrors, func(index1, index2 int) bool {
			if language.FrequentErrors[index1].Count != language.FrequentErrors[index2].Count {
				return language.FrequentErrors[index1].Count > language.FrequentErrors[index2].Count
			}

			return language.FrequentErrors[index1].Reason > language.FrequentErrors[index2].Reason
		})
	}

	jsonOutputLocation, err := createEmptyFile(outputDirectory, fileName)
	if err != nil {
		return err
	}
	return marshalAndWriteJSON(allLanguageStatistics, jsonOutputLocation)
}

// The third mode, which lists failure reaons, quantities and percentages for the provider as a whole.
func (ce *coverageExportUtil) exportOverall(outputDirectory string, fileName string) error {

	// The Coverage Tracker data structure is flattened to gather statistics about the provider
	type NumPct struct {
		Number int
		Pct    float64
	}

	type ErrorMessage struct {
		Reason string
		Count  int
	}

	type ProviderStatistic struct {
		Name             string
		Version          string
		Examples         int
		TotalConversions int
		Successes        NumPct
		Warnings         NumPct
		Failures         NumPct
		Fatals           NumPct
		_errorHistogram  map[string]int
		ConversionErrors []ErrorMessage
	}

	// Main variable for holding the overall provider conversion results
	var providerStatistic = ProviderStatistic{ce.Tracker.ProviderName,
		ce.Tracker.ProviderVersion, 0, 0, NumPct{0, 0.0},
		NumPct{0, 0.0}, NumPct{0, 0.0},
		NumPct{0, 0.0}, make(map[string]int), []ErrorMessage{}}

	// All the conversion attempts for each example are iterated by language name and
	// their results are added to the overall statistic
	for _, page := range ce.Tracker.EncounteredPages {
		for _, example := range page.Examples {
			providerStatistic.Examples++
			for _, conversionResult := range example.ConversionResults {
				providerStatistic.TotalConversions++
				if conversionResult.FailureSeverity == Success {
					providerStatistic.Successes.Number++
				} else {

					// A failure occurred during conversion so we take the failure info
					// and add it to the histogram
					providerStatistic._errorHistogram[conversionResult.FailureInfo]++

					switch conversionResult.FailureSeverity {
					case Warning:
						providerStatistic.Warnings.Number++
					case Failure:
						providerStatistic.Failures.Number++
					default:
						providerStatistic.Fatals.Number++
					}
				}
			}
		}
	}

	// Calculating overall error percentages
	if providerStatistic.TotalConversions > 0 {
		providerStatistic.Successes.Pct = float64(providerStatistic.Successes.Number) /
			float64(providerStatistic.TotalConversions) * 100.0
		providerStatistic.Warnings.Pct = float64(providerStatistic.Warnings.Number) /
			float64(providerStatistic.TotalConversions) * 100.0
		providerStatistic.Failures.Pct = float64(providerStatistic.Failures.Number) /
			float64(providerStatistic.TotalConversions) * 100.0
		providerStatistic.Fatals.Pct = float64(providerStatistic.Fatals.Number) /
			float64(providerStatistic.TotalConversions) * 100.0
	}

	// Appending and sorting conversion errors by their frequency
	for reason, count := range providerStatistic._errorHistogram {
		providerStatistic.ConversionErrors = append(providerStatistic.ConversionErrors,
			ErrorMessage{reason, count})
	}
	sort.Slice(providerStatistic.ConversionErrors, func(index1, index2 int) bool {
		if providerStatistic.ConversionErrors[index1].Count != providerStatistic.ConversionErrors[index2].Count {
			return providerStatistic.ConversionErrors[index1].Count > providerStatistic.ConversionErrors[index2].Count
		}

		return providerStatistic.ConversionErrors[index1].Reason > providerStatistic.ConversionErrors[index2].Reason
	})

	jsonOutputLocation, err := createEmptyFile(outputDirectory, fileName)
	if err != nil {
		return err
	}
	return marshalAndWriteJSON(providerStatistic, jsonOutputLocation)
}

// The fifth mode, which provides outputs a markdown file with:
// - the example's name
// - the original HCL
// - the conversion results for all languages
func (ce *coverageExportUtil) exportMarkdown(outputDirectory string, fileName string) error {

	// The Coverage Tracker data structure is flattened down to the example level, and they all
	// get individually written to the file in order to not have the "{ }" brackets at the start and end
	type FlattenedExample struct {
		ExampleName       string
		OriginalHCL       string `json:"OriginalHCL,omitempty"`
		ConversionResults map[string]*LanguageConversionResult
	}

	// All the examples in the tracker are iterated by page ID + index, and marshalled into one large byte
	// array separated by \n, making the end result look like a bunch of Json files that got concatenated
	var brokenExamples []FlattenedExample

	for _, page := range ce.Tracker.EncounteredPages {
		for index, example := range page.Examples {
			flattenedName := page.Name
			if len(page.Examples) > 1 {
				flattenedName += fmt.Sprintf("#%d", index)
			}

			noErrors := true
			for _, result := range example.ConversionResults {
				if result.FailureSeverity == Success {
					continue
				}
				noErrors = false
			}
			if noErrors {
				break
			}

			brokenExamples = append(brokenExamples, FlattenedExample{
				ExampleName:       flattenedName,
				OriginalHCL:       example.OriginalHCL,
				ConversionResults: example.ConversionResults,
			})
		}
	}
	targetFile, err := createEmptyFile(outputDirectory, fileName)
	if err != nil {
		return err
	}

	// Make coverage output stable. For examples.
	sort.Slice(brokenExamples, func(i, j int) bool {
		return brokenExamples[i].ExampleName < brokenExamples[j].ExampleName
	})

	out := ""
	for _, example := range brokenExamples {

		type exampleResult struct {
			lang   string
			result LanguageConversionResult
		}

		successes := []exampleResult{}
		failures := []exampleResult{}

		for lang, result := range example.ConversionResults {
			if result.FailureSeverity == Success {
				successes = append(successes, exampleResult{
					lang:   lang,
					result: *result,
				})
				continue
			}
			failures = append(failures, exampleResult{
				lang:   lang,
				result: *result,
			})
		}

		// Make coverage output stable. For language results.
		sort.Slice(successes, func(i, j int) bool {
			return successes[i].lang < successes[j].lang
		})
		sort.Slice(failures, func(i, j int) bool {
			return failures[i].lang < failures[j].lang
		})

		isCompleteFailure := len(successes) == 0

		// print example header
		summaryText := "*partial failure*"
		if isCompleteFailure {
			summaryText = "**complete failure**"
		}

		out += "\n---\n"
		out += fmt.Sprintf("\n## [%s] %s\n", summaryText, example.ExampleName)

		// print original HCL
		out += "\n### HCL\n"
		out += "\n```terraform\n"
		out += example.OriginalHCL + "\n"
		out += "\n```\n"

		// print failures
		out += "\n### Failed Languages\n"
		for _, fail := range failures {
			out += fmt.Sprintf("\n#### %s\n", fail.lang)
			out += "\n```text\n"

			errMsg := fail.result.FailureInfo
			if len(fail.result.FailureInfo) > 1000 {
				// truncate extremely long error messages
				errMsg = fail.result.FailureInfo[:1000]
			}
			out += errMsg
			out += "\n```\n"
		}

		if isCompleteFailure {
			// it's a complete failure, no successes to print
			continue
		}

		// print successes
		out += "\n### Successes\n"
		for _, success := range successes {
			out += "\n<details>\n"
			out += fmt.Sprintf("\n<summary>%s</summary>\n", success.lang)
			out += fmt.Sprintf("\n```%s\n", success.lang)
			out += success.result.Program
			out += "\n```\n"
			out += "\n</details>\n"
		}
	}

	return os.WriteFile(targetFile, []byte(out), 0600)
}

// The Coverage Tracker data structure is flattened to gather statistics about each language
type LanguageStatistic struct {
	Total     int
	Successes int
}

type ProviderStatistic struct {
	Name             string
	Examples         int
	TotalConversions int
	Successes        int
}

func (ce coverageExportUtil) produceStatistics() (map[string]*LanguageStatistic, ProviderStatistic) {
	// Main maps for holding the overall provider summary, and each language conversion statistic
	var allLanguageStatistics = make(map[string]*LanguageStatistic)
	var providerStatistic = ProviderStatistic{ce.Tracker.ProviderName, 0, 0, 0}

	// All the conversion attempts for each example are iterated by language name and
	// their results are added to the main map
	for _, page := range ce.Tracker.EncounteredPages {
		for _, example := range page.Examples {
			providerStatistic.Examples++
			for languageName, conversionResult := range example.ConversionResults {
				providerStatistic.TotalConversions++

				// Obtaining the current language we will be creating statistics for
				var currentLanguage *LanguageStatistic
				if val, ok := allLanguageStatistics[languageName]; ok {

					// Current language already exists in main map
					currentLanguage = val
				} else {

					// The main map doesn't yet contain this language, and it needs to be added
					allLanguageStatistics[languageName] = &LanguageStatistic{0, 0}
					currentLanguage = allLanguageStatistics[languageName]
				}

				// The language's entry in the summarized results is updated and any
				currentLanguage.Total++
				if conversionResult.FailureSeverity == Success {
					providerStatistic.Successes++
					currentLanguage.Successes++
				}
			}
		}
	}

	return allLanguageStatistics, providerStatistic
}

func (ce *coverageExportUtil) produceHumanReadableSummary() string {
	allLanguageStatistics, providerStatistic := ce.produceStatistics()

	// Forming a string which will eventually be written to the target file
	fileString := fmt.Sprintf("Provider:     %s\nSuccess rate: %.2f%% (%d/%d)\n\n",
		providerStatistic.Name,
		float64(providerStatistic.Successes)/float64(providerStatistic.TotalConversions)*100.0,
		providerStatistic.Successes,
		providerStatistic.TotalConversions,
	)

	// Adding language results to the string in alphabetical order
	keys := make([]string, 0, len(allLanguageStatistics))
	for languageName := range allLanguageStatistics {
		keys = append(keys, languageName)
	}
	sort.Strings(keys)

	for _, languageName := range keys {
		languageStatistic := allLanguageStatistics[languageName]

		fileString += fmt.Sprintf("Converted %.2f%% of %s examples (%d/%d)\n",
			float64(languageStatistic.Successes)/float64(languageStatistic.Total)*100.0,
			languageName,
			languageStatistic.Successes,
			languageStatistic.Total,
		)
	}

	return fileString
}

// The fourth mode, which simply gives the provider name, and success percentage.
func (ce *coverageExportUtil) exportHumanReadable(outputDirectory string, fileName string) error {
	targetFile, err := createEmptyFile(outputDirectory, fileName)
	if err != nil {
		return err
	}
	fileString := ce.produceHumanReadableSummary()

	return os.WriteFile(targetFile, []byte(fileString), 0600)
}

// Minor helper functions to assist with exporting results
func createEmptyFile(outputDirectory string, fileName string) (string, error) {
	outputLocation := filepath.Join(outputDirectory, fileName)
	err := os.MkdirAll(outputDirectory, 0700)
	return outputLocation, err
}

func marshalAndWriteJSON(unmarshalledData interface{}, finalDestination string) error {
	jsonBytes, err := json.MarshalIndent(unmarshalledData, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(finalDestination, jsonBytes, 0600)
}
