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
//
// How to run:
//
//	cd ~/code/pulumi-aws
//	go run ~/code/pulumi-terraform-bridge/unstable/scripts/exconverter/main.go -dir "$PWD/workspace"
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	dir := flag.String("dir", "", "folder to use for stats; if none given, use a temp folder")
	flag.Parse()

	tmpdir, err := os.MkdirTemp("", "example-coverage-metrics")
	noerr(err)
	if *dir != "" {
		tmpdir = *dir
	} else {
		defer os.RemoveAll(tmpdir)
	}

	baselinedir := filepath.Join(tmpdir, "baseline")
	experimentaldir := filepath.Join(tmpdir, "experimental")

	if !exists(baselinedir) || !exists(experimentaldir) {
		os.Setenv("GOWORK", "off")

		os.Setenv("COVERAGE_OUTPUT_DIR", baselinedir)
		tfgen(false)

		os.Setenv("COVERAGE_OUTPUT_DIR", experimentaldir)
		tfgen(true)
	}

	baselinestats := readstats(baselinedir)
	fmt.Println("Baseline     ", len(baselinestats.exampleIDs),
		"API locations with examples")
	fmt.Println(baselinestats.shortSummary)

	experimentalstats := readstats(experimentaldir)
	fmt.Println("Experimental ", len(experimentalstats.exampleIDs),
		"API locations with examples")
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

	giveExamplesForFrequentErrors(experimentaldir, experimentalstats)

	fmt.Println("DONE")
}

func exists(dir string) bool {
	_, err := os.Stat(dir)
	return err == nil
}

type exampleID string

func newExampleID(rawExampleID string) exampleID {
	if i := strings.LastIndex(rawExampleID, "#"); i != -1 && i != 0 {
		return exampleID(rawExampleID[0:i])
	}
	return exampleID(rawExampleID)
}

type stats struct {
	shortSummary string
	exampleByHCL map[string]flattenedExample
	exampleIDs   map[exampleID]struct{}
}

func (s stats) dropped(new stats) []flattenedExample {
	out := []flattenedExample{}
	for k, v := range s.exampleByHCL {
		e := newExampleID(k)
		if _, ok := new.exampleIDs[e]; !ok {
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
		exampleIDs:   map[exampleID]struct{}{},
	}

	dec := json.NewDecoder(f)
	for {
		var entry flattenedExample
		err := dec.Decode(&entry)
		if err != nil || entry.ExampleName == "" {
			break
		}

		s.exampleByHCL[entry.ExampleName] = entry
		s.exampleIDs[newExampleID(entry.ExampleName)] = struct{}{}
	}

	ss, err := os.ReadFile(filepath.Join(dir, "shortSummary.txt"))
	noerr(err)
	s.shortSummary = string(ss)
	return s
}

func tfgen(convert bool) {
	var cmd *exec.Cmd
	if !convert {
		cmd = exec.Command("make", "tfgen", "PULUMI_CONVERT=0")
	} else {
		cmd = exec.Command("make", "tfgen", "PULUMI_CONVERT=1")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	noerr(err)
}

func noerr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func giveExamplesForFrequentErrors(dir string, st stats) {
	f, err := os.Open(filepath.Join(dir, "summary.json"))
	noerr(err)
	defer func() {
		noerr(f.Close())
	}()
	type ce struct {
		Reason string
		Count  int
	}
	type summary struct {
		ConversionErrors []ce
	}
	var s summary
	dec := json.NewDecoder(f)
	noerr(dec.Decode(&s))

	for i, ce := range s.ConversionErrors {
		fmt.Printf("\n# Error %d\n\n", i+1)
		fmt.Printf("%d examples failed with the following error:\n\n```\n%s\n```\n\n", ce.Count, ce.Reason)
		if ex := findBad(st, ce.Reason); ex != nil {
			var languages []string
			for lang, cr := range ex.ConversionResults {
				if cr.FailureInfo == ce.Reason {
					languages = append(languages, lang)
				}
			}
			fmt.Printf("Failures include converting the %q example with the following HCL to %s:\n",
				ex.ExampleName, strings.Join(languages, ", "))
			fmt.Printf("\n```\n%s\n```\n\n", ex.OriginalHCL)
		}
	}
}

func findBad(st stats, failure string) *flattenedExample {
	for _, e := range st.exampleByHCL {
		for _, cr := range e.ConversionResults {
			if failure == cr.FailureInfo {
				return &e
			}
		}
	}
	return nil
}
