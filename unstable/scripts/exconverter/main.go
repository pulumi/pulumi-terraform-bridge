// Copyright 2016-2023, Pulumi Corporation.
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

// This script assists the rollout of the new example converter across bridged providers by
// computing the difference in example generation metrics. It will run `make tfgen` twice, capture
// example metrics, print them, and print a detailed comparison on degraded examples.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	keep := flag.Bool("keep", false, "set to true to keep the temp file")
	flag.Parse()

	tmpdir, err := os.MkdirTemp("", "example-coverage-metrics")
	noerr(err)
	if !*keep {
		defer os.RemoveAll(tmpdir)
	}

	baselinedir := filepath.Join(tmpdir, "baseline")
	experimentaldir := filepath.Join(tmpdir, "experimental")

	os.Setenv("COVERAGE_OUTPUT_DIR", experimentaldir)
	os.Setenv("PULUMI_CONVERT", "true")
	tfgen()

	os.Setenv("COVERAGE_OUTPUT_DIR", baselinedir)
	os.Setenv("PULUMI_CONVERT", "")
	os.Setenv("GOWORK", "off")
	tfgen()

	baselinestats := readstats(baselinedir)
	fmt.Println("Baseline    ", len(baselinestats.exampleByHCL))
	fmt.Println(baselinestats.shortSummary)

	experimentalstats := readstats(experimentaldir)
	fmt.Println("Experimental", len(experimentalstats.exampleByHCL))
	fmt.Println(experimentalstats.shortSummary)

	for _, e := range baselinestats.newlyFailing(experimentalstats) {
		fmt.Printf("Example started failing: %v\n", e.ExampleName)
		for lang, cr := range e.ConversionResults {
			fmt.Println(lang, cr.FailureSeverity, cr.FailureInfo)
		}
	}

	for _, e := range baselinestats.dropped(experimentalstats) {
		fmt.Printf("Example dropped: %v\n", e.ExampleName)
	}

	fmt.Println("DONE")
}

type stats struct {
	shortSummary string
	exampleByHCL map[string]flattenedExample
}

func (s stats) dropped(new stats) []flattenedExample {
	out := []flattenedExample{}
	for k, v := range s.exampleByHCL {
		if _, ok := new.exampleByHCL[k]; !ok {
			out = append(out, v)
		}
	}
	return out
}

func (s stats) newlyFailing(new stats) []flattenedExample {
	out := []flattenedExample{}
	for k, v := range s.exampleByHCL {
		if vn, ok := new.exampleByHCL[k]; ok {
			for lang, newCR := range vn.ConversionResults {
				if oldCR, ok := v.ConversionResults[lang]; ok {
					if oldCR.FailureSeverity < newCR.FailureSeverity &&
						newCR.FailureSeverity > warning {
						out = append(out, vn)
					}
				}
			}
		}
	}
	return out
}

type flattenedExample struct {
	ExampleName       string
	OriginalHCL       string `json:"OriginalHCL,omitempty"`
	ConversionResults map[string]*languageConversionResult
}

const (
	success = 0
	warning = 1
	failure = 2
	fatal   = 3
)

type languageConversionResult struct {
	FailureSeverity  int    // This conversion's outcome: [Success, Warning, Failure, Fatal]
	FailureInfo      string // Additional in-depth information
	Program          string // Converted program
	TranslationCount int
}

func readstats(dir string) stats {
	f, err := os.Open(filepath.Join(dir, "byExample.json"))
	noerr(err)
	defer func() {
		noerr(f.Close())
	}()

	s := stats{
		exampleByHCL: map[string]flattenedExample{},
	}

	dec := json.NewDecoder(f)
	for {
		var entry flattenedExample
		err := dec.Decode(&entry)
		if err != nil || entry.ExampleName == "" {
			break
		}

		s.exampleByHCL[entry.ExampleName] = entry
	}

	ss, err := os.ReadFile(filepath.Join(dir, "shortSummary.txt"))
	noerr(err)
	s.shortSummary = string(ss)
	return s
}

func tfgen() {
	cmd := exec.Command("make", "tfgen")
	err := cmd.Run()
	noerr(err)
}

func noerr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
